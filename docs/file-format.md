# File format

A workspace is a folder. Everything Roundtrip saves there lives under `collections/`, as YAML files you can read, edit by hand and review in a pull request.

## Layout

```
<workspace>/
  collections/
    Users API.yaml             a collection, with its folders and requests
    Billing.yaml
```

- A **collection** is one `.yaml` file under `collections/`.
- A **folder** is an entry under `folders:` in that file. Folders nest.
- A **request** is an entry under `requests:`, in the collection or in a folder.

The file name follows the collection name you give in the app. Characters that are not valid in file names on macOS or Windows are replaced with `-`, and two collections whose names would give the same file name get `Name.yaml` and `Name 2.yaml`. The real name is always stored inside the file. Renaming a collection renames its file; deleting it removes the file.

Roundtrip only manages the `.yaml` files directly under `collections/`. Other files you place there, such as a `README.md`, are left alone. Files whose names start with a dot or `_` are ignored.

Folders and requests are written folders first, then requests, each sorted by name, which is also how they are shown. The order does not depend on how you arranged things in the app, and a collection that has not changed is never rewritten, so the file only shows up in a diff when its content changes.

## Collection files

```yaml
id: 9d2c6c0a51f84b17
name: Billing
proto:
  roots:
    - protos
    - '{{protoRoot}}/third_party'
  files:
    - billing/v1/payments.proto
folders:
  - id: 1b7e0c55d2a94f06
    name: Refunds
    requests:
      - id: 77a1f3c09e2b4d58
        name: Refund a charge
        method: POST
        url: '{{baseUrl}}/refunds'
requests:
  - id: 5f0c1c1e9a3b4d21
    name: List invoices
    method: GET
    url: '{{baseUrl}}/invoices'
```

| Field | Meaning |
|---|---|
| `id` | Stable identifier. Generated for you. |
| `name` | Display name. Defaults to the file name. |
| `proto.roots` | Import roots for gRPC. See [gRPC](grpc.md#using-proto-files). |
| `proto.files` | Proto files to load. Optional. |
| `folders` | Folders, each with `id`, `name` and its own `folders` and `requests` |
| `requests` | Requests, described below |

Empty lists are left out.

### Workspaces saved by earlier versions

Earlier versions stored a collection as a directory: `_collection.yaml` for its id, name and proto configuration, a directory with a `_folder.yaml` per folder, and one `.yaml` file per request. Such a directory is still read. The first time Roundtrip saves that collection it writes the single file and removes the old files (`_collection.yaml`, `_folder.yaml` and the request files) and any directories left empty. Anything else in the directory, such as a `README.md`, stays where it is. If a collection exists in both forms, the single file is used.

## Requests

Each entry under `requests:` holds one request. Shown here on its own; in the file it is indented under `requests:` and starts with `- `.

```yaml
id: 5f0c1c1e9a3b4d21
name: Create user
method: POST
url: '{{baseUrl}}/users'
headers:
  - key: Accept
    value: application/json
    enabled: true
auth:
  type: bearer
  token: '{{token}}'
body:
  mode: raw
  raw: |-
    {
      "name": "Ada",
      "role": "admin"
    }
  contentType: application/json
tests:
  - target: status
    op: eq
    value: "201"
  - target: body.id
    op: exists
extract:
  - var: userId
    from: body.id
docs: |-
  Creates a user and saves its id as {{userId}}.
  Needs an admin token.
```

Fields that are empty, or hold their default (no auth, no body, an API key sent as a header), are left out.

### Top level

| Field | Meaning |
|---|---|
| `id` | Stable identifier. If you write a file by hand you can omit it; one is derived from the file and the names leading to the request, and written the next time Roundtrip saves the collection. |
| `name` | Display name. Defaults to `Untitled`. |
| `protocol` | `grpc` for gRPC requests. Omitted for HTTP and GraphQL. |
| `method` | HTTP method, or `package.Service/Method` for gRPC |
| `url` | Request URL, or the server address for gRPC |
| `headers` | List of `key`, `value`, `enabled`. gRPC metadata for gRPC requests. |
| `auth` | See below |
| `body` | See below |
| `tests` | List of `target`, `op`, `value`. See [Tests](tests.md). |
| `extract` | List of `var`, `from`. See [Saving values](tests.md#saving-values-for-later-requests). |
| `docs` | Free text describing the request, edited in the **Docs** tab |

### `auth`

| `type` | Other fields |
|---|---|
| `none` | |
| `bearer` | `token` |
| `basic` | `username`, `password` |
| `apikey` | `key`, `value`, and `in`: `header` or `query` |

### `body`

| `mode` | Other fields |
|---|---|
| `none` | |
| `raw` | `raw` (the text) and `contentType` |
| `urlencoded` | `form`: list of `key`, `value`, `enabled` |
| `form` | `form`: list of `key`, `value`, `enabled`, sent as multipart |
| `graphql` | `raw` (the query) and `variables` (a JSON object as text) |

For gRPC requests the message is JSON text in `body.raw` with `mode: raw`.

### GraphQL and gRPC examples

```yaml
name: Invoices
method: POST
url: '{{baseUrl}}/graphql'
body:
  mode: graphql
  raw: |-
    query ($limit: Int!) {
      invoices(first: $limit) { id total }
    }
  variables: |-
    {
      "limit": 10
    }
```

```yaml
name: Charge
protocol: grpc
method: billing.v1.Payments/Charge
url: grpcs://billing.example.com:443
body:
  mode: raw
  raw: |-
    {
      "amount": { "currency": "EUR", "units": 12 }
    }
```

## Editing files by hand

Hand-written files work. A few YAML details to keep in mind:

- Quote values that YAML would read as something else, such as `"200"`, `"true"` or anything starting with `{`.
- Multi-line text is easiest as a block scalar (`|-`), as in the examples.
- Unknown fields are ignored and removed the next time Roundtrip saves the file.
- Folders and requests may be in any order; Roundtrip sorts them the next time it saves the file. A folder without a `name` is called `Untitled`, like a request.
- A file that cannot be parsed, for example one containing merge-conflict markers, is reported by path when the workspace loads.

## What is not in the workspace

Environments, history and values captured during a session are stored per machine and never written here. See [Variables and environments](variables.md#where-environments-are-stored).
