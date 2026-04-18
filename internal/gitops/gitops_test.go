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

func TestCloneClonesRepositoryIntoConfiguredRoot(t *testing.T) {
	remotePath := createBareRemoteRepo(t)
	root := t.TempDir()

	targetPath, _, err := Clone(remotePath, root, "")
	if err != nil {
		t.Fatalf("Clone returned error: %v", err)
	}

	expectedPath := filepath.Join(root, "remote")
	if targetPath != expectedPath {
		t.Fatalf("expected clone target %s, got %s", expectedPath, targetPath)
	}

	repo := Inspect(targetPath)
	if repo.LastCommitMessage != "Initial commit" {
		t.Fatalf("expected cloned repo to contain initial commit, got %q", repo.LastCommitMessage)
	}
	if repo.Remote != remotePath {
		t.Fatalf("expected cloned repo remote %q, got %q", remotePath, repo.Remote)
	}
}

func TestDefaultCloneDirNameSupportsGitHubInputs(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "shorthand", source: "owner/repo", want: "repo"},
		{name: "https", source: "https://github.com/owner/repo.git", want: "repo"},
		{name: "ssh", source: "git@github.com:owner/repo.git", want: "repo"},
		{name: "github host without scheme", source: "github.com/owner/repo", want: "repo"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DefaultCloneDirName(test.source); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestCloneRejectsNestedFolderName(t *testing.T) {
	remotePath := createBareRemoteRepo(t)
	root := t.TempDir()

	if _, _, err := Clone(remotePath, root, "../bad"); err == nil {
		t.Fatal("expected Clone to reject a nested folder name")
	}
}

func createTestRepo(t *testing.T, name string) string {
	t.Helper()
	return createRepoAt(t, filepath.Join(t.TempDir(), name))
}

func createBareRemoteRepo(t *testing.T) string {
	t.Helper()

	workRepo := createTestRepo(t, "source")
	writeFile(t, filepath.Join(workRepo, "README.md"), "# cloned test\n")
	if _, err := CommitAll(workRepo, "Initial commit"); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	remotePath := filepath.Join(t.TempDir(), "remote.git")
	if _, err := RunGit("", "init", "--bare", remotePath); err != nil {
		t.Fatalf("failed to create bare remote: %v", err)
	}
	if _, err := RunGit(workRepo, "remote", "add", "origin", remotePath); err != nil {
		t.Fatalf("failed to add origin remote: %v", err)
	}
	if _, err := RunGit(workRepo, "push", "-u", "origin", "HEAD"); err != nil {
		t.Fatalf("failed to push initial commit: %v", err)
	}

	return filepath.Clean(remotePath)
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
