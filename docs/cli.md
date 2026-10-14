# CLI

`roundtrip run` sends the requests of a workspace one after another and checks their [tests](tests.md). It is the same binary as the app, so there is nothing else to install.

```sh
roundtrip run [flags] [path ...]
```

## Choosing what to run

Without a path, every collection runs. A path names a collection, folder or request by the names shown in the app, joined with `/`:

```sh
roundtrip run                                  # everything
roundtrip run "Users API"                      # one collection
roundtrip run "Users API/Admin"                # one folder
roundtrip run "Users API/Admin/Delete user"    # one request
```

Requests run in the order shown in the app: folders first, then requests, each sorted by name. Prefix names with numbers (`1 Log in`, `2 List users`) when the order matters.

Flags must come before paths.

## Flags

| Flag | Meaning |
|---|---|
| `-workspace DIR` | Workspace to run, the folder containing `collections/`. Defaults to the current directory. |
| `-env NAME` | Use an environment saved on this machine |
| `-env-file FILE` | Read variables from a YAML or JSON file |
| `-var name=value` | Set one variable. Repeatable. |
| `-bail` | Stop at the first request that fails |
| `-timeout DURATION` | Time allowed per request. Defaults to `1m`. |
| `-data DIR` | Where `-env` looks for environments. Defaults to the per-user data folder. |

## Variables

Variables are applied in this order, and later sources win:

1. `-env`
2. `-env-file`
3. `-var`
4. values saved by earlier requests in the run

Environments are stored per machine and are not in the repository, so `-env` only works where that environment exists. On CI, use `-env-file` and `-var`.

An environment file is a flat map of names to values:

```yaml
# ci.yaml
baseUrl: https://staging.example.com
tenant: acme
```

Keep secrets out of files and pass them from your CI system's secret store:

```sh
roundtrip run -env-file ci.yaml -var token="$API_TOKEN"
```

## Output

```console
$ roundtrip run -env-file ci.yaml "Users API"
PASS  POST    Users API / Auth / Log in        200 OK  41ms
PASS  GET     Users API / List users           200 OK  18ms
FAIL  DELETE  Users API / Admin / Delete user  403 Forbidden  12ms
      status eq 204: got 403

3 requests: 2 passed, 1 failed in 71ms
```

A request passes when it gets a response and all of its tests pass. A request without tests passes when the status is below 400 (gRPC: status `0`).

Colour is used only when the output is a terminal, and never when `NO_COLOR` is set.

## Exit status

| Status | Meaning |
|---|---|
| `0` | Every request passed |
| `1` | At least one request failed |
| `2` | The run could not start: bad flags, unknown path, unreadable workspace |

## gRPC

gRPC requests use the [proto files](grpc.md#using-proto-files) configured on their collection, resolved relative to the workspace, and server reflection when the collection has none.

## Example: GitHub Actions

```yaml
name: API checks
on: [push]

jobs:
  api:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - uses: pnpm/action-setup@v4
        with:
          version: 10
      - uses: actions/setup-node@v4
        with:
          node-version: 22

      - name: Build Roundtrip
        run: |
          git clone --depth 1 https://github.com/kodato-is/roundtrip.git /tmp/roundtrip
          make -C /tmp/roundtrip build

      - name: Run requests
        run: /tmp/roundtrip/roundtrip run -workspace api-requests -env-file api-requests/ci.yaml -var token="${{ secrets.API_TOKEN }}"
```

Building from source is needed only until packaged releases are available.

The CLI never writes history or any other per-machine data.
