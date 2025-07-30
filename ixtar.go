package ixtar

import (
	"archive/tar"
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

type TarIndex struct {
	Files map[string]FileIndex `json:"files"`
}

type IxTar struct {
	bundlePath string
	index      TarIndex
	csvSize    int64
	file       *os.File
	tarReader  *tar.Reader
	tarOffset  int64
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

	tarOffset := 32 + csvSize
	if _, err := file.Seek(tarOffset, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek to TAR start: %w", err)
	}

	tarReader := tar.NewReader(file)

	return &IxTar{
		bundlePath: bundlePath,
		index:      index,
		csvSize:    csvSize,
		file:       file,
		tarReader:  tarReader,
		tarOffset:  tarOffset,
	}, nil
}

func parseCSVIndex(csvData []byte) (TarIndex, error) {
	reader := csv.NewReader(bytes.NewReader(csvData))
	records, err := reader.ReadAll()
	if err != nil {
		return TarIndex{}, fmt.Errorf("failed to parse CSV: %w", err)
	}

	index := TarIndex{Files: make(map[string]FileIndex)}
	for _, record := range records {
		if len(record) != 3 {
			return TarIndex{}, fmt.Errorf("invalid CSV record: expected 3 fields, got %d", len(record))
		}

		hash := record[0]
		start, err := strconv.ParseInt(record[1], 10, 64)
		if err != nil {
			return TarIndex{}, fmt.Errorf("invalid start position: %w", err)
		}

		size, err := strconv.ParseInt(record[2], 10, 64)
		if err != nil {
			return TarIndex{}, fmt.Errorf("invalid file size: %w", err)
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
	cleanPath := filepath.Clean(filePath)
	hash := hashFilePath(cleanPath)

	if _, exists := ix.index.Files[hash]; !exists {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	if _, err := ix.file.Seek(ix.tarOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to TAR start: %w", err)
	}

	ix.tarReader = tar.NewReader(ix.file)

	for {
		header, err := ix.tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read TAR header: %w", err)
		}

		headerCleanPath := filepath.Clean(header.Name)
		if headerCleanPath == cleanPath {
			if header.Typeflag != tar.TypeReg {
				return nil, fmt.Errorf("file is not a regular file: %s", filePath)
			}

			data := make([]byte, header.Size)
			if _, err := io.ReadFull(ix.tarReader, data); err != nil {
				return nil, fmt.Errorf("failed to read file data: %w", err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("file not found in TAR: %s", filePath)
}

func (ix *IxTar) ListFiles() []string {
	var files []string
	for hash := range ix.index.Files {
		files = append(files, hash)
	}
	return files
}

func CreateBundle(sourceDir, bundlePath string) error {
	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)
	index := TarIndex{Files: make(map[string]FileIndex)}

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		header.Name = relPath
		cleanPath := filepath.Clean(header.Name)
		hash := hashFilePath(cleanPath)

		if _, exists := index.Files[hash]; exists {
			panic(fmt.Sprintf("hash collision detected for path: %s", cleanPath))
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			start := int64(buf.Len())
			size, err := io.Copy(tarWriter, file)
			if err != nil {
				return err
			}

			index.Files[hash] = FileIndex{Start: start, Size: size}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	csvData, err := generateCSV(index)
	if err != nil {
		return fmt.Errorf("failed to generate CSV: %w", err)
	}

	bundleFile, err := os.Create(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to create bundle file: %w", err)
	}
	defer bundleFile.Close()

	var csvSizeBytes [32]byte
	binary.BigEndian.PutUint64(csvSizeBytes[24:], uint64(len(csvData)))

	if _, err := bundleFile.Write(csvSizeBytes[:]); err != nil {
		return fmt.Errorf("failed to write CSV size: %w", err)
	}

	if _, err := bundleFile.Write(csvData); err != nil {
		return fmt.Errorf("failed to write CSV data: %w", err)
	}

	if _, err := bundleFile.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write TAR data: %w", err)
	}

	return nil
}

func generateCSV(index TarIndex) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	for hash, fileIndex := range index.Files {
		record := []string{
			hash,
			strconv.FormatInt(fileIndex.Start, 10),
			strconv.FormatInt(fileIndex.Size, 10),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}