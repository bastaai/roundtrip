# Getting started

This guide takes you from nothing to a saved, shareable request.

## 1. Build Roundtrip

Roundtrip does not have packaged releases yet, so you build it from source. You need:

- [Go](https://go.dev/dl/) 1.26 or newer
- [Node.js](https://nodejs.org/) 22 or newer and [pnpm](https://pnpm.io/)
- git (also used at runtime for syncing with a repository)

```sh
git clone https://github.com/bastaai/roundtrip.git
cd roundtrip
make build
```

This produces a single `roundtrip` binary (`roundtrip.exe` on Windows) with the interface built in. Put it somewhere on your `PATH` if you like.

## 2. Start it

Give Roundtrip a folder to keep your requests in. This folder is your **workspace**. Any folder works; one inside a git repository lets you version your requests with your code.

```sh
roundtrip ~/code/my-service/api-requests
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

The native app is the same interface in its own window. The first time it runs it uses `~/Roundtrip` as its workspace; select the folder name in the header to open another one.

## 3. Send a request

1. Type a URL in the bar at the top, for example `https://api.github.com/zen`.
2. Press <kbd>⌘</kbd><kbd>↵</kbd> on macOS or <kbd>Ctrl</kbd><kbd>Enter</kbd> on Windows and Linux.

The request goes out on the left and the response comes back on the right, headed by its status, time and size. JSON is formatted automatically. In a narrow window the response moves below the request; the button at the end of the address bar switches between the two arrangements.

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

The files are plain text, so they diff and review like code.

## 5. Use a variable

Hard-coded hosts and tokens make requests hard to share. Open **Environments**, create one called `Local`, and add a variable:

| Variable | Value |
|---|---|
| `baseUrl` | `https://api.github.com` |

Select `Local` in the environment menu and change the request URL to `{{baseUrl}}/zen`. Add a second environment with a different `baseUrl` to switch targets without touching the request.

Environments are stored on your machine only. See [Variables and environments](variables.md).

## 6. Commit your requests

If the workspace is inside a git repository, Roundtrip shows the number of uncommitted changes next to the folder name in the header. Commit them with git as you would any other file, or select the folder name and choose **Commit**.

Not in a repository yet? Select the folder name, enter a folder inside one of your repositories under **Open another folder**, and tick **Copy my current collections into it**. See [Source control](git-sync.md).

## Keyboard shortcuts

| Action | macOS | Windows / Linux |
|---|---|---|
| Send request | <kbd>⌘</kbd><kbd>↵</kbd> | <kbd>Ctrl</kbd><kbd>Enter</kbd> |
| Save request | <kbd>⌘</kbd><kbd>S</kbd> | <kbd>Ctrl</kbd><kbd>S</kbd> |
| Go to a request or run an action | <kbd>⌘</kbd><kbd>K</kbd> | <kbd>Ctrl</kbd><kbd>K</kbd> |
| Hide or show the sidebar | <kbd>⌘</kbd><kbd>B</kbd> | <kbd>Ctrl</kbd><kbd>B</kbd> |
| Rename the selected item | <kbd>F2</kbd> | <kbd>F2</kbd> |
| Move in the collection tree | Arrow keys | Arrow keys |
| Close a dialog | <kbd>Esc</kbd> | <kbd>Esc</kbd> |

## Finding your way around

- **Go to** (<kbd>⌘</kbd><kbd>K</kbd> / <kbd>Ctrl</kbd><kbd>K</kbd>) jumps to any request by name, or by the initials of its words: `lu` finds *List users*. It also runs common actions, such as switching environment or workspace.
- **Filter**, at the top of the sidebar, narrows the collection tree as you type.
- Hide the sidebar to give the request and response the whole window.

## Next steps

- [Requests](requests.md) for bodies, authentication and GraphQL
- [gRPC](grpc.md) for proto files and import roots
- [Tests and chaining](tests.md) to check responses automatically
- [CLI](cli.md) to run collections in CI
