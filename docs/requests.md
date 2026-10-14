# Requests

Every request has a protocol, chosen with the menu at the left of the URL bar: **HTTP**, **GraphQL** or **gRPC**. This page covers HTTP and GraphQL; gRPC has [its own page](grpc.md).

## HTTP

### Method and URL

Pick a method and enter a URL. If you leave out the scheme, `http://` is assumed. The URL may contain [variables](variables.md), for example `{{baseUrl}}/users/{{userId}}`.

### Query parameters

The **Params** tab is a table view of the query string. Editing either one updates the other. Values are sent exactly as you type them, so encode special characters yourself where the API requires it.

### Headers

Add headers in the **Headers** tab. Untick a row to keep it without sending it.

Roundtrip adds two headers when you have not set them yourself: a `Content-Type` that matches the body, and a `User-Agent`.

### Authentication

The **Auth** tab builds the right header for you:

| Type | What is sent |
|---|---|
| Bearer token | `Authorization: Bearer <token>` |
| Basic auth | `Authorization: Basic <base64 of user:password>` |
| API key | A header or a query parameter with the name and value you choose |

A header you add by hand with the same name takes priority over the auth helper.

> [!IMPORTANT]
> Requests are saved to your repository exactly as written. Do not type credentials into a request. Put them in an environment variable and reference it, for example `{{token}}`. Environments never leave your machine.

### Body

| Mode | Use it for |
|---|---|
| None | Requests without a body |
| Raw | JSON, plain text, XML or HTML. Choose the content type next to the editor. **Beautify** formats valid JSON. |
| URL-encoded | Classic HTML form fields, sent as `application/x-www-form-urlencoded` |
| Form data | `multipart/form-data` with text fields |

File uploads in form data are not supported yet.

## GraphQL

Choose **GraphQL** in the method menu at the start of the address bar. The **Query** tab has two editors:

- **Query**: the GraphQL document.
- **Variables**: a JSON object.

Roundtrip sends them as a standard `POST` with a JSON body of the form `{"query": "...", "variables": {...}}`. Headers, auth and variables work the same as for HTTP requests. If the variables are not valid JSON, the request is not sent and the error says so.

## Documenting a request

The **Docs** tab holds free text about the request: what it is for, what it needs, what comes back. It is saved in the request's file, so it is shared through git with the request.

## The response

| Part | Notes |
|---|---|
| Status, time, size | Shown at the top right of the response panel |
| Body | JSON is formatted and highlighted; switch between **Pretty** and **Raw**, or **Copy** it. In **Pretty**, the arrow beside an object or array folds it (Alt-click folds or unfolds everything inside), and **Collapse all** / **Expand all** fold everything below the top level |
| Headers | Every response header |
| Tests | Appears when the request has [tests](tests.md) |

Binary responses are not displayed. Responses larger than 50 MB are truncated.

A request that could not be sent at all (DNS failure, refused connection, an unresolved `{{variable}}` in the host) shows the error in place of a response.

## cURL

- **Import**: run *Import cURL* from **Go to** (<kbd>⌘</kbd><kbd>K</kbd> / <kbd>Ctrl</kbd><kbd>K</kbd>) and paste a command. It becomes a new request in the current collection. Most of what browser developer tools and API documentation produce is understood, including headers, data flags, `-u`, `-F`, `-G` and `--json`.
- **Export**: run *Copy as cURL* from **Go to** to copy the current request as a command. Variables are replaced with their current values, so the result may contain secrets.

## History

The **History** tab in the sidebar lists the last 200 requests you sent, newest first. Select one to open it again with the response it got; it is added to a collection only if you edit it.

Each saved request also keeps its own history. A request opens with the last response it received, even after a restart, and once it has been sent more than once the menu at the top right of the response lists its earlier responses with their time, status and duration. Choose one to see it again.

History, including the responses, is kept on your machine and is never committed. Response bodies over 1 MB are not kept. Responses can contain whatever the server sent back, including tokens; **Clear** in the History tab deletes them all.
