package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	depsdev "github.com/mappedsky/depsdevmcp/internal/depsdev"
	"github.com/mappedsky/depsdevmcp/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultCacheCapacity = 256

func main() {
	log.SetFlags(0)
	log.SetPrefix("depsdevmcp: ")
	log.SetOutput(os.Stderr)

	cacheCapacity := flag.Int("cache-capacity", cacheCapacityFromEnv(), "maximum number of deps.dev responses held in the in-memory LRU cache")
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
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
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
