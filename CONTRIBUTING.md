# Contributing to Roundtrip

Thank you for helping make Roundtrip better. Contributions of every size are welcome: bug reports, documentation fixes, small improvements and new features.

By participating you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Ways to contribute

- **Report a bug.** Open an [issue](https://github.com/bastaai/roundtrip/issues/new/choose) with the steps to reproduce it. A request file that shows the problem, with credentials removed, is the most useful thing you can include.
- **Suggest a feature.** Open a feature request describing the problem you want solved. Starting from the problem, not the solution, leads to better designs.
- **Improve the documentation.** The guides live in [`docs/`](docs). Clarifications and corrections are always appreciated.
- **Write code.** Pick an issue labelled `good first issue` or `help wanted`, or propose something new.

For anything larger than a small fix, please open an issue first so the approach can be agreed before you invest time in it.

## Development setup

You need Go 1.26 or newer, Node.js 22 or newer with pnpm, and git.

```sh
git clone https://github.com/bastaai/roundtrip.git
cd roundtrip
make build     # builds the interface and the roundtrip binary
make test      # runs the Go and interface tests
make dev       # API on :8787 and the Vite dev server on :5173 with hot reload
```

[Development](docs/development.md) describes the architecture, how the code is organised, and the design rules it follows. Please read it before making larger changes.

Use `-data` and a scratch folder when running the binary during development, so your own environments and history are never touched:

```sh
./roundtrip -no-open -addr 127.0.0.1:8899 -data /tmp/rt-data /tmp/rt-workspace
```

## Making a change

1. Fork the repository and create a branch from `main`.
2. Make your change, with tests. Go tests run against real servers (`httptest`, in-process gRPC); interface logic that can be a pure function lives in `web/src/lib` with a Vitest test beside it.
3. Run the checks:

   ```sh
   make test
   gofmt -l .                              # should print nothing
   go vet ./...
   GOOS=windows go build -o /dev/null .    # keeps Windows building
   ```

4. Update `README.md` or `docs/` if the change affects behaviour users see.
5. Open a pull request and fill in the template.

## Guidelines

- **Keep changes focused.** One concern per pull request makes review faster.
- **Workspace files are user data.** Request files are committed to people's repositories. Keep the format minimal and stable, never rewrite unchanged files, and never write secrets into the workspace.
- **Cross-platform.** Roundtrip runs on macOS, Windows and Linux. Use `path/filepath`, avoid shelling out (git is the only exception), and keep the main binary free of cgo.
- **Match the surrounding code.** Follow the existing naming, structure and comment style.
- **The interface is deliberately quiet.** Colour carries meaning (status, method, errors), not decoration. Use the tokens in `web/src/index.css` and the primitives in `web/src/components/ui.tsx`.

## Commit messages

Write the subject in the imperative mood ("Add proto file browser", not "Added..."), keep it under about 72 characters, and use the body to explain what changed and why.

## Review

A maintainer will review your pull request, usually within a few working days. Expect questions and suggestions; they are part of getting the change right, not a judgement of the work. Once approved, a maintainer merges it.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
