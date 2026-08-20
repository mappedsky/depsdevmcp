package mcpserver

import (
	"context"
	"strings"
	"testing"

	definition "github.com/edoardottt/depsdev/pkg/depsdev/definitions"
	service "github.com/mappedsky/depsdevmcp/internal/depsdev"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeAPI struct {
	packageCalls       int
	dependencyCalls    int
	dependencyResponse definition.Dependencies
}

func (f *fakeAPI) GetPackage(system, name string) (definition.Package, error) {
	f.packageCalls++
	return definition.Package{PackageKey: definition.PackageKey{System: system, Name: name}}, nil
}

func (*fakeAPI) GetVersion(_, _, _ string) (definition.Version, error) {
	return definition.Version{}, nil
}

func (f *fakeAPI) GetDependencies(_, _, _ string) (definition.Dependencies, error) {
	f.dependencyCalls++
	return f.dependencyResponse, nil
}

func (*fakeAPI) GetRequirements(_, _, _ string) (definition.Requirements, error) {
	return definition.Requirements{}, nil
}

func (*fakeAPI) GetProject(_ string) (definition.Project, error) {
	return definition.Project{}, nil
}

func (*fakeAPI) GetProjectPackageVersions(_ string) (definition.PackageVersions, error) {
	return definition.PackageVersions{}, nil
}

func (*fakeAPI) GetAdvisory(_ string) (definition.Advisory, error) {
	return definition.Advisory{}, nil
}

func (*fakeAPI) Query(_ string) (definition.Results, error) {
	return definition.Results{}, nil
}

func TestServerNegotiates20260728AndListsTools(t *testing.T) {
	ctx := context.Background()
	api := &fakeAPI{}
	server := New(service.NewWithAPI(api, 4))
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer serverSession.Close()

	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer clientSession.Close()

	if got := clientSession.InitializeResult().ProtocolVersion; got != "2026-07-28" {
		t.Fatalf("protocol version = %q; want 2026-07-28", got)
	}

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if got := len(tools.Tools); got != 10 {
		t.Fatalf("tool count = %d; want 10", got)
	}
	for _, tool := range tools.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q is not annotated read-only", tool.Name)
		}
	}
}

func TestPackageToolReturnsStructuredCachedResponse(t *testing.T) {
	ctx := context.Background()
	api := &fakeAPI{}
	server := New(service.NewWithAPI(api, 4))
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer clientSession.Close()

	for call := range 2 {
		result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "depsdev_get_package",
			Arguments: map[string]any{"system": "NPM", "name": "react"},
		})
		if err != nil {
			t.Fatalf("CallTool %d: %v", call, err)
		}
		if result.IsError {
			t.Fatalf("CallTool %d returned a tool error: %+v", call, result.Content)
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

func TestDependencyPathToolReturnsStructuredCachedResponse(t *testing.T) {
	ctx := context.Background()
	api := &fakeAPI{dependencyResponse: definition.Dependencies{
		Nodes: []definition.Node{
			{VersionKey: definition.VersionKey{Name: "botocore", Version: "1.34.100"}, Relation: "SELF"},
			{VersionKey: definition.VersionKey{Name: "urllib3", Version: "2.7.0"}, Relation: "DIRECT"},
		},
		Edges: []definition.Edge{{FromNode: 0, ToNode: 1, Requirement: ">=1.25.4,<1.27"}},
	}}
	server := New(service.NewWithAPI(api, 4))
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer clientSession.Close()

	arguments := map[string]any{
		"system": "pypi", "name": "botocore", "version": "1.34.100", "target": "urllib3",
	}
	for call := range 2 {
		result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "depsdev_find_dependency_path",
			Arguments: arguments,
		})
		if err != nil {
			t.Fatalf("CallTool %d: %v", call, err)
		}
		if result.IsError {
			t.Fatalf("CallTool %d returned a tool error: %+v", call, result.Content)
		}
		structured, ok := result.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("structured content type = %T; want map[string]any", result.StructuredContent)
		}
		if got, want := structured["cached"], call == 1; got != want {
			t.Fatalf("CallTool %d cached = %v; want %v", call, got, want)
		}
		data, ok := structured["data"].(map[string]any)
		if !ok || data["pulls_in"] != true {
			t.Fatalf("CallTool %d data = %#v; want pulls_in true", call, structured["data"])
		}
	}
	if api.dependencyCalls != 1 {
		t.Fatalf("upstream dependency calls = %d; want 1", api.dependencyCalls)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "depsdev_find_dependency_path",
		Arguments: map[string]any{
			"system": "go", "name": "example.com/root", "version": "v1.0.0", "target": "anything",
		},
	})
	if err != nil {
		t.Fatalf("unsupported CallTool: %v", err)
	}
	if !result.IsError || len(result.Content) != 1 {
		t.Fatalf("unsupported CallTool result = %+v; want unsupported package system error", result)
	}
	message, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(strings.ToLower(message.Text), "unsupported package system") {
		t.Fatalf("unsupported CallTool content = %+v; want unsupported package system error", result.Content)
	}
}
