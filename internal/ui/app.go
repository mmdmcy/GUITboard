package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"guitboard/internal/config"
	"guitboard/internal/gitops"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type focusArea int

const (
	focusGlobalActions focusArea = iota
	focusRepoList
	focusRepoActions
)

type modalKind int

const (
	modalFilter modalKind = iota
	modalRoot
	modalCommitOnly
	modalCommitAndPush
	modalBulkCommit
	modalClone
)

type globalAction int

const (
	globalActionChangeRoot globalAction = iota
	globalActionRefresh
	globalActionFilter
	globalActionDirtyOnly
	globalActionClone
	globalActionFetchAll
	globalActionBulkCommit
)

type repoAction int

const (
	repoActionStageAll repoAction = iota
	repoActionCommitPush
	repoActionCommitOnly
	repoActionPull
	repoActionPush
)

type actionButton struct {
	label   string
	enabled bool
}

type modalState struct {
	kind        modalKind
	title       string
	prompt      string
	placeholder string
	repo        gitops.Repo
	input       textinput.Model
}

type dashboardModel struct {
	cfg config.Config

	repos        []gitops.Repo
	filtered     []gitops.Repo
	selectedPath string
	selectedIdx  int
	repoScroll   int

	filterQuery string
	dirtyOnly   bool

	focus           focusArea
	globalActionIdx int
	repoActionIdx   int
	modal           *modalState
	logs            []string
	status          string
	busy            bool
	busyLabel       string
	spinner         spinner.Model
	width           int
	height          int
}

type scanFinishedMsg struct {
	repos []gitops.Repo
	err   error
}

type operationFinishedMsg struct {
	status       string
	logs         []string
	refresh      bool
	selectedPath string
}

type refreshTickMsg time.Time

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	root, err := ensureRootPath(cfg.RootPath)
	if err != nil {
		return err
	}
	if root != cfg.RootPath {
		cfg.RootPath = root
		_ = config.Save(cfg)
	}

	model := newDashboardModel(cfg)
	program := tea.NewProgram(model, tea.WithAltScreen())

	_, err = program.Run()
	return err
}

func newDashboardModel(cfg config.Config) dashboardModel {
	spin := spinner.New()
	spin.Spinner = spinner.Spinner{
		Frames: []string{"/", "|", "\\", "-"},
		FPS:    time.Second / 9,
	}
	spin.Style = styles.spinner

	model := dashboardModel{
		cfg:             cfg,
		focus:           focusRepoList,
		selectedIdx:     -1,
		spinner:         spin,
		status:          "Use the dashboard actions to refresh, filter, clone, or sync repositories.",
		width:           80,
		height:          24,
		globalActionIdx: int(globalActionRefresh),
		repoActionIdx:   int(repoActionCommitPush),
	}

	if strings.TrimSpace(cfg.RootPath) != "" {
		model.busy = true
		model.busyLabel = fmt.Sprintf("Scanning %s ...", cfg.RootPath)
		model.status = model.busyLabel
	}

	return model
}

func (m dashboardModel) Init() tea.Cmd {
	cmds := []tea.Cmd{refreshTickCmd(m.autoRefreshInterval())}
	if m.busy && strings.TrimSpace(m.cfg.RootPath) != "" {
		cmds = append(cmds, m.spinner.Tick, scanReposCmd(m.cfg.RootPath))
	}
	return tea.Batch(cmds...)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.modal != nil {
			m.modal.input.Width = m.modalInputWidth()
		}
		m.ensureRepoVisible()
		return m, nil

	case spinner.TickMsg:
		if !m.busy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scanFinishedMsg:
		m.busy = false
		m.busyLabel = ""
		if msg.err != nil {
			m.status = fmt.Sprintf("Scan failed: %v", msg.err)
			return m, nil
		}

		m.repos = msg.repos
		m.cfg.LastScan = time.Now()
		_ = config.Save(m.cfg)
		m.applyFilters()
		m.status = fmt.Sprintf("Loaded %d repositories from %s", len(m.repos), valueOrDash(m.cfg.RootPath))
		return m, nil

	case operationFinishedMsg:
		m.busy = false
		m.busyLabel = ""
		m.status = msg.status
		m.appendLogs(msg.logs)
		if msg.selectedPath != "" {
			m.selectedPath = msg.selectedPath
		}
		if msg.refresh {
			return m.startRefresh("Refreshing dashboard after git operation...")
		}
		return m, nil

	case refreshTickMsg:
		if m.busy || m.modal != nil || strings.TrimSpace(m.cfg.RootPath) == "" {
			return m, refreshTickCmd(m.autoRefreshInterval())
		}

		updated, cmd := m.startRefresh("Auto-refreshing repositories...")
		return updated, tea.Batch(cmd, refreshTickCmd(updated.autoRefreshInterval()))

	case tea.KeyMsg:
		if m.modal != nil {
			return m.updateModal(msg)
		}
		return m.updateDashboard(msg)
	}

	return m, nil
}

func (m dashboardModel) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		m.focus = m.nextFocus()
		return m, nil
	case "shift+tab":
		m.focus = m.previousFocus()
		return m, nil
	case "/":
		m.openModal(modalFilter, "Filter Repositories", "Filter by repo name, branch, path, remote, or commit message.", m.filterQuery, "repo name, branch, path...")
		return m, nil
	case "d":
		if m.busy {
			return m, nil
		}
		m.dirtyOnly = !m.dirtyOnly
		m.applyFilters()
		if m.dirtyOnly {
			m.status = "Dirty-only filter enabled."
		} else {
			m.status = "Dirty-only filter disabled."
		}
		return m, nil
	case "r":
		if m.busy {
			return m, nil
		}
		return m.startRefresh("Refreshing repositories...")
	case "left", "h":
		m.moveLeft()
		return m, nil
	case "right", "l":
		m.moveRight()
		return m, nil
	case "up", "k":
		m.moveUp()
		return m, nil
	case "down", "j":
		m.moveDown()
		return m, nil
	case "enter":
		return m.activateFocused()
	}

	return m, nil
}

func (m dashboardModel) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal == nil {
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.modal = nil
		m.status = "Cancelled."
		return m, nil
	case "enter":
		return m.submitModal()
	}

	var cmd tea.Cmd
	modal := m.modal
	modal.input, cmd = modal.input.Update(msg)
	m.modal = modal
	return m, cmd
}

func (m dashboardModel) submitModal() (tea.Model, tea.Cmd) {
	if m.modal == nil {
		return m, nil
	}

	modal := m.modal
	m.modal = nil
	value := strings.TrimSpace(modal.input.Value())

	switch modal.kind {
	case modalFilter:
		m.filterQuery = value
		m.applyFilters()
		if value == "" {
			m.status = "Filter cleared."
		} else {
			m.status = fmt.Sprintf("Filtering repositories by %q", value)
		}
		return m, nil

	case modalRoot:
		root, err := ensureRootPath(value)
		if err != nil {
			m.status = fmt.Sprintf("Unable to use %q: %v", value, err)
			return m, nil
		}

		m.cfg.RootPath = root
		m.cfg.LastOpened = time.Now()
		_ = config.Save(m.cfg)
		return m.startRefresh(fmt.Sprintf("Scanning %s ...", root))

	case modalCommitOnly:
		message := value
		if message == "" {
			message = modal.placeholder
		}
		repo := modal.repo
		m.busy = true
		m.busyLabel = fmt.Sprintf("Committing all changes in %s ...", repo.Name)
		m.status = m.busyLabel
		return m, tea.Batch(
			m.spinner.Tick,
			singleRepoActionCmd("Commit All", repo, func() (string, error) {
				return gitops.CommitAll(repo.Path, message)
			}),
		)

	case modalCommitAndPush:
		message := value
		if message == "" {
			message = modal.placeholder
		}
		repo := modal.repo
		m.busy = true
		m.busyLabel = fmt.Sprintf("Committing and pushing %s ...", repo.Name)
		m.status = m.busyLabel
		return m, tea.Batch(
			m.spinner.Tick,
			singleRepoActionCmd("Commit + Push", repo, func() (string, error) {
				return gitops.CommitAndPush(repo.Path, message)
			}),
		)

	case modalBulkCommit:
		message := value
		if message == "" {
			message = modal.placeholder
		}
		m.busy = true
		m.busyLabel = "Committing and pushing every dirty repository..."
		m.status = m.busyLabel
		return m, tea.Batch(
			m.spinner.Tick,
			bulkCommitPushCmd(m.repos, message),
		)

	case modalClone:
		source := value
		if source == "" {
			m.status = "Clone source cannot be empty."
			return m, nil
		}

		root, err := ensureRootPath(m.cfg.RootPath)
		if err != nil {
			m.status = fmt.Sprintf("Unable to prepare clone root: %v", err)
			return m, nil
		}

		m.cfg.RootPath = root
		_ = config.Save(m.cfg)
		m.busy = true
		m.busyLabel = fmt.Sprintf("Cloning %s into %s ...", source, root)
		m.status = m.busyLabel
		return m, tea.Batch(
			m.spinner.Tick,
			cloneRepoCmd(root, source),
		)
	}

	return m, nil
}

func (m dashboardModel) activateFocused() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusGlobalActions:
		return m.activateGlobalAction()
	case focusRepoList:
		if len(m.filtered) == 0 {
			return m, nil
		}
		m.focus = focusRepoActions
		return m, nil
	case focusRepoActions:
		return m.activateRepoAction()
	}

	return m, nil
}

func (m dashboardModel) activateGlobalAction() (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}

	switch globalAction(m.globalActionIdx) {
	case globalActionChangeRoot:
		m.openModal(modalRoot, "Change Repository Root", "Type the folder that contains the repositories you want to manage.", m.cfg.RootPath, defaultRootPath())
		return m, nil

	case globalActionRefresh:
		return m.startRefresh("Refreshing repositories...")

	case globalActionFilter:
		m.openModal(modalFilter, "Filter Repositories", "Filter by repo name, branch, path, remote, or commit message.", m.filterQuery, "repo name, branch, path...")
		return m, nil

	case globalActionDirtyOnly:
		m.dirtyOnly = !m.dirtyOnly
		m.applyFilters()
		if m.dirtyOnly {
			m.status = "Dirty-only filter enabled."
		} else {
			m.status = "Dirty-only filter disabled."
		}
		return m, nil

	case globalActionClone:
		m.openModal(modalClone, "Clone Repository", "Paste a Git URL or use owner/repo shorthand. GUITboard will clone into the current root folder.", "", "owner/repo")
		return m, nil

	case globalActionFetchAll:
		if !m.hasSyncableRepos() {
			m.status = "No repositories with a remote were found."
			return m, nil
		}
		m.busy = true
		m.busyLabel = "Fetching and fast-forwarding repositories where possible..."
		m.status = m.busyLabel
		return m, tea.Batch(m.spinner.Tick, fetchAllCmd(m.repos))

	case globalActionBulkCommit:
		if !m.hasDirtyRepos() {
			m.status = "No repositories currently have uncommitted changes."
			return m, nil
		}
		m.openModal(modalBulkCommit, "Commit Dirty Repositories", "Type one commit message. GUITboard will stage, commit, and push every dirty repository.", "", defaultCommitMessage())
		return m, nil
	}

	return m, nil
}

func (m dashboardModel) activateRepoAction() (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}

	repo, ok := m.selectedRepo()
	if !ok {
		m.status = "Select a repository first."
		return m, nil
	}

	switch repoAction(m.repoActionIdx) {
	case repoActionStageAll:
		if !repo.Dirty {
			m.status = fmt.Sprintf("%s is already clean.", repo.Name)
			return m, nil
		}
		m.busy = true
		m.busyLabel = fmt.Sprintf("Staging every change in %s ...", repo.Name)
		m.status = m.busyLabel
		return m, tea.Batch(
			m.spinner.Tick,
			singleRepoActionCmd("Stage All", repo, func() (string, error) {
				return gitops.StageAll(repo.Path)
			}),
		)

	case repoActionCommitPush:
		if !repo.Dirty {
			m.status = fmt.Sprintf("%s has no local changes to commit.", repo.Name)
			return m, nil
		}
		m.openModal(modalCommitAndPush, "Quick Commit + Push", fmt.Sprintf("Type a commit message for %s. GUITboard will stage and push the rest for you.", repo.Name), "", defaultCommitMessage())
		return m, nil

	case repoActionCommitOnly:
		if !repo.Dirty {
			m.status = fmt.Sprintf("%s has no local changes to commit.", repo.Name)
			return m, nil
		}
		m.openModal(modalCommitOnly, "Commit All Changes", fmt.Sprintf("Type a commit message for %s. GUITboard will stage everything before committing.", repo.Name), "", defaultCommitMessage())
		return m, nil

	case repoActionPull:
		if repo.Upstream == "" || repo.UpstreamGone {
			m.status = fmt.Sprintf("%s does not have a usable upstream branch.", repo.Name)
			return m, nil
		}
		m.busy = true
		m.busyLabel = fmt.Sprintf("Pulling %s ...", repo.Name)
		m.status = m.busyLabel
		return m, tea.Batch(
			m.spinner.Tick,
			singleRepoActionCmd("Pull", repo, func() (string, error) {
				return gitops.Pull(repo.Path)
			}),
		)

	case repoActionPush:
		if repo.Remote == "" {
			m.status = fmt.Sprintf("%s does not have a remote configured.", repo.Name)
			return m, nil
		}
		m.busy = true
		m.busyLabel = fmt.Sprintf("Pushing %s ...", repo.Name)
		m.status = m.busyLabel
		return m, tea.Batch(
			m.spinner.Tick,
			singleRepoActionCmd("Push", repo, func() (string, error) {
				return gitops.Push(repo.Path)
			}),
		)
	}

	return m, nil
}

func (m dashboardModel) startRefresh(status string) (dashboardModel, tea.Cmd) {
	if strings.TrimSpace(m.cfg.RootPath) == "" {
		m.status = "Set a repository root before refreshing."
		return m, nil
	}

	m.busy = true
	m.busyLabel = status
	m.status = status
	return m, tea.Batch(m.spinner.Tick, scanReposCmd(m.cfg.RootPath))
}

func (m *dashboardModel) openModal(kind modalKind, title, prompt, value, placeholder string) {
	input := textinput.New()
	input.Prompt = "> "
	input.SetValue(value)
	input.Placeholder = placeholder
	input.CharLimit = 240
	input.Width = m.modalInputWidth()
	input.Focus()

	m.modal = &modalState{
		kind:        kind,
		title:       title,
		prompt:      prompt,
		placeholder: placeholder,
		input:       input,
	}
}

func (m dashboardModel) modalInputWidth() int {
	return maxInt(12, minInt(56, m.width-14))
}

func (m *dashboardModel) applyFilters() {
	query := strings.TrimSpace(strings.ToLower(m.filterQuery))

	filtered := make([]gitops.Repo, 0, len(m.repos))
	for _, repo := range m.repos {
		if m.dirtyOnly && !repo.Dirty {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				repo.Name,
				repo.Path,
				repo.Branch,
				repo.Remote,
				repo.LastCommitMessage,
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		filtered = append(filtered, repo)
	}

	m.filtered = filtered
	m.restoreSelection()
	m.ensureRepoVisible()
}

func (m *dashboardModel) restoreSelection() {
	if len(m.filtered) == 0 {
		m.selectedIdx = -1
		m.selectedPath = ""
		m.repoScroll = 0
		if m.focus == focusRepoActions {
			m.focus = focusRepoList
		}
		return
	}

	if m.selectedPath != "" {
		for idx, repo := range m.filtered {
			if repo.Path == m.selectedPath {
				m.selectedIdx = idx
				return
			}
		}
	}

	m.selectedIdx = 0
	m.selectedPath = m.filtered[0].Path
}

func (m *dashboardModel) moveLeft() {
	switch m.focus {
	case focusGlobalActions:
		if m.globalActionIdx > 0 {
			m.globalActionIdx--
		}
	case focusRepoList:
		m.focus = focusGlobalActions
	case focusRepoActions:
		if m.repoActionIdx > 0 {
			m.repoActionIdx--
		}
	}
}

func (m *dashboardModel) moveRight() {
	switch m.focus {
	case focusGlobalActions:
		if m.globalActionIdx < len(m.globalActionButtons())-1 {
			m.globalActionIdx++
		}
	case focusRepoList:
		if len(m.filtered) > 0 {
			m.focus = focusRepoActions
		}
	case focusRepoActions:
		if m.repoActionIdx < len(m.repoActionButtons())-1 {
			m.repoActionIdx++
		}
	}
}

func (m *dashboardModel) moveUp() {
	switch m.focus {
	case focusGlobalActions:
		return
	case focusRepoList:
		if m.selectedIdx > 0 {
			m.selectedIdx--
			m.selectedPath = m.filtered[m.selectedIdx].Path
			m.ensureRepoVisible()
		}
	case focusRepoActions:
		m.focus = focusRepoList
	}
}

func (m *dashboardModel) moveDown() {
	switch m.focus {
	case focusGlobalActions:
		if len(m.filtered) > 0 {
			m.focus = focusRepoList
		}
	case focusRepoList:
		if m.selectedIdx >= 0 && m.selectedIdx < len(m.filtered)-1 {
			m.selectedIdx++
			m.selectedPath = m.filtered[m.selectedIdx].Path
			m.ensureRepoVisible()
		}
	case focusRepoActions:
		return
	}
}

func (m dashboardModel) nextFocus() focusArea {
	switch m.focus {
	case focusGlobalActions:
		return focusRepoList
	case focusRepoList:
		if len(m.filtered) > 0 {
			return focusRepoActions
		}
		return focusGlobalActions
	default:
		return focusGlobalActions
	}
}

func (m dashboardModel) previousFocus() focusArea {
	switch m.focus {
	case focusRepoActions:
		return focusRepoList
	case focusRepoList:
		return focusGlobalActions
	default:
		if len(m.filtered) > 0 {
			return focusRepoActions
		}
		return focusRepoList
	}
}

func (m *dashboardModel) ensureRepoVisible() {
	if m.selectedIdx < 0 {
		m.repoScroll = 0
		return
	}

	visible := m.visibleRepoCount()
	if visible <= 0 {
		return
	}

	if m.selectedIdx < m.repoScroll {
		m.repoScroll = m.selectedIdx
	}
	if m.selectedIdx >= m.repoScroll+visible {
		m.repoScroll = m.selectedIdx - visible + 1
	}
	if m.repoScroll < 0 {
		m.repoScroll = 0
	}
}

func (m dashboardModel) selectedRepo() (gitops.Repo, bool) {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.filtered) {
		repo := m.filtered[m.selectedIdx]
		m.selectedPath = repo.Path
		return repo, true
	}

	if m.selectedPath == "" {
		return gitops.Repo{}, false
	}

	for _, repo := range m.repos {
		if repo.Path == m.selectedPath {
			return repo, true
		}
	}

	return gitops.Repo{}, false
}

func (m *dashboardModel) appendLogs(blocks []string) {
	if len(blocks) == 0 {
		return
	}

	m.logs = append(blocks, m.logs...)
	if len(m.logs) > 80 {
		m.logs = m.logs[:80]
	}
}

func (m dashboardModel) autoRefreshInterval() time.Duration {
	seconds := m.cfg.AutoRefreshSeconds
	if seconds < 10 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func refreshTickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

func scanReposCmd(root string) tea.Cmd {
	root = strings.TrimSpace(root)
	return func() tea.Msg {
		repos, err := gitops.Scan(root)
		return scanFinishedMsg{repos: repos, err: err}
	}
}

func singleRepoActionCmd(title string, repo gitops.Repo, action func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		output, err := action()
		status := fmt.Sprintf("%s finished for %s", title, repo.Name)
		if err != nil {
			status = fmt.Sprintf("%s failed for %s", title, repo.Name)
		}
		return operationFinishedMsg{
			status:       status,
			logs:         []string{formatLogBlock(title, repo, output, err)},
			refresh:      true,
			selectedPath: repo.Path,
		}
	}
}

func fetchAllCmd(repos []gitops.Repo) tea.Cmd {
	snapshot := append([]gitops.Repo(nil), repos...)
	return func() tea.Msg {
		logs := make([]string, 0, len(snapshot))
		processed := 0
		failures := 0

		for _, repo := range snapshot {
			if strings.TrimSpace(repo.Remote) == "" {
				continue
			}
			processed++
			output, err := gitops.Sync(repo)
			logs = append(logs, formatLogBlock("Fetch All", repo, output, err))
			if err != nil {
				failures++
			}
		}

		switch {
		case processed == 0:
			return operationFinishedMsg{status: "No repositories with a remote were found."}
		case failures > 0:
			return operationFinishedMsg{
				status:  fmt.Sprintf("Fetch all completed with %d failures.", failures),
				logs:    logs,
				refresh: true,
			}
		default:
			return operationFinishedMsg{
				status:  fmt.Sprintf("Fetch all completed across %d repositories.", processed),
				logs:    logs,
				refresh: true,
			}
		}
	}
}

func bulkCommitPushCmd(repos []gitops.Repo, message string) tea.Cmd {
	snapshot := append([]gitops.Repo(nil), repos...)
	return func() tea.Msg {
		logs := make([]string, 0, len(snapshot))
		processed := 0
		failures := 0

		for _, repo := range snapshot {
			if !repo.Dirty {
				continue
			}
			processed++
			output, err := gitops.CommitAndPush(repo.Path, message)
			logs = append(logs, formatLogBlock("Commit + Push", repo, output, err))
			if err != nil {
				failures++
			}
		}

		switch {
		case processed == 0:
			return operationFinishedMsg{status: "No repositories currently have local changes."}
		case failures > 0:
			return operationFinishedMsg{
				status:  fmt.Sprintf("Commit + push finished with %d failures.", failures),
				logs:    logs,
				refresh: true,
			}
		default:
			return operationFinishedMsg{
				status:  fmt.Sprintf("Commit + push finished across %d repositories.", processed),
				logs:    logs,
				refresh: true,
			}
		}
	}
}

func cloneRepoCmd(root, source string) tea.Cmd {
	root = strings.TrimSpace(root)
	source = strings.TrimSpace(source)

	return func() tea.Msg {
		targetPath, output, err := gitops.Clone(source, root, "")

		repoName := gitops.DefaultCloneDirName(source)
		if repoName == "" && targetPath != "" {
			repoName = filepath.Base(targetPath)
		}
		if repoName == "" {
			repoName = source
		}

		repo := gitops.Repo{
			Name: repoName,
			Path: targetPath,
		}

		status := fmt.Sprintf("Clone finished for %s", repoName)
		if err != nil {
			status = fmt.Sprintf("Clone failed for %s", repoName)
		}

		return operationFinishedMsg{
			status:       status,
			logs:         []string{formatLogBlock("Clone", repo, output, err)},
			refresh:      true,
			selectedPath: targetPath,
		}
	}
}

func (m dashboardModel) View() string {
	base := ""
	if m.isCompactLayout() {
		base = m.renderCompactView()
	} else {
		base = m.renderFullView()
	}

	if m.modal != nil {
		return overlayCentered(base, m.renderModal(), m.width, m.height)
	}

	return base
}

func (m dashboardModel) renderFullView() string {
	header := m.renderHeader()
	actions := renderPanel("Dashboard Actions", m.renderGlobalActionRows(m.width), m.width)
	stats := m.renderSummaryRow()

	layout := m.layout()
	leftPanel := renderPanel("Repositories", m.renderRepoList(layout.leftWidth), layout.leftWidth)
	rightBody := lipgloss.JoinVertical(
		lipgloss.Left,
		renderPanel("Selected Repository", m.renderSelectedRepo(layout.rightWidth), layout.rightWidth),
		renderPanel("Repo Actions", m.renderRepoActionRows(layout.rightWidth), layout.rightWidth),
		renderPanel("Operation Log", m.renderLogBlocks(layout.logLines), layout.rightWidth),
	)
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightBody)

	footer := styles.help.Render("Arrows move the dashboard. Enter runs the highlighted action. / filters, d toggles dirty-only, r refreshes, q quits.")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		actions,
		stats,
		body,
		footer,
	)
}

func (m dashboardModel) renderCompactView() string {
	width := maxInt(20, m.width)

	topLines := []string{
		m.renderCompactTitleLine(width),
		m.renderCompactContextLine(width),
	}

	actionLines := splitNonEmptyLines(m.renderWrappedActions(m.globalActionButtons(), m.globalActionIdx, m.focus == focusGlobalActions, width))
	topLines = append(topLines, actionLines...)

	total, dirty, ahead, behind := m.summaryCounts()
	summaryLine := fmt.Sprintf("Repos %d | Dirty %d | Ahead %d | Behind %d", total, dirty, ahead, behind)
	topLines = append(topLines, styles.mutedText.Render(truncateText(summaryLine, width)))
	topLines = append(topLines, strings.Repeat("─", width))

	maxBottom := maxInt(0, m.height-len(topLines)-3)
	bottomLines := m.renderCompactBottom(maxBottom, width)
	repoLines := maxInt(3, m.height-len(topLines)-len(bottomLines))
	repoSection := m.renderCompactRepoSection(width, repoLines)

	lines := append([]string{}, topLines...)
	lines = append(lines, repoSection...)
	lines = append(lines, bottomLines...)

	if len(lines) > m.height {
		lines = lines[:m.height]
	}

	return strings.Join(lines, "\n")
}

func (m dashboardModel) renderCompactTitleLine(width int) string {
	status := m.status
	if m.busy {
		status = fmt.Sprintf("%s %s", m.spinner.View(), m.busyLabel)
	}

	line := fmt.Sprintf("GUITboard | %s", status)
	return styles.title.Render(truncateText(line, width))
}

func (m dashboardModel) renderCompactContextLine(width int) string {
	context := fmt.Sprintf(
		"Root %s | Filter %s | %s",
		valueOrDash(m.cfg.RootPath),
		valueOrDefault(m.filterQuery, "all"),
		dirtyModeLabel(m.dirtyOnly),
	)
	return styles.headerText.Render(truncateText(context, width))
}

func (m dashboardModel) renderCompactRepoSection(width, maxLines int) []string {
	lines := []string{
		styles.mutedText.Render(
			truncateText(
				fmt.Sprintf("%d shown of %d total repositories", len(m.filtered), len(m.repos)),
				width,
			),
		),
	}

	if maxLines <= 1 {
		return lines[:minInt(len(lines), maxLines)]
	}

	if len(m.filtered) == 0 {
		lines = append(lines, styles.emptyState.Render(truncateText("No repositories match the current root and filter settings.", width)))
		return lines[:minInt(len(lines), maxLines)]
	}

	rows := maxLines - len(lines)
	if rows <= 0 {
		return lines[:minInt(len(lines), maxLines)]
	}

	start := minInt(m.repoScroll, maxInt(0, len(m.filtered)-rows))
	end := minInt(len(m.filtered), start+rows)

	for idx := start; idx < end; idx++ {
		showMore := idx == end-1 && end < len(m.filtered) && rows > 1
		if showMore {
			lines = append(lines, styles.mutedText.Render(truncateText(fmt.Sprintf("... %d more repositories", len(m.filtered)-end+1), width)))
			break
		}

		repo := m.filtered[idx]
		lines = append(lines, m.renderCompactRepoRow(repo, idx == m.selectedIdx, width))
	}

	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	return lines
}

func (m dashboardModel) renderCompactRepoRow(repo gitops.Repo, selected bool, width int) string {
	statusToken := "clean"
	if repo.Dirty {
		statusToken = fmt.Sprintf("%dchg", repo.ChangedCount)
	}
	aheadBehind := ""
	if repo.Ahead > 0 || repo.Behind > 0 {
		aheadBehind = fmt.Sprintf(" ↑%d ↓%d", repo.Ahead, repo.Behind)
	}

	line := fmt.Sprintf("%s %s  %s  %s%s", selectionMarker(selected), repo.Name, compactBranch(repo.Branch), statusToken, aheadBehind)
	line = truncateText(line, width)

	switch {
	case selected && m.focus == focusRepoList:
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#102A43", Dark: "#E6EEF7"}).Background(lipgloss.AdaptiveColor{Light: "#DCEEF8", Dark: "#143042"}).Width(width).Render(line)
	case selected:
		return lipgloss.NewStyle().Bold(true).Width(width).Render(line)
	default:
		return styles.detailValue.Width(width).Render(line)
	}
}

func (m dashboardModel) renderCompactBottom(maxLines, width int) []string {
	if maxLines <= 0 {
		return nil
	}

	var lines []string
	repo, ok := m.selectedRepo()
	if ok {
		lines = append(lines, strings.Repeat("─", width))
		lines = append(lines, truncateText(fmt.Sprintf("Selected %s | %s | %s", repo.Name, compactBranch(repo.Branch), compactChangeStatus(repo)), width))

		if maxLines-len(lines) > 0 {
			lines = append(lines, truncateText("Path "+repo.Path, width))
		}
		if maxLines-len(lines) > 0 && width >= 48 {
			lines = append(lines, truncateText("Last "+lastCommitSummary(repo), width))
		}
	}

	if remaining := maxLines - len(lines); remaining > 0 {
		actionLines := splitNonEmptyLines(m.renderWrappedActions(m.repoActionButtons(), m.repoActionIdx, m.focus == focusRepoActions, width))
		if len(actionLines) > remaining {
			actionLines = actionLines[:remaining]
		}
		lines = append(lines, actionLines...)
	}

	if remaining := maxLines - len(lines); remaining > 0 {
		lines = append(lines, truncateText("Log "+m.compactLogPreview(), width))
	}

	if remaining := maxLines - len(lines); remaining > 0 && m.height >= 16 {
		lines = append(lines, styles.help.Render(truncateText("Arrows move. Enter runs. / filter. d dirty. r refresh. q quit.", width)))
	}

	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	return lines
}

func (m dashboardModel) compactLogPreview() string {
	if len(m.logs) == 0 {
		return "No git operations yet."
	}

	first := strings.Split(m.logs[0], "\n")
	if len(first) == 0 {
		return "No git operations yet."
	}

	return first[0]
}

func (m dashboardModel) isCompactLayout() bool {
	return m.width < 96 || m.height < 28
}

type dashboardLayout struct {
	leftWidth  int
	rightWidth int
	logLines   int
}

func (m dashboardModel) layout() dashboardLayout {
	leftWidth := maxInt(38, minInt(52, m.width*42/100))
	rightWidth := maxInt(42, m.width-leftWidth-1)
	logLines := maxInt(8, m.height-26)

	return dashboardLayout{
		leftWidth:  leftWidth,
		rightWidth: rightWidth,
		logLines:   logLines,
	}
}

func (m dashboardModel) renderHeader() string {
	status := m.status
	if m.busy {
		status = fmt.Sprintf("%s %s", m.spinner.View(), m.busyLabel)
	}

	line1 := lipgloss.JoinHorizontal(
		lipgloss.Center,
		styles.title.Render("GUITboard"),
		styles.headerDivider.Render("terminal dashboard"),
	)

	context := fmt.Sprintf(
		"Root: %s  |  Filter: %s  |  Mode: %s  |  Status: %s",
		valueOrDash(m.cfg.RootPath),
		valueOrDefault(m.filterQuery, "all repositories"),
		dirtyModeLabel(m.dirtyOnly),
		status,
	)

	return renderPanel("", line1+"\n"+styles.headerText.Render(truncateText(context, m.width-8)), m.width)
}

func (m dashboardModel) renderSummaryRow() string {
	total, dirty, ahead, behind := m.summaryCounts()
	cardWidth := maxInt(18, (m.width-3)/4)

	cards := []string{
		renderStatCard("Repositories", fmt.Sprintf("%d", total), cardWidth),
		renderStatCard("Dirty", fmt.Sprintf("%d", dirty), cardWidth),
		renderStatCard("Ahead", fmt.Sprintf("%d", ahead), cardWidth),
		renderStatCard("Behind", fmt.Sprintf("%d", behind), cardWidth),
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

func (m dashboardModel) renderGlobalActionRows(width int) string {
	buttons := m.globalActionButtons()
	maxWidth := maxInt(24, width-styles.panel.GetHorizontalFrameSize())
	return m.renderWrappedActions(buttons, m.globalActionIdx, m.focus == focusGlobalActions, maxWidth)
}

func (m dashboardModel) renderRepoActionRows(width int) string {
	buttons := m.repoActionButtons()
	maxWidth := maxInt(24, width-styles.panel.GetHorizontalFrameSize())
	return m.renderWrappedActions(buttons, m.repoActionIdx, m.focus == focusRepoActions, maxWidth)
}

func (m dashboardModel) renderWrappedActions(buttons []actionButton, selectedIndex int, focused bool, maxWidth int) string {
	lines := make([]string, 0, 2)
	currentLine := ""

	for idx, button := range buttons {
		rendered := renderActionButton(button, idx == selectedIndex && focused, focused)
		candidate := rendered
		if currentLine != "" {
			candidate = currentLine + " " + rendered
		}

		if currentLine != "" && lipgloss.Width(candidate) > maxWidth {
			lines = append(lines, currentLine)
			currentLine = rendered
			continue
		}

		currentLine = candidate
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n")
}

func (m dashboardModel) renderRepoList(width int) string {
	innerWidth := width - styles.panel.GetHorizontalFrameSize()
	if innerWidth < 20 {
		innerWidth = 20
	}

	info := fmt.Sprintf(
		"%d shown of %d total repositories",
		len(m.filtered),
		len(m.repos),
	)
	if m.dirtyOnly {
		info += " | dirty only"
	}
	if m.filterQuery != "" {
		info += " | filter active"
	}

	lines := []string{styles.sectionInfo.Render(info)}
	if len(m.filtered) == 0 {
		lines = append(lines, styles.emptyState.Render("No repositories match the current root and filter settings."))
		return strings.Join(lines, "\n\n")
	}

	visible := m.visibleRepoCount()
	start := minInt(m.repoScroll, maxInt(0, len(m.filtered)-visible))
	end := minInt(len(m.filtered), start+visible)

	for idx := start; idx < end; idx++ {
		repo := m.filtered[idx]
		selected := idx == m.selectedIdx
		lines = append(lines, m.renderRepoRow(repo, selected, innerWidth))
	}

	if end < len(m.filtered) {
		lines = append(lines, styles.sectionInfo.Render(fmt.Sprintf("More repositories below: %d", len(m.filtered)-end)))
	}

	return strings.Join(lines, "\n")
}

func (m dashboardModel) renderRepoRow(repo gitops.Repo, selected bool, width int) string {
	chips := []string{
		renderBadge(branchBadgeLabel(repo), styles.badgeMuted),
	}
	if repo.Dirty {
		chips = append(chips, renderBadge(fmt.Sprintf("%d changed", repo.ChangedCount), styles.badgeDirty))
	} else {
		chips = append(chips, renderBadge("clean", styles.badgeClean))
	}
	if repo.Ahead > 0 {
		chips = append(chips, renderBadge(fmt.Sprintf("ahead %d", repo.Ahead), styles.badgeAhead))
	}
	if repo.Behind > 0 {
		chips = append(chips, renderBadge(fmt.Sprintf("behind %d", repo.Behind), styles.badgeBehind))
	}

	nameLine := truncateText(repo.Name, maxInt(16, width-2))
	metaLine := truncateText(strings.Join([]string{repo.Path, formatTime(repo.LastActivity)}, "  |  "), maxInt(16, width-2))
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		nameLine+"  "+strings.Join(chips, " "),
		styles.mutedText.Render(metaLine),
	)

	if selected {
		return styles.repoSelected.Width(width).Render(content)
	}
	return styles.repoRow.Width(width).Render(content)
}

func (m dashboardModel) renderSelectedRepo(width int) string {
	repo, ok := m.selectedRepo()
	if !ok {
		return styles.emptyState.Render("Select a repository from the list to inspect it and run actions.")
	}

	lines := []string{
		styles.repoTitle.Render(repo.Name),
		renderDetailLine("Path", repo.Path, width),
		renderDetailLine("Branch", branchSummary(repo), width),
		renderDetailLine("Remote", valueOrDash(repo.Remote), width),
		renderDetailLine("Last Commit", lastCommitSummary(repo), width),
		renderDetailLine("Activity", formatTime(repo.LastActivity), width),
		renderDetailLine("Status", statusSummary(repo), width),
	}

	return strings.Join(lines, "\n")
}

func (m dashboardModel) renderLogBlocks(maxLines int) string {
	if len(m.logs) == 0 {
		return styles.emptyState.Render("No git operations have been run yet.")
	}

	var blocks []string
	linesLeft := maxLines

	for _, block := range m.logs {
		blockLines := strings.Split(block, "\n")
		if len(blockLines) > linesLeft {
			if linesLeft <= 0 {
				break
			}
			blocks = append(blocks, strings.Join(blockLines[:linesLeft], "\n"))
			linesLeft = 0
			break
		}

		blocks = append(blocks, block)
		linesLeft -= len(blockLines)
		if linesLeft <= 1 {
			break
		}
		linesLeft--
	}

	return strings.Join(blocks, "\n\n")
}

func (m dashboardModel) renderModal() string {
	if m.modal == nil {
		return ""
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.modalTitle.Render(m.modal.title),
		styles.modalText.Render(m.modal.prompt),
		styles.modalInput.Render(m.modal.input.View()),
		styles.help.Render("Enter submits. Esc closes the popup."),
	)

	availableWidth := m.width - 4
	if m.isCompactLayout() {
		availableWidth = m.width - 10
	}

	return styles.modalBox.Width(minInt(84, maxInt(20, availableWidth))).Render(body)
}

func (m dashboardModel) globalActionButtons() []actionButton {
	return []actionButton{
		{label: "Root", enabled: !m.busy},
		{label: "Refresh", enabled: !m.busy},
		{label: "Filter", enabled: !m.busy},
		{label: fmt.Sprintf("Dirty: %s", onOffLabel(m.dirtyOnly)), enabled: !m.busy},
		{label: "Clone", enabled: !m.busy},
		{label: "Fetch All", enabled: !m.busy && m.hasSyncableRepos()},
		{label: "Commit Dirty", enabled: !m.busy && m.hasDirtyRepos()},
	}
}

func (m dashboardModel) repoActionButtons() []actionButton {
	repo, selected := m.selectedRepo()
	return []actionButton{
		{label: "Stage", enabled: selected && !m.busy && repo.Dirty},
		{label: "Commit+Push", enabled: selected && !m.busy && repo.Dirty},
		{label: "Commit", enabled: selected && !m.busy && repo.Dirty},
		{label: "Pull", enabled: selected && !m.busy && repo.Upstream != "" && !repo.UpstreamGone},
		{label: "Push", enabled: selected && !m.busy && repo.Remote != ""},
	}
}

func (m dashboardModel) summaryCounts() (int, int, int, int) {
	dirty := 0
	ahead := 0
	behind := 0

	for _, repo := range m.repos {
		if repo.Dirty {
			dirty++
		}
		if repo.Ahead > 0 {
			ahead++
		}
		if repo.Behind > 0 {
			behind++
		}
	}

	return len(m.repos), dirty, ahead, behind
}

func (m dashboardModel) hasDirtyRepos() bool {
	for _, repo := range m.repos {
		if repo.Dirty {
			return true
		}
	}
	return false
}

func (m dashboardModel) hasSyncableRepos() bool {
	for _, repo := range m.repos {
		if strings.TrimSpace(repo.Remote) != "" {
			return true
		}
	}
	return false
}

func (m dashboardModel) visibleRepoCount() int {
	if m.isCompactLayout() {
		reserved := 10
		if m.height < 20 {
			reserved = 8
		}
		return maxInt(3, m.height-reserved)
	}

	return maxInt(5, (m.height-17)/3)
}

func renderPanel(title, body string, width int) string {
	panelWidth := maxInt(24, width)
	innerWidth := panelWidth - styles.panel.GetHorizontalFrameSize()
	if innerWidth < 10 {
		innerWidth = 10
	}

	lines := make([]string, 0, 2)
	if strings.TrimSpace(title) != "" {
		lines = append(lines, styles.panelTitle.Width(innerWidth).Render(title))
	}
	lines = append(lines, lipgloss.NewStyle().Width(innerWidth).Render(body))

	return styles.panel.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func renderStatCard(label, value string, width int) string {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.statLabel.Render(label),
		styles.statValue.Render(value),
	)
	return styles.statCard.Width(width).Render(body)
}

func renderActionButton(button actionButton, selected bool, focused bool) string {
	switch {
	case !button.enabled:
		return styles.actionDisabled.Render(button.label)
	case selected && focused:
		return styles.actionActive.Render(button.label)
	case selected:
		return styles.actionSelected.Render(button.label)
	default:
		return styles.actionButton.Render(button.label)
	}
}

func renderBadge(label string, style lipgloss.Style) string {
	return style.Render(label)
}

func renderDetailLine(label, value string, width int) string {
	maxWidth := maxInt(16, width-styles.panel.GetHorizontalFrameSize()-15)
	return styles.detailLabel.Render(label+":") + " " + styles.detailValue.Render(truncateText(value, maxWidth))
}

func overlayCentered(base, overlay string, width, height int) string {
	width = maxInt(1, width)
	height = maxInt(1, height)

	baseLines := normalizeViewportLines(base, width, height)
	overlayLines := strings.Split(overlay, "\n")
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := len(overlayLines)

	if overlayWidth <= 0 || overlayHeight <= 0 {
		return strings.Join(baseLines, "\n")
	}

	x := 0
	y := 0
	if width > overlayWidth {
		x = (width - overlayWidth) / 2
	}
	if height > overlayHeight {
		y = (height - overlayHeight) / 2
	}

	for idx, overlayLine := range overlayLines {
		target := y + idx
		if target < 0 || target >= len(baseLines) {
			continue
		}

		line := baseLines[target]
		left := ansi.Cut(line, 0, x)
		rightStart := minInt(width, x+overlayWidth)
		right := ansi.Cut(line, rightStart, width)
		baseLines[target] = left + overlayLine + right
	}

	return strings.Join(baseLines, "\n")
}

func normalizeViewportLines(view string, width, height int) []string {
	width = maxInt(1, width)
	height = maxInt(1, height)

	source := strings.Split(view, "\n")
	lines := make([]string, 0, height)

	for idx := 0; idx < len(source) && idx < height; idx++ {
		lines = append(lines, fitANSIWidth(source[idx], width))
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return lines
}

func fitANSIWidth(value string, width int) string {
	current := ansi.StringWidth(value)
	switch {
	case current == width:
		return value
	case current > width:
		return ansi.Cut(value, 0, width)
	default:
		return value + strings.Repeat(" ", width-current)
	}
}

func splitNonEmptyLines(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	lines := strings.Split(value, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		filtered = append(filtered, line)
	}

	return filtered
}

func selectionMarker(selected bool) string {
	if selected {
		return ">"
	}
	return " "
}

func compactBranch(branch string) string {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

func compactRepoStatus(repo gitops.Repo) string {
	parts := []string{compactBranch(repo.Branch)}
	if repo.Dirty {
		parts = append(parts, fmt.Sprintf("%d changed", repo.ChangedCount))
	} else {
		parts = append(parts, "clean")
	}
	if repo.Ahead > 0 || repo.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↑%d ↓%d", repo.Ahead, repo.Behind))
	}
	return strings.Join(parts, " | ")
}

func compactChangeStatus(repo gitops.Repo) string {
	parts := []string{}
	if repo.Dirty {
		parts = append(parts, fmt.Sprintf("%d changed", repo.ChangedCount))
	} else {
		parts = append(parts, "clean")
	}
	if repo.Ahead > 0 || repo.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↑%d ↓%d", repo.Ahead, repo.Behind))
	}
	return strings.Join(parts, " | ")
}

func defaultCommitMessage() string {
	return "Dashboard sync " + time.Now().Format("2006-01-02 15:04")
}

func ensureRootPath(raw string) (string, error) {
	root := strings.TrimSpace(raw)
	if root == "" {
		root = defaultRootPath()
	}
	if root == "" {
		return "", fmt.Errorf("unable to determine a repository root")
	}

	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	return root, nil
}

func defaultRootPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	candidates := rootPathCandidates(home)
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	return candidates[0]
}

func rootPathCandidates(home string) []string {
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(home, "Documents", "GitHub"),
			filepath.Join(home, "source", "repos"),
			filepath.Join(home, "GitHub"),
		}
	}

	return []string{
		filepath.Join(home, "Documents", "github"),
		filepath.Join(home, "Documents", "GitHub"),
		filepath.Join(home, "github"),
		filepath.Join(home, "GitHub"),
	}
}

func branchBadgeLabel(repo gitops.Repo) string {
	branch := strings.TrimSpace(repo.Branch)
	if branch == "" {
		return "(unknown)"
	}
	return branch
}

func branchSummary(repo gitops.Repo) string {
	parts := []string{valueOrDash(repo.Branch)}
	if repo.Upstream != "" {
		parts = append(parts, "tracking "+repo.Upstream)
	}
	if repo.UpstreamGone {
		parts = append(parts, "upstream missing")
	}
	if repo.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("ahead %d", repo.Ahead))
	}
	if repo.Behind > 0 {
		parts = append(parts, fmt.Sprintf("behind %d", repo.Behind))
	}
	return strings.Join(parts, " | ")
}

func statusSummary(repo gitops.Repo) string {
	if repo.LastError != "" {
		return repo.LastError
	}
	if !repo.Dirty {
		return "Working tree clean"
	}

	parts := []string{fmt.Sprintf("%d total changed", repo.ChangedCount)}
	if repo.StagedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", repo.StagedCount))
	}
	if repo.UnstagedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", repo.UnstagedCount))
	}
	if repo.UntrackedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", repo.UntrackedCount))
	}
	if repo.ConflictedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d conflicted", repo.ConflictedCount))
	}

	return strings.Join(parts, " | ")
}

func lastCommitSummary(repo gitops.Repo) string {
	if repo.LastCommitTime.IsZero() {
		return repo.LastCommitMessage
	}
	return fmt.Sprintf("%s (%s)", repo.LastCommitMessage, formatTime(repo.LastCommitTime))
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04")
}

func formatLogBlock(title string, repo gitops.Repo, output string, err error) string {
	lines := []string{
		fmt.Sprintf("[%s] %s  %s", time.Now().Format("2006-01-02 15:04:05"), title, valueOrDash(repo.Name)),
	}

	if repo.Path != "" {
		lines = append(lines, repo.Path)
	}
	if output != "" {
		lines = append(lines, output)
	}
	if err != nil {
		lines = append(lines, "ERROR: "+err.Error())
	} else {
		lines = append(lines, "Completed successfully.")
	}

	return strings.Join(lines, "\n")
}

func dirtyModeLabel(enabled bool) string {
	if enabled {
		return "dirty only"
	}
	return "all repos"
}

func onOffLabel(enabled bool) string {
	if enabled {
		return "On"
	}
	return "Off"
}

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
