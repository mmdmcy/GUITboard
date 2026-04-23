# GUITboard

GUITboard is a local-only cross-platform terminal dashboard (TUI) for managing a whole folder of Git repositories from one keyboard-first interface.

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
- Defaults to common Git roots such as `Documents/GitHub` on Windows and `~/Documents/github`, `~/Developer`, or `~/Code` on Unix-like systems.

## Requirements

- Windows 10 or 11, macOS, or Linux
- [Git](https://git-scm.com/)
- Go 1.25+ if you want to run or build from source

## Terminal Notes

- A terminal around `96x28` or larger gives the dashboard enough room to show the list, details, actions, and log together.
- No GUI toolkit or desktop-specific graphics packages are required anymore.

## Quick Start

Linux or macOS:

```bash
sh ./portui.sh --run run-dashboard
```

Windows PowerShell:

```powershell
.\portui.ps1 -Run run-dashboard
```

Command Prompt:

```cmd
portui.cmd --run run-dashboard
```

Direct Go entrypoint:

```bash
go run ./cmd/guitboard
```

## Optional Local Build

This is secondary. The normal portable workflow is to clone the repo and run the TUI through Go or the repo-local PortUI launchers.

If you want a local binary for the machine you are currently on:

Windows:

```powershell
if (-not (Test-Path dist)) { New-Item -ItemType Directory -Path dist | Out-Null }
go build -o dist/GUITboard.exe ./cmd/guitboard
```

Linux or macOS:

```bash
mkdir -p dist
go build -o dist/GUITboard ./cmd/guitboard
```

Then launch that local binary for your platform:

```powershell
.\dist\GUITboard.exe
```

```bash
./dist/GUITboard
```

## PortUI

GUITboard vendors the PortUI runtime into this repo, so PortUI acts as the repo-local launcher layer for this TUI without needing a separate checkout.

Linux or macOS:

```bash
sh ./portui.sh --list
sh ./portui.sh --run run-dashboard
sh ./portui.sh --run test
```

Windows PowerShell:

```powershell
.\portui.ps1 -List
.\portui.ps1 -Run run-dashboard
.\portui.ps1 -Run test
```

Command Prompt:

```cmd
portui.cmd --list
```

The bundled actions cover running the dashboard from source, running the Go test suite, and optionally building a local binary into `dist/`. The action definitions live in [`portui/`](./portui), while the vendored runtime lives in [`.portui-runtime/`](./.portui-runtime).

If you maintain both repos and want to refresh the vendored PortUI engine from the source `portui` repo:

```bash
sh ../portui/portui.sh --install-project .
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

- Local build artifacts like `guitboard`, `GUITboard`, `dist/`, and `*.exe` are ignored by Git.
- User-specific app settings are stored outside the repository.
- `portui.sh`, `portui.ps1`, `portui.cmd`, and `.portui-runtime/` are the vendored PortUI engine entrypoints for this repo.
