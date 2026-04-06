package gitops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCommitAllStagesAndCommits(t *testing.T) {
	repoPath := createTestRepo(t, "commit-repo")

	writeFile(t, filepath.Join(repoPath, "README.md"), "# test\n")

	if _, err := CommitAll(repoPath, "Initial commit"); err != nil {
		t.Fatalf("CommitAll returned error: %v", err)
	}

	repo := Inspect(repoPath)
	if repo.Dirty {
		t.Fatalf("expected repo to be clean after commit")
	}
	if repo.LastCommitMessage != "Initial commit" {
		t.Fatalf("expected last commit message to be recorded, got %q", repo.LastCommitMessage)
	}
}

func TestScanSortsByLatestDirtyActivity(t *testing.T) {
	root := t.TempDir()
	olderRepo := createRepoAt(t, filepath.Join(root, "older"))
	newerRepo := createRepoAt(t, filepath.Join(root, "newer"))

	olderFile := filepath.Join(olderRepo, "file.txt")
	newerFile := filepath.Join(newerRepo, "file.txt")
	writeFile(t, olderFile, "older")
	writeFile(t, newerFile, "newer")

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(olderFile, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set old file time: %v", err)
	}
	if err := os.Chtimes(newerFile, newTime, newTime); err != nil {
		t.Fatalf("failed to set new file time: %v", err)
	}

	repos, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Path != newerRepo {
		t.Fatalf("expected newer repo first, got %s", repos[0].Path)
	}
	if repos[1].Path != olderRepo {
		t.Fatalf("expected older repo second, got %s", repos[1].Path)
	}
}

func createTestRepo(t *testing.T, name string) string {
	t.Helper()
	return createRepoAt(t, filepath.Join(t.TempDir(), name))
}

func createRepoAt(t *testing.T, repoPath string) string {
	t.Helper()

	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	if _, err := RunGit(repoPath, "init"); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	if _, err := RunGit(repoPath, "config", "user.name", "GUITboard Test"); err != nil {
		t.Fatalf("failed to set user.name: %v", err)
	}
	if _, err := RunGit(repoPath, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to set user.email: %v", err)
	}

	return filepath.Clean(repoPath)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent dirs: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}
