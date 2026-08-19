package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStaticAssets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "diagram.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	writeTestFile(t, root, "files/archive.bin", []byte{0x00, 0x01, 0xfe, 0xff})
	writeTestFile(t, root, "nested/image.png", []byte("\x89PNG\r\n\x1a\nvalid png bytes"))
	writeTestFile(t, root, "notes.txt", []byte("license text"))
	writeTestFile(t, root, "percent%note.txt", []byte("percent filename"))

	server := NewServer(root)
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "SVG", path: "/diagram.svg", want: `<svg xmlns="http://www.w3.org/2000/svg"></svg>`},
		{name: "nested", path: "/nested/image.png", want: "\x89PNG\r\n\x1a\nvalid png bytes"},
		{name: "text", path: "/notes.txt", want: "license text"},
		{name: "percent", path: "/percent%25note.txt", want: "percent filename"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response := serveRequest(server, test.path)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
			}
			if got := response.Body.String(); got != test.want {
				t.Fatalf("body = %q, want %q", got, test.want)
			}
			if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
			}
		})
	}
}

func TestStaticAssetsSupportContentTypesRangesAndEscapedSpaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "images/a diagram.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	writeTestFile(t, root, "files/data.txt", []byte("0123456789"))
	writeTestFile(t, root, "nested/image.png", []byte("\x89PNG\r\n\x1a\nvalid png bytes"))
	server := NewServer(root)

	svgResponse := serveRequest(server, "/images/a%20diagram.svg")
	if svgResponse.Code != http.StatusOK {
		t.Fatalf("SVG status = %d, want %d", svgResponse.Code, http.StatusOK)
	}
	if contentType := svgResponse.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "image/svg+xml") {
		t.Errorf("SVG Content-Type = %q, want image/svg+xml", contentType)
	}

	request := httptest.NewRequest(http.MethodGet, "/files/data.txt", nil)
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want %d", response.Code, http.StatusPartialContent)
	}
	if body := response.Body.String(); body != "2345" {
		t.Errorf("range body = %q, want %q", body, "2345")
	}
	if contentRange := response.Header().Get("Content-Range"); contentRange != "bytes 2-5/10" {
		t.Errorf("Content-Range = %q, want %q", contentRange, "bytes 2-5/10")
	}

	pngResponse := serveRequest(server, "/nested/image.png")
	if contentType := pngResponse.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "image/png") {
		t.Errorf("PNG Content-Type = %q, want image/png", contentType)
	}
}

func TestStaticAssetsRejectUnsafeOrUnavailablePaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "directory/visible.txt", []byte("must not list"))
	writeTestFile(t, root, "source.md", []byte("raw markdown sentinel"))
	writeTestFile(t, root, ".env", []byte("secret"))
	writeTestFile(t, root, "page.html", []byte("raw HTML"))
	writeTestFile(t, root, "script.js", []byte("alert('unsafe')"))
	writeTestFile(t, root, "archive.bin", []byte("binary"))

	server := NewServer(root)
	for _, requestPath := range []string{
		"/missing.txt",
		"/.env",
		"/page.html",
		"/script.js",
		"/archive.bin",
		"/LICENSE",
		"/directory",
		"/../outside.txt",
		"/%2e%2e/outside.txt",
		"/directory/%2e%2e/%2e%2e/outside.txt",
		"/directory%2f..%2foutside.txt",
		"/directory%5c..%5coutside.txt",
	} {
		t.Run(requestPath, func(t *testing.T) {
			t.Parallel()

			response := serveRequest(server, requestPath)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; location = %q", response.Code, http.StatusNotFound, response.Header().Get("Location"))
			}
			if strings.Contains(response.Body.String(), "must not list") {
				t.Fatal("response exposed directory contents")
			}
		})
	}
}

func TestStaticAssetsRejectSymlinkEscapingRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("outside secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	server := NewServer(root)
	for _, requestPath := range []string{"/escape.txt", "/escape.md", "/escape"} {
		t.Run(requestPath, func(t *testing.T) {
			t.Parallel()

			response := serveRequest(server, requestPath)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
			if strings.Contains(response.Body.String(), "outside secret") {
				t.Fatal("response exposed a file outside the configured root")
			}
		})
	}
}

func TestRelativeMarkdownAssetsResolveUnderDocumentDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "docs/guide.md", []byte("# Guide\n\n![Diagram](./images/diagram.svg)\n"))
	writeTestFile(t, root, "docs/images/diagram.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))

	server := NewServer(root)
	document := serveRequest(server, "/docs/guide")
	if document.Code != http.StatusOK {
		t.Fatalf("document status = %d, want %d; body = %q", document.Code, http.StatusOK, document.Body.String())
	}
	if !strings.Contains(document.Body.String(), `src="./images/diagram.svg"`) {
		t.Fatalf("document does not preserve relative asset URL: %q", document.Body.String())
	}

	asset := serveRequest(server, "/docs/images/diagram.svg")
	if asset.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want %d; body = %q", asset.Code, http.StatusOK, asset.Body.String())
	}
	if contentType := asset.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "image/svg+xml") {
		t.Errorf("asset Content-Type = %q, want image/svg+xml", contentType)
	}
}

func TestMarkdownRoutingRemainsExtensionlessCaseInsensitiveAndPreferred(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "Nested/Guide.MD", []byte("# Markdown route sentinel"))
	writeTestFile(t, root, "Nested/Guide", []byte("static asset sentinel"))

	server := NewServer(root)
	for _, requestPath := range []string{"/nEsTeD/gUiDe", "/nEsTeD/gUiDe.mD"} {
		t.Run(requestPath, func(t *testing.T) {
			t.Parallel()

			response := serveRequest(server, requestPath)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
			}
			body := response.Body.String()
			if !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") {
				t.Errorf("Content-Type = %q, want HTML", response.Header().Get("Content-Type"))
			}
			if !strings.Contains(body, ">Markdown route sentinel</h1>") {
				t.Fatalf("body does not contain rendered Markdown heading: %q", body)
			}
			if strings.Contains(body, "static asset sentinel") {
				t.Fatal("extensionless static asset took priority over Markdown")
			}
		})
	}
}

func TestEmbeddedMermaidRuntime(t *testing.T) {
	t.Parallel()

	response := serveRequest(NewServer(t.TempDir()), mermaidRuntimePath)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", got)
	}
	if response.Body.Len() != len(mermaidRuntime) {
		t.Fatalf("runtime length = %d, want %d", response.Body.Len(), len(mermaidRuntime))
	}
}

func TestContentFragmentReturnsRenderedAndRawContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	raw := "# Heading\n\nSome *content*.\n"
	writeTestFile(t, root, "guide.md", []byte(raw))

	response := serveRequest(NewServer(root), "/_mdviewer/content/guide")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
	}

	var fragment contentFragment
	if err := json.Unmarshal(response.Body.Bytes(), &fragment); err != nil {
		t.Fatalf("decode fragment: %v", err)
	}
	if fragment.Raw != raw {
		t.Errorf("raw = %q, want %q", fragment.Raw, raw)
	}
	if !strings.Contains(fragment.HTML, "<h1") {
		t.Errorf("html does not contain rendered heading: %q", fragment.HTML)
	}

	wantETag := fmt.Sprintf("%q", contentETag([]byte(raw)))
	if got := response.Header().Get("ETag"); got != wantETag {
		t.Errorf("ETag = %q, want %q", got, wantETag)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}

func TestContentFragmentNotModifiedWhenETagMatches(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	raw := "# Heading\n"
	writeTestFile(t, root, "guide.md", []byte(raw))

	request := httptest.NewRequest(http.MethodGet, "/_mdviewer/content/guide", nil)
	request.Header.Set("If-None-Match", fmt.Sprintf("%q", contentETag([]byte(raw))))
	response := httptest.NewRecorder()
	NewServer(root).ServeHTTP(response, request)

	if response.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotModified)
	}
	if response.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", response.Body.Len())
	}
}

func TestContentFragmentSameSizeChangeProducesDifferentETag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "guide.md", []byte("aaaa"))
	server := NewServer(root)

	first := serveRequest(server, "/_mdviewer/content/guide")
	firstETag := first.Header().Get("ETag")

	writeTestFile(t, root, "guide.md", []byte("bbbb"))
	second := serveRequest(server, "/_mdviewer/content/guide")
	secondETag := second.Header().Get("ETag")

	if firstETag == "" || secondETag == "" {
		t.Fatalf("missing ETag: first = %q, second = %q", firstETag, secondETag)
	}
	if firstETag == secondETag {
		t.Errorf("same-size content change did not alter ETag: %q", firstETag)
	}
}

func TestContentFragmentRejectsMissingOrUnsafePaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "guide.md", []byte("# Guide\n"))
	server := NewServer(root)

	for _, requestPath := range []string{
		"/_mdviewer/content/missing",
		"/_mdviewer/content/../outside",
		"/_mdviewer/content/.env",
	} {
		t.Run(requestPath, func(t *testing.T) {
			t.Parallel()

			response := serveRequest(server, requestPath)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}

func TestMarkdownPageEmbedsContentETag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	raw := "# Guide\n"
	writeTestFile(t, root, "guide.md", []byte(raw))

	response := serveRequest(NewServer(root), "/guide")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	want := fmt.Sprintf(`data-content-etag="%s"`, contentETag([]byte(raw)))
	if !strings.Contains(response.Body.String(), want) {
		t.Errorf("page does not embed %q", want)
	}
}

func writeTestFile(t *testing.T, root, name string, content []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func serveRequest(handler http.Handler, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// streamRecorder is a concurrency-safe ResponseWriter that captures streaming
// (SSE) output. httptest.ResponseRecorder buffers but is not safe to read while
// a handler goroutine is still writing, so SSE tests use this instead.
type streamRecorder struct {
	mu     sync.Mutex
	header http.Header
	body   strings.Builder
	code   int
}

func (r *streamRecorder) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *streamRecorder) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.code == 0 {
		r.code = http.StatusOK
	}
	written, _ := r.body.Write(data)
	return written, nil
}

func (r *streamRecorder) WriteHeader(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.code == 0 {
		r.code = code
	}
}

// Flush is a no-op, as required by the http.Flusher interface.
func (r *streamRecorder) Flush() {}

func (r *streamRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func TestSSEStreamsChangeEvents(t *testing.T) {
	t.Parallel()

	server := NewServer(t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, eventsPath, nil).WithContext(ctx)
	response := &streamRecorder{}

	done := make(chan struct{})
	go func() {
		server.ServeHTTP(response, request)
		close(done)
	}()

	// Wait for the initial connection comment, then deliver a change.
	waitFor := func(want string) {
		deadline := time.Now().Add(2 * time.Second)
		for !strings.Contains(response.String(), want) && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
	}

	waitFor(": connected")
	server.hub.broadcast()
	waitFor("data: reload")

	cancel()
	<-done

	if response.code != http.StatusOK {
		t.Errorf("status = %d, want %d", response.code, http.StatusOK)
	}
	if ct := response.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if body := response.String(); !strings.Contains(body, "data: reload") {
		t.Errorf("SSE body missing change event: %q", body)
	}
}

func TestFileWatcherBroadcastsOnChange(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	server := NewServer(root)

	stop, err := server.startFileWatcher(root)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer stop()

	sub := server.hub.subscribe()
	defer server.hub.unsubscribe(sub)

	writeTestFile(t, root, "guide.md", []byte("# Hello\n"))

	select {
	case <-sub:
	case <-time.After(5 * time.Second):
		t.Fatal("no change broadcast after file write")
	}
}
