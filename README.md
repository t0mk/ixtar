# ixtar

A Go library for creating and reading ixtar bundles - efficient file archives that combine CSV indexes with TAR data for fast random access to files.

## What is ixtar?

ixtar is a file archiving format that stores a CSV index alongside TAR data in a single bundle file. This design allows for:

- **Fast file lookups** without scanning the entire archive
- **Efficient random access** to individual files
- **Optimized for network storage** (Google Cloud Storage, S3, etc.) via fuse mounts
- **Simple bundle format**: `[32-byte CSV size][CSV index][TAR data]`

## Installation

```bash
go get github.com/t0mk/ixtar
```

## CLI Usage

### Install the CLI tool

```bash
go install github.com/t0mk/ixtar/cmd/ixtar@latest
```

### Create a bundle from a directory

```bash
ixtar create /path/to/directory output.ixtar
```

### List files in a bundle

```bash
ixtar list bundle.ixtar
```

### Extract a specific file from a bundle

```bash
ixtar extract bundle.ixtar path/to/file.txt
```

### Get bundle information

```bash
ixtar info bundle.ixtar
```

## Library Usage

### Creating bundles

```go
package main

import (
    "log"
    "github.com/t0mk/ixtar"
)

func main() {
    // Create a bundle from a directory
    err := ixtar.CreateBundle("/path/to/source/directory", "output.ixtar")
    if err != nil {
        log.Fatal(err)
    }
}
```

### Reading files from bundles

```go
package main

import (
    "fmt"
    "log"
    "github.com/t0mk/ixtar"
)

func main() {
    // Open an ixtar bundle
    ix, err := ixtar.NewIxTar("bundle.ixtar")
    if err != nil {
        log.Fatal(err)
    }
    defer ix.Close() // Important: always close to free resources
    
    // Extract a specific file
    data, err := ix.ExtractBytesOfFile("path/to/file.txt")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("File content: %s\n", string(data))
    
    // List all files (returns hashes)
    files := ix.ListFiles()
    fmt.Printf("Bundle contains %d files\n", len(files))
    
    // Get bundle information
    fileCount, csvSize := ix.Info()
    fmt.Printf("Files: %d, CSV index size: %d bytes\n", fileCount, csvSize)
}
```

### Multiple file extractions (optimized)

```go
package main

import (
    "fmt"
    "log"
    "github.com/t0mk/ixtar"
)

func main() {
    ix, err := ixtar.NewIxTar("bundle.ixtar")
    if err != nil {
        log.Fatal(err)
    }
    defer ix.Close()
    
    // Multiple extractions reuse the same file handle
    // Perfect for network/fuse-mounted storage
    filesToRead := []string{"file1.txt", "file2.txt", "dir/file3.txt"}
    
    for _, filepath := range filesToRead {
        data, err := ix.ExtractBytesOfFile(filepath)
        if err != nil {
            log.Printf("Error reading %s: %v", filepath, err)
            continue
        }
        fmt.Printf("%s: %s\n", filepath, string(data))
    }
}
```

## Bundle Format

ixtar bundles use a simple, efficient format:

```
[32 bytes: CSV size (big-endian)]
[CSV data: hash,start,size]
[TAR data: standard tar format]
```

- **CSV Index**: Maps MD5 hash (16 chars) to file position and size
- **File lookup**: O(1) hash table lookup in CSV index
- **File paths**: Cleaned with `filepath.Clean()` before hashing
- **Hash collisions**: Panic on collision (extremely rare with MD5 truncated to 16 chars)

## API Reference

### Types

```go
type FileIndex struct {
    Start int64 `json:"start"` // Starting byte position in TAR
    Size  int64 `json:"size"`  // Size of the file in bytes
}

type TarIndex struct {
    Files map[string]FileIndex `json:"files"` // Hash -> FileIndex mapping
}

type IxTar struct {
    // Contains open file handle and TAR reader for efficiency
}
```

### Functions

```go
// Create a new ixtar bundle from a directory
func CreateBundle(sourceDir, bundlePath string) error

// Open an existing ixtar bundle
func NewIxTar(bundlePath string) (*IxTar, error)

// Extract file content by path
func (ix *IxTar) ExtractBytesOfFile(filePath string) ([]byte, error)

// List all file hashes in the bundle
func (ix *IxTar) ListFiles() []string

// Get bundle information (file count and CSV index size)
func (ix *IxTar) Info() (fileCount int, csvSizeBytes int64)

// Close the bundle and free resources
func (ix *IxTar) Close() error
```

## Performance Characteristics

- **Bundle creation**: O(n) where n is total file size
- **File lookup**: O(1) hash table lookup + O(1) file seek
- **Memory usage**: Minimal - only CSV index loaded into memory
- **Network optimization**: Single file handle reduces connection overhead
- **Fuse-friendly**: Optimized for network-mounted filesystems

## Use Cases

- **Microservices**: Fast access to configuration files and assets
- **Cloud storage**: Efficient file serving from GCS/S3 via fuse mounts  
- **Content delivery**: Quick extraction of specific files from large archives
- **Data processing**: Random access to files in large datasets
- **Container images**: Alternative to tar for faster file access

## License

MIT

## Contributing

Pull requests welcome! Please ensure tests pass:

```bash
go test -v
```