<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/brand/mono/roundtrip-horizontal-white.svg">
    <img src="assets/brand/mono/roundtrip-horizontal-black.svg" alt="Roundtrip" width="300">
  </picture>
</p>

<p align="center">
  A fast, local API client for HTTP, GraphQL and gRPC.<br>
  Your requests are plain files in a git repository you own.
</p>

<p align="center">
  <a href="docs/getting-started.md">Getting started</a> ·
  <a href="docs/README.md">Documentation</a> ·
  <a href="docs/cli.md">CLI</a> ·
  <a href="docs/development.md">Contributing</a>
</p>

<p align="center">
  <img src="docs/images/app.jpg" alt="Roundtrip showing a collection tree, a request and its JSON response" width="900">
</p>

## Why Roundtrip

- **Your requests live in git.** Every request is one readable YAML file. Review changes in a pull request, branch them with your code, and roll them back like anything else.
- **No account, no cloud.** Roundtrip runs entirely on your machine. Nothing is uploaded anywhere unless you push it to your own repository.
- **Secrets stay local.** Environments and their values are stored on your machine only and are never written to the repository.
- **One tool for three protocols.** HTTP, GraphQL and gRPC (with `.proto` files or server reflection) share the same collections, variables and tests.
- **Runs in CI.** The same binary runs your collections from the terminal and exits non-zero when a check fails.
- **Small and quick.** A single Go binary with the interface built in. No runtime to install.

## Features

| | |
|---|---|
| **Requests** | Methods, query parameters, headers, JSON / text / form bodies, Bearer, Basic and API-key auth |
| **GraphQL** | Query and variables editors |
| **gRPC** | Unary calls using local `.proto` files with import roots, or server reflection |
| **Collections** | Nested folders, tabs, history, cURL import and export |
| **Variables** | `{{name}}` in URLs, headers, auth, bodies and proto paths; multiple environments |
| **Tests** | Declarative checks on status, headers, body fields and timing |
| **Chaining** | Save a value from one response and use it in the next request |
| **Git sync** | Connect a repository, review changes, commit, push and pull from the app |
| **CLI** | `roundtrip run` for terminals and CI pipelines |
| **Desktop** | Native window on macOS; runs in any browser on macOS, Windows and Linux |

## Quick start

Roundtrip is in early development and does not have packaged releases yet. Build it from source:

```sh
git clone https://github.com/kodato-is/roundtrip.git
cd roundtrip
make build
```

You need [Go](https://go.dev/dl/) 1.26 or newer, [Node.js](https://nodejs.org/) 22 or newer with [pnpm](https://pnpm.io/), and git.

Start it on a folder for your requests:

```sh
./roundtrip ~/api-requests
```

Roundtrip opens in your browser. Create a collection, add a request and press <kbd>⌘</kbd><kbd>↵</kbd> (<kbd>Ctrl</kbd><kbd>Enter</kbd> on Windows and Linux) to send it. To share your requests, choose **Connect GitHub** and paste a repository address.

On macOS you can build the native app instead:

```sh
make desktop
open dist/Roundtrip.app
```

See [Getting started](docs/getting-started.md) for a full walkthrough.

## What a request looks like on disk

```yaml
id: 5f0c1c1e9a3b4d21
name: Log in
method: POST
url: '{{baseUrl}}/login'
body:
  mode: raw
  raw: |-
    {
      "email": "ada@example.com",
      "password": "{{password}}"
    }
  contentType: application/json
tests:
  - target: status
    op: eq
    value: "200"
extract:
  - var: token
    from: body.access_token
```

Folders are directories and each request is one file, so a change to a request is a small, readable diff. The full layout is described in [File format](docs/file-format.md).

## Run collections from the terminal

```console
$ roundtrip run -workspace ./api-requests -env-file ci.yaml "Users API"
PASS  POST    Users API / Auth / Log in      200 OK  41ms
PASS  GET     Users API / List users         200 OK  18ms
FAIL  DELETE  Users API / Admin / Delete user  403 Forbidden  12ms
      status eq 204: got 403

3 requests: 2 passed, 1 failed in 71ms
```

The exit status is `0` when everything passed and `1` when a request failed, so it drops straight into a CI job. See [CLI](docs/cli.md).

## Documentation

| Guide | What it covers |
|---|---|
| [Getting started](docs/getting-started.md) | Install, first request, first collection |
| [Requests](docs/requests.md) | HTTP, GraphQL, auth, bodies, cURL |
| [gRPC](docs/grpc.md) | Proto files, import roots, reflection |
| [Variables and environments](docs/variables.md) | `{{variables}}`, environments, where values are stored |
| [Tests and chaining](docs/tests.md) | Checks on responses, saving values for later requests |
| [Git sync](docs/git-sync.md) | Connecting a repository, committing, pulling, conflicts |
| [CLI](docs/cli.md) | `roundtrip run`, CI usage, exit codes |
| [File format](docs/file-format.md) | Workspace layout and the request file reference |
| [Development](docs/development.md) | Architecture, building, testing, contributing |

## Status

Roundtrip is pre-release software. It is usable day to day, but file formats and commands may still change, and the Windows build has not been tested as thoroughly as macOS. Bug reports and feedback are welcome in [issues](https://github.com/kodato-is/roundtrip/issues).

## Contributing

Contributions are welcome. [Development](docs/development.md) explains how the project is laid out and how to build and test it. For anything larger than a small fix, please open an issue first so the approach can be agreed before you invest time in it.
