# Roundtrip documentation

Roundtrip is a local API client for HTTP, GraphQL and gRPC that stores your requests as plain files in a git repository.

## Start here

- [Getting started](getting-started.md): build Roundtrip, send your first request, save it in a collection.

## Using Roundtrip

- [Requests](requests.md): HTTP and GraphQL requests, authentication, bodies, cURL import and export.
- [gRPC](grpc.md): calling gRPC methods with `.proto` files or server reflection.
- [Variables and environments](variables.md): `{{variables}}` and switching between environments.
- [Tests and chaining](tests.md): checking responses and passing values between requests.
- [Source control](git-sync.md): keeping your requests in a git repository.

## Automation

- [CLI](cli.md): running collections from a terminal or CI pipeline.

## Reference

- [File format](file-format.md): the workspace layout and every field of a request file.
- [Development](development.md): architecture, building from source, testing and contributing.
