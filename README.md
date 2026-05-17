# GUITboard

GUITboard is a local-only cross-platform terminal dashboard (TUI) for managing a whole folder of Git repositories from one keyboard-first interface.

## Features

- Scans a chosen root folder for nested Git repositories.
- Sorts repositories by most recent activity so the busiest projects stay at the top.
- Shows branch, upstream, ahead/behind, dirty state, last commit details, and an operation log in one split dashboard.
- Keeps every major workflow on the keyboard: arrow keys move around the dashboard and `Enter` runs the highlighted action.
- Lets you `Stage`, `Commit`, `Commit+Push`, `Pull`, or `Push` the selected repository from a compact popup-driven flow.
- Supports `Update All`, which fetches every repo with a remote and fast-forwards clean repos that have a usable upstream branch.
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

GUITboard's PortUI action runs the app from source, so install Go once before launching it.

macOS with Homebrew:

```bash
brew install go
go version
```

Windows PowerShell with WinGet:

```powershell
winget install -e --id GoLang.Go
go version
```

Linux package manager examples:

```bash
# Debian or Ubuntu
sudo apt update && sudo apt install -y golang-go

# Fedora
sudo dnf install -y golang

# Arch
sudo pacman -S go
```

Linux distro packages can lag. If `go version` reports less than Go 1.25, use the official installer from [go.dev/doc/install](https://go.dev/doc/install/).

Then launch GUITboard from the repository root.

| OS | What to run |
| --- | --- |
| Windows 10/11 | Double-click `GUITboard.cmd` |
| macOS or Linux | `sh ./portui.sh --run run-dashboard` |
| Any OS, from source | `go run ./cmd/guitboard` |

If Homebrew says Go is installed but your Mac still reports `go: command not found`, refresh Homebrew's shell path:

```bash
eval "$(/opt/homebrew/bin/brew shellenv)"
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
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

Most people only need the launch commands above. Use PortUI for maintenance actions:

| OS | List actions | Run tests |
| --- | --- | --- |
| Windows PowerShell | `.\portui.ps1 -List` | `.\portui.ps1 -Run test` |
| Windows Command Prompt | `portui.cmd --list` | `portui.cmd --run test` |
| macOS or Linux | `sh ./portui.sh --list` | `sh ./portui.sh --run test` |

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
5. Use `Update All` or press `u` to bring every clean repository up to date with GitHub, `Clone` to add another repo into the root, `/` to filter, `d` to toggle dirty-only mode, and `r` to refresh on demand.

The app remembers the last folder you picked in your user config directory, so it works on any machine without hardcoded personal paths.

## Local-only behavior

- GUITboard does not use a backend or cloud service.
- It only runs Git commands on your local machine.
- Push and pull use the Git credentials already configured on that computer.

## Repository notes

- Local build artifacts like `guitboard`, `GUITboard`, `dist/`, and `*.exe` are ignored by Git.
- User-specific app settings are stored outside the repository.
- `GUITboard.cmd` is the direct Windows launcher.
- `portui.sh`, `portui.ps1`, `portui.cmd`, and `.portui-runtime/` are the vendored PortUI engine entrypoints for this repo.
