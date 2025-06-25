package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "mdviewer",
		Usage: "A web server that displays markdown files as HTML",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "dir",
				Aliases: []string{"d"},
				Value:   ".",
				Usage:   "Directory containing markdown files",
			},
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   8888,
				Usage:   "Port to serve on",
			},
		},
		Action: runServer,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func runServer(c *cli.Context) error {
	dir := c.String("dir")
	port := c.Int("port")

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", dir)
	}

	server := NewServer(dir)

	addr := ":" + strconv.Itoa(port)
	fmt.Printf("Server starting on http://localhost%s\n", addr)
	fmt.Printf("Serving markdown files from: %s\n", dir)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to listen and serve: %w", err)
	}
	return nil
}

type Server struct {
	markdownDir string
}

func NewServer(markdownDir string) *Server {
	return &Server{
		markdownDir: markdownDir,
	}
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "GET" {
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(request.URL.Path, "/")
	if path == "" {
		s.handleIndex(writer, request)
		return
	}

	s.handleMarkdownFile(writer, request, path)
}

func (s *Server) handleIndex(writer http.ResponseWriter, request *http.Request) {
	files, err := s.findMarkdownFiles()
	if err != nil {
		http.Error(writer, "Error reading directory", http.StatusInternalServerError)
		return
	}

	component := IndexPage(files)
	if err := component.Render(request.Context(), writer); err != nil {
		log.Printf("error rendering index page: %v", err)
	}
}

func (s *Server) handleMarkdownFile(
	writer http.ResponseWriter,
	request *http.Request,
	filename string,
) {
	content, err := s.getMarkdownContent(filename)
	if err != nil {
		http.Error(writer, "File not found", http.StatusNotFound)
		return
	}

	htmlContent, err := convertMarkdownToHTML(content)
	if err != nil {
		http.Error(
			writer,
			"Error converting markdown",
			http.StatusInternalServerError,
		)
		return
	}

	component := MarkdownPage(filename, htmlContent)
	if err := component.Render(request.Context(), writer); err != nil {
		log.Printf("error rendering markdown page: %v", err)
	}
}

func (s *Server) findMarkdownFiles() ([]string, error) {
	var files []string

	walkErr := filepath.Walk(
		s.markdownDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && isMarkdownFile(info.Name()) {
				relPath, err := filepath.Rel(s.markdownDir, path)
				if err != nil {
					return fmt.Errorf("failed to get relative path for %s: %w", path, err)
				}
				files = append(files, relPath)
			}

			return nil
		},
	)

	if walkErr != nil {
		return nil, fmt.Errorf("error walking directory %s: %w", s.markdownDir, walkErr)
	}

	return files, nil
}

func (s *Server) getMarkdownContent(filename string) ([]byte, error) {
	// Try to find the file with case-insensitive matching
	foundPath, err := s.findFileIgnoreCase(filename)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(foundPath)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", foundPath, err)
	}
	return content, nil
}

func (s *Server) findFileIgnoreCase(filename string) (string, error) {
	baseNameWithoutExt := strings.ToLower(removeMarkdownExt(filename))

	var foundPath string
	walkErr := filepath.Walk(
		s.markdownDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && isMarkdownFile(info.Name()) {
				fileNameWithoutExt := strings.ToLower(
					removeMarkdownExt(info.Name()),
				)
				if fileNameWithoutExt == baseNameWithoutExt {
					// Found a match
					relPath, err := filepath.Rel(s.markdownDir, path)
					if err != nil {
						return fmt.Errorf("could not get relative path for %s: %w", path, err)
					}
					foundPath = relPath
					return filepath.SkipAll // Stop walking
				}
			}

			return nil
		},
	)

	if walkErr != nil && walkErr != filepath.SkipAll {
		return "", fmt.Errorf("error walking directory %s: %w", s.markdownDir, walkErr)
	}

	if foundPath == "" {
		return "", fmt.Errorf("file not found: %s", filename)
	}

	fullPath := filepath.Join(s.markdownDir, foundPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", fullPath)
	}

	return fullPath, nil
}

func isMarkdownFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	markdownExts := []string{
		".md",
		".markdown",
		".mdown",
		".mkd",
		".mkdn",
		".mdwn",
		".mdtxt",
		".mdtext",
		".text",
	}

	for _, mdExt := range markdownExts {
		if ext == mdExt {
			return true
		}
	}

	return false
}

func removeMarkdownExt(filename string) string {
	ext := filepath.Ext(filename)
	if isMarkdownFile(filename) {
		return strings.TrimSuffix(filename, ext)
	}
	return filename
}
