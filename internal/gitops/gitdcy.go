package gitops

// GitDCY integration. When GitDCY is installed with its `checkouts` command,
// GUITboard reads every repository through `gitdcy checkouts --format json`
// and Update All runs `gitdcy checkouts update`, so GUITboard, GitDCY and
// anything else built on it show one picture from one engine. Without it, or
// with GUITBOARD_GITDCY=off, GUITboard reads git itself as before.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	gitdcyReadTimeout   = 2 * time.Minute
	gitdcyUpdateTimeout = 15 * time.Minute
)

// gitdcyProgram is the gitdcy to use, or "" for none; tests replace it.
var gitdcyProgram = sync.OnceValue(findGitdcy)

func findGitdcy() string {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GUITBOARD_GITDCY")), "off") {
		return ""
	}
	path, err := exec.LookPath("gitdcy")
	if err != nil {
		return ""
	}
	// An older gitdcy has no checkouts command; GUITboard then reads git itself.
	if _, err := runCLICommand(path, gitTimeout, nil, "checkouts", "--help"); err != nil {
		return ""
	}
	return path
}

// Engine names what reads the repositories: "gitdcy" or "git".
func Engine() string {
	if gitdcyProgram() != "" {
		return "gitdcy"
	}
	return "git"
}

type gitdcyFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type gitdcyCheckout struct {
	Path           string       `json:"path"`
	State          string       `json:"state"`
	Branch         string       `json:"branch"`
	TrackingBranch string       `json:"tracking_branch"`
	Remote         string       `json:"remote"`
	Behind         int          `json:"behind"`
	Ahead          int          `json:"ahead"`
	Files          []gitdcyFile `json:"files"`
	Head           string       `json:"head"`
	HeadTime       int64        `json:"head_time"`
	Updated        bool         `json:"updated"`
	FetchError     string       `json:"fetch_error"`
	Detail         string       `json:"detail"`
}

type gitdcyAnswer struct {
	SchemaVersion string `json:"schema_version"`
	Component     string `json:"component"`
	Data          *struct {
		Checkouts []gitdcyCheckout `json:"checkouts"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// gitdcyCheckouts runs `gitdcy checkouts [update]` for these paths and
// returns one checkout per path, in the order given.
func gitdcyCheckouts(program string, update bool, paths []string) ([]gitdcyCheckout, error) {
	args := []string{"checkouts"}
	timeout := gitdcyReadTimeout
	if update {
		args = append(args, "update")
		timeout = gitdcyUpdateTimeout
	} else {
		// A dashboard that refreshes every 30 seconds does not fetch.
		args = append(args, "--no-fetch")
	}
	args = append(args, "--format", "json", "--")
	args = append(args, paths...)
	output, runErr := runCLICommandStdout(program, timeout, args...)
	var answer gitdcyAnswer
	if err := json.Unmarshal([]byte(output), &answer); err != nil {
		if runErr != nil {
			return nil, runErr
		}
		return nil, fmt.Errorf("gitdcy's answer could not be read: %w", err)
	}
	if answer.SchemaVersion != "linuxmice.response.v1" || answer.Component != "katteke.gitdcy" {
		return nil, errors.New("gitdcy answered in a format GUITboard does not know")
	}
	if answer.Error != nil {
		return nil, errors.New(answer.Error.Message)
	}
	if answer.Data == nil {
		return nil, errors.New("gitdcy's answer has no checkouts")
	}
	byPath := make(map[string]gitdcyCheckout, len(answer.Data.Checkouts))
	for _, checkout := range answer.Data.Checkouts {
		byPath[checkout.Path] = checkout
	}
	list := make([]gitdcyCheckout, len(paths))
	for i, path := range paths {
		checkout, ok := byPath[path]
		if !ok {
			checkout = gitdcyCheckout{Path: path, State: "error", Detail: "gitdcy did not report this repository"}
		}
		list[i] = checkout
	}
	return list, nil
}

// repoFromGitdcy is the same Repo that Inspect reads with git.
func repoFromGitdcy(checkout gitdcyCheckout) Repo {
	repo := Repo{
		Name:     filepath.Base(checkout.Path),
		Path:     checkout.Path,
		Branch:   checkout.Branch,
		Upstream: checkout.TrackingBranch,
		Remote:   checkout.Remote,
		Ahead:    checkout.Ahead,
		Behind:   checkout.Behind,
	}
	switch checkout.State {
	case "missing", "error":
		repo.LastError = checkout.Detail
		return repo
	}
	for _, file := range checkout.Files {
		path := filepath.FromSlash(file.Path)
		if path != "" {
			repo.ChangedFiles = append(repo.ChangedFiles, path)
		}
		tallyStatus(&repo, file.Status)
	}
	if parts := strings.SplitN(checkout.Head, " ", 3); len(parts) == 3 {
		repo.LastCommitMessage = parts[2]
	}
	if checkout.HeadTime > 0 {
		repo.LastCommitTime = time.Unix(checkout.HeadTime, 0).Local()
	}
	if repo.Branch == "" && checkout.Head != "" {
		repo.Branch = "detached HEAD"
	}
	finishRepo(&repo)
	return repo
}

func readWithGitdcy(program string, paths []string) ([]Repo, error) {
	list, err := gitdcyCheckouts(program, false, paths)
	if err != nil {
		return nil, fmt.Errorf("gitdcy checkouts: %w (GUITBOARD_GITDCY=off reads with git instead)", err)
	}
	repos := make([]Repo, len(list))
	for i, checkout := range list {
		repos[i] = repoFromGitdcy(checkout)
	}
	return repos, nil
}

// syncResultFromGitdcy says what `gitdcy checkouts update` did, in Update All's words.
func syncResultFromGitdcy(checkout gitdcyCheckout) (SyncResult, error) {
	result := SyncResult{Output: checkout.Detail, Status: SyncStatusCurrent}
	switch {
	case checkout.Updated:
		result.Status = SyncStatusUpdated
	case checkout.FetchError != "":
		return result, errors.New(checkout.FetchError)
	case checkout.State == "missing" || checkout.State == "error":
		return result, errors.New(checkout.Detail)
	case checkout.State == "current" || checkout.State == "ahead":
	default:
		result.Status = SyncStatusSkipped
		result.Reason = checkout.Detail
	}
	return result, nil
}

// UpdateResult is what Update All did to one repository.
type UpdateResult struct {
	Repo   Repo
	Result SyncResult
	Err    error
}

// UpdateAll fetches every repository and fast-forwards the clean ones that
// can be: through `gitdcy checkouts update` when GitDCY is installed, else
// with SyncDetailed one by one.
func UpdateAll(repos []Repo) []UpdateResult {
	results := make([]UpdateResult, len(repos))
	program := gitdcyProgram()
	if program == "" {
		for i, repo := range repos {
			result, err := SyncDetailed(repo)
			results[i] = UpdateResult{Repo: repo, Result: result, Err: err}
		}
		return results
	}
	paths := make([]string, len(repos))
	for i, repo := range repos {
		paths[i] = repo.Path
	}
	list, err := gitdcyCheckouts(program, true, paths)
	for i, repo := range repos {
		if err != nil {
			results[i] = UpdateResult{Repo: repo, Err: fmt.Errorf("gitdcy checkouts update: %w", err)}
			continue
		}
		result, resultErr := syncResultFromGitdcy(list[i])
		results[i] = UpdateResult{Repo: repo, Result: result, Err: resultErr}
	}
	return results
}
