# GUITboard

GUITboard is a local-only desktop dashboard for managing many Git repositories from one place on Windows and Linux Mint.

## Features

- Scans a chosen root folder for nested Git repositories.
- Sorts repositories by most recent activity so the busiest projects stay at the top.
- Shows branch, upstream, ahead/behind, dirty state, and last commit details.
- Clones GitHub repositories directly into the configured root folder.
- Lets you `Stage All`, `Commit All`, `Pull`, `Push`, or `Commit + Push` from the GUI.
- Supports bulk pull across all tracked repositories.
- Supports bulk commit-and-push across every changed repository.
- Refreshes automatically every 30 seconds and stores the selected root folder locally per user.
- Defaults to `Documents/GitHub` on Windows and `~/Documents/github` on Linux Mint.

## Requirements

- Windows 11 or Windows 10, or Linux Mint
- [Git](https://git-scm.com/)
- Go 1.25+ if you want to run or build from source

### Linux Mint development prerequisites

To run or build the app from source on Linux Mint, install Go from the official Go install guide and install the Linux graphics/toolchain packages required by Fyne:

```bash
sudo apt-get install gcc libgl1-mesa-dev xorg-dev libxkbcommon-dev
```

References:

- https://go.dev/doc/install
- https://docs.fyne.io/started/quick/

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
2. Confirm the default root folder or click `Choose Folder`.
3. Optional: click `Clone GitHub Repo...` and paste either `owner/repo` or a full Git URL.
4. Select a repository in the list and use the action buttons on the right.

The app remembers the last folder you picked in your user config directory, so it works on any machine without hardcoded personal paths.

## Local-only behavior

- GUITboard does not use a backend or cloud service.
- It only runs Git commands on your local machine.
- Push and pull use the Git credentials already configured on that computer.

## Repository notes

- Build artifacts like `GUITboard` and `GUITboard.exe` are ignored by Git.
- User-specific app settings are stored outside the repository.
