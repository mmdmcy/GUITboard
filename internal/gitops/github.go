package gitops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type CloneProtocol string

const (
	CloneProtocolHTTPS CloneProtocol = "https"
	CloneProtocolSSH   CloneProtocol = "ssh"
)

type CloneableRepo struct {
	Name          string
	FullName      string
	Owner         string
	Description   string
	Visibility    string
	DefaultBranch string
	HTTPSURL      string
	SSHURL        string
	HTMLURL       string
	Private       bool
	Fork          bool
	Archived      bool
	UpdatedAt     time.Time
}

func (repo CloneableRepo) CloneURL(protocol CloneProtocol) string {
	if protocol == CloneProtocolSSH && strings.TrimSpace(repo.SSHURL) != "" {
		return repo.SSHURL
	}
	return strings.TrimSpace(repo.HTTPSURL)
}

var ghCommandRunner = func(args ...string) (string, error) {
	return runCLICommand("gh", ghTimeout, nil, args...)
}

func ListCloneableRepositories() ([]CloneableRepo, error) {
	output, err := ghCommandRunner(
		"api",
		"--method", "GET",
		"--paginate",
		"--slurp",
		"user/repos?affiliation=owner,collaborator,organization_member&sort=updated&per_page=100",
	)
	if err != nil {
		return nil, normalizeGHError(err)
	}

	var pages [][]githubRepoPayload
	if err := json.Unmarshal([]byte(output), &pages); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub repository list: %w", err)
	}

	repos := make([]CloneableRepo, 0)
	for _, page := range pages {
		for _, item := range page {
			repo := item.toCloneableRepo()
			if repo.FullName == "" {
				continue
			}
			if repo.HTTPSURL == "" && repo.SSHURL == "" {
				continue
			}
			repos = append(repos, repo)
		}
	}

	sort.Slice(repos, func(i, j int) bool {
		left := repos[i]
		right := repos[j]
		switch {
		case left.UpdatedAt.After(right.UpdatedAt):
			return true
		case left.UpdatedAt.Before(right.UpdatedAt):
			return false
		default:
			return strings.ToLower(left.FullName) < strings.ToLower(right.FullName)
		}
	})

	return repos, nil
}

type githubRepoPayload struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Visibility    string `json:"visibility"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	HTMLURL       string `json:"html_url"`
	Private       bool   `json:"private"`
	Fork          bool   `json:"fork"`
	Archived      bool   `json:"archived"`
	UpdatedAt     string `json:"updated_at"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (payload githubRepoPayload) toCloneableRepo() CloneableRepo {
	repo := CloneableRepo{
		Name:          strings.TrimSpace(payload.Name),
		FullName:      strings.TrimSpace(payload.FullName),
		Owner:         strings.TrimSpace(payload.Owner.Login),
		Description:   strings.TrimSpace(payload.Description),
		Visibility:    normalizeVisibility(payload.Visibility, payload.Private),
		DefaultBranch: strings.TrimSpace(payload.DefaultBranch),
		HTTPSURL:      strings.TrimSpace(payload.CloneURL),
		SSHURL:        strings.TrimSpace(payload.SSHURL),
		HTMLURL:       strings.TrimSpace(payload.HTMLURL),
		Private:       payload.Private,
		Fork:          payload.Fork,
		Archived:      payload.Archived,
	}

	if updatedAt := strings.TrimSpace(payload.UpdatedAt); updatedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			repo.UpdatedAt = parsed.Local()
		}
	}

	if repo.Name == "" && repo.FullName != "" {
		parts := strings.Split(repo.FullName, "/")
		repo.Name = parts[len(parts)-1]
	}

	return repo
}

func normalizeVisibility(value string, isPrivate bool) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed != "" {
		return trimmed
	}
	if isPrivate {
		return "private"
	}
	return "public"
}

func normalizeGHError(err error) error {
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return errors.New("GitHub CLI (gh) is not installed. Install gh and run `gh auth login` to browse repositories.")
	}

	message := err.Error()
	lower := strings.ToLower(message)
	if strings.Contains(lower, "http 401") || strings.Contains(lower, "requires authentication") || strings.Contains(lower, "authentication required") {
		return errors.New("GitHub CLI is not authenticated. Run `gh auth login` to browse repositories.")
	}

	return fmt.Errorf("failed to load repositories from GitHub CLI: %w", err)
}
