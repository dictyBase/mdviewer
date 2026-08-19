package viewer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// contentFragment holds the rendered and raw markdown for a single document.
// The client fetches this endpoint on an SSE change event (or polling fallback)
// to swap the preview in place without a full page refresh.
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
