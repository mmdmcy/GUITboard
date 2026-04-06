# GUITboard

GUITboard is a local-only Windows desktop dashboard for managing many Git repositories from one place.

## Features

- Scans a chosen root folder for nested Git repositories.
- Sorts repositories by most recent activity so the busiest projects stay at the top.
- Shows branch, upstream, ahead/behind, dirty state, and last commit details.
- Lets you `Stage All`, `Commit All`, `Pull`, `Push`, or `Commit + Push` from the GUI.
- Supports bulk pull across all tracked repositories.
- Supports bulk commit-and-push across every changed repository.
- Refreshes automatically every 30 seconds and stores the selected root folder locally per user.

## Requirements

- Windows 11 or Windows 10
- [Git for Windows](https://git-scm.com/download/win)
- Go 1.25+ if you want to run or build from source

## Run from source

```powershell
go run ./cmd/guitboard
```

## Build an executable

```powershell
go build -o GUITboard.exe ./cmd/guitboard
```

Then launch:

```powershell
.\GUITboard.exe
```

## How it works

1. Start the app.
2. Click `Choose Folder`.
3. Pick the folder that contains your Git repositories.
4. Select a repository in the list and use the action buttons on the right.

The app remembers the last folder you picked in your Windows user config directory, so it works on any machine without hardcoded personal paths.

## Local-only behavior

- GUITboard does not use a backend or cloud service.
- It only runs Git commands on your local machine.
- Push and pull use the Git credentials already configured on that computer.

## Repository notes

- Build artifacts like `GUITboard.exe` are ignored by Git.
- User-specific app settings are stored outside the repository.
