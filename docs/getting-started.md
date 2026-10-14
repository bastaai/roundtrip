# Getting started

This guide takes you from nothing to a saved, shareable request.

## 1. Build Roundtrip

Roundtrip does not have packaged releases yet, so you build it from source. You need:

- [Go](https://go.dev/dl/) 1.26 or newer
- [Node.js](https://nodejs.org/) 22 or newer and [pnpm](https://pnpm.io/)
- git (also used at runtime for syncing with a repository)

```sh
git clone https://github.com/kodato-is/roundtrip.git
cd roundtrip
make build
```

This produces a single `roundtrip` binary (`roundtrip.exe` on Windows) with the interface built in. Put it somewhere on your `PATH` if you like.

## 2. Start it

Give Roundtrip a folder to keep your requests in. This folder is your **workspace**.

```sh
roundtrip ~/api-requests
```

Roundtrip starts a local server and opens the interface in your browser. If you run it again without a folder, it reopens the workspace you used last.

| Flag | Meaning |
|---|---|
| `-addr 127.0.0.1:8787` | Address to listen on |
| `-no-open` | Do not open the browser |
| `-data DIR` | Where per-machine data is kept (see [Variables](variables.md#where-environments-are-stored)) |

Roundtrip only accepts connections from your own machine.

### Native app on macOS

```sh
make desktop
open dist/Roundtrip.app
```

The native app is the same interface in its own window. The first time it runs it uses `~/Roundtrip` as its workspace.

## 3. Send a request

1. Type a URL in the bar at the top, for example `https://api.github.com/zen`.
2. Press <kbd>⌘</kbd><kbd>↵</kbd> on macOS or <kbd>Ctrl</kbd><kbd>Enter</kbd> on Windows and Linux.

The response appears below with its status, time and size. JSON is formatted automatically.

## 4. Save it in a collection

Press <kbd>⌘</kbd><kbd>S</kbd> / <kbd>Ctrl</kbd><kbd>S</kbd>, give the request a name and choose **New collection**.

A collection is a folder in your workspace. You can nest folders inside it, and every request is saved as one YAML file:

```
api-requests/
  collections/
    GitHub/
      _collection.yaml
      Zen.yaml
```

## 5. Use a variable

Hard-coded hosts and tokens make requests hard to share. Open **Environments**, create one called `Local`, and add a variable:

| Variable | Value |
|---|---|
| `baseUrl` | `https://api.github.com` |

Select `Local` in the environment menu and change the request URL to `{{baseUrl}}/zen`. Add a second environment with a different `baseUrl` to switch targets without touching the request.

Environments are stored on your machine only. See [Variables and environments](variables.md).

## 6. Share your requests

Choose **Connect GitHub** in the header and paste the address of a repository. Roundtrip clones it, moves your collections into it, and from then on shows how many changes are waiting to be committed. See [Git sync](git-sync.md).

## Keyboard shortcuts

| Action | macOS | Windows / Linux |
|---|---|---|
| Send request | <kbd>⌘</kbd><kbd>↵</kbd> | <kbd>Ctrl</kbd><kbd>Enter</kbd> |
| Save request | <kbd>⌘</kbd><kbd>S</kbd> | <kbd>Ctrl</kbd><kbd>S</kbd> |
| Rename the selected item | <kbd>F2</kbd> | <kbd>F2</kbd> |
| Move in the collection tree | Arrow keys | Arrow keys |
| Close a dialog | <kbd>Esc</kbd> | <kbd>Esc</kbd> |

## Next steps

- [Requests](requests.md) for bodies, authentication and GraphQL
- [gRPC](grpc.md) for proto files and import roots
- [Tests and chaining](tests.md) to check responses automatically
- [CLI](cli.md) to run collections in CI
