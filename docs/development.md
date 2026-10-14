# Development

Thank you for your interest in contributing to Roundtrip.

## Architecture

Roundtrip is one Go binary. It serves a React interface from an embedded bundle, exposes a small JSON API to it on localhost, and sends the actual requests itself. The native macOS app is the same handler inside a system web view.

```
main.go                     starts the server, or dispatches `roundtrip run`
cmd/roundtrip-desktop/      native window (Wails) around the same handler
internal/
  model/        shared types; their JSON form is the API and file contract
  store/        the workspace on disk: YAML request files, local environments, history
  sender/       variable resolution, auth helpers, HTTP and GraphQL
  grpcclient/   unary gRPC through proto files or server reflection
  check/        tests and value extraction, evaluated against a response
  runner/       sends one request end to end; the `roundtrip run` command
  curl/         cURL parsing and export
  gitsync/      git status, pull, commit and push through the system git
  api/          the JSON API, the localhost-only guard, the embedded interface
web/            React, TypeScript, Vite and Tailwind CSS
  src/lib/          pure logic with unit tests
  src/components/   the interface
assets/brand/   logos
```

Design rules the code follows:

- **Workspace files are user data.** They are committed to people's repositories, so they stay minimal and stable. Unchanged content is never rewritten and no secret is ever written to the workspace.
- **The workspace is read from disk on every call.** A `git pull` or a manual edit is picked up without a restart.
- **Cross-platform.** Paths go through `path/filepath`, and nothing is shelled out except `git`. The main binary builds without cgo.
- **Local only.** The API accepts requests addressed to localhost and rejects cross-origin ones, so a web page cannot use it to reach your network.

## Prerequisites

- Go 1.26 or newer
- Node.js 22 or newer and pnpm
- git
- Xcode command line tools, for the macOS desktop app only

## Building

| Command | Result |
|---|---|
| `make build` | Builds the interface and the `roundtrip` binary |
| `make dist` | Cross-compiles binaries for macOS and Windows into `dist/` |
| `make desktop` | Builds `dist/Roundtrip.app` for macOS |
| `make test` | Runs the Go and interface tests |
| `make dev` | Runs the Go API on `:8787` and the Vite dev server on `:5173` with hot reload |

`make dev` uses `./workspace` as its workspace.

The interface bundle in `web/dist` is embedded into the binary, so it must exist before `go build`. `make build` takes care of the order.

### Working without touching your own data

Pass `-data` and a scratch workspace so that development runs never read or write your real environments or history:

```sh
./roundtrip -no-open -addr 127.0.0.1:8899 -data /tmp/rt-data /tmp/rt-workspace
```

### Desktop app on Windows

The desktop entry point cross-compiles from macOS and Linux:

```sh
GOOS=windows GOARCH=amd64 go build -tags desktop,production \
  -ldflags "-w -s -H windowsgui" -o dist/Roundtrip.exe ./cmd/roundtrip-desktop
```

It requires the WebView2 runtime, which ships with current versions of Windows. This build compiles but has not yet been tested on Windows; reports are welcome.

## Testing

```sh
make test
```

- Go tests exercise real servers: `net/http/httptest` for HTTP, in-process gRPC servers for gRPC, and local bare repositories for git. There are no mocks to keep in sync.
- Interface logic that can be a pure function lives in `web/src/lib` with a Vitest test beside it.

Before opening a pull request, please also run:

```sh
gofmt -l .          # should print nothing
go vet ./...
GOOS=windows go build -o /dev/null .
pnpm -C web build   # type-checks the interface
```

## Changing the request model

The types in `internal/model` define three things at once: the JSON API, the YAML files on disk, and the TypeScript types in `web/src/types.ts`. Change all of them together, keep new fields optional, and omit them when empty so existing files do not change.

## Pull requests

- Open an issue first for anything beyond a small fix, so the approach can be agreed.
- Keep changes focused; one concern per pull request.
- Add or update tests for behaviour you change.
- Match the style of the surrounding code.

## Reporting bugs

Please include your operating system, how you run Roundtrip (browser or desktop app), what you did, what you expected, and what happened. A request file that reproduces the problem, with any credentials removed, helps a great deal.
