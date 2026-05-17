package ui

import (
	"os"
	"path/filepath"
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

func TestGlobalActionsExposeUpdateAll(t *testing.T) {
	model := newDashboardModel(configStub())
	model.busy = false
	model.repos = []gitops.Repo{
		{
			Name:   "kapotteke",
			Path:   "/tmp/kapotteke",
			Remote: "https://github.com/example/kapotteke.git",
		},
	}

	buttons := model.globalActionButtons()
	if buttons[int(globalActionUpdateAll)].label != "Update All" {
		t.Fatalf("expected global update action label, got %q", buttons[int(globalActionUpdateAll)].label)
	}
	if !buttons[int(globalActionUpdateAll)].enabled {
		t.Fatal("expected Update All to be enabled when a remote repo exists")
	}
}

func TestUpdateAllStatusSummarizesOutcomes(t *testing.T) {
	status := updateAllStatus(4, 1, 1, 1, 1)
	want := "Update all finished: 1 updated, 1 current, 1 skipped, 1 failed."
	if status != want {
		t.Fatalf("expected %q, got %q", want, status)
	}
}

func TestUpdateAllShortcutStartsBatch(t *testing.T) {
	model := newDashboardModel(configStub())
	model.busy = false
	model.repos = []gitops.Repo{
		{
			Name:   "kapotteke",
			Path:   "/tmp/kapotteke",
			Remote: "https://github.com/example/kapotteke.git",
		},
	}

	updated, cmd := model.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	result := updated.(dashboardModel)
	if !result.busy {
		t.Fatal("expected Update All shortcut to mark the dashboard busy")
	}
	if result.status != "Updating repositories from their remotes..." {
		t.Fatalf("expected update status, got %q", result.status)
	}
	if cmd == nil {
		t.Fatal("expected Update All shortcut to return a command")
	}
}

func TestKeyHelpTextFitsDefaultTerminalWidth(t *testing.T) {
	if len(keyHelpText()) > 80 {
		t.Fatalf("expected key help to fit 80 columns, got %d characters", len(keyHelpText()))
	}
}

func TestActionButtonLabelAddsVisibleSelectionMarkers(t *testing.T) {
	if got := actionButtonLabel("Update All", true, true, true); got != "> Update All <" {
		t.Fatalf("expected focused marker, got %q", got)
	}
	if got := actionButtonLabel("Update All", true, false, true); got != "[Update All]" {
		t.Fatalf("expected selected marker, got %q", got)
	}
	if got := actionButtonLabel("Update All", true, true, false); got != "Update All" {
		t.Fatalf("expected disabled label to stay plain, got %q", got)
	}
}

func TestRepoRowNameAddsVisibleSelectionMarker(t *testing.T) {
	if got := repoRowName("kapotteke", true); got != "> kapotteke" {
		t.Fatalf("expected selected repo marker, got %q", got)
	}
	if got := repoRowName("kapotteke", false); got != "  kapotteke" {
		t.Fatalf("expected unselected repo spacing, got %q", got)
	}
}

func configStub() config.Config {
	return config.Config{
		RootPath:           "/tmp",
		AutoRefreshSeconds: 30,
	}
}

func TestShouldUseAltScreenDisablesPortUILaunch(t *testing.T) {
	t.Setenv("PORTUI_INTERACTIVE", "1")

	if shouldUseAltScreen() {
		t.Fatal("expected PortUI launches to render in the current terminal screen")
	}
}

func TestShouldUseAltScreenDisablesDirectWindowsLauncher(t *testing.T) {
	t.Setenv("GUITBOARD_NO_ALT_SCREEN", "1")

	if shouldUseAltScreen() {
		t.Fatal("expected direct Windows launcher to render in the current terminal screen")
	}
}

func TestPlainTerminalModeSupportsDirectWindowsLauncher(t *testing.T) {
	t.Setenv("GUITBOARD_PLAIN_TERMINAL", "1")

	if !plainTerminalMode() {
		t.Fatal("expected direct Windows launcher to use plain terminal styling")
	}
}

func TestDefaultRootFromCurrentRepoUsesParentFolder(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "GUITboard")
	nestedPath := filepath.Join(repoPath, "portui")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatalf("failed to create git marker: %v", err)
	}
	if err := os.MkdirAll(nestedPath, 0o755); err != nil {
		t.Fatalf("failed to create nested working dir: %v", err)
	}

	t.Chdir(nestedPath)

	if got := defaultRootFromCurrentRepo(); got != filepath.Clean(root) {
		t.Fatalf("expected %s, got %s", filepath.Clean(root), got)
	}
}

func TestRootPathCandidatesForGOOS(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "rei")

	tests := []struct {
		name string
		goos string
		want []string
	}{
		{
			name: "windows",
			goos: "windows",
			want: []string{
				filepath.Join(home, "Documents", "GitHub"),
				filepath.Join(home, "Documents", "github"),
				filepath.Join(home, "source", "repos"),
				filepath.Join(home, "GitHub"),
			},
		},
		{
			name: "darwin",
			goos: "darwin",
			want: []string{
				filepath.Join(home, "Developer"),
				filepath.Join(home, "Code"),
				filepath.Join(home, "Documents", "github"),
				filepath.Join(home, "Documents", "GitHub"),
				filepath.Join(home, "github"),
				filepath.Join(home, "GitHub"),
				filepath.Join(home, "src"),
			},
		},
		{
			name: "linux",
			goos: "linux",
			want: []string{
				filepath.Join(home, "Documents", "github"),
				filepath.Join(home, "Documents", "GitHub"),
				filepath.Join(home, "github"),
				filepath.Join(home, "GitHub"),
				filepath.Join(home, "code"),
				filepath.Join(home, "Code"),
				filepath.Join(home, "src"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rootPathCandidatesForGOOS(home, tt.goos)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d candidates, got %d", len(tt.want), len(got))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("candidate %d mismatch: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}
