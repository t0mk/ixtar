package main

import (
	"fmt"
	"log"
	"os"

	"github.com/t0mk/ixtar"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "create":
		if len(os.Args) != 4 {
			fmt.Fprintf(os.Stderr, "Usage: ixtar create <directory> <output.ixtar>\n")
			os.Exit(1)
		}
		sourceDir := os.Args[2]
		outputPath := os.Args[3]
		
		err := ixtar.CreateBundleWithProgress(sourceDir, outputPath, func(current, total int, filename string) {
			percent := float64(current) / float64(total) * 100
			fmt.Printf("\r[%3.0f%%]", percent)
		})
		
		if err != nil {
			fmt.Println()
			log.Fatalf("Failed to create bundle: %v", err)
		}
		
		fmt.Printf("\nBundle created: %s\n", outputPath)

	case "list":
		if len(os.Args) != 3 {
			fmt.Fprintf(os.Stderr, "Usage: ixtar list <bundle.ixtar>\n")
			os.Exit(1)
		}
		bundlePath := os.Args[2]
		
		ix, err := ixtar.NewIxTar(bundlePath)
		if err != nil {
			log.Fatalf("Failed to open bundle: %v", err)
		}
		defer ix.Close()
		
		files := ix.ListFiles()
		fmt.Printf("Files in bundle (%d total):\n", len(files))
		for _, hash := range files {
			fmt.Printf("  %s\n", hash)
		}

	case "extract":
		if len(os.Args) != 4 {
			fmt.Fprintf(os.Stderr, "Usage: ixtar extract <bundle.ixtar> <file-path>\n")
			os.Exit(1)
		}
		bundlePath := os.Args[2]
		filePath := os.Args[3]
		
		ix, err := ixtar.NewIxTar(bundlePath)
		if err != nil {
			log.Fatalf("Failed to open bundle: %v", err)
		}
		defer ix.Close()
		
		data, err := ix.ExtractBytesOfFile(filePath)
		if err != nil {
			log.Fatalf("Failed to extract file: %v", err)
		}
		
		os.Stdout.Write(data)

	case "info":
		if len(os.Args) != 3 {
			fmt.Fprintf(os.Stderr, "Usage: ixtar info <bundle.ixtar>\n")
			os.Exit(1)
		}
		bundlePath := os.Args[2]
		
		ix, err := ixtar.NewIxTar(bundlePath)
		if err != nil {
			log.Fatalf("Failed to open bundle: %v", err)
		}
		defer ix.Close()
		
		fileCount, csvSize := ix.Info()
		fmt.Printf("Bundle: %s\n", bundlePath)
		fmt.Printf("Files: %d\n", fileCount)
		fmt.Printf("CSV index size: %d bytes\n", csvSize)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  ixtar create <directory> <output.ixtar>\n")
	fmt.Fprintf(os.Stderr, "  ixtar list <bundle.ixtar>\n")
	fmt.Fprintf(os.Stderr, "  ixtar extract <bundle.ixtar> <file-path>\n")
	fmt.Fprintf(os.Stderr, "  ixtar info <bundle.ixtar>\n")
}