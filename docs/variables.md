# Variables and environments

Variables keep hosts, tokens and other values that change out of your requests.

## Using a variable

Write `{{name}}` anywhere in:

- the URL
- header names and values
- authentication fields
- raw bodies, form fields, GraphQL queries and variables
- gRPC metadata and messages
- proto import roots and files
- expected values in [tests](tests.md)

When the request is sent, each `{{name}}` is replaced with the variable's value. A name with no value is left as written, which usually makes the problem obvious in the error or the response.

## Environments

An environment is a named set of variables, such as `Local`, `Staging` and `Production`. Choose the active one in the menu at the top right. Requests stay the same; only the values change.

Open **Environments** to create, rename and delete environments and edit their variables. Each variable has:

| Control | Effect |
|---|---|
| Checkbox | Turn the variable off without deleting it |
| Lock | Mask the value on screen, for use when sharing your screen |

Changes are saved when you close the dialog.

## Where environments are stored

Environments are stored **on your machine only**. They are never written to the workspace and never committed, so tokens and passwords cannot end up in your repository by accident.

| System | Location |
|---|---|
| macOS | `~/Library/Application Support/roundtrip` |
| Windows | `%AppData%\roundtrip` |
| Linux | `~/.config/roundtrip` |

Environments are kept per workspace. Use `-data DIR` to keep this data somewhere else.

Because environments are local, teammates do not receive them. A common approach is to agree on variable names (`baseUrl`, `token`) in your repository's own documentation and let each person fill in their values.

## Captured values

A request can save part of its response as a variable for later requests, for example a token returned by a login call. See [Tests and chaining](tests.md#saving-values-for-later-requests).

Captured values:

- take priority over an environment variable with the same name
- are listed under **Captured this session** in the Environments dialog, where you can clear them
- are held in memory only and disappear when Roundtrip closes or you switch workspace

## Variables in the CLI

`roundtrip run` reads variables from several places. See [CLI](cli.md#variables).
