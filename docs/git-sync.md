# Git sync

Roundtrip keeps your collections in an ordinary git repository. You can use git yourself, or let Roundtrip commit, push and pull from the interface.

Roundtrip runs the `git` installed on your machine, so it uses the SSH keys or credential helper you already have. There is no separate sign-in.

## Connecting a repository

Choose **Connect GitHub** in the header (or the repository name, once connected) and paste an address:

```
git@github.com:your-org/api-requests.git
https://github.com/your-org/api-requests.git
```

Roundtrip clones the repository into its data folder and switches to it. If the repository has no collections yet and you already created some, they are copied in so you can commit them.

Any git host works; the address does not have to be on GitHub.

### Using a folder you already cloned

If you prefer to manage the clone yourself, enter its full path under **open a folder you already cloned**, or start Roundtrip with the folder as an argument:

```sh
roundtrip ~/code/api-requests
```

The workspace can also be a subfolder of a larger repository, for example `my-service/api-requests`.

## Committing and pushing

The header shows how many changes are waiting. Open the repository dialog to see the changed files, write a commit message and choose **Commit & push**.

Roundtrip then:

1. commits the changes under `collections/`
2. pulls and rebases onto the remote branch
3. pushes

Only files under `collections/` are ever staged. Other changes in the same repository are left exactly as they are, which makes it safe to keep requests next to your code.

## Pulling

Choose **Pull** in the repository dialog to fetch your teammates' changes. Roundtrip reads the workspace from disk each time, so pulling from a terminal works too; use the reload button at the top of the sidebar to refresh.

## Conflicts

If your changes and the remote's touch the same lines, the sync stops, your files are left as they were, and the error explains what happened. Resolve the conflict with git in the workspace folder, then sync again.

Because each request is its own file, conflicts only happen when two people edit the same request.

## What is and is not committed

| In the repository | On your machine only |
|---|---|
| Collections, folders and requests | Environments and their values |
| Tests and saved-value rules | History |
| Proto configuration of each collection | Values captured during a session |

See [File format](file-format.md) for the layout.

## Troubleshooting

| Message | Cause |
|---|---|
| `git is not installed` | Install git and restart Roundtrip |
| Authentication errors when cloning or pushing | Roundtrip cannot prompt for a password. Make sure `git clone` of the same address works in a terminal without asking, using an SSH key or a credential helper. |
| `Please tell me who you are` | Set your identity with `git config --global user.name` and `user.email` |
