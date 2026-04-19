package gitops

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestListCloneableRepositoriesParsesAndSortsRepos(t *testing.T) {
	original := ghCommandRunner
	t.Cleanup(func() {
		ghCommandRunner = original
	})

	ghCommandRunner = func(args ...string) (string, error) {
		if got := strings.Join(args, " "); !strings.Contains(got, "user/repos?affiliation=owner,collaborator,organization_member") {
			t.Fatalf("unexpected gh arguments: %v", args)
		}

		return `[
			[
				{
					"name": "beta",
					"full_name": "rei/beta",
					"description": "Second repo",
					"visibility": "private",
					"default_branch": "main",
					"clone_url": "https://github.com/rei/beta.git",
					"ssh_url": "git@github.com:rei/beta.git",
					"html_url": "https://github.com/rei/beta",
					"private": true,
					"fork": false,
					"archived": false,
					"updated_at": "2026-04-17T10:30:00Z",
					"owner": {"login": "rei"}
				},
				{
					"name": "alpha",
					"full_name": "rei/alpha",
					"description": "First repo",
					"visibility": "public",
					"default_branch": "main",
					"clone_url": "https://github.com/rei/alpha.git",
					"ssh_url": "git@github.com:rei/alpha.git",
					"html_url": "https://github.com/rei/alpha",
					"private": false,
					"fork": true,
					"archived": false,
					"updated_at": "2026-04-18T13:00:00Z",
					"owner": {"login": "rei"}
				}
			],
			[
				{
					"name": "skipme",
					"full_name": "",
					"clone_url": "",
					"ssh_url": "",
					"updated_at": "2026-04-19T09:15:00Z",
					"owner": {"login": "rei"}
				}
			]
		]`, nil
	}

	repos, err := ListCloneableRepositories()
	if err != nil {
		t.Fatalf("ListCloneableRepositories returned error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repositories, got %d", len(repos))
	}
	if repos[0].FullName != "rei/alpha" {
		t.Fatalf("expected most recently updated repo first, got %s", repos[0].FullName)
	}
	if got := repos[0].CloneURL(CloneProtocolSSH); got != "git@github.com:rei/alpha.git" {
		t.Fatalf("expected SSH clone url, got %q", got)
	}
	if got := repos[1].Visibility; got != "private" {
		t.Fatalf("expected visibility to be preserved, got %q", got)
	}
}

func TestListCloneableRepositoriesExplainsMissingGitHubCLI(t *testing.T) {
	original := ghCommandRunner
	t.Cleanup(func() {
		ghCommandRunner = original
	})

	ghCommandRunner = func(args ...string) (string, error) {
		return "", &exec.Error{Name: "gh", Err: exec.ErrNotFound}
	}

	_, err := ListCloneableRepositories()
	if err == nil {
		t.Fatal("expected ListCloneableRepositories to fail when gh is missing")
	}
	if !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("expected helpful gh guidance, got %v", err)
	}
}

func TestListCloneableRepositoriesExplainsAuthenticationFailures(t *testing.T) {
	original := ghCommandRunner
	t.Cleanup(func() {
		ghCommandRunner = original
	})

	ghCommandRunner = func(args ...string) (string, error) {
		return "", errors.New("gh: HTTP 401: Requires authentication")
	}

	_, err := ListCloneableRepositories()
	if err == nil {
		t.Fatal("expected ListCloneableRepositories to fail when gh auth is missing")
	}
	if !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("expected authentication guidance, got %v", err)
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
