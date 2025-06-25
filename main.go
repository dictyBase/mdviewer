package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
		Action: func(c *cli.Context) error {
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

			return http.ListenAndServe(addr, server)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

type Server struct {
	markdownDir string
}

func NewServer(markdownDir string) *Server {
	return &Server{
		markdownDir: markdownDir,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		s.handleIndex(w, r)
		return
	}

	s.handleMarkdownFile(w, r, path)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	files, err := s.findMarkdownFiles()
	if err != nil {
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}

	component := IndexPage(files)
	component.Render(r.Context(), w)
}

func (s *Server) handleMarkdownFile(
	w http.ResponseWriter,
	r *http.Request,
	filename string,
) {
	content, err := s.getMarkdownContent(filename)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	htmlContent, err := convertMarkdownToHTML(content)
	if err != nil {
		http.Error(
			w,
			"Error converting markdown",
			http.StatusInternalServerError,
		)
		return
	}

	component := MarkdownPage(filename, htmlContent)
	component.Render(r.Context(), w)
}

func (s *Server) findMarkdownFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(
		s.markdownDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && isMarkdownFile(info.Name()) {
				relPath, err := filepath.Rel(s.markdownDir, path)
				if err != nil {
					return err
				}
				files = append(files, relPath)
			}

			return nil
		},
	)

	return files, err
}

func (s *Server) getMarkdownContent(filename string) ([]byte, error) {
	// Try to find the file with case-insensitive matching
	foundPath, err := s.findFileIgnoreCase(filename)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(foundPath)
}

func (s *Server) findFileIgnoreCase(filename string) (string, error) {
	baseNameWithoutExt := strings.ToLower(removeMarkdownExt(filename))

	err := filepath.Walk(
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
						return err
					}
					filename = relPath
					return filepath.SkipAll // Stop walking
				}
			}

			return nil
		},
	)

	if err != nil && err != filepath.SkipAll {
		return "", err
	}

	fullPath := filepath.Join(s.markdownDir, filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", filename)
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
