// Package mcpserver exposes the deps.dev API as Model Context Protocol tools.
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	definition "github.com/edoardottt/depsdev/pkg/depsdev/definitions"
	"github.com/edoardottt/depsdev/pkg/input"
	service "github.com/mappedsky/depsdevmcp/internal/depsdev"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	Name    = "depsdevmcp"
	Version = "0.2.0"
)

type PackageInput struct {
	System string `json:"system" jsonschema:"Package ecosystem: go, rubygems, npm, cargo, maven, pypi, or nuget."`
	Name   string `json:"name" jsonschema:"Package name in the ecosystem's native format."`
}

type VersionInput struct {
	System  string `json:"system" jsonschema:"Package ecosystem: go, rubygems, npm, cargo, maven, pypi, or nuget."`
	Name    string `json:"name" jsonschema:"Package name in the ecosystem's native format."`
	Version string `json:"version" jsonschema:"Exact package version."`
}

type ProjectInput struct {
	Project string `json:"project" jsonschema:"Project identifier including host, for example github.com/facebook/react."`
}

type AdvisoryInput struct {
	ID string `json:"id" jsonschema:"OSV advisory identifier, for example GHSA-2qrg-x229-3v8q."`
}

type QueryInput struct {
	Query string `json:"query" jsonschema:"deps.dev v3 query string, for example versionKey.system=NPM&versionKey.name=react&versionKey.version=18.2.0."`
}

type Output[T any] struct {
	Data   T    `json:"data" jsonschema:"The response returned by deps.dev."`
	Cached bool `json:"cached" jsonschema:"Whether this response came from the in-memory LRU cache."`
}

// New creates an MCP server with all stable deps.dev v3 query tools.
func New(deps *service.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:        Name,
		Title:       "deps.dev",
		Description: "Query package, dependency, project, and advisory data from deps.dev.",
		Version:     Version,
		WebsiteURL:  "https://deps.dev",
	}, nil)

	mcp.AddTool(server, tool(
		"depsdev_get_package",
		"Get package metadata and all known versions, including the default version.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in PackageInput) (*mcp.CallToolResult, Output[definition.Package], error) {
		if err := validatePackage(in.System, in.Name, input.AllValidPackageManagers); err != nil {
			return nil, Output[definition.Package]{}, err
		}
		value, hit, err := deps.GetPackage(in.System, in.Name)
		return nil, Output[definition.Package]{Data: value, Cached: hit}, err
	})

	mcp.AddTool(server, tool(
		"depsdev_get_version",
		"Get metadata for an exact package version, including licenses, advisories, links, and attestations.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in VersionInput) (*mcp.CallToolResult, Output[definition.Version], error) {
		if err := validateVersion(in, input.AllValidPackageManagers); err != nil {
			return nil, Output[definition.Version]{}, err
		}
		value, hit, err := deps.GetVersion(in.System, in.Name, in.Version)
		return nil, Output[definition.Version]{Data: value, Cached: hit}, err
	})

	mcp.AddTool(server, tool(
		"depsdev_get_dependencies",
		"Get the resolved dependency graph for an exact npm, Cargo, Maven, or PyPI package version.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in VersionInput) (*mcp.CallToolResult, Output[definition.Dependencies], error) {
		if err := validateVersion(in, input.DepsValidPackageManagers); err != nil {
			return nil, Output[definition.Dependencies]{}, err
		}
		value, hit, err := deps.GetDependencies(in.System, in.Name, in.Version)
		return nil, Output[definition.Dependencies]{Data: value, Cached: hit}, err
	})

	mcp.AddTool(server, tool(
		"depsdev_get_requirements",
		"Get ecosystem-specific declared dependency requirements for an exact package version.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in VersionInput) (*mcp.CallToolResult, Output[definition.Requirements], error) {
		if err := validateVersion(in, input.AllValidPackageManagers); err != nil {
			return nil, Output[definition.Requirements]{}, err
		}
		value, hit, err := deps.GetRequirements(in.System, in.Name, in.Version)
		return nil, Output[definition.Requirements]{Data: value, Cached: hit}, err
	})

	mcp.AddTool(server, tool(
		"depsdev_get_project",
		"Get metadata for a known GitHub, GitLab, or Bitbucket project.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in ProjectInput) (*mcp.CallToolResult, Output[definition.Project], error) {
		if err := require("project", in.Project); err != nil {
			return nil, Output[definition.Project]{}, err
		}
		value, hit, err := deps.GetProject(in.Project)
		return nil, Output[definition.Project]{Data: value, Cached: hit}, err
	})

	mcp.AddTool(server, tool(
		"depsdev_get_project_package_versions",
		"Get known mappings between a GitHub, GitLab, or Bitbucket project and package versions.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in ProjectInput) (*mcp.CallToolResult, Output[definition.PackageVersions], error) {
		if err := require("project", in.Project); err != nil {
			return nil, Output[definition.PackageVersions]{}, err
		}
		value, hit, err := deps.GetProjectPackageVersions(in.Project)
		return nil, Output[definition.PackageVersions]{Data: value, Cached: hit}, err
	})

	mcp.AddTool(server, tool(
		"depsdev_get_advisory",
		"Get details for an OSV security advisory.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in AdvisoryInput) (*mcp.CallToolResult, Output[definition.Advisory], error) {
		if err := require("id", in.ID); err != nil {
			return nil, Output[definition.Advisory]{}, err
		}
		value, hit, err := deps.GetAdvisory(in.ID)
		return nil, Output[definition.Advisory]{Data: value, Cached: hit}, err
	})

	mcp.AddTool(server, tool(
		"depsdev_query",
		"Query package versions by a version key, artifact content hash, or both, using deps.dev v3 query parameters.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in QueryInput) (*mcp.CallToolResult, Output[definition.Results], error) {
		if err := require("query", in.Query); err != nil {
			return nil, Output[definition.Results]{}, err
		}
		value, hit, err := deps.Query(in.Query)
		return nil, Output[definition.Results]{Data: value, Cached: hit}, err
	})

	mcp.AddTool(server, tool(
		"depsdev_generate_dependency_graph",
		"Generate a Graphviz DOT dependency graph for an exact npm, Cargo, Maven, or PyPI package version.",
	), func(_ context.Context, _ *mcp.CallToolRequest, in VersionInput) (*mcp.CallToolResult, Output[string], error) {
		if err := validateVersion(in, input.DepsValidPackageManagers); err != nil {
			return nil, Output[string]{}, err
		}
		value, hit, err := deps.GenerateDependencyGraph(in.System, in.Name, in.Version)
		return nil, Output[string]{Data: value, Cached: hit}, err
	})

	return server
}

func tool(name, description string) *mcp.Tool {
	readOnly, openWorld, destructive := true, true, false
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			IdempotentHint:  true,
			OpenWorldHint:   &openWorld,
			ReadOnlyHint:    readOnly,
		},
	}
}

func validateVersion(in VersionInput, systems []string) error {
	if err := validatePackage(in.System, in.Name, systems); err != nil {
		return err
	}
	return require("version", in.Version)
}

func validatePackage(system, name string, systems []string) error {
	if err := require("system", system); err != nil {
		return err
	}
	if !input.IsValidPackageManager(system, systems) {
		return fmt.Errorf("unsupported package system %q; supported systems: %s", system, strings.Join(systems, ", "))
	}
	return require("name", name)
}

func require(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	return nil
}
