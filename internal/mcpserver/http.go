package mcpserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewHTTPHandler exposes server over stateless streamable HTTP at path.
// Stateless mode is required for MCP protocol version 2026-07-28 over HTTP.
func NewHTTPHandler(server *mcp.Server, path string) (http.Handler, error) {
	if server == nil {
		return nil, fmt.Errorf("MCP server must not be nil")
	}
	if path == "" || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#{} \t\r\n") {
		return nil, fmt.Errorf("HTTP path must be an absolute path without a query or fragment, got %q", path)
	}

	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:                    true,
			PropagateRequestCancellation: true,
		},
	)

	// Reject unsafe cross-origin browser requests. The MCP SDK separately keeps
	// its default localhost DNS-rebinding protection enabled.
	protected := http.NewCrossOriginProtection().Handler(streamable)
	mux := http.NewServeMux()
	mux.Handle(path, protected)
	return mux, nil
}
