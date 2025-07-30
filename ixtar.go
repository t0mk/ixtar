package ixtar

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const HashLen = 16

type FileIndex struct {
	Start int64 `json:"start"`
	Size  int64 `json:"size"`
}

type DataIndex struct {
	Files map[string]FileIndex `json:"files"`
}

type IxTar struct {
	bundlePath string
	index      DataIndex
	csvSize    int64
	file       *os.File
	dataOffset int64
}

func hashFilePath(filePath string) string {
	h := md5.New()
	h.Write([]byte(filePath))
	return hex.EncodeToString(h.Sum(nil))[:HashLen]
}

func NewIxTar(bundlePath string) (*IxTar, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}

	var csvSizeBytes [32]byte
	if _, err := io.ReadFull(file, csvSizeBytes[:]); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read CSV size: %w", err)
	}

	csvSize := int64(binary.BigEndian.Uint64(csvSizeBytes[24:]))

	csvData := make([]byte, csvSize)
	if _, err := io.ReadFull(file, csvData); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read CSV data: %w", err)
	}

	index, err := parseCSVIndex(csvData)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to parse CSV index: %w", err)
	}

	dataOffset := 32 + csvSize

	return &IxTar{
		bundlePath: bundlePath,
		index:      index,
		csvSize:    csvSize,
		file:       file,
		dataOffset: dataOffset,
	}, nil
}

func parseCSVIndex(csvData []byte) (DataIndex, error) {
	reader := csv.NewReader(bytes.NewReader(csvData))
	records, err := reader.ReadAll()
	if err != nil {
		return DataIndex{}, fmt.Errorf("failed to parse CSV: %w", err)
	}

	index := DataIndex{Files: make(map[string]FileIndex)}
	for _, record := range records {
		if len(record) != 3 {
			return DataIndex{}, fmt.Errorf("invalid CSV record: expected 3 fields, got %d", len(record))
		}

		hash := record[0]
		start, err := strconv.ParseInt(record[1], 10, 64)
		if err != nil {
			return DataIndex{}, fmt.Errorf("invalid start position: %w", err)
		}

		size, err := strconv.ParseInt(record[2], 10, 64)
		if err != nil {
			return DataIndex{}, fmt.Errorf("invalid file size: %w", err)
		}

		index.Files[hash] = FileIndex{Start: start, Size: size}
	}

	return index, nil
}

func (ix *IxTar) Close() error {
	if ix.file != nil {
		return ix.file.Close()
	}
	return nil
}

func (ix *IxTar) ExtractBytesOfFile(filePath string) ([]byte, error) {
	if ix == nil {
		return nil, fmt.Errorf("IxTar instance is nil")
	}
	
	cleanPath := filepath.Clean(filePath)
	hash := hashFilePath(cleanPath)

	fileIndex, exists := ix.index.Files[hash]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	// Seek to the file position within the raw data
	fileOffset := ix.dataOffset + fileIndex.Start
	if _, err := ix.file.Seek(fileOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to file position: %w", err)
	}

	// Read the file data directly
	data := make([]byte, fileIndex.Size)
	if _, err := io.ReadFull(ix.file, data); err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	return data, nil
}

func (ix *IxTar) ListFiles() []string {
	var files []string
	for hash := range ix.index.Files {
		files = append(files, hash)
	}
	return files
}

func (ix *IxTar) Info() (fileCount int, csvSizeBytes int64) {
	return len(ix.index.Files), ix.csvSize
}

type ProgressCallback func(current, total int, filename string)

func CreateBundle(sourceDir, bundlePath string) error {
	return CreateBundleWithProgress(sourceDir, bundlePath, nil)
}

func CreateBundleWithProgress(sourceDir, bundlePath string, progress ProgressCallback) error {
	// Create temporary file for raw file data
	tmpDataFile, err := os.CreateTemp("", "ixtar-data-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp data file: %w", err)
	}
	defer os.Remove(tmpDataFile.Name())
	defer tmpDataFile.Close()

	// Create temporary CSV file
	tmpCsvFile, err := os.CreateTemp("", "ixtar-csv-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp csv file: %w", err)
	}
	defer os.Remove(tmpCsvFile.Name())
	defer tmpCsvFile.Close()

	csvWriter := csv.NewWriter(tmpCsvFile)

	// Count files first if progress callback is provided
	totalFiles := 0
	if progress != nil {
		filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			relPath, err := filepath.Rel(sourceDir, path)
			if err != nil || relPath == "." || info.IsDir() {
				return nil
			}
			totalFiles++
			return nil
		})
	}

	// Phase 1: Create raw data file and build index simultaneously
	currentFile := 0
	currentPos := int64(0) // Track position in raw data file
	csvFileCount := 0

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if relPath == "." || info.IsDir() {
			return nil
		}

		currentFile++
		if currentFile%1000 == 0 {
			if progress != nil {
				progress(currentFile, totalFiles, "")
			}
		}

		if info.Mode().IsRegular() {
			cleanPath := filepath.Clean(relPath)
			hash := hashFilePath(cleanPath)

			// Record position in CSV - this is where file data starts
			record := []string{
				hash,
				strconv.FormatInt(currentPos, 10),
				strconv.FormatInt(info.Size(), 10),
			}
			if err := csvWriter.Write(record); err != nil {
				return fmt.Errorf("failed to write CSV record: %w", err)
			}

			csvFileCount++
			if csvFileCount%1000 == 0 {
				csvWriter.Flush()
				if err := csvWriter.Error(); err != nil {
					return fmt.Errorf("CSV flush error: %w", err)
				}
			}

			// Write file data directly to raw data file
			file, err := os.Open(path)
			if err != nil {
				return err
			}

			buf := make([]byte, 32*1024) // 32KB buffer
			written, err := io.CopyBuffer(tmpDataFile, file, buf)
			file.Close()
			if err != nil {
				return err
			}

			// Update position
			currentPos += written
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV writer: %w", err)
	}

	// Get CSV size
	csvSize, err := tmpCsvFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get CSV size: %w", err)
	}

	// Phase 2: Assemble final bundle
	bundleFile, err := os.Create(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to create bundle file: %w", err)
	}
	defer bundleFile.Close()

	var csvSizeBytes [32]byte
	binary.BigEndian.PutUint64(csvSizeBytes[24:], uint64(csvSize))

	if _, err := bundleFile.Write(csvSizeBytes[:]); err != nil {
		return fmt.Errorf("failed to write CSV size: %w", err)
	}

	// Copy CSV data
	if _, err := tmpCsvFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek CSV temp file: %w", err)
	}

	if _, err := io.Copy(bundleFile, tmpCsvFile); err != nil {
		return fmt.Errorf("failed to copy CSV data: %w", err)
	}

	// Copy raw data
	if _, err := tmpDataFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek data temp file: %w", err)
	}

	if _, err := io.Copy(bundleFile, tmpDataFile); err != nil {
		return fmt.Errorf("failed to copy raw data: %w", err)
	}

	return nil
}

