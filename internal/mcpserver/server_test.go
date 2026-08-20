package mcpserver

import (
	"context"
	"testing"

	definition "github.com/edoardottt/depsdev/pkg/depsdev/definitions"
	service "github.com/mappedsky/depsdevmcp/internal/depsdev"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeAPI struct {
	packageCalls int
}

func (f *fakeAPI) GetPackage(system, name string) (definition.Package, error) {
	f.packageCalls++
	return definition.Package{PackageKey: definition.PackageKey{System: system, Name: name}}, nil
}

func (*fakeAPI) GetVersion(_, _, _ string) (definition.Version, error) {
	return definition.Version{}, nil
}

func (*fakeAPI) GetDependencies(_, _, _ string) (definition.Dependencies, error) {
	return definition.Dependencies{}, nil
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
	if got := len(tools.Tools); got != 9 {
		t.Fatalf("tool count = %d; want 9", got)
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
