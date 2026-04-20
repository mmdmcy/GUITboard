package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"guitboard/internal/config"
	"guitboard/internal/gitops"
)

func TestOpenModalStoresSelectedRepo(t *testing.T) {
	model := newDashboardModel(configStub())
	repo := gitops.Repo{
		Name: "kapotteke",
		Path: "/tmp/kapotteke",
	}

	model.openModal(
		modalCommitAndPush,
		"Quick Commit + Push",
		"commit message",
		"",
		"default message",
		repo,
	)

	if model.modal == nil {
		t.Fatal("expected modal to be created")
	}
	if model.modal.repo.Path != repo.Path {
		t.Fatalf("expected modal repo path %q, got %q", repo.Path, model.modal.repo.Path)
	}
	if model.modal.repo.Name != repo.Name {
		t.Fatalf("expected modal repo name %q, got %q", repo.Name, model.modal.repo.Name)
	}
}

func TestEscBacksOutOfRepoSections(t *testing.T) {
	model := newDashboardModel(configStub())
	model.busy = false
	model.filtered = []gitops.Repo{
		{
			Name:     "kapotteke",
			Path:     "/tmp/kapotteke",
			Remote:   "https://github.com/example/kapotteke.git",
			Upstream: "origin/main",
		},
	}
	model.selectedIdx = 0
	model.selectedPath = "/tmp/kapotteke"
	model.focus = focusRepoActions

	updated, _ := model.updateDashboard(tea.KeyMsg{Type: tea.KeyEsc})
	afterRepoActions := updated.(dashboardModel)
	if afterRepoActions.focus != focusRepoList {
		t.Fatalf("expected esc from repo actions to move to repo list, got %v", afterRepoActions.focus)
	}

	updated, _ = afterRepoActions.updateDashboard(tea.KeyMsg{Type: tea.KeyEsc})
	afterRepoList := updated.(dashboardModel)
	if afterRepoList.focus != focusGlobalActions {
		t.Fatalf("expected esc from repo list to move to global actions, got %v", afterRepoList.focus)
	}
}

func TestArrowBoundariesMoveAcrossSections(t *testing.T) {
	model := newDashboardModel(configStub())
	model.busy = false
	model.filtered = []gitops.Repo{
		{
			Name:     "kapotteke",
			Path:     "/tmp/kapotteke",
			Remote:   "https://github.com/example/kapotteke.git",
			Upstream: "origin/main",
		},
	}
	model.selectedIdx = 0
	model.selectedPath = "/tmp/kapotteke"
	model.focus = focusRepoActions
	model.repoActionIdx = 0

	updated, _ := model.updateDashboard(tea.KeyMsg{Type: tea.KeyLeft})
	leftResult := updated.(dashboardModel)
	if leftResult.focus != focusRepoList {
		t.Fatalf("expected left from first repo action to move to repo list, got %v", leftResult.focus)
	}

	leftResult.focus = focusRepoList
	leftResult.selectedIdx = 0
	updated, _ = leftResult.updateDashboard(tea.KeyMsg{Type: tea.KeyUp})
	upResult := updated.(dashboardModel)
	if upResult.focus != focusGlobalActions {
		t.Fatalf("expected up from first repo row to move to global actions, got %v", upResult.focus)
	}

	leftResult.focus = focusRepoList
	leftResult.selectedIdx = 0
	updated, _ = leftResult.updateDashboard(tea.KeyMsg{Type: tea.KeyRight})
	rightFromRepos := updated.(dashboardModel)
	if rightFromRepos.focus != focusRepoActions {
		t.Fatalf("expected right from repo list to move to repo actions, got %v", rightFromRepos.focus)
	}

	upResult.focus = focusGlobalActions
	upResult.globalActionIdx = len(upResult.globalActionButtons()) - 1
	updated, _ = upResult.updateDashboard(tea.KeyMsg{Type: tea.KeyRight})
	rightResult := updated.(dashboardModel)
	if rightResult.focus != focusRepoList {
		t.Fatalf("expected right from last global action to move to repo list, got %v", rightResult.focus)
	}
}

func TestRepoActionsSkipDisabledButtons(t *testing.T) {
	model := newDashboardModel(configStub())
	model.busy = false
	model.filtered = []gitops.Repo{
		{
			Name:     "kapotteke",
			Path:     "/tmp/kapotteke",
			Remote:   "https://github.com/example/kapotteke.git",
			Upstream: "origin/main",
		},
	}
	model.selectedIdx = 0
	model.selectedPath = "/tmp/kapotteke"
	model.focus = focusRepoList

	updated, _ := model.updateDashboard(tea.KeyMsg{Type: tea.KeyRight})
	afterEnterRepoActions := updated.(dashboardModel)
	if afterEnterRepoActions.focus != focusRepoActions {
		t.Fatalf("expected right from repo list to enter repo actions, got %v", afterEnterRepoActions.focus)
	}
	if afterEnterRepoActions.repoActionIdx != int(repoActionPull) {
		t.Fatalf("expected first enabled repo action to be Pull, got index %d", afterEnterRepoActions.repoActionIdx)
	}

	updated, _ = afterEnterRepoActions.updateDashboard(tea.KeyMsg{Type: tea.KeyRight})
	afterMoveRight := updated.(dashboardModel)
	if afterMoveRight.repoActionIdx != int(repoActionPush) {
		t.Fatalf("expected right to skip disabled actions and land on Push, got index %d", afterMoveRight.repoActionIdx)
	}

	updated, _ = afterMoveRight.updateDashboard(tea.KeyMsg{Type: tea.KeyLeft})
	afterMoveLeft := updated.(dashboardModel)
	if afterMoveLeft.repoActionIdx != int(repoActionPull) {
		t.Fatalf("expected left to skip disabled actions and land on Pull, got index %d", afterMoveLeft.repoActionIdx)
	}
}

func configStub() config.Config {
	return config.Config{
		RootPath:           "/tmp",
		AutoRefreshSeconds: 30,
	}
}
