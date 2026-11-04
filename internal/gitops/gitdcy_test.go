package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The tests read git themselves unless a test hands in a gitdcy, so they do
// not depend on what is installed on the machine running them.
func TestMain(m *testing.M) {
	gitdcyProgram = func() string { return "" }
	os.Exit(m.Run())
}

func useGitdcy(t *testing.T, program string) {
	t.Helper()
	previous := gitdcyProgram
	gitdcyProgram = func() string { return program }
	t.Cleanup(func() { gitdcyProgram = previous })
}

// fakeGitdcy is a gitdcy that writes its arguments to a file and prints answer.
func fakeGitdcy(t *testing.T, answer string) (program, argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake gitdcy is a shell script")
	}
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	answerFile := filepath.Join(dir, "answer.json")
	writeFile(t, answerFile, answer)
	program = filepath.Join(dir, "gitdcy")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > '%s'\ncat '%s'\n", argsFile, answerFile)
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gitdcy: %v", err)
	}
	return program, argsFile
}

func envelope(checkouts string) string {
	return `{"schema_version":"gitdcy.response.v1","component":"katteke.gitdcy","operation":"checkouts","outcome":"success","data":{"checkouts":[` + checkouts + `]},"warnings":[],"error":null,"correlation_id":"t"}`
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fake gitdcy args: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestScanReadsGitdcysPictureWhenItIsInstalled(t *testing.T) {
	root := t.TempDir()
	app := createRepoAt(t, filepath.Join(root, "app"))
	lib := createRepoAt(t, filepath.Join(root, "lib"))
	writeFile(t, filepath.Join(app, "notes.txt"), "new\n")
	answer := envelope(fmt.Sprintf(`
		{"path":%q,"state":"behind","branch":"main","tracking_branch":"origin/main","remote":"github.com/example/app","behind":2,"ahead":0,"changed":3,
		 "files":[{"path":"src/main.go","status":" M"},{"path":"notes.txt","status":"??"},{"path":"both.txt","status":"UU"}],
		 "head":"abc1234 2026-11-01 Fix the parser","head_time":1793556791,"upstream":"def5678 2026-11-02 newer","updated":false,"fetch_error":"","detail":"2 commit(s) behind"},
		{"path":%q,"state":"error","detail":"fatal: bad object HEAD"}`, app, lib))
	program, argsFile := fakeGitdcy(t, answer)
	useGitdcy(t, program)

	if Engine() != "gitdcy" {
		t.Fatalf("expected the gitdcy engine, got %q", Engine())
	}
	repos, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	args := readArgs(t, argsFile)
	want := []string{"checkouts", "--no-fetch", "--format", "json", "--", app, lib}
	if strings.Join(args, "|") != strings.Join(want, "|") {
		t.Fatalf("gitdcy was called with %q, want %q", args, want)
	}

	byPath := map[string]Repo{}
	for _, repo := range repos {
		byPath[repo.Path] = repo
	}
	got := byPath[app]
	if got.Branch != "main" || got.Upstream != "origin/main" || got.Remote != "github.com/example/app" {
		t.Fatalf("branch, upstream or remote not taken from gitdcy: %+v", got)
	}
	if got.Behind != 2 || got.Ahead != 0 || !got.Dirty || got.ChangedCount != 3 {
		t.Fatalf("counts not taken from gitdcy: %+v", got)
	}
	if got.UnstagedCount != 2 || got.UntrackedCount != 1 || got.ConflictedCount != 1 || got.StagedCount != 1 {
		t.Fatalf("file states not tallied like git's own: %+v", got)
	}
	if got.LastCommitMessage != "Fix the parser" || got.LastCommitTime.Unix() != 1793556791 {
		t.Fatalf("last commit not taken from gitdcy: %q %v", got.LastCommitMessage, got.LastCommitTime)
	}
	if !got.LastActivity.After(got.LastCommitTime) {
		t.Fatalf("a changed file newer than the commit should be the last activity: %v", got.LastActivity)
	}
	if byPath[lib].LastError != "fatal: bad object HEAD" {
		t.Fatalf("an error from gitdcy should be the repository's error: %+v", byPath[lib])
	}
}

func TestUpdateAllRunsGitdcyUpdateOnceAndSaysWhatHappened(t *testing.T) {
	answer := envelope(`
		{"path":"/r/a","state":"current","updated":true,"detail":"fast-forwarded 1 commit(s); up to date"},
		{"path":"/r/b","state":"behind","behind":1,"changed":2,"detail":"1 commit(s) behind, but 2 local change(s): commit or stash them first"},
		{"path":"/r/c","state":"current","detail":"up to date"},
		{"path":"/r/d","state":"current","fetch_error":"could not resolve host","detail":"up to date; the fetch failed"}`)
	program, argsFile := fakeGitdcy(t, answer)
	useGitdcy(t, program)

	results := UpdateAll([]Repo{{Path: "/r/a"}, {Path: "/r/b"}, {Path: "/r/c"}, {Path: "/r/d"}})
	args := readArgs(t, argsFile)
	if strings.Join(args[:5], " ") != "checkouts update --format json --" {
		t.Fatalf("gitdcy was called with %q", args)
	}
	if results[0].Result.Status != SyncStatusUpdated || results[0].Err != nil {
		t.Fatalf("a fast-forward should be updated: %+v", results[0])
	}
	if results[1].Result.Status != SyncStatusSkipped || !strings.Contains(results[1].Result.Reason, "commit or stash") {
		t.Fatalf("a dirty checkout should be skipped with gitdcy's reason: %+v", results[1])
	}
	if results[2].Result.Status != SyncStatusCurrent {
		t.Fatalf("an up-to-date checkout should be current: %+v", results[2])
	}
	if results[3].Err == nil || results[3].Err.Error() != "could not resolve host" {
		t.Fatalf("a failed fetch should be a failure: %+v", results[3])
	}
}

func TestAnOlderGitdcysEnvelopeIsStillRead(t *testing.T) {
	answer := strings.Replace(envelope(`{"path":"/r/a","state":"current","branch":"main"}`), `"gitdcy.response.v1"`, `"older.response.v1"`, 1)
	program, _ := fakeGitdcy(t, answer)
	useGitdcy(t, program)
	if repo := Inspect("/r/a"); repo.LastError != "" || repo.Branch != "main" {
		t.Fatalf("an older release's envelope should be read the same: %+v", repo)
	}
}

func TestAnAnswerThatIsNotGitdcysIsAnError(t *testing.T) {
	program, _ := fakeGitdcy(t, `{"schema_version":"other","component":"x"}`)
	useGitdcy(t, program)
	repo := Inspect("/r/a")
	if !strings.Contains(repo.LastError, "GUITBOARD_GITDCY=off") {
		t.Fatalf("the error should say how to read with git instead: %q", repo.LastError)
	}
}

// With GUITBOARD_TEST_GITDCY set to a real gitdcy, both engines read the same
// repositories and must give the same picture.
func TestGitdcyAndGitGiveTheSamePicture(t *testing.T) {
	program := os.Getenv("GUITBOARD_TEST_GITDCY")
	if program == "" {
		t.Skip("set GUITBOARD_TEST_GITDCY to a gitdcy with the checkouts command")
	}
	workRepo, _, behind := createRemoteClonePair(t)
	_, _, dirty := createRemoteClonePair(t)
	writeFile(t, filepath.Join(workRepo, "feature.txt"), "remote update\n")
	if _, err := CommitAll(workRepo, "Remote update"); err != nil {
		t.Fatalf("remote update: %v", err)
	}
	if _, err := RunGit(workRepo, "push"); err != nil {
		t.Fatalf("push: %v", err)
	}
	if _, err := Fetch(behind); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	writeFile(t, filepath.Join(dirty, "README.md"), "edited\n")
	writeFile(t, filepath.Join(dirty, "staged.txt"), "staged\n")
	if _, err := RunGit(dirty, "add", "staged.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}
	writeFile(t, filepath.Join(dirty, "scratch", "a.txt"), "untracked\n")

	for _, path := range []string{workRepo, behind, dirty} {
		own := inspectWithGit(path)
		useGitdcy(t, program)
		via := Inspect(path)
		gitdcyProgram = func() string { return "" }
		sort.Strings(own.ChangedFiles)
		sort.Strings(via.ChangedFiles)
		compare := func(r Repo) string {
			return fmt.Sprintf("branch=%s upstream=%s ahead=%d behind=%d dirty=%v changed=%d staged=%d unstaged=%d untracked=%d conflicted=%d files=%v commit=%q time=%d err=%q",
				r.Branch, r.Upstream, r.Ahead, r.Behind, r.Dirty, r.ChangedCount, r.StagedCount, r.UnstagedCount, r.UntrackedCount, r.ConflictedCount, r.ChangedFiles, r.LastCommitMessage, r.LastCommitTime.Unix(), r.LastError)
		}
		if compare(own) != compare(via) {
			t.Fatalf("%s:\n git:    %s\n gitdcy: %s", path, compare(own), compare(via))
		}
	}

	useGitdcy(t, program)
	results := UpdateAll([]Repo{{Path: behind}, {Path: dirty}})
	if results[0].Err != nil || results[0].Result.Status != SyncStatusUpdated {
		t.Fatalf("the clean clone behind should be fast-forwarded: %+v", results[0])
	}
	if results[1].Result.Status != SyncStatusCurrent && results[1].Result.Status != SyncStatusSkipped {
		t.Fatalf("the dirty clone should be left alone: %+v", results[1])
	}
	if got := Inspect(behind).LastCommitMessage; got != "Remote update" {
		t.Fatalf("expected the fast-forward to the remote update, got %q", got)
	}
	if data, _ := os.ReadFile(filepath.Join(dirty, "README.md")); string(data) != "edited\n" {
		t.Fatalf("a local change was touched: %q", data)
	}
}
