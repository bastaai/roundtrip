# File format

A workspace is a folder. Everything Roundtrip saves there lives under `collections/`, as YAML files you can read, edit by hand and review in a pull request.

## Layout

```
<workspace>/
  collections/
    Users API/                 a collection
      _collection.yaml         its id, name and proto configuration
      List users.yaml          a request
      Auth/                    a folder
        _folder.yaml           its id and name
        Log in.yaml
```

- A **collection** is a directory under `collections/`.
- A **folder** is a directory inside a collection. Folders nest.
- A **request** is a `.yaml` file.

File and directory names follow the names you give in the app. Characters that are not valid in file names on macOS or Windows are replaced with `-`. The real name is always stored inside the file.

Roundtrip only manages `.yaml` files and directories under `collections/`. Other files you place there, such as a `README.md`, are left alone. Files and directories whose names start with a dot are ignored.

Items are shown folders first, then requests, each sorted by name.

## `_collection.yaml`

```yaml
id: 9d2c6c0a51f84b17
name: Billing
proto:
  roots:
    - protos
    - '{{protoRoot}}/third_party'
  files:
    - billing/v1/payments.proto
```

| Field | Meaning |
|---|---|
| `id` | Stable identifier. Generated for you. |
| `name` | Display name |
| `proto.roots` | Import roots for gRPC. See [gRPC](grpc.md#using-proto-files). |
| `proto.files` | Proto files to load. Optional. |

`_folder.yaml` has `id` and `name` only.

## Request files

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
| `id` | Stable identifier. If you write a file by hand you can omit it. |
| `name` | Display name. Defaults to the file name. |
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
- A file that cannot be parsed, for example one containing merge-conflict markers, is reported by path when the workspace loads.

## What is not in the workspace

Environments, history and values captured during a session are stored per machine and never written here. See [Variables and environments](variables.md#where-environments-are-stored).
