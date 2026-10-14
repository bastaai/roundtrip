# Security policy

## Reporting a vulnerability

Please do not report security vulnerabilities in public issues, discussions or pull requests.

Report them privately through GitHub instead: open the [Security tab](https://github.com/bastaai/roundtrip/security) and choose **Report a vulnerability**. Include:

- a description of the problem and its impact
- steps to reproduce it, or a proof of concept
- the version or commit you tested

We will acknowledge your report as soon as we can, keep you informed while we work on a fix, and credit you in the release notes unless you would rather stay anonymous.

## Supported versions

Roundtrip is pre-release software. Security fixes are made on the `main` branch and included in the next release.

## Security model

Knowing what Roundtrip is designed to protect helps in judging whether something is a vulnerability:

- **The local server only accepts connections addressed to the local machine** and rejects requests from other origins, so that web pages you visit cannot use it to send requests into your network. A way around this check is a vulnerability.
- **Environments and their values never leave your machine.** They are stored in your user data folder and are never written to the workspace. Anything that writes them into a workspace or sends them elsewhere is a vulnerability.
- **Request files are saved exactly as written.** A credential typed directly into a request ends up in your repository; that is expected behaviour, which is why the documentation recommends environment variables for secrets.
- **Roundtrip sends the requests you tell it to send**, including to your local network. That is its purpose, not a vulnerability.
