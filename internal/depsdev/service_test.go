package depsdev

import (
	"errors"
	"strings"
	"testing"

	definition "github.com/edoardottt/depsdev/pkg/depsdev/definitions"
)

type fakeAPI struct {
	packageCalls      int
	dependencyCalls   int
	packageErr        error
	dependencyErr     error
	packageResponse   definition.Package
	dependencyResults definition.Dependencies
}

func (f *fakeAPI) GetPackage(_, _ string) (definition.Package, error) {
	f.packageCalls++
	return f.packageResponse, f.packageErr
}

func (f *fakeAPI) GetVersion(_, _, _ string) (definition.Version, error) {
	return definition.Version{}, nil
}

func (f *fakeAPI) GetDependencies(_, _, _ string) (definition.Dependencies, error) {
	f.dependencyCalls++
	return f.dependencyResults, f.dependencyErr
}

func (f *fakeAPI) GetRequirements(_, _, _ string) (definition.Requirements, error) {
	return definition.Requirements{}, nil
}

func (f *fakeAPI) GetProject(_ string) (definition.Project, error) {
	return definition.Project{}, nil
}

func (f *fakeAPI) GetProjectPackageVersions(_ string) (definition.PackageVersions, error) {
	return definition.PackageVersions{}, nil
}

func (f *fakeAPI) GetAdvisory(_ string) (definition.Advisory, error) {
	return definition.Advisory{}, nil
}

func (f *fakeAPI) Query(_ string) (definition.Results, error) {
	return definition.Results{}, nil
}

func TestServiceCachesSuccessfulResponses(t *testing.T) {
	api := &fakeAPI{packageResponse: definition.Package{
		PackageKey: definition.PackageKey{System: "NPM", Name: "react"},
	}}
	service := NewWithAPI(api, 2)

	first, hit, err := service.GetPackage("NPM", "react")
	if err != nil {
		t.Fatalf("first GetPackage: %v", err)
	}
	if hit {
		t.Fatal("first GetPackage unexpectedly hit cache")
	}
	second, hit, err := service.GetPackage("npm", "react")
	if err != nil {
		t.Fatalf("second GetPackage: %v", err)
	}
	if !hit {
		t.Fatal("second GetPackage did not hit cache")
	}
	if first.PackageKey != second.PackageKey {
		t.Fatalf("cached response = %+v; want %+v", second, first)
	}
	if api.packageCalls != 1 {
		t.Fatalf("GetPackage upstream calls = %d; want 1", api.packageCalls)
	}
}

func TestServiceDoesNotCacheErrors(t *testing.T) {
	api := &fakeAPI{packageErr: errors.New("upstream unavailable")}
	service := NewWithAPI(api, 2)

	for range 2 {
		if _, hit, err := service.GetPackage("npm", "react"); err == nil || hit {
			t.Fatalf("GetPackage error = %v, hit = %v; want error and miss", err, hit)
		}
	}
	if api.packageCalls != 2 {
		t.Fatalf("GetPackage upstream calls = %d; want 2", api.packageCalls)
	}
}

func TestGenerateDependencyGraphSharesDependencyCache(t *testing.T) {
	api := &fakeAPI{dependencyResults: definition.Dependencies{
		Nodes: []definition.Node{{
			VersionKey: definition.VersionKey{System: "NPM", Name: "react", Version: "18.2.0"},
		}},
	}}
	service := NewWithAPI(api, 2)

	if _, hit, err := service.GetDependencies("npm", "react", "18.2.0"); err != nil || hit {
		t.Fatalf("GetDependencies error = %v, hit = %v; want nil and miss", err, hit)
	}
	graph, hit, err := service.GenerateDependencyGraph("npm", "react", "18.2.0")
	if err != nil {
		t.Fatalf("GenerateDependencyGraph: %v", err)
	}
	if !hit {
		t.Fatal("GenerateDependencyGraph did not reuse dependency cache")
	}
	if graph != "digraph {\n  0 [label=\"react@18.2.0\"];\n}\n" {
		t.Fatalf("unexpected graph:\n%s", graph)
	}
	if api.dependencyCalls != 1 {
		t.Fatalf("GetDependencies upstream calls = %d; want 1", api.dependencyCalls)
	}
}

func TestFindDependencyPathDirectAndCacheReuse(t *testing.T) {
	api := &fakeAPI{dependencyResults: definition.Dependencies{
		Nodes: []definition.Node{
			{
				VersionKey: definition.VersionKey{System: "PYPI", Name: "botocore", Version: "1.34.100"},
				Relation:   "SELF",
			},
			{
				VersionKey: definition.VersionKey{System: "PYPI", Name: "urllib3", Version: "2.7.0"},
				Relation:   "DIRECT",
			},
		},
		Edges: []definition.Edge{{FromNode: 0, ToNode: 1, Requirement: ">=1.25.4,<1.27"}},
	}}
	service := NewWithAPI(api, 2)

	result, hit, err := service.FindDependencyPath("pypi", "botocore", "1.34.100", "UrLlIb3")
	if err != nil {
		t.Fatalf("FindDependencyPath: %v", err)
	}
	if hit {
		t.Fatal("first FindDependencyPath unexpectedly hit cache")
	}
	if !result.PullsIn {
		t.Fatal("FindDependencyPath PullsIn = false; want true")
	}
	wantVersions := []DependencyPathVersion{{Name: "urllib3", Version: "2.7.0", Relation: "DIRECT"}}
	if len(result.Versions) != 1 || result.Versions[0] != wantVersions[0] {
		t.Fatalf("FindDependencyPath versions = %+v; want %+v", result.Versions, wantVersions)
	}
	wantPath := "urllib3@2.7.0 <-(>=1.25.4,<1.27) botocore@1.34.100"
	if len(result.Paths) != 1 || result.Paths[0] != wantPath {
		t.Fatalf("FindDependencyPath paths = %q; want [%q]", result.Paths, wantPath)
	}

	if _, hit, err := service.FindDependencyPath("PYPI", "botocore", "1.34.100", "urllib3"); err != nil || !hit {
		t.Fatalf("second FindDependencyPath error = %v, hit = %v; want nil and hit", err, hit)
	}
	if api.dependencyCalls != 1 {
		t.Fatalf("GetDependencies upstream calls = %d; want 1", api.dependencyCalls)
	}
}

func TestFindDependencyPathTransitive(t *testing.T) {
	api := &fakeAPI{dependencyResults: definition.Dependencies{
		Nodes: []definition.Node{
			{VersionKey: definition.VersionKey{Name: "express", Version: "4.18.2"}, Relation: "SELF"},
			{VersionKey: definition.VersionKey{Name: "debug", Version: "2.6.9"}, Relation: "DIRECT"},
			{VersionKey: definition.VersionKey{Name: "ms", Version: "2.0.0"}, Relation: "INDIRECT"},
		},
		Edges: []definition.Edge{
			{FromNode: 0, ToNode: 1, Requirement: "2.6.9"},
			{FromNode: 1, ToNode: 2, Requirement: "2.0.0"},
		},
	}}
	service := NewWithAPI(api, 2)

	result, _, err := service.FindDependencyPath("npm", "express", "4.18.2", "ms")
	if err != nil {
		t.Fatalf("FindDependencyPath: %v", err)
	}
	want := "ms@2.0.0 <-(2.0.0) debug@2.6.9 <-(2.6.9) express@4.18.2"
	if len(result.Paths) != 1 || result.Paths[0] != want {
		t.Fatalf("FindDependencyPath paths = %q; want [%q]", result.Paths, want)
	}
}

func TestFindDependencyPathAbsentIsSuccessful(t *testing.T) {
	api := &fakeAPI{dependencyResults: definition.Dependencies{Nodes: []definition.Node{{
		VersionKey: definition.VersionKey{Name: "jmespath", Version: "1.0.1"},
		Relation:   "SELF",
	}}}}
	service := NewWithAPI(api, 2)

	result, _, err := service.FindDependencyPath("pypi", "jmespath", "1.0.1", "urllib3")
	if err != nil {
		t.Fatalf("FindDependencyPath: %v", err)
	}
	if result.PullsIn || len(result.Versions) != 0 || len(result.Paths) != 0 || result.Note == "" {
		t.Fatalf("unexpected absent result: %+v", result)
	}
}

func TestFindDependencyPathSurfacesUpstreamErrors(t *testing.T) {
	t.Run("request error", func(t *testing.T) {
		service := NewWithAPI(&fakeAPI{dependencyErr: errors.New("version not found")}, 2)
		if _, _, err := service.FindDependencyPath("pypi", "botocore", "99.99.99", "urllib3"); err == nil || !strings.Contains(err.Error(), "version not found") {
			t.Fatalf("FindDependencyPath error = %v; want upstream not-found error", err)
		}
	})

	t.Run("graph error", func(t *testing.T) {
		service := NewWithAPI(&fakeAPI{dependencyResults: definition.Dependencies{Error: "package version not found"}}, 2)
		if _, _, err := service.FindDependencyPath("pypi", "botocore", "99.99.99", "urllib3"); err == nil || !strings.Contains(err.Error(), "package version not found") {
			t.Fatalf("FindDependencyPath error = %v; want graph error", err)
		}
	})
}

func TestFindDependencyPathCycleIsBounded(t *testing.T) {
	api := &fakeAPI{dependencyResults: definition.Dependencies{
		Nodes: []definition.Node{
			{VersionKey: definition.VersionKey{Name: "a", Version: "1.0.0"}, Relation: "INDIRECT"},
			{VersionKey: definition.VersionKey{Name: "b", Version: "1.0.0"}, Relation: "INDIRECT"},
		},
		Edges: []definition.Edge{
			{FromNode: 0, ToNode: 1, Requirement: "b-range"},
			{FromNode: 1, ToNode: 0, Requirement: "a-range"},
		},
	}}
	service := NewWithAPI(api, 2)

	result, _, err := service.FindDependencyPath("npm", "root", "1.0.0", "b")
	if err != nil {
		t.Fatalf("FindDependencyPath: %v", err)
	}
	if len(result.Paths) != 1 || strings.Count(result.Paths[0], " <-(") != maxDependencyPathDepth {
		t.Fatalf("cycle path hop count = %d; want %d", strings.Count(result.Paths[0], " <-("), maxDependencyPathDepth)
	}
	if !strings.Contains(result.Note, "truncated") {
		t.Fatalf("cycle note = %q; want truncation explanation", result.Note)
	}
}
