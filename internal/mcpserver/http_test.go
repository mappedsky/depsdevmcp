package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	service "github.com/mappedsky/depsdevmcp/internal/depsdev"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStreamableHTTPNegotiates20260728AndCallsTool(t *testing.T) {
	api := &fakeAPI{}
	server := New(service.NewWithAPI(api, 4))
	handler, err := NewHTTPHandler(server, "/mcp")
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "http-test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer session.Close()

	if got := session.InitializeResult().ProtocolVersion; got != "2026-07-28" {
		t.Fatalf("protocol version = %q; want 2026-07-28", got)
	}
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if got := len(tools.Tools); got != 10 {
		t.Fatalf("tool count = %d; want 10", got)
	}

	for call := range 2 {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "depsdev_get_package",
			Arguments: map[string]any{"system": "npm", "name": "react"},
		})
		if err != nil {
			t.Fatalf("CallTool %d: %v", call, err)
		}
		if result.IsError {
			t.Fatalf("CallTool %d returned tool error: %+v", call, result.Content)
		}
		structured, ok := result.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("structured content type = %T; want map[string]any", result.StructuredContent)
		}
		if got, want := structured["cached"], call == 1; got != want {
			t.Fatalf("CallTool %d cached = %v; want %v", call, got, want)
		}
	}
	if api.packageCalls != 1 {
		t.Fatalf("upstream package calls = %d; want 1", api.packageCalls)
	}
}

func TestStreamableHTTPIsStateless(t *testing.T) {
	server := New(service.NewWithAPI(&fakeAPI{}, 4))
	handler, err := NewHTTPHandler(server, "/mcp")
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://localhost/mcp", nil)
	handler.ServeHTTP(recorder, request)

	if got := recorder.Code; got != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d; want %d", got, http.StatusMethodNotAllowed)
	}
	if got := recorder.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("Allow header = %q; want %q", got, http.MethodPost)
	}
}

func TestHTTPHandlerRejectsCrossOriginRequests(t *testing.T) {
	server := New(service.NewWithAPI(&fakeAPI{}, 4))
	handler, err := NewHTTPHandler(server, "/mcp")
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader("{}"))
	request.Header.Set("Origin", "https://attacker.example")
	handler.ServeHTTP(recorder, request)

	if got := recorder.Code; got != http.StatusForbidden {
		t.Fatalf("cross-origin POST status = %d; want %d", got, http.StatusForbidden)
	}
}

func TestNewHTTPHandlerRejectsInvalidPaths(t *testing.T) {
	server := New(service.NewWithAPI(&fakeAPI{}, 4))
	for _, path := range []string{"", "mcp", "/mcp?debug=1", "/mcp#fragment", "/mcp/{id}", "/mcp path"} {
		t.Run(path, func(t *testing.T) {
			if _, err := NewHTTPHandler(server, path); err == nil {
				t.Fatalf("NewHTTPHandler path %q succeeded; want error", path)
			}
		})
	}
}
