# depsdevmcp

An MCP server for [deps.dev](https://deps.dev), built in Go with the
[`github.com/edoardottt/depsdev`](https://github.com/edoardottt/depsdev) stable
v3 client module.

The server communicates over stdio, supports MCP protocol version `2026-07-28`,
and returns both structured JSON and a JSON text fallback for every tool result.

## Tools

| Tool | Purpose |
| --- | --- |
| `depsdev_get_package` | Package metadata and known versions |
| `depsdev_get_version` | Exact version metadata, licenses, and advisories |
| `depsdev_get_dependencies` | Resolved dependency graph |
| `depsdev_get_requirements` | Declared dependency requirements |
| `depsdev_get_project` | GitHub, GitLab, or Bitbucket project metadata |
| `depsdev_get_project_package_versions` | Project-to-package-version mappings |
| `depsdev_get_advisory` | OSV advisory details |
| `depsdev_query` | Version-key or content-hash query |
| `depsdev_generate_dependency_graph` | Graphviz DOT dependency graph |

All tools are read-only and are annotated as such in their MCP definitions.

## Cache

Successful deps.dev responses are stored in a concurrency-safe, process-local
LRU cache. The default capacity is 256 entries. Set it with either the command
line flag or environment variable:

```sh
depsdevmcp -cache-capacity 512
DEPSDEVMCP_CACHE_CAPACITY=512 depsdevmcp
```

Each result contains a `cached` boolean. Errors are never cached. The cache has
no TTL and is cleared when the process exits.

## Run locally

Go 1.25 or newer is required because the MCP SDK requires it.

```sh
go build -o depsdevmcp ./cmd/depsdevmcp
./depsdevmcp
```

Example MCP client configuration:

```json
{
  "mcpServers": {
    "deps.dev": {
      "command": "/absolute/path/to/depsdevmcp",
      "args": ["-cache-capacity", "256"]
    }
  }
}
```

## Run with Docker

```sh
docker build -t depsdevmcp .
docker run --rm -i depsdevmcp
```

Example MCP client configuration using the published GHCR image:

```json
{
  "mcpServers": {
    "deps.dev": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "ghcr.io/mappedsky/depsdevmcp:main"
      ]
    }
  }
}
```

The GitHub workflow builds `linux/amd64` and `linux/arm64` images. Pushes to
`main` and `v*` tags publish to `ghcr.io/<owner>/<repository>`; pull requests
only build the image. All workflow actions are pinned to immutable commit SHAs.

## Query examples

Package version query:

```json
{
  "query": "versionKey.system=NPM&versionKey.name=react&versionKey.version=18.2.0"
}
```

Artifact hash query:

```json
{
  "query": "hash.type=SHA1&hash.value=ulXBPXrC%2FUTfnMgHRFVxmjPzdbk%3D"
}
```
