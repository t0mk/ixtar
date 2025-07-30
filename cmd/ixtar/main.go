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
		
		if err := ixtar.CreateBundle(sourceDir, outputPath); err != nil {
			log.Fatalf("Failed to create bundle: %v", err)
		}
		fmt.Printf("Bundle created: %s\n", outputPath)

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
}