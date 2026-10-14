# gRPC

Roundtrip calls unary gRPC methods. It needs to know the shape of each method's messages, and it can learn that in two ways:

- **Proto files**: you point a collection at your `.proto` files. This works with any server.
- **Server reflection**: Roundtrip asks the server. This needs no setup, but the server must have gRPC reflection enabled.

A collection with proto files configured always uses them. A collection without uses reflection.

## Making a call

1. Choose **gRPC** in the protocol menu.
2. Enter the server address:

   | Address | Connection |
   |---|---|
   | `grpc://localhost:50051` or `localhost:50051` | Plaintext |
   | `grpcs://api.example.com:443` | TLS |

3. Enter the method as `package.Service/Method`, or choose **Load methods** to list what is available and pick one.
4. Write the request message as JSON in the **Message** tab. An empty message is `{}`.
5. Send.

The response shows the gRPC status, the response message as JSON, and the response metadata. A non-OK status such as `NotFound` is shown as a response with its message, not as a failure to send.

**Metadata** is the gRPC equivalent of headers and has its own tab. The **Auth** tab works as it does for HTTP: a bearer token is sent as `authorization` metadata.

## Using proto files

Proto files are configured per collection, so everyone who uses the collection gets the same setup through git.

Hover the collection in the sidebar and choose **Proto files for gRPC**. There are two lists:

| List | Meaning |
|---|---|
| **Import roots** | Folders that imports are resolved from. The equivalent of `-I` for `protoc`. |
| **Files** | Proto files to load, written relative to a root. Leave this empty to load every `.proto` file found under the roots. |

Choose **Check** to compile the configuration before saving. It reports the number of files, services and methods found, or the exact error.

### Paths

- A relative path is relative to the workspace folder.
- Absolute paths and paths starting with `~` are accepted.
- Paths may contain [variables](variables.md).

If your proto files live in a different place on each person's machine, commit a root such as `{{protoRoot}}` and let everyone set `protoRoot` in their own environment. Environments are local, so each person's path stays on their machine.

### Imports

The standard Google types under `google/protobuf/` are built in. Any other import, for example `google/api/annotations.proto`, must exist under one of your import roots. When an import cannot be found, the error names the import, the file that needs it, and the roots that were searched.

### Example

A repository laid out like this:

```
api-requests/
  collections/
    Billing/
  protos/
    billing/v1/payments.proto
    common/money.proto
```

needs a single import root, `protos`. With the files list left empty, both proto files are loaded, and `payments.proto` can `import "common/money.proto";`.

The configuration is saved in the collection:

```yaml
# collections/Billing/_collection.yaml
id: 9d2c6c0a51f84b17
name: Billing
proto:
  roots:
    - protos
```

## Using server reflection

If the collection has no proto configuration, **Load methods** and **Send** ask the server for its service definitions. If the server does not expose reflection, the error says so; configure proto files instead.

A request that has not been saved into a collection always uses reflection, because proto files are configured on collections.

## Limitations

- Only unary methods can be called. Streaming methods are listed but rejected when sent.
- Proto files are compiled each time you send, so edits and `git pull` take effect immediately. Very large proto trees may add a noticeable delay.
- A root that contains duplicate copies of the same definitions can fail with duplicate-symbol errors. List the files you need explicitly in that case.
