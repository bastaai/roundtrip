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
- Documentation for each request, saved in its file.
- Foldable JSON responses.
- History for each request: it opens with its last response, and earlier responses can be shown again.
- cURL import and export.

### Workspaces and files

- Requests are stored as readable YAML, one file per collection with its folders and requests, in a folder you choose. Workspaces saved as a directory per collection and a file per request are still read and move to the single file the next time each collection is saved.
- Git status, optional commit, push and pull for workspaces inside a repository.
- Switching between recently used workspaces.
- In-app browser for picking workspace folders, proto roots and proto files.

### Variables and tests

- Environments stored on the local machine only, with masked values.
- Declarative tests on status, headers, body fields and timing.
- Values captured from responses for use in later requests.

### Interface

- Side-by-side request and response layout with a full-width address bar.
- Requests are saved as you type; one request is shown at a time, headed by its name.
- Drag and drop requests and folders between folders and collections.
- Quick open (Cmd/Ctrl+K), collapsible and filterable sidebar, tabs and history.
- Native desktop app for macOS.

### Command line

- `roundtrip run` runs collections from a terminal or CI pipeline and exits non-zero on failure.
