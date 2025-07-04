package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(cltx *cli.Context) error {
	dir := cltx.String("dir")
	port := cltx.Int("port")

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return cli.Exit(fmt.Sprintf("directory %s does not exist", dir), 1)
	}

	server := NewServer(dir)

	addr := fmt.Sprintf(":%s", strconv.Itoa(port))
	fmt.Printf("Server starting on http://localhost%s\n", addr)
	fmt.Printf("Serving markdown files from: %s\n", dir)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := httpServer.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		return cli.Exit(fmt.Sprintf("failed to listen and serve: %v", err), 1)
	}
	return nil
}

type Server struct {
	markdownDir string
	mux         *http.ServeMux
}

func NewServer(markdownDir string) *Server {
	srv := &Server{
		markdownDir: markdownDir,
		mux:         http.NewServeMux(),
	}
	srv.routes()
	return srv
}

func (srv *Server) routes() {
	srv.mux.HandleFunc("GET /{path...}", srv.handleFileOrIndex)
}

func (srv *Server) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	srv.mux.ServeHTTP(writer, request)
}

func (srv *Server) handleFileOrIndex(
	writer http.ResponseWriter,
	request *http.Request,
) {
	path := request.PathValue("path")
	if path == "" {
		srv.handleIndex(writer, request)
		return
	}
	srv.handleMarkdownFile(writer, request)
}

func (srv *Server) handleIndex(
	writer http.ResponseWriter,
	request *http.Request,
) {
	files, err := srv.findMarkdownFiles()
	if err != nil {
		http.Error(
			writer,
			"Error reading directory",
			http.StatusInternalServerError,
		)
		return
	}

	component := IndexPage(files)
	if err := component.Render(request.Context(), writer); err != nil {
		log.Printf("error rendering index page: %v", err)
	}
}

func (srv *Server) handleMarkdownFile(
	writer http.ResponseWriter,
	request *http.Request,
) {
	filename := request.PathValue("path")
	content, err := srv.getMarkdownContent(filename)
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
	// Try to find the file with case-insensitive matching
	foundPath, err := srv.findFileIgnoreCase(filename)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(foundPath)
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
