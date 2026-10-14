# Source control

Roundtrip saves your requests as plain YAML files in a folder you choose. Put that folder inside a git repository and your requests are versioned, reviewed and shared exactly like your code. There is no account to connect and nothing to sync with a service.

## Choosing the folder

The folder Roundtrip saves to is called the **workspace**. Its name is shown in the header; select it to see the full path and to open a different folder.

Use any folder inside a repository. A common layout keeps the requests next to the service they belong to:

```
my-service/
  src/
  api-requests/          <- the workspace
    collections/
      Users API/
        _collection.yaml
        List users.yaml
```

You can open a workspace in three ways:

- In the workspace dialog, enter the full path under **Open another folder**, or use **Browse…** to pick the folder.
- Start Roundtrip with the folder as an argument: `roundtrip ~/code/my-service/api-requests`.
- Start Roundtrip without an argument to reopen the folder you used last.

The folder is created if it does not exist.

### Switching between workspaces

You can keep requests in several places, for example one folder per service. Roundtrip remembers the folders you have opened. They are listed under **Switch workspace** in the workspace dialog, and as *Switch to workspace …* actions in **Go to** (<kbd>⌘</kbd><kbd>K</kbd> / <kbd>Ctrl</kbd><kbd>K</kbd>); select one to switch to it. A folder that has been moved or deleted drops off the list. Each workspace has its own collections, environments and history.

### Moving existing requests into a repository

If you started in one folder and want to continue in another, tick **Copy my current collections into it** before opening the new folder. The copy only happens when the new folder has no collections yet, so existing work is never overwritten. Your environments follow on your machine.

## Committing

Roundtrip detects when the workspace is inside a git repository. The header then shows the number of uncommitted changes, and the workspace dialog lists the changed files.

You can commit in whichever way you prefer:

- **With git**, as you would any other change:

  ```sh
  git add api-requests
  git commit -m "Add user requests"
  ```

- **From Roundtrip**: write a message in the workspace dialog and choose **Commit**. If the repository has a remote named `origin`, the button reads **Commit & push**: it commits, rebases onto the remote branch and pushes. **Pull** fetches your teammates' changes.

Committing from Roundtrip is a convenience and entirely optional. It only ever stages files under `collections/`; other changes in the repository are left exactly as they are. It runs the `git` installed on your machine, so it uses the credentials you already have.

Roundtrip reads the workspace from disk each time it needs it, so changes made outside the app, such as a `git pull` or `git checkout` in a terminal, are picked up. Use the reload button at the top of the sidebar to refresh the view.

## Conflicts

Each request is its own file, so two people only conflict when they edit the same request. Resolve the conflict with git as usual. If a file still contains conflict markers, Roundtrip reports it by path when the workspace loads.

When you commit from Roundtrip and the remote has conflicting changes, the operation stops, your files are left as they were, and the error explains what happened.

## What is and is not in the repository

| In the workspace folder | On your machine only |
|---|---|
| Collections, folders and requests | Environments and their values |
| Tests and saved-value rules | History |
| Proto configuration of each collection | Values captured during a session |

Secrets belong in environments, which never enter the workspace. See [File format](file-format.md) for the layout and [Variables and environments](variables.md#where-environments-are-stored) for where local data is kept.

## Troubleshooting

| Message | Cause |
|---|---|
| The dialog says the folder is not in a git repository | Open a folder inside a repository, or run `git init` in it |
| `git was not found on this machine` | Requests are still saved as files. Install git to see changes and commit from Roundtrip. |
| Authentication errors when pushing or pulling | Roundtrip cannot prompt for a password. Make sure `git push` works in a terminal without asking, using an SSH key or a credential helper. |
| `Please tell me who you are` | Set your identity with `git config --global user.name` and `user.email` |
