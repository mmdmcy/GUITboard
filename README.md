# GUITboard

GUITboard is a local-only terminal dashboard for managing a whole folder of Git repositories from one keyboard-first interface.

## Features

- Scans a chosen root folder for nested Git repositories.
- Sorts repositories by most recent activity so the busiest projects stay at the top.
- Shows branch, upstream, ahead/behind, dirty state, last commit details, and an operation log in one split dashboard.
- Keeps every major workflow on the keyboard: arrow keys move around the dashboard and `Enter` runs the highlighted action.
- Lets you `Stage`, `Commit`, `Commit+Push`, `Pull`, or `Push` the selected repository from a compact popup-driven flow.
- Supports `Fetch All`, which fetches every repo with a remote and fast-forwards clean repos that have a usable upstream branch.
- Supports bulk commit-and-push across every changed repository with one shared commit message.
- Clones a repository into the current root from either a full Git URL or `owner/repo` shorthand.
- Refreshes automatically every 30 seconds and stores the selected root folder locally per user.
- Defaults to `Documents/GitHub` on Windows and `~/Documents/github` on Linux Mint.

## Requirements

- Windows 11 or Windows 10, or Linux Mint
- [Git](https://git-scm.com/)
- Go 1.25+ if you want to run or build from source

## Terminal Notes

- A terminal around `96x28` or larger gives the dashboard enough room to show the list, details, actions, and log together.
- No GUI toolkit or desktop-specific graphics packages are required anymore.

## Run from source

```bash
go run ./cmd/guitboard
```

## Build an executable

Windows:

```powershell
go build -o GUITboard.exe ./cmd/guitboard
```

Linux Mint:

```bash
go build -o GUITboard ./cmd/guitboard
```

Then launch the binary for your platform:

```powershell
.\GUITboard.exe
```

```bash
./GUITboard
```

## How it works

1. Start the app.
2. Confirm the detected root folder or use the `Root` action to point GUITboard at another directory.
3. Use the arrow keys to move between dashboard actions, the repository list, and the selected-repo action bar.
4. Press `Enter` on `Commit+Push` or `Commit Dirty`, type the commit message in the popup, and GUITboard stages and runs the rest.
5. Use `Fetch All` to refresh the whole folder, `Clone` to add another repo into the root, `/` to filter, `d` to toggle dirty-only mode, and `r` to refresh on demand.

The app remembers the last folder you picked in your user config directory, so it works on any machine without hardcoded personal paths.

## Local-only behavior

- GUITboard does not use a backend or cloud service.
- It only runs Git commands on your local machine.
- Push and pull use the Git credentials already configured on that computer.

## Repository notes

- Build artifacts like `GUITboard` and `GUITboard.exe` are ignored by Git.
- User-specific app settings are stored outside the repository.
