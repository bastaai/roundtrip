# Tests and chaining

Tests are checks that run every time a request is sent, in the app and in the [CLI](cli.md). They are declarative: you choose what to look at, how to compare it, and the expected value. There is no scripting language to learn.

## Adding a test

Open the **Tests** tab of a request and fill in a row:

| Target | Check | Expected |
|---|---|---|
| `status` | equals | `200` |
| `header.Content-Type` | contains | `json` |
| `body.user.id` | exists | |
| `time` | less than | `500` |

After sending, the response panel gains a **Tests** tab showing each check and, for a failed one, what was found instead.

## Targets

| Target | Meaning |
|---|---|
| `status` | HTTP status code, or the gRPC status code (`0` is OK) |
| `time` | Response time in milliseconds |
| `header.<Name>` | A response header (or gRPC metadata key). Not case-sensitive. |
| `body` | The whole response body as text |
| `body.<path>` | A value inside a JSON body |

Paths into JSON use dots and array indexes:

```
body.user.name
body.data.items[0].id
body[0].id
```

## Checks

| Check | In files | Passes when |
|---|---|---|
| equals | `eq` | The value equals the expected value |
| does not equal | `ne` | The value differs from the expected value |
| contains | `contains` | The value contains the expected text |
| exists | `exists` | The target is present. No expected value. |
| less than | `lt` | The value is a smaller number |
| greater than | `gt` | The value is a larger number |
| matches regex | `matches` | The value matches the regular expression |

Details worth knowing:

- When both sides are numbers they are compared as numbers, so `10.50` equals `10.5`.
- A target that is missing fails every check except `exists`.
- A JSON `null` counts as present.
- Regular expressions are not anchored; use `^` and `$` for a full match.
- Expected values may contain [variables](variables.md).

## Saving values for later requests

The lower table in the **Tests** tab saves part of a response as a variable.

| Variable | From |
|---|---|
| `token` | `body.access_token` |
| `location` | `header.Location` |

**From** accepts the same targets as tests. After the request is sent, `{{token}}` is available to every request you send afterwards.

A typical flow:

1. **Log in** saves `token` from `body.access_token`.
2. **List users** sends `Authorization: Bearer {{token}}`.

If a value cannot be found in the response, the request is marked as failed. This surfaces a broken login immediately instead of as a string of `401` responses later on.

Saved values are kept in memory only. See [Captured values](variables.md#captured-values).

## Requests without tests

A request with no tests is still judged when run from the CLI: it passes when it receives a response with a status below 400 (or gRPC status `0`).
