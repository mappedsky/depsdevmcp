package depsdev

import (
	"errors"
	"testing"

	definition "github.com/edoardottt/depsdev/pkg/depsdev/definitions"
)

type fakeAPI struct {
	packageCalls      int
	dependencyCalls   int
	packageErr        error
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
	return f.dependencyResults, nil
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
