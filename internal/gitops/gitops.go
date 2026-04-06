package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const gitTimeout = 25 * time.Second

type Repo struct {
	Name              string
	Path              string
	Branch            string
	Upstream          string
	Remote            string
	LastCommitMessage string
	LastCommitTime    time.Time
	LastActivity      time.Time
	Ahead             int
	Behind            int
	ChangedCount      int
	StagedCount       int
	UnstagedCount     int
	UntrackedCount    int
	ConflictedCount   int
	Dirty             bool
	UpstreamGone      bool
	LastError         string
}

func Discover(root string) ([]string, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var repos []string
	seen := map[string]struct{}{}
	stack := []string{root}

	for len(stack) > 0 {
		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.Name() == ".git" {
				if _, ok := seen[dir]; !ok {
					repos = append(repos, dir)
					seen[dir] = struct{}{}
				}
				break
			}
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if entry.Name() == ".git" {
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			stack = append(stack, filepath.Join(dir, entry.Name()))
		}
	}

	sort.Strings(repos)
	return repos, nil
}

func Scan(root string) ([]Repo, error) {
	paths, err := Discover(root)
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return []Repo{}, nil
	}

	repos := make([]Repo, len(paths))
	type job struct {
		index int
		path  string
	}

	jobs := make(chan job)
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range jobs {
				repos[work.index] = Inspect(work.path)
			}
		}()
	}

	for idx, path := range paths {
		jobs <- job{index: idx, path: path}
	}
	close(jobs)
	wg.Wait()

	sort.Slice(repos, func(i, j int) bool {
		left := repos[i]
		right := repos[j]
		switch {
		case left.LastActivity.After(right.LastActivity):
			return true
		case left.LastActivity.Before(right.LastActivity):
			return false
		case left.Dirty != right.Dirty:
			return left.Dirty
		default:
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
	})

	return repos, nil
}

func Inspect(repoPath string) Repo {
	repo := Repo{
		Name: filepath.Base(repoPath),
		Path: repoPath,
	}

	statusOutput, err := RunGit(repoPath, "status", "--porcelain=v1", "--branch", "--untracked-files=all")
	if err != nil {
		repo.LastError = trimOutput(err.Error())
		return repo
	}

	changedPaths := parseStatus(&repo, statusOutput)

	if commitOutput, err := RunGit(repoPath, "log", "-1", "--format=%ct%n%s"); err == nil {
		lines := strings.SplitN(strings.TrimSpace(commitOutput), "\n", 2)
		if len(lines) > 0 {
			if unix, parseErr := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64); parseErr == nil {
				repo.LastCommitTime = time.Unix(unix, 0).Local()
			}
		}
		if len(lines) > 1 {
			repo.LastCommitMessage = strings.TrimSpace(lines[1])
		}
	}

	if remoteOutput, err := RunGit(repoPath, "config", "--get", "remote.origin.url"); err == nil {
		repo.Remote = strings.TrimSpace(remoteOutput)
	}

	repo.LastActivity = repo.LastCommitTime
	if changedTime := latestChangedFileTime(repoPath, changedPaths); changedTime.After(repo.LastActivity) {
		repo.LastActivity = changedTime
	}
	if repo.LastActivity.IsZero() {
		if info, err := os.Stat(repoPath); err == nil {
			repo.LastActivity = info.ModTime().Local()
		}
	}
	if repo.LastCommitMessage == "" {
		repo.LastCommitMessage = "No commits yet"
	}
	if repo.Branch == "" {
		repo.Branch = "(unknown)"
	}

	return repo
}

func StageAll(repoPath string) (string, error) {
	return RunGit(repoPath, "add", "-A")
}

func CommitAll(repoPath, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("commit message cannot be empty")
	}

	var output bytes.Buffer

	stageOutput, err := StageAll(repoPath)
	if stageOutput != "" {
		output.WriteString(stageOutput)
	}
	if err != nil {
		return trimOutput(output.String()), err
	}

	commitOutput, err := RunGit(repoPath, "commit", "-m", message)
	if commitOutput != "" {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(commitOutput)
	}

	return trimOutput(output.String()), err
}

func Pull(repoPath string) (string, error) {
	return RunGit(repoPath, "pull", "--ff-only")
}

func Push(repoPath string) (string, error) {
	return RunGit(repoPath, "push")
}

func CommitAndPush(repoPath, message string) (string, error) {
	commitOutput, err := CommitAll(repoPath, message)
	if err != nil {
		return commitOutput, err
	}

	pushOutput, err := Push(repoPath)
	if pushOutput == "" {
		return trimOutput(commitOutput), err
	}
	if commitOutput == "" {
		return trimOutput(pushOutput), err
	}

	return trimOutput(commitOutput + "\n" + pushOutput), err
}

func RunGit(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoPath}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	trimmed := trimOutput(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		return trimmed, fmt.Errorf("git command timed out")
	}
	if err != nil {
		if trimmed != "" {
			return trimmed, fmt.Errorf("%w: %s", err, trimmed)
		}
		return trimmed, err
	}

	return trimmed, nil
}

func parseStatus(repo *Repo, output string) []string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	var changedPaths []string

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "## ") {
			parseBranchHeader(repo, strings.TrimPrefix(line, "## "))
			continue
		}

		if len(line) < 3 {
			continue
		}

		status := line[:2]
		pathText := strings.TrimSpace(line[3:])
		path := decodeStatusPath(pathText)
		if path != "" {
			changedPaths = append(changedPaths, path)
		}

		repo.Dirty = true
		repo.ChangedCount++

		switch status {
		case "??":
			repo.UntrackedCount++
			continue
		case "!!":
			repo.ChangedCount--
			continue
		}

		if strings.Contains(status, "U") || status == "AA" || status == "DD" {
			repo.ConflictedCount++
		}
		if status[0] != ' ' {
			repo.StagedCount++
		}
		if status[1] != ' ' {
			repo.UnstagedCount++
		}
	}

	return changedPaths
}

func parseBranchHeader(repo *Repo, header string) {
	if strings.HasPrefix(header, "No commits yet on ") {
		repo.Branch = strings.TrimPrefix(header, "No commits yet on ")
		return
	}

	branchPart := header
	trackingPart := ""
	if idx := strings.Index(header, " ["); idx >= 0 {
		branchPart = header[:idx]
		trackingPart = strings.TrimSuffix(strings.TrimPrefix(header[idx:], " ["), "]")
	}

	if strings.Contains(branchPart, "...") {
		parts := strings.SplitN(branchPart, "...", 2)
		repo.Branch = strings.TrimSpace(parts[0])
		repo.Upstream = strings.TrimSpace(parts[1])
	} else {
		repo.Branch = strings.TrimSpace(branchPart)
	}

	if repo.Branch == "HEAD (no branch)" {
		repo.Branch = "detached HEAD"
	}

	if trackingPart == "" {
		return
	}

	if trackingPart == "gone" {
		repo.UpstreamGone = true
		return
	}

	for _, segment := range strings.Split(trackingPart, ",") {
		part := strings.TrimSpace(segment)
		if strings.HasPrefix(part, "ahead ") {
			repo.Ahead, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(part, "ahead ")))
		}
		if strings.HasPrefix(part, "behind ") {
			repo.Behind, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(part, "behind ")))
		}
	}
}

func decodeStatusPath(path string) string {
	if path == "" {
		return ""
	}

	if idx := strings.LastIndex(path, " -> "); idx >= 0 {
		path = path[idx+4:]
	}

	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "\"") && strings.HasSuffix(path, "\"") {
		if decoded, err := strconv.Unquote(path); err == nil {
			path = decoded
		}
	}

	return filepath.FromSlash(path)
}

func latestChangedFileTime(repoPath string, changedPaths []string) time.Time {
	var latest time.Time

	for _, relativePath := range changedPaths {
		fullPath := filepath.Join(repoPath, relativePath)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime().Local()
		}
	}

	return latest
}

func trimOutput(output string) string {
	return strings.TrimSpace(strings.ReplaceAll(output, "\r\n", "\n"))
}
