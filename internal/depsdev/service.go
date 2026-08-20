// Package depsdev wraps the upstream deps.dev client with a bounded LRU cache.
package depsdev

import (
	"encoding/json"
	"fmt"
	"strings"

	definition "github.com/edoardottt/depsdev/pkg/depsdev/definitions"
	depsdevv3 "github.com/edoardottt/depsdev/pkg/depsdev/v3"
	"github.com/edoardottt/depsdev/pkg/output"
	"github.com/mappedsky/depsdevmcp/internal/cache"
)

const maxDependencyPathDepth = 64

// DependencyPathResult is a compact reachability view over a resolved graph.
type DependencyPathResult struct {
	Package  string                  `json:"package" jsonschema:"Root package and version searched."`
	Target   string                  `json:"target" jsonschema:"Requested dependency package name."`
	PullsIn  bool                    `json:"pulls_in" jsonschema:"Whether the target occurs in the resolved dependency graph."`
	Versions []DependencyPathVersion `json:"versions" jsonschema:"Distinct target versions found in the graph."`
	Paths    []string                `json:"paths" jsonschema:"One dependency chain from each target version back toward the root."`
	Note     string                  `json:"note,omitempty" jsonschema:"Additional context, including a plain explanation when the target is absent."`
}

// DependencyPathVersion identifies a target version found in the graph.
type DependencyPathVersion struct {
	Name     string `json:"name" jsonschema:"Canonical target package name."`
	Version  string `json:"version" jsonschema:"Resolved target package version."`
	Relation string `json:"relation" jsonschema:"Relationship to the root: SELF, DIRECT, or INDIRECT."`
}

type parentEdge struct {
	parent      int
	requirement string
}

// API is the stable v3 surface used by Service.
type API interface {
	GetPackage(packageManager, packageName string) (definition.Package, error)
	GetVersion(packageManager, packageName, version string) (definition.Version, error)
	GetDependencies(packageManager, packageName, version string) (definition.Dependencies, error)
	GetRequirements(packageManager, packageName, version string) (definition.Requirements, error)
	GetProject(projectName string) (definition.Project, error)
	GetProjectPackageVersions(projectName string) (definition.PackageVersions, error)
	GetAdvisory(advisory string) (definition.Advisory, error)
	Query(query string) (definition.Results, error)
}

// Service provides cached access to the stable deps.dev v3 API.
type Service struct {
	api   API
	cache *cache.LRU[string, json.RawMessage]
}

// New creates a service backed by the edoardottt/depsdev v3 client.
func New(cacheCapacity int) *Service {
	return NewWithAPI(depsdevv3.NewV3API(), cacheCapacity)
}

// NewWithAPI creates a service with a supplied API implementation.
func NewWithAPI(api API, cacheCapacity int) *Service {
	return &Service{
		api:   api,
		cache: cache.NewLRU[string, json.RawMessage](cacheCapacity),
	}
}

func (s *Service) GetPackage(system, name string) (definition.Package, bool, error) {
	system = normalizeSystem(system)
	return cached(s.cache, cacheKey("package", system, name), func() (definition.Package, error) {
		return s.api.GetPackage(system, name)
	})
}

func (s *Service) GetVersion(system, name, version string) (definition.Version, bool, error) {
	system = normalizeSystem(system)
	return cached(s.cache, cacheKey("version", system, name, version), func() (definition.Version, error) {
		return s.api.GetVersion(system, name, version)
	})
}

func (s *Service) GetDependencies(system, name, version string) (definition.Dependencies, bool, error) {
	system = normalizeSystem(system)
	return cached(s.cache, cacheKey("dependencies", system, name, version), func() (definition.Dependencies, error) {
		return s.api.GetDependencies(system, name, version)
	})
}

func (s *Service) GetRequirements(system, name, version string) (definition.Requirements, bool, error) {
	system = normalizeSystem(system)
	return cached(s.cache, cacheKey("requirements", system, name, version), func() (definition.Requirements, error) {
		return s.api.GetRequirements(system, name, version)
	})
}

// FindDependencyPath reports whether target occurs in the resolved dependency
// graph and returns one parent chain for each distinct target version.
func (s *Service) FindDependencyPath(system, name, version, target string) (DependencyPathResult, bool, error) {
	dependencies, hit, err := s.GetDependencies(system, name, version)
	if err != nil {
		return DependencyPathResult{}, false, err
	}
	if dependencies.Error != "" {
		return DependencyPathResult{}, hit, fmt.Errorf("resolved dependency graph error: %s", dependencies.Error)
	}

	return findDependencyPath(dependencies, name, version, target), hit, nil
}

func (s *Service) GetProject(project string) (definition.Project, bool, error) {
	return cached(s.cache, cacheKey("project", project), func() (definition.Project, error) {
		return s.api.GetProject(project)
	})
}

func (s *Service) GetProjectPackageVersions(project string) (definition.PackageVersions, bool, error) {
	return cached(s.cache, cacheKey("project-package-versions", project), func() (definition.PackageVersions, error) {
		return s.api.GetProjectPackageVersions(project)
	})
}

func (s *Service) GetAdvisory(id string) (definition.Advisory, bool, error) {
	return cached(s.cache, cacheKey("advisory", id), func() (definition.Advisory, error) {
		return s.api.GetAdvisory(id)
	})
}

func (s *Service) Query(query string) (definition.Results, bool, error) {
	return cached(s.cache, cacheKey("query", query), func() (definition.Results, error) {
		return s.api.Query(query)
	})
}

// GenerateDependencyGraph returns a Graphviz DOT graph. Its dependency data uses
// the same cache entry as GetDependencies.
func (s *Service) GenerateDependencyGraph(system, name, version string) (string, bool, error) {
	dependencies, hit, err := s.GetDependencies(system, name, version)
	if err != nil {
		return "", false, err
	}

	graph, err := output.GenerateGraph(dependencies)
	return graph, hit, err
}

func cached[T any](c *cache.LRU[string, json.RawMessage], key string, fetch func() (T, error)) (T, bool, error) {
	if raw, ok := c.Get(key); ok {
		var value T
		if err := json.Unmarshal(raw, &value); err == nil {
			return value, true, nil
		}
		c.Delete(key)
	}

	value, err := fetch()
	if err != nil {
		var zero T
		return zero, false, err
	}

	raw, err := json.Marshal(value)
	if err != nil {
		var zero T
		return zero, false, fmt.Errorf("marshal deps.dev response for cache: %w", err)
	}
	c.Add(key, raw)
	return value, false, nil
}

func normalizeSystem(system string) string {
	return strings.ToLower(strings.TrimSpace(system))
}

func findDependencyPath(dependencies definition.Dependencies, name, version, target string) DependencyPathResult {
	result := DependencyPathResult{
		Package:  packageLabel(name, version),
		Target:   target,
		Versions: make([]DependencyPathVersion, 0),
		Paths:    make([]string, 0),
	}

	parents := make(map[int]parentEdge, len(dependencies.Edges))
	for _, edge := range dependencies.Edges {
		if edge.FromNode < 0 || edge.FromNode >= len(dependencies.Nodes) ||
			edge.ToNode < 0 || edge.ToNode >= len(dependencies.Nodes) {
			continue
		}
		if _, exists := parents[edge.ToNode]; !exists {
			parents[edge.ToNode] = parentEdge{parent: edge.FromNode, requirement: edge.Requirement}
		}
	}

	seenVersions := make(map[string]struct{})
	truncated := false
	for nodeIndex, node := range dependencies.Nodes {
		if !strings.EqualFold(node.VersionKey.Name, target) {
			continue
		}

		versionKey := strings.ToLower(node.VersionKey.Name) + "\x00" + node.VersionKey.Version
		if _, seen := seenVersions[versionKey]; seen {
			continue
		}
		seenVersions[versionKey] = struct{}{}

		result.PullsIn = true
		result.Versions = append(result.Versions, DependencyPathVersion{
			Name:     node.VersionKey.Name,
			Version:  node.VersionKey.Version,
			Relation: node.Relation,
		})
		path, wasTruncated := dependencyPath(dependencies.Nodes, parents, nodeIndex)
		result.Paths = append(result.Paths, path)
		truncated = truncated || wasTruncated
	}

	if !result.PullsIn {
		result.Note = fmt.Sprintf("target %q is not present in the resolved dependency graph for %s", target, result.Package)
	} else if truncated {
		result.Note = fmt.Sprintf("one or more paths were truncated at %d hops because the graph contains a cycle or is unusually deep", maxDependencyPathDepth)
	}
	return result
}

func dependencyPath(nodes []definition.Node, parents map[int]parentEdge, start int) (string, bool) {
	path := packageLabel(nodes[start].VersionKey.Name, nodes[start].VersionKey.Version)
	current := start
	for depth := 0; depth < maxDependencyPathDepth; depth++ {
		if nodes[current].Relation == "SELF" {
			return path, false
		}
		edge, ok := parents[current]
		if !ok {
			return path, false
		}
		path += " <-(" + edge.requirement + ") " + packageLabel(
			nodes[edge.parent].VersionKey.Name,
			nodes[edge.parent].VersionKey.Version,
		)
		current = edge.parent
	}
	return path, true
}

func packageLabel(name, version string) string {
	return name + "@" + version
}

func cacheKey(parts ...string) string {
	raw, _ := json.Marshal(parts)
	return string(raw)
}
