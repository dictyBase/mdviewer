package viewer

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// Config holds the runtime options for the markdown viewer server.
type Config struct {
	Directory string
	Host      string
	Port      int
}

// Run serves rendered markdown files from config.Directory until the process
// is terminated. It returns ordinary wrapped errors; the caller owns process
// exit and CLI presentation.
func Run(config Config) error {
	if _, err := os.Stat(config.Directory); os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", config.Directory)
	}

	server := NewServer(config.Directory)

	stopWatcher, err := server.startFileWatcher(config.Directory)
	if err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}
	defer stopWatcher()

	addr := net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port))
	fmt.Printf("Server starting on http://%s\n", addr)
	fmt.Printf("Serving markdown files from: %s\n", config.Directory)

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
