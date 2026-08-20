package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	depsdev "github.com/mappedsky/depsdevmcp/internal/depsdev"
	"github.com/mappedsky/depsdevmcp/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultCacheCapacity = 256
	defaultTransport     = "stdio"
	defaultHTTPAddress   = "127.0.0.1:8080"
	defaultHTTPPath      = "/mcp"
	shutdownTimeout      = 10 * time.Second
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("depsdevmcp: ")
	log.SetOutput(os.Stderr)

	cacheCapacity := flag.Int("cache-capacity", cacheCapacityFromEnv(), "maximum number of deps.dev responses held in the in-memory LRU cache")
	transport := flag.String("transport", envOrDefault("DEPSDEVMCP_TRANSPORT", defaultTransport), "MCP transport: stdio or http")
	httpAddress := flag.String("http-address", envOrDefault("DEPSDEVMCP_HTTP_ADDRESS", defaultHTTPAddress), "streamable HTTP listen address")
	httpPath := flag.String("http-path", envOrDefault("DEPSDEVMCP_HTTP_PATH", defaultHTTPPath), "streamable HTTP endpoint path")
	showVersion := flag.Bool("version", false, "print the server version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(mcpserver.Version)
		return
	}
	if *cacheCapacity <= 0 {
		log.Fatal("cache capacity must be positive")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := mcpserver.New(depsdev.New(*cacheCapacity))
	if err := run(ctx, server, *transport, *httpAddress, *httpPath); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, server *mcp.Server, transport, httpAddress, httpPath string) error {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "stdio":
		err := server.Run(ctx, &mcp.StdioTransport{})
		if err != nil && ctx.Err() != nil {
			return nil
		}
		return err
	case "http":
		return runHTTP(ctx, server, httpAddress, httpPath)
	default:
		return fmt.Errorf("unsupported transport %q; supported transports: stdio, http", transport)
	}
}

func runHTTP(ctx context.Context, mcpServer *mcp.Server, address, path string) error {
	if strings.TrimSpace(address) == "" {
		return fmt.Errorf("HTTP address must not be empty")
	}
	handler, err := mcpserver.NewHTTPHandler(mcpServer, path)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	defer listener.Close()

	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	shutdownDone := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		shutdownDone <- httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("streamable HTTP listening on http://%s%s", listener.Addr(), path)
	err = httpServer.Serve(listener)
	if !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve streamable HTTP: %w", err)
	}
	if err := <-shutdownDone; err != nil {
		return fmt.Errorf("shut down streamable HTTP: %w", err)
	}
	return nil
}

func cacheCapacityFromEnv() int {
	raw := os.Getenv("DEPSDEVMCP_CACHE_CAPACITY")
	if raw == "" {
		return defaultCacheCapacity
	}

	capacity, err := strconv.Atoi(raw)
	if err != nil || capacity <= 0 {
		log.Fatalf("DEPSDEVMCP_CACHE_CAPACITY must be a positive integer, got %q", raw)
	}
	return capacity
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
