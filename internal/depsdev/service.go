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

func cacheKey(parts ...string) string {
	raw, _ := json.Marshal(parts)
	return string(raw)
}
