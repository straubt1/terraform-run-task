package helper

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/straubt1/terraform-run-task/internal/sdk/api"
)

const (
	DefaultDirPermissions  = 0755
	DefaultFilePermissions = 0644
)

// FileManager handles file operations for the run task
type FileManager struct{}

// NewFileManager creates a new FileManager instance
func NewFileManager() *FileManager {
	return &FileManager{}
}

// SaveRequestToFile saves the run task request (JSON) to a file
func (fm *FileManager) SaveRequestToFile(outputDirectory string, request api.Request) error {
	filePath := filepath.Join(outputDirectory, "request.json")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(request); err != nil {
		return fmt.Errorf("failed to encode request to JSON: %w", err)
	}
	return nil
}

// ExtractTarGz extracts a tar.gz file to a directory with the specified ID
func (fm *FileManager) ExtractTarGz(archiveFile, targetDir, id string) error {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(targetDir, DefaultDirPermissions); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// Open the tar.gz file
	file, err := os.Open(archiveFile)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", archiveFile, err)
	}
	defer file.Close()

	// Create a gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files from the tar archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Create the full path for the file
		targetPath := filepath.Join(targetDir, header.Name)

		// Ensure the target path is within the target directory (security check)
		if !fm.isValidPath(targetPath, targetDir) {
			return fmt.Errorf("invalid file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := fm.extractFile(tarReader, targetPath, header); err != nil {
				return err
			}
		}
	}

	return nil
}

// isValidPath checks if the target path is within the expected directory
func (fm *FileManager) isValidPath(targetPath, baseDir string) bool {
	// Clean both paths to handle any ".." or similar path traversal attempts
	cleanTarget := filepath.Clean(targetPath)
	cleanBase := filepath.Clean(baseDir)

	// Use filepath.Rel to check if target is within base directory
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", it's outside the base directory
	return !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..")
}

// extractFile extracts a single file from the tar reader
func (fm *FileManager) extractFile(tarReader *tar.Reader, targetPath string, header *tar.Header) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), DefaultDirPermissions); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
	}

	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, tarReader); err != nil {
		return fmt.Errorf("failed to write file %s: %w", targetPath, err)
	}

	return nil
}
