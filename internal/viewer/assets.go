package viewer

import (
	_ "embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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

func (srv *Server) handleMermaidRuntime(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = writer.Write(mermaidRuntime)
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
