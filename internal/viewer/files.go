package viewer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func localRequestPath(urlPath string) (string, bool) {
	if !strings.HasPrefix(urlPath, "/") || strings.HasPrefix(urlPath, "//") {
		return "", false
	}

	relativePath := strings.TrimPrefix(urlPath, "/")
	if relativePath == "" || strings.Contains(relativePath, `\`) || !fs.ValidPath(relativePath) {
		return "", false
	}

	localized, err := filepath.Localize(relativePath)
	if err != nil {
		return "", false
	}
	return localized, true
}

func (srv *Server) findMarkdownFiles() ([]string, error) {
	var files []string

	walkErr := filepath.Walk(
		srv.markdownDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && isMarkdownFile(info.Name()) {
				relPath, err := filepath.Rel(srv.markdownDir, path)
				if err != nil {
					return fmt.Errorf(
						"failed to get relative path for %s: %w",
						path,
						err,
					)
				}
				files = append(files, relPath)
			}

			return nil
		},
	)

	if walkErr != nil {
		return nil, fmt.Errorf(
			"error walking directory %s: %w",
			srv.markdownDir,
			walkErr,
		)
	}

	return files, nil
}

func (srv *Server) getMarkdownContent(filename string) ([]byte, error) {
	// Try to find the file with case-insensitive matching.
	foundPath, err := srv.findFileIgnoreCase(filename)
	if err != nil {
		return nil, err
	}

	relativePath, err := filepath.Rel(srv.markdownDir, foundPath)
	if err != nil {
		return nil, fmt.Errorf("could not resolve file %s: %w", foundPath, err)
	}

	root, err := os.OpenRoot(srv.markdownDir)
	if err != nil {
		return nil, fmt.Errorf("could not open root %s: %w", srv.markdownDir, err)
	}
	defer root.Close()

	content, err := root.ReadFile(relativePath)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", foundPath, err)
	}
	return content, nil
}

func (srv *Server) findFileIgnoreCase(filename string) (string, error) {
	baseNameWithoutExt := strings.ToLower(removeMarkdownExt(filename))

	var foundPath string
	walkErr := filepath.Walk(
		srv.markdownDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && isMarkdownFile(info.Name()) {
				relPath, err := filepath.Rel(srv.markdownDir, path)
				if err != nil {
					return fmt.Errorf(
						"could not get relative path for %s: %w",
						path,
						err,
					)
				}

				relPathWithoutExt := removeMarkdownExt(relPath)

				if strings.ToLower(relPathWithoutExt) == baseNameWithoutExt {
					// Found a match
					foundPath = relPath
					return filepath.SkipAll // Stop walking
				}
			}

			return nil
		},
	)

	if walkErr != nil && walkErr != filepath.SkipAll {
		return "", fmt.Errorf(
			"error walking directory %s: %w",
			srv.markdownDir,
			walkErr,
		)
	}

	if foundPath == "" {
		return "", fmt.Errorf("file not found: %s", filename)
	}

	fullPath := filepath.Join(srv.markdownDir, foundPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", fullPath)
	}

	return fullPath, nil
}

func isMarkdownFile(filename string) bool {
	return slices.Contains(
		[]string{
			".md",
			".markdown",
			".mdown",
			".mkd",
			".mkdn",
			".mdwn",
			".mdtxt",
			".mdtext",
		},
		strings.ToLower(filepath.Ext(filename)),
	)
}

func removeMarkdownExt(filename string) string {
	ext := filepath.Ext(filename)
	if isMarkdownFile(filename) {
		return strings.TrimSuffix(filename, ext)
	}
	return filename
}
