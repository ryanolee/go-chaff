package http_schema_server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"time"
)

// TestServer wraps an HTTP server for testing schema resolution
type TestServer struct {
	server   *http.Server
	listener net.Listener
	Port     int
	BaseURL  string
}

// StartTestServer starts a test HTTP server serving JSON schemas
func StartTestServer() (*TestServer, error) {
	// Get the directory where this file is located
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("failed to get current file path")
	}

	schemasDir := filepath.Join(filepath.Dir(currentFile), "schemas")

	// Create a listener on port 8080
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	port := 8080
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Create file server with JSON content type
	fs := http.FileServer(http.Dir(schemasDir))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fs.ServeHTTP(w, r)
	})

	server := &http.Server{
		Handler: mux,
	}

	ts := &TestServer{
		server:   server,
		listener: listener,
		Port:     port,
		BaseURL:  baseURL,
	}

	// Start serving in a goroutine
	go func() {
		_ = server.Serve(listener)
	}()

	// Wait for server to be ready
	if err := ts.waitForReady(5 * time.Second); err != nil {
		ts.Stop()
		return nil, err
	}

	return ts, nil
}

// waitForReady waits for the server to be ready to accept connections
func (ts *TestServer) waitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ts.Port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("server failed to become ready within %v", timeout)
}

// Stop gracefully shuts down the test server
func (ts *TestServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ts.server.Shutdown(ctx)
}

// SchemaURL returns the full URL for a schema file
func (ts *TestServer) SchemaURL(path string) string {
	return fmt.Sprintf("%s/%s", ts.BaseURL, path)
}
