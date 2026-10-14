# Changelog

All notable changes to Roundtrip are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project will follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) from its first release.

## [Unreleased]

The first public version, under active development.

### Requests

- HTTP requests with query parameters, headers, raw, URL-encoded and multipart bodies.
- Bearer, Basic and API-key authentication helpers.
- GraphQL requests with query and variables editors.
- Unary gRPC calls using local `.proto` files with import roots, or server reflection.
- Method suggestions and generated example request messages for gRPC.
- cURL import and export.

### Workspaces and files

- Requests are stored as readable YAML files, one per request, in a folder you choose.
- Git status, optional commit, push and pull for workspaces inside a repository.
- Switching between recently used workspaces.
- In-app browser for picking workspace folders, proto roots and proto files.

### Variables and tests

- Environments stored on the local machine only, with masked values.
- Declarative tests on status, headers, body fields and timing.
- Values captured from responses for use in later requests.

### Interface

- Side-by-side request and response layout with a full-width address bar.
- Quick open (Cmd/Ctrl+K), collapsible and filterable sidebar, tabs and history.
- Native desktop app for macOS.

### Command line

- `roundtrip run` runs collections from a terminal or CI pipeline and exits non-zero on failure.
