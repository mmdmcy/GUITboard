package ui

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"guitboard/internal/config"
	"guitboard/internal/gitops"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type cloneBrowser struct {
	dashboard *dashboard
	win       fyne.Window

	root             string
	repos            []gitops.CloneableRepo
	filtered         []gitops.CloneableRepo
	selectedFullName string
	protocol         gitops.CloneProtocol
	autoFolderName   string
	loading          bool
	closed           bool
	localRepoIndex   map[string]string

	rootLabel        *widget.Label
	resultLabel      *widget.Label
	statusLabel      *widget.Label
	searchEntry      *widget.Entry
	sourceEntry      *widget.Entry
	folderEntry      *widget.Entry
	detailName       *widget.Label
	detailDesc       *widget.Label
	detailVisibility *widget.Label
	detailBranch     *widget.Label
	detailUpdated    *widget.Label
	detailLocal      *widget.Label
	progress         *widget.ProgressBarInfinite
	protocolGroup    *widget.RadioGroup
	list             *widget.List
	refreshButton    *widget.Button
	changeRootButton *widget.Button
	cloneButton      *widget.Button
}

func newCloneBrowser(d *dashboard, root string) *cloneBrowser {
	win := d.app.NewWindow("Browse GitHub Repositories")

	browser := &cloneBrowser{
		dashboard:      d,
		win:            win,
		root:           root,
		protocol:       defaultCloneProtocol(),
		localRepoIndex: buildLocalRepoIndex(d.repos),
	}

	browser.buildUI()
	win.SetContent(browser.content())
	win.Resize(fyne.NewSize(1160, 760))
	win.CenterOnScreen()
	win.SetOnClosed(func() {
		browser.closed = true
		if d.cloneWindow == win {
			d.cloneWindow = nil
		}
	})

	return browser
}

func (b *cloneBrowser) show() {
	b.win.Show()
	b.win.RequestFocus()
	b.loadRepos()
}

func (b *cloneBrowser) buildUI() {
	b.rootLabel = widget.NewLabel(b.root)
	b.rootLabel.Wrapping = fyne.TextWrapWord

	b.resultLabel = widget.NewLabel("Checking GitHub...")
	b.resultLabel.Wrapping = fyne.TextWrapWord

	b.statusLabel = widget.NewLabel("Loading repositories from GitHub via GitHub CLI...")
	b.statusLabel.Wrapping = fyne.TextWrapWord

	b.searchEntry = widget.NewEntry()
	b.searchEntry.SetPlaceHolder("Search by owner, repo name, description, branch, or visibility...")
	b.searchEntry.OnChanged = func(string) {
		b.applyFilter()
	}

	b.sourceEntry = widget.NewEntry()
	b.sourceEntry.SetPlaceHolder("git@github.com:owner/repo.git or https://github.com/owner/repo.git")
	b.sourceEntry.OnChanged = func(string) {
		b.updateCloneActionState()
	}

	b.folderEntry = widget.NewEntry()
	b.folderEntry.SetPlaceHolder("Optional. Defaults to the repository name")
	b.folderEntry.OnChanged = func(string) {
		b.updateCloneActionState()
	}

	b.detailName = widget.NewLabel("Select a repository or paste a custom source")
	b.detailName.TextStyle = fyne.TextStyle{Bold: true}
	b.detailName.Wrapping = fyne.TextWrapWord

	b.detailDesc = widget.NewLabel("GUITboard can browse the repositories your GitHub account can clone, then fill the source automatically. You can still paste a custom Git URL if needed.")
	b.detailDesc.Wrapping = fyne.TextWrapWord

	b.detailVisibility = widget.NewLabel("-")
	b.detailVisibility.Wrapping = fyne.TextWrapWord

	b.detailBranch = widget.NewLabel("-")
	b.detailBranch.Wrapping = fyne.TextWrapWord

	b.detailUpdated = widget.NewLabel("-")
	b.detailUpdated.Wrapping = fyne.TextWrapWord

	b.detailLocal = widget.NewLabel("Not checked yet")
	b.detailLocal.Wrapping = fyne.TextWrapWord

	b.progress = widget.NewProgressBarInfinite()
	b.progress.Hide()

	b.protocolGroup = widget.NewRadioGroup([]string{"SSH", "HTTPS"}, func(value string) {
		b.protocol = cloneProtocolForLabel(value)
		b.syncSourceFromSelection()
	})
	b.protocolGroup.Horizontal = true
	b.protocolGroup.SetSelected(protocolLabel(b.protocol))

	b.list = widget.NewList(
		func() int {
			return len(b.filtered)
		},
		func() fyne.CanvasObject {
			name := widget.NewLabel("owner/repo")
			name.TextStyle = fyne.TextStyle{Bold: true}
			name.Wrapping = fyne.TextWrapWord

			description := widget.NewLabel("Description")
			description.Wrapping = fyne.TextWrapWord

			meta := widget.NewLabel("visibility | updated | local")
			meta.Wrapping = fyne.TextWrapWord

			return container.NewVBox(name, description, meta)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id < 0 || id >= len(b.filtered) {
				return
			}

			repo := b.filtered[id]
			objects := item.(*fyne.Container).Objects

			nameLabel := objects[0].(*widget.Label)
			descLabel := objects[1].(*widget.Label)
			metaLabel := objects[2].(*widget.Label)

			nameLabel.SetText(repo.FullName)
			descLabel.SetText(valueOrDefault(repo.Description, "No description provided."))
			metaLabel.SetText(cloneRepoListSummary(repo, b.localPathForRepo(repo)))
		},
	)
	b.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(b.filtered) {
			return
		}

		b.selectedFullName = b.filtered[id].FullName
		b.refreshSelection(true)
	}

	b.refreshButton = widget.NewButtonWithIcon("Refresh GitHub List", theme.ViewRefreshIcon(), b.loadRepos)
	b.changeRootButton = widget.NewButtonWithIcon("Change Root", theme.FolderOpenIcon(), b.chooseRoot)
	b.cloneButton = widget.NewButtonWithIcon("Clone into Root", theme.DownloadIcon(), b.cloneSelected)
	b.cloneButton.Importance = widget.HighImportance

	b.updateCloneActionState()
}

func (b *cloneBrowser) content() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("GitHub Repository Browser", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("GUITboard checks the repositories your GitHub account can clone, then fills the source and folder name for the one you pick.")
	subtitle.Wrapping = fyne.TextWrapWord

	rootCard := widget.NewCard("Destination root", "", container.NewVBox(
		b.rootLabel,
		container.NewHBox(
			b.changeRootButton,
			layout.NewSpacer(),
			widget.NewLabel("Preferred protocol"),
			b.protocolGroup,
		),
	))

	leftPanel := widget.NewCard("GitHub repositories", "", container.NewBorder(
		container.NewVBox(
			b.searchEntry,
			container.NewHBox(b.resultLabel, layout.NewSpacer(), b.refreshButton),
			b.progress,
		),
		nil,
		nil,
		nil,
		b.list,
	))

	details := container.NewVBox(
		b.detailName,
		b.detailDesc,
		metadataRow("Visibility", b.detailVisibility),
		metadataRow("Default branch", b.detailBranch),
		metadataRow("Last updated", b.detailUpdated),
		metadataRow("Local status", b.detailLocal),
		widget.NewSeparator(),
		metadataRow("Clone source", b.sourceEntry),
		metadataRow("Folder name", b.folderEntry),
	)

	rightPanel := widget.NewCard("Clone target", "", container.NewVScroll(details))

	split := container.NewHSplit(leftPanel, rightPanel)
	split.Offset = 0.49

	bottomBar := container.NewHBox(
		b.statusLabel,
		layout.NewSpacer(),
		widget.NewButton("Close", func() {
			b.win.Close()
		}),
		b.cloneButton,
	)

	return container.NewBorder(
		container.NewVBox(title, subtitle, rootCard),
		bottomBar,
		nil,
		nil,
		split,
	)
}

func (b *cloneBrowser) loadRepos() {
	if b.loading {
		return
	}

	b.loading = true
	b.localRepoIndex = buildLocalRepoIndex(b.dashboard.repos)
	b.refreshButton.Disable()
	b.progress.Show()
	b.progress.Start()
	b.statusLabel.SetText("Loading repositories from GitHub via GitHub CLI...")

	go func() {
		repos, err := gitops.ListCloneableRepositories()
		fyne.Do(func() {
			if b.closed {
				return
			}

			b.loading = false
			b.progress.Stop()
			b.progress.Hide()
			b.refreshButton.Enable()

			if err != nil {
				b.statusLabel.SetText(err.Error() + " You can still paste a custom Git source and clone manually.")
				b.applyFilter()
				return
			}

			b.repos = repos
			b.applyFilter()
			if len(repos) == 0 {
				b.statusLabel.SetText("GitHub returned no repositories. You can still paste a custom Git source and clone manually.")
				return
			}

			b.statusLabel.SetText(fmt.Sprintf("Loaded %d cloneable repositories from GitHub.", len(repos)))
		})
	}()
}

func (b *cloneBrowser) applyFilter() {
	query := strings.TrimSpace(strings.ToLower(b.searchEntry.Text))

	b.filtered = b.filtered[:0]
	for _, repo := range b.repos {
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				repo.FullName,
				repo.Owner,
				repo.Description,
				repo.DefaultBranch,
				repo.Visibility,
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}

		b.filtered = append(b.filtered, repo)
	}

	if len(b.repos) == 0 {
		b.resultLabel.SetText("No GitHub repositories loaded yet")
	} else if query == "" {
		b.resultLabel.SetText(fmt.Sprintf("%d repositories", len(b.filtered)))
	} else {
		b.resultLabel.SetText(fmt.Sprintf("%d of %d repositories", len(b.filtered), len(b.repos)))
	}

	b.list.Refresh()
	for idx := range b.filtered {
		b.list.SetItemHeight(idx, 86)
	}

	b.restoreSelection()
	b.refreshSelection(false)
}

func (b *cloneBrowser) restoreSelection() {
	if b.selectedFullName == "" && len(b.filtered) > 0 {
		b.selectedFullName = b.filtered[0].FullName
		b.list.Select(0)
		return
	}

	if b.selectedFullName == "" {
		b.list.UnselectAll()
		return
	}

	for idx, repo := range b.filtered {
		if repo.FullName == b.selectedFullName {
			b.list.Select(idx)
			return
		}
	}

	if len(b.filtered) == 0 {
		b.selectedFullName = ""
		b.list.UnselectAll()
		return
	}

	b.selectedFullName = b.filtered[0].FullName
	b.list.Select(0)
}

func (b *cloneBrowser) refreshSelection(updateFolder bool) {
	repo, ok := b.selectedRepo()
	if !ok {
		b.detailName.SetText("Select a repository or paste a custom source")
		b.detailDesc.SetText("Pick a repository from the GitHub list to fill the clone source automatically, or paste a custom Git URL if it is not listed.")
		b.detailVisibility.SetText("-")
		b.detailBranch.SetText("-")
		b.detailUpdated.SetText("-")
		b.detailLocal.SetText("No GitHub repository selected")
		b.updateCloneActionState()
		return
	}

	b.detailName.SetText(repo.FullName)
	b.detailDesc.SetText(valueOrDefault(repo.Description, "No description provided."))
	b.detailVisibility.SetText(cloneRepoVisibilitySummary(repo))
	b.detailBranch.SetText(valueOrDash(repo.DefaultBranch))
	b.detailUpdated.SetText(formatTime(repo.UpdatedAt))
	if localPath := b.localPathForRepo(repo); localPath != "" {
		b.detailLocal.SetText("Already in dashboard: " + localPath)
	} else {
		b.detailLocal.SetText("Not in the current dashboard list")
	}

	previousAuto := b.autoFolderName
	b.autoFolderName = repo.Name
	if updateFolder {
		currentFolder := strings.TrimSpace(b.folderEntry.Text)
		if currentFolder == "" || currentFolder == previousAuto {
			b.folderEntry.SetText(repo.Name)
		}
	}

	b.sourceEntry.SetText(repo.CloneURL(b.protocol))
	b.updateCloneActionState()
}

func (b *cloneBrowser) syncSourceFromSelection() {
	repo, ok := b.selectedRepo()
	if !ok {
		return
	}

	b.sourceEntry.SetText(repo.CloneURL(b.protocol))
}

func (b *cloneBrowser) selectedRepo() (gitops.CloneableRepo, bool) {
	for _, repo := range b.repos {
		if repo.FullName == b.selectedFullName {
			return repo, true
		}
	}

	return gitops.CloneableRepo{}, false
}

func (b *cloneBrowser) chooseRoot() {
	picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, b.win)
			return
		}
		if uri == nil {
			return
		}

		root := filepath.Clean(uri.Path())
		if err := os.MkdirAll(root, 0o755); err != nil {
			dialog.ShowError(err, b.win)
			return
		}

		b.root = root
		b.rootLabel.SetText(root)
		b.dashboard.cfg.RootPath = root
		b.dashboard.cfg.LastOpened = time.Now()
		_ = config.Save(b.dashboard.cfg)
		b.dashboard.refresh(false)
		b.statusLabel.SetText("Destination root updated.")
	}, b.win)

	if b.root != "" {
		location, err := storage.ListerForURI(storage.NewFileURI(b.root))
		if err == nil {
			picker.SetLocation(location)
		}
	}

	picker.Show()
}

func (b *cloneBrowser) cloneSelected() {
	if !b.dashboard.runCloneRepo(b.sourceEntry.Text, b.folderEntry.Text) {
		return
	}

	b.win.Close()
}

func (b *cloneBrowser) updateCloneActionState() {
	canClone := strings.TrimSpace(b.sourceEntry.Text) != "" && !b.dashboard.isActing && !b.dashboard.isScanning
	setEnabled(b.cloneButton, canClone)
}

func defaultCloneProtocol() gitops.CloneProtocol {
	if runtime.GOOS == "windows" {
		return gitops.CloneProtocolHTTPS
	}
	return gitops.CloneProtocolSSH
}

func cloneProtocolForLabel(value string) gitops.CloneProtocol {
	if strings.EqualFold(strings.TrimSpace(value), "ssh") {
		return gitops.CloneProtocolSSH
	}
	return gitops.CloneProtocolHTTPS
}

func protocolLabel(protocol gitops.CloneProtocol) string {
	if protocol == gitops.CloneProtocolSSH {
		return "SSH"
	}
	return "HTTPS"
}

func cloneRepoVisibilitySummary(repo gitops.CloneableRepo) string {
	parts := []string{valueOrDash(repo.Visibility)}
	if repo.Private && repo.Visibility != "private" {
		parts = append(parts, "private")
	}
	if repo.Fork {
		parts = append(parts, "fork")
	}
	if repo.Archived {
		parts = append(parts, "archived")
	}
	return strings.Join(parts, " | ")
}

func cloneRepoListSummary(repo gitops.CloneableRepo, localPath string) string {
	parts := []string{cloneRepoVisibilitySummary(repo)}
	if !repo.UpdatedAt.IsZero() {
		parts = append(parts, "updated "+formatTime(repo.UpdatedAt))
	}
	if localPath != "" {
		parts = append(parts, "already tracked")
	}
	return strings.Join(parts, " | ")
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func buildLocalRepoIndex(repos []gitops.Repo) map[string]string {
	index := make(map[string]string, len(repos))
	for _, repo := range repos {
		key := canonicalRemote(repo.Remote)
		if key == "" {
			continue
		}
		index[key] = repo.Path
	}
	return index
}

func (b *cloneBrowser) localPathForRepo(repo gitops.CloneableRepo) string {
	for _, alias := range cloneRepoAliases(repo) {
		if alias == "" {
			continue
		}
		if path, ok := b.localRepoIndex[alias]; ok {
			return path
		}
	}
	return ""
}

func cloneRepoAliases(repo gitops.CloneableRepo) []string {
	values := []string{
		repo.FullName,
		"github.com/" + repo.FullName,
		repo.HTTPSURL,
		repo.SSHURL,
	}

	seen := map[string]struct{}{}
	aliases := make([]string, 0, len(values))
	for _, value := range values {
		normalized := canonicalRemote(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		aliases = append(aliases, normalized)
	}

	return aliases
}

func canonicalRemote(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	trimmed = strings.TrimSuffix(trimmed, ".git")

	if strings.HasPrefix(trimmed, "git@") {
		rest := strings.TrimPrefix(trimmed, "git@")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) == 2 {
			host := strings.TrimSpace(parts[0])
			path := strings.Trim(strings.TrimSpace(parts[1]), "/")
			if host != "" && path != "" {
				return host + "/" + path
			}
		}
	}

	if strings.Contains(trimmed, "://") {
		if parsed, err := url.Parse(trimmed); err == nil {
			host := strings.TrimSpace(strings.ToLower(parsed.Host))
			path := strings.Trim(strings.TrimSuffix(parsed.Path, ".git"), "/")
			if host != "" && path != "" {
				return host + "/" + path
			}
		}
	}

	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	trimmed = strings.Trim(strings.TrimSuffix(trimmed, ".git"), "/")
	if strings.Count(trimmed, "/") == 1 {
		return "github.com/" + trimmed
	}

	return trimmed
}
