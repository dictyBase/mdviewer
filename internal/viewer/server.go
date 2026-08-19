package viewer

import (
	"net/http"
)

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
