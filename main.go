package main

import (
	_ "embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
			&cli.StringFlag{
				Name:  "host",
				Value: "127.0.0.1",
				Usage: "Host interface to serve on",
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
	host := cltx.String("host")

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return cli.Exit(fmt.Sprintf("directory %s does not exist", dir), 1)
	}

	server := NewServer(dir)

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	fmt.Printf("Server starting on http://%s\n", addr)
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

const mermaidRuntimePath = "/_mdviewer/assets/mermaid-11.12.2.min.js"

//go:embed assets/mermaid-11.12.2.min.js
var mermaidRuntime []byte

var allowedAssetExtensions = map[string]struct{}{
	".avif": {}, ".bmp": {}, ".css": {}, ".csv": {}, ".gif": {},
	".jpeg": {}, ".jpg": {}, ".mp3": {}, ".mp4": {}, ".oga": {},
	".ogg": {}, ".pdf": {}, ".png": {}, ".svg": {}, ".txt": {},
	".wav": {}, ".webm": {}, ".webp": {},
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
	srv.mux.HandleFunc("GET "+mermaidRuntimePath, srv.handleMermaidRuntime)
	srv.mux.HandleFunc("GET /{path...}", srv.handleFileOrIndex)
}

func (srv *Server) handleMermaidRuntime(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = writer.Write(mermaidRuntime)
}

func (srv *Server) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if request.URL.Path == mermaidRuntimePath {
		srv.mux.ServeHTTP(writer, request)
		return
	}
	if request.URL.Path != "/" {
		if _, ok := localRequestPath(request.URL.Path); !ok {
			http.NotFound(writer, request)
			return
		}
	}
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
	if srv.handleMarkdownFile(writer, request) {
		return
	}
	srv.handleStaticAsset(writer, request)
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
) bool {
	filename := request.PathValue("path")
	content, err := srv.getMarkdownContent(filename)
	if err != nil {
		return false
	}

	document, err := convertMarkdown(content)
	if err != nil {
		http.Error(
			writer,
			"Error converting markdown",
			http.StatusInternalServerError,
		)
		return true
	}

	component := MarkdownPage(filename, document.HTML, document.Headings, string(content))
	if err := component.Render(request.Context(), writer); err != nil {
		log.Printf("error rendering markdown page: %v", err)
	}
	return true
}

func (srv *Server) handleStaticAsset(
	writer http.ResponseWriter,
	request *http.Request,
) {
	filename, ok := localRequestPath(request.URL.Path)
	if !ok || isMarkdownFile(filename) || !isAllowedAsset(filename) {
		http.NotFound(writer, request)
		return
	}

	root, err := os.OpenRoot(srv.markdownDir)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer root.Close()

	file, err := root.Open(filename)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(writer, request)
		return
	}

	if contentType := assetContentType(filename); contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(writer, request, info.Name(), info.ModTime(), file)
}

var assetContentTypes = map[string]string{
	".css": "text/css; charset=utf-8", ".csv": "text/csv; charset=utf-8", ".txt": "text/plain; charset=utf-8",
	".pdf": "application/pdf", ".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".avif": "image/avif", ".bmp": "image/bmp",
	".mp3": "audio/mpeg", ".mp4": "video/mp4",
	".oga": "audio/ogg", ".ogg": "audio/ogg", ".wav": "audio/wav", ".webm": "video/webm",
}

func assetContentType(filename string) string {
	return assetContentTypes[strings.ToLower(filepath.Ext(filename))]
}

func isAllowedAsset(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, allowed := allowedAssetExtensions[ext]
	return allowed
}

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
