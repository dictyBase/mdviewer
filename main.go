package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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

	stopWatcher, err := server.startFileWatcher(dir)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to watch directory: %v", err), 1)
	}
	defer stopWatcher()

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

const contentFragmentPath = "/_mdviewer/content/"

const eventsPath = "/_mdviewer/events"

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
	hub         *changeHub
}

func NewServer(markdownDir string) *Server {
	srv := &Server{
		markdownDir: markdownDir,
		mux:         http.NewServeMux(),
		hub:         newChangeHub(),
	}
	srv.routes()
	return srv
}

func (srv *Server) routes() {
	srv.mux.HandleFunc("GET "+mermaidRuntimePath, srv.handleMermaidRuntime)
	srv.mux.HandleFunc("GET "+contentFragmentPath+"{path...}", srv.handleContentFragment)
	srv.mux.HandleFunc("GET "+eventsPath, srv.handleEvents)
	srv.mux.HandleFunc("GET /{path...}", srv.handleFileOrIndex)
}

func (srv *Server) handleMermaidRuntime(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = writer.Write(mermaidRuntime)
}

// contentFragment holds the rendered and raw markdown for a single document.
// The client polls this endpoint to live-reload the preview without a full page refresh.
type contentFragment struct {
	HTML string `json:"html"`
	Raw  string `json:"raw"`
}

func (srv *Server) handleContentFragment(
	writer http.ResponseWriter,
	request *http.Request,
) {
	filename := request.PathValue("path")
	content, err := srv.getMarkdownContent(filename)
	if err != nil {
		http.NotFound(writer, request)
		return
	}

	etag := contentETag(content)
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("ETag", fmt.Sprintf("%q", etag))
	if request.Header.Get("If-None-Match") == fmt.Sprintf("%q", etag) {
		writer.WriteHeader(http.StatusNotModified)
		return
	}

	document, err := convertMarkdown(content)
	if err != nil {
		http.Error(
			writer,
			"Error converting markdown",
			http.StatusInternalServerError,
		)
		return
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(writer).Encode(contentFragment{
		HTML: document.HTML,
		Raw:  string(content),
	}); err != nil {
		log.Printf("error encoding content fragment: %v", err)
	}
}

// contentETag returns a hex SHA-256 digest of the raw markdown, used to detect
// content changes independent of file modification time and size.
func contentETag(content []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(content))
}

// changeHub fans out file-change notifications to every connected SSE client.
type changeHub struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

func newChangeHub() *changeHub {
	return &changeHub{subs: make(map[chan struct{}]struct{})}
}

func (h *changeHub) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *changeHub) unsubscribe(ch chan struct{}) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

func (h *changeHub) broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- struct{}{}:
		default: // drop for slow clients; they re-sync on reconnect
		}
	}
}

func (srv *Server) handleEvents(writer http.ResponseWriter, request *http.Request) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")

	// The HTTP server sets a write deadline from WriteTimeout; clear it so the
	// long-lived stream is not killed between events.
	if err := http.NewResponseController(writer).SetWriteDeadline(time.Time{}); err != nil {
		log.Printf("unable to clear write deadline for SSE: %v", err)
	}

	writer.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(writer, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	sub := srv.hub.subscribe()
	defer srv.hub.unsubscribe(sub)

	for {
		select {
		case <-request.Context().Done():
			return
		case <-sub:
			if _, err := fmt.Fprint(writer, "data: reload\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// startFileWatcher recursively watches dir with fsnotify and broadcasts a
// debounced change notification on every relevant filesystem event. It returns
// a stop function that closes the watcher and blocks until the goroutine exits.
func (srv *Server) startFileWatcher(dir string) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	if err := addWatchTree(watcher, dir); err != nil {
		watcher.Close()
		return nil, err
	}

	relay := newChangeRelay(srv.hub)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchChanges(watcher, relay)
	}()

	return func() {
		watcher.Close()
		<-done
	}, nil
}

// changeRelay coalesces a burst of filesystem events into a single broadcast.
type changeRelay struct {
	hub     *changeHub
	timer   *time.Timer
	pending bool
}

func newChangeRelay(hub *changeHub) *changeRelay {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	return &changeRelay{hub: hub, timer: timer}
}

func (r *changeRelay) note() {
	r.pending = true
	if !r.timer.Stop() {
		select {
		case <-r.timer.C:
		default:
		}
	}
	r.timer.Reset(150 * time.Millisecond)
}

func (r *changeRelay) flush() {
	if r.pending {
		r.pending = false
		r.hub.broadcast()
	}
}

func watchChanges(watcher *fsnotify.Watcher, relay *changeRelay) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				relay.flush()
				return
			}
			handleWatchEvent(watcher, relay, event)
		case err, ok := <-watcher.Errors:
			if !ok {
				relay.flush()
				return
			}
			log.Printf("fsnotify error: %v", err)
		case <-relay.timer.C:
			relay.flush()
		}
	}
}

func handleWatchEvent(watcher *fsnotify.Watcher, relay *changeRelay, event fsnotify.Event) {
	const relevantOps = fsnotify.Write | fsnotify.Create | fsnotify.Remove | fsnotify.Rename | fsnotify.Chmod

	if event.Op&fsnotify.Create != 0 {
		if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
			_ = watcher.Add(event.Name)
		}
	}
	if event.Op&relevantOps != 0 {
		relay.note()
	}
}

func addWatchTree(watcher *fsnotify.Watcher, dir string) error {
	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if err := watcher.Add(path); err != nil {
			return fmt.Errorf("watch %s: %w", path, err)
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("walk directory tree %s: %w", dir, walkErr)
	}
	return nil
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

	component := MarkdownPage(
		filename,
		document.HTML,
		document.Headings,
		string(content),
		contentETag(content),
	)
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
