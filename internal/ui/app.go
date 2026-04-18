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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type dashboard struct {
	app fyne.App
	win fyne.Window

	cfg          config.Config
	repos        []gitops.Repo
	filtered     []gitops.Repo
	selectedPath string
	isScanning   bool
	isActing     bool

	rootLabel        *widget.Label
	statusLabel      *widget.Label
	searchEntry      *widget.Entry
	changedOnly      *widget.Check
	list             *widget.List
	logEntry         *widget.Entry
	detailName       *widget.Label
	detailPath       *widget.Label
	detailBranch     *widget.Label
	detailRemote     *widget.Label
	detailCommit     *widget.Label
	detailActivity   *widget.Label
	detailStatus     *widget.Label
	totalValue       *widget.Label
	dirtyValue       *widget.Label
	aheadValue       *widget.Label
	behindValue      *widget.Label
	stageButton      *widget.Button
	commitButton     *widget.Button
	pullButton       *widget.Button
	pushButton       *widget.Button
	commitPushButton *widget.Button
	refreshButton    *widget.Button
	cloneButton      *widget.Button
	bulkCommitButton *widget.Button
	pullAllButton    *widget.Button
}

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	if strings.TrimSpace(cfg.RootPath) == "" {
		cfg.RootPath = defaultRootPath()
		if cfg.RootPath != "" {
			if err := os.MkdirAll(cfg.RootPath, 0o755); err == nil {
				_ = config.Save(cfg)
			}
		}
	}

	application := app.NewWithID("local.guitboard")
	window := application.NewWindow("GUITboard")

	d := &dashboard{
		app: application,
		win: window,
		cfg: cfg,
	}

	d.buildUI()

	window.Resize(fyne.NewSize(cfg.WindowWidth, cfg.WindowHeight))
	window.SetContent(d.content())
	window.SetMaster()
	window.SetCloseIntercept(func() {
		size := window.Canvas().Size()
		d.cfg.WindowWidth = size.Width
		d.cfg.WindowHeight = size.Height
		d.cfg.LastOpened = time.Now()
		_ = config.Save(d.cfg)
		window.Close()
	})

	d.applyFilters()
	d.refresh(false)
	d.startAutoRefresh()

	window.ShowAndRun()
	return nil
}

func (d *dashboard) buildUI() {
	d.rootLabel = widget.NewLabel("")
	d.rootLabel.Wrapping = fyne.TextWrapWord

	d.statusLabel = widget.NewLabel("Ready")
	d.statusLabel.Wrapping = fyne.TextWrapWord

	d.searchEntry = widget.NewEntry()
	d.searchEntry.SetPlaceHolder("Filter by repo name, path, branch, or remote...")
	d.searchEntry.OnChanged = func(string) {
		d.applyFilters()
	}

	d.changedOnly = widget.NewCheck("Only show repos with changes", func(bool) {
		d.applyFilters()
	})

	d.totalValue = widget.NewLabel("0")
	d.dirtyValue = widget.NewLabel("0")
	d.aheadValue = widget.NewLabel("0")
	d.behindValue = widget.NewLabel("0")

	d.detailName = widget.NewLabel("Select a repository")
	d.detailName.TextStyle = fyne.TextStyle{Bold: true}

	d.detailPath = widget.NewLabel("-")
	d.detailPath.Wrapping = fyne.TextWrapWord
	d.detailBranch = widget.NewLabel("-")
	d.detailBranch.Wrapping = fyne.TextWrapWord
	d.detailRemote = widget.NewLabel("-")
	d.detailRemote.Wrapping = fyne.TextWrapWord
	d.detailCommit = widget.NewLabel("-")
	d.detailCommit.Wrapping = fyne.TextWrapWord
	d.detailActivity = widget.NewLabel("-")
	d.detailActivity.Wrapping = fyne.TextWrapWord
	d.detailStatus = widget.NewLabel("-")
	d.detailStatus.Wrapping = fyne.TextWrapWord

	d.logEntry = widget.NewMultiLineEntry()
	d.logEntry.Wrapping = fyne.TextWrapWord
	d.logEntry.SetMinRowsVisible(12)
	d.logEntry.Disable()
	d.logEntry.SetText("Operation output will appear here.\n")

	d.list = widget.NewList(
		func() int {
			return len(d.filtered)
		},
		func() fyne.CanvasObject {
			name := widget.NewLabel("Repository")
			name.TextStyle = fyne.TextStyle{Bold: true}

			path := widget.NewLabel("Path")
			path.Wrapping = fyne.TextWrapWord

			meta := widget.NewLabel("Status")
			meta.Wrapping = fyne.TextWrapWord

			return container.NewVBox(name, path, meta)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id < 0 || id >= len(d.filtered) {
				return
			}

			repo := d.filtered[id]
			objects := item.(*fyne.Container).Objects

			nameLabel := objects[0].(*widget.Label)
			pathLabel := objects[1].(*widget.Label)
			metaLabel := objects[2].(*widget.Label)

			nameLabel.SetText(repo.Name)
			pathLabel.SetText(repo.Path)
			metaLabel.SetText(repoListSummary(repo))
		},
	)

	d.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(d.filtered) {
			return
		}
		d.selectedPath = d.filtered[id].Path
		d.refreshDetails()
	}

	d.stageButton = widget.NewButtonWithIcon("Stage All", theme.ContentAddIcon(), func() {
		repo, ok := d.selectedRepo()
		if !ok {
			return
		}
		d.runRepoAction(repo, "Stage All", func() (string, error) {
			return gitops.StageAll(repo.Path)
		})
	})

	d.commitButton = widget.NewButton("Commit All...", func() {
		repo, ok := d.selectedRepo()
		if !ok {
			return
		}
		d.promptCommitMessage("Commit all changes", repo.Name, func(message string) {
			d.runRepoAction(repo, "Commit All", func() (string, error) {
				return gitops.CommitAll(repo.Path, message)
			})
		})
	})

	d.pullButton = widget.NewButtonWithIcon("Pull", theme.DownloadIcon(), func() {
		repo, ok := d.selectedRepo()
		if !ok {
			return
		}
		d.runRepoAction(repo, "Pull", func() (string, error) {
			return gitops.Pull(repo.Path)
		})
	})

	d.pushButton = widget.NewButtonWithIcon("Push", theme.UploadIcon(), func() {
		repo, ok := d.selectedRepo()
		if !ok {
			return
		}
		d.runRepoAction(repo, "Push", func() (string, error) {
			return gitops.Push(repo.Path)
		})
	})

	d.commitPushButton = widget.NewButton("Commit + Push...", func() {
		repo, ok := d.selectedRepo()
		if !ok {
			return
		}
		d.promptCommitMessage("Commit and push all changes", repo.Name, func(message string) {
			d.runRepoAction(repo, "Commit + Push", func() (string, error) {
				return gitops.CommitAndPush(repo.Path, message)
			})
		})
	})

	d.refreshButton = widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		d.refresh(true)
	})

	d.cloneButton = widget.NewButtonWithIcon("Clone GitHub Repo...", theme.DownloadIcon(), d.promptCloneRepo)

	d.bulkCommitButton = widget.NewButton("Commit + Push Changed Repos...", func() {
		d.promptBulkCommit()
	})

	d.pullAllButton = widget.NewButton("Pull All Repos", func() {
		d.confirmPullAll()
	})

	d.updateActionState()
}

func (d *dashboard) content() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("Local GitHub Dashboard", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	rootCard := widget.NewCard("Repository root", "", container.NewVBox(
		d.rootLabel,
		container.NewHBox(
			widget.NewButtonWithIcon("Choose Folder", theme.FolderOpenIcon(), d.chooseFolder),
			d.cloneButton,
			d.refreshButton,
			layout.NewSpacer(),
			d.pullAllButton,
			d.bulkCommitButton,
		),
	))

	summary := container.NewGridWithColumns(4,
		widget.NewCard("Repositories", "", d.totalValue),
		widget.NewCard("Dirty", "", d.dirtyValue),
		widget.NewCard("Ahead", "", d.aheadValue),
		widget.NewCard("Behind", "", d.behindValue),
	)

	leftPanel := container.NewBorder(
		container.NewVBox(
			rootCard,
			summary,
			widget.NewCard("Filter", "", container.NewVBox(d.searchEntry, d.changedOnly)),
		),
		nil,
		nil,
		nil,
		d.list,
	)

	detailGrid := container.NewVBox(
		d.detailName,
		metadataRow("Path", d.detailPath),
		metadataRow("Branch", d.detailBranch),
		metadataRow("Remote", d.detailRemote),
		metadataRow("Last commit", d.detailCommit),
		metadataRow("Last activity", d.detailActivity),
		metadataRow("Status", d.detailStatus),
	)

	actionBar := container.NewGridWithColumns(3,
		d.stageButton,
		d.commitButton,
		d.commitPushButton,
		d.pullButton,
		d.pushButton,
		widget.NewLabel(""),
	)

	detailsPanel := container.NewVBox(
		widget.NewCard("Selected repository", "", detailGrid),
		widget.NewCard("Actions", "", actionBar),
	)

	logPanel := widget.NewCard(
		"Operation log",
		"",
		container.NewBorder(
			nil,
			container.NewPadded(d.statusLabel),
			nil,
			nil,
			container.NewVScroll(d.logEntry),
		),
	)

	rightPanel := container.NewBorder(detailsPanel, nil, nil, nil, logPanel)

	split := container.NewHSplit(leftPanel, rightPanel)
	split.Offset = 0.52

	return container.NewBorder(
		container.NewVBox(title),
		nil,
		nil,
		nil,
		split,
	)
}

func (d *dashboard) chooseFolder() {
	picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, d.win)
			return
		}
		if uri == nil {
			return
		}

		d.cfg.RootPath = filepath.Clean(uri.Path())
		d.cfg.LastOpened = time.Now()
		_ = config.Save(d.cfg)
		d.refresh(true)
	}, d.win)

	if d.cfg.RootPath != "" {
		location, err := storage.ListerForURI(storage.NewFileURI(d.cfg.RootPath))
		if err == nil {
			picker.SetLocation(location)
		}
	}

	picker.Show()
}

func (d *dashboard) promptCloneRepo() {
	root, err := d.cloneRootPath()
	if err != nil {
		dialog.ShowError(err, d.win)
		return
	}

	sourceEntry := widget.NewEntry()
	sourceEntry.SetPlaceHolder("owner/repo or https://github.com/owner/repo.git")

	folderEntry := widget.NewEntry()
	folderEntry.SetPlaceHolder("Optional. Defaults to the repository name")

	destinationLabel := widget.NewLabel(root)
	destinationLabel.Wrapping = fyne.TextWrapWord

	dialog.ShowForm(
		"Clone GitHub repository",
		"Clone",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Destination", destinationLabel),
			widget.NewFormItem("Repository", sourceEntry),
			widget.NewFormItem("Folder name", folderEntry),
		},
		func(confirm bool) {
			if !confirm {
				return
			}
			d.runCloneRepo(sourceEntry.Text, folderEntry.Text)
		},
		d.win,
	)
}

func (d *dashboard) runCloneRepo(source, dirName string) {
	if d.isActing || d.isScanning {
		return
	}

	if strings.TrimSpace(source) == "" {
		dialog.ShowError(fmt.Errorf("repository URL cannot be empty"), d.win)
		return
	}

	root, err := d.cloneRootPath()
	if err != nil {
		dialog.ShowError(err, d.win)
		return
	}

	displayName := strings.TrimSpace(dirName)
	if displayName == "" {
		displayName = gitops.DefaultCloneDirName(source)
	}
	if displayName == "" {
		displayName = strings.TrimSpace(source)
	}

	d.isActing = true
	d.updateActionState()
	d.setStatus(fmt.Sprintf("Cloning %s into %s ...", source, root))

	progress := dialog.NewProgressInfinite("Clone repository", fmt.Sprintf("Cloning into %s", root), d.win)
	progress.Show()

	go func() {
		targetPath, output, err := gitops.Clone(source, root, dirName)
		fyne.Do(func() {
			progress.Hide()
			d.isActing = false
			d.updateActionState()

			if targetPath == "" && displayName != "" {
				targetPath = filepath.Join(root, displayName)
			}

			repo := gitops.Repo{
				Name: displayName,
				Path: targetPath,
			}
			if repo.Name == "" && targetPath != "" {
				repo.Name = filepath.Base(targetPath)
			}
			if repo.Name == "" {
				repo.Name = strings.TrimSpace(source)
			}
			if repo.Path == "" {
				repo.Path = root
			}

			d.appendLog("Clone", repo, output, err)
			if err != nil {
				dialog.ShowError(err, d.win)
				d.setStatus(fmt.Sprintf("Clone failed for %s", repo.Name))
			} else {
				d.selectedPath = targetPath
				d.setStatus(fmt.Sprintf("Clone finished for %s", repo.Name))
			}

			d.refresh(false)
		})
	}()
}

func (d *dashboard) refresh(showProgress bool) {
	if d.isScanning {
		return
	}

	root := strings.TrimSpace(d.cfg.RootPath)
	d.rootLabel.SetText(root)
	if root == "" {
		d.setStatus("Choose a folder that contains your repositories.")
		return
	}

	if _, err := os.Stat(root); err != nil {
		d.setStatus(fmt.Sprintf("Root folder is not available: %v", err))
		return
	}

	d.isScanning = true
	d.updateActionState()
	d.setStatus(fmt.Sprintf("Scanning %s ...", root))

	progress := dialog.NewProgressInfinite("Scanning repositories", "Reading repositories and git status...", d.win)
	if showProgress {
		progress.Show()
	}

	go func() {
		repos, err := gitops.Scan(root)
		fyne.Do(func() {
			if showProgress {
				progress.Hide()
			}
			d.isScanning = false
			d.updateActionState()
			if err != nil {
				d.setStatus(fmt.Sprintf("Scan failed: %v", err))
				dialog.ShowError(err, d.win)
				return
			}

			d.repos = repos
			d.cfg.LastScan = time.Now()
			_ = config.Save(d.cfg)
			d.applyFilters()
			d.setStatus(fmt.Sprintf("Loaded %d repositories from %s", len(repos), root))
		})
	}()
}

func (d *dashboard) applyFilters() {
	query := strings.TrimSpace(strings.ToLower(d.searchEntry.Text))
	changedOnly := d.changedOnly.Checked

	d.filtered = d.filtered[:0]
	for _, repo := range d.repos {
		if changedOnly && !repo.Dirty {
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
		d.filtered = append(d.filtered, repo)
	}

	d.updateSummary()
	d.list.Refresh()
	d.restoreSelection()
	d.refreshDetails()
}

func (d *dashboard) updateSummary() {
	total := len(d.repos)
	dirty := 0
	ahead := 0
	behind := 0

	for _, repo := range d.repos {
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

	d.totalValue.SetText(fmt.Sprintf("%d", total))
	d.dirtyValue.SetText(fmt.Sprintf("%d", dirty))
	d.aheadValue.SetText(fmt.Sprintf("%d", ahead))
	d.behindValue.SetText(fmt.Sprintf("%d", behind))
}

func (d *dashboard) restoreSelection() {
	if d.selectedPath == "" && len(d.filtered) > 0 {
		d.selectedPath = d.filtered[0].Path
		d.list.Select(0)
		return
	}

	if d.selectedPath == "" {
		d.list.UnselectAll()
		return
	}

	for idx, repo := range d.filtered {
		if repo.Path == d.selectedPath {
			d.list.Select(idx)
			return
		}
	}

	if len(d.filtered) == 0 {
		d.selectedPath = ""
		d.list.UnselectAll()
		return
	}

	d.selectedPath = d.filtered[0].Path
	d.list.Select(0)
}

func (d *dashboard) refreshDetails() {
	repo, ok := d.selectedRepo()
	if !ok {
		d.detailName.SetText("No repository selected")
		d.detailPath.SetText("-")
		d.detailBranch.SetText("-")
		d.detailRemote.SetText("-")
		d.detailCommit.SetText("-")
		d.detailActivity.SetText("-")
		d.detailStatus.SetText("-")
		d.updateActionState()
		return
	}

	d.detailName.SetText(repo.Name)
	d.detailPath.SetText(repo.Path)
	d.detailBranch.SetText(branchSummary(repo))
	d.detailRemote.SetText(valueOrDash(repo.Remote))
	d.detailCommit.SetText(lastCommitSummary(repo))
	d.detailActivity.SetText(formatTime(repo.LastActivity))
	d.detailStatus.SetText(statusSummary(repo))
	d.updateActionState()
}

func (d *dashboard) selectedRepo() (gitops.Repo, bool) {
	for _, repo := range d.repos {
		if repo.Path == d.selectedPath {
			return repo, true
		}
	}
	return gitops.Repo{}, false
}

func (d *dashboard) runRepoAction(repo gitops.Repo, title string, action func() (string, error)) {
	if d.isActing || d.isScanning {
		return
	}

	d.isActing = true
	d.updateActionState()
	d.setStatus(fmt.Sprintf("%s on %s ...", title, repo.Name))

	progress := dialog.NewProgressInfinite(title, fmt.Sprintf("Running git action in %s", repo.Path), d.win)
	progress.Show()

	go func() {
		output, err := action()
		fyne.Do(func() {
			progress.Hide()
			d.isActing = false
			d.updateActionState()

			d.appendLog(title, repo, output, err)
			if err != nil {
				dialog.ShowError(err, d.win)
				d.setStatus(fmt.Sprintf("%s failed for %s", title, repo.Name))
			} else {
				d.setStatus(fmt.Sprintf("%s finished for %s", title, repo.Name))
			}

			d.refresh(false)
		})
	}()
}

func (d *dashboard) promptCommitMessage(title, repoName string, onSubmit func(message string)) {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Dashboard sync " + time.Now().Format("2006-01-02 15:04"))

	dialog.ShowForm(
		title,
		"Run",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Repository", widget.NewLabel(repoName)),
			widget.NewFormItem("Commit message", entry),
		},
		func(confirm bool) {
			if !confirm {
				return
			}

			message := strings.TrimSpace(entry.Text)
			if message == "" {
				message = entry.PlaceHolder
			}
			onSubmit(message)
		},
		d.win,
	)
}

func (d *dashboard) promptBulkCommit() {
	changed := 0
	for _, repo := range d.repos {
		if repo.Dirty {
			changed++
		}
	}
	if changed == 0 {
		dialog.ShowInformation("Nothing to upload", "No repositories currently have uncommitted changes.", d.win)
		return
	}

	entry := widget.NewEntry()
	entry.SetPlaceHolder("Dashboard sync " + time.Now().Format("2006-01-02 15:04"))

	dialog.ShowForm(
		"Commit and push changed repositories",
		"Run",
		"Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Repositories", widget.NewLabel(fmt.Sprintf("%d changed repositories", changed))),
			widget.NewFormItem("Commit message", entry),
		},
		func(confirm bool) {
			if !confirm {
				return
			}

			message := strings.TrimSpace(entry.Text)
			if message == "" {
				message = entry.PlaceHolder
			}
			d.runBulkCommitPush(message)
		},
		d.win,
	)
}

func (d *dashboard) runBulkCommitPush(message string) {
	if d.isActing || d.isScanning {
		return
	}

	d.isActing = true
	d.updateActionState()
	d.setStatus("Committing and pushing all changed repositories ...")

	progress := dialog.NewProgressInfinite("Bulk upload", "Staging, committing, and pushing every changed repository...", d.win)
	progress.Show()

	go func() {
		var lines []string
		failures := 0

		for _, repo := range d.repos {
			if !repo.Dirty {
				continue
			}
			output, err := gitops.CommitAndPush(repo.Path, message)
			lines = append(lines, formatLogBlock("Commit + Push", repo, output, err))
			if err != nil {
				failures++
			}
		}

		fyne.Do(func() {
			progress.Hide()
			d.isActing = false
			d.updateActionState()

			if len(lines) > 0 {
				d.logEntry.SetText(strings.Join(lines, "\n\n") + "\n")
			}

			if failures > 0 {
				d.setStatus(fmt.Sprintf("Bulk upload completed with %d failures", failures))
				dialog.ShowInformation("Bulk upload finished", fmt.Sprintf("Completed with %d failures. Review the log for details.", failures), d.win)
			} else {
				d.setStatus("Bulk upload completed successfully")
			}

			d.refresh(false)
		})
	}()
}

func (d *dashboard) confirmPullAll() {
	eligible := 0
	for _, repo := range d.repos {
		if repo.Upstream != "" && !repo.UpstreamGone {
			eligible++
		}
	}
	if eligible == 0 {
		dialog.ShowInformation("Nothing to pull", "No repositories with a configured upstream were found.", d.win)
		return
	}

	dialog.ShowConfirm(
		"Pull all repositories",
		fmt.Sprintf("Run git pull --ff-only on %d repositories with an upstream?", eligible),
		func(confirm bool) {
			if confirm {
				d.runPullAll()
			}
		},
		d.win,
	)
}

func (d *dashboard) runPullAll() {
	if d.isActing || d.isScanning {
		return
	}

	d.isActing = true
	d.updateActionState()
	d.setStatus("Pulling all repositories with an upstream ...")

	progress := dialog.NewProgressInfinite("Pull all repositories", "Running git pull --ff-only where possible...", d.win)
	progress.Show()

	go func() {
		var lines []string
		failures := 0

		for _, repo := range d.repos {
			if repo.Upstream == "" || repo.UpstreamGone {
				continue
			}
			output, err := gitops.Pull(repo.Path)
			lines = append(lines, formatLogBlock("Pull", repo, output, err))
			if err != nil {
				failures++
			}
		}

		fyne.Do(func() {
			progress.Hide()
			d.isActing = false
			d.updateActionState()

			if len(lines) > 0 {
				d.logEntry.SetText(strings.Join(lines, "\n\n") + "\n")
			}

			if failures > 0 {
				d.setStatus(fmt.Sprintf("Pull completed with %d failures", failures))
			} else {
				d.setStatus("Pull completed successfully")
			}

			d.refresh(false)
		})
	}()
}

func (d *dashboard) appendLog(title string, repo gitops.Repo, output string, err error) {
	block := formatLogBlock(title, repo, output, err)
	current := strings.TrimSpace(d.logEntry.Text)
	if current == "" {
		d.logEntry.SetText(block + "\n")
		return
	}
	d.logEntry.SetText(block + "\n\n" + current + "\n")
}

func (d *dashboard) updateActionState() {
	repo, repoSelected := d.selectedRepo()
	busy := d.isScanning || d.isActing

	hasDirtyRepos := false
	hasPullableRepos := false
	for _, item := range d.repos {
		if item.Dirty {
			hasDirtyRepos = true
		}
		if item.Upstream != "" && !item.UpstreamGone {
			hasPullableRepos = true
		}
	}

	setEnabled(d.stageButton, repoSelected && repo.Dirty && !busy)
	setEnabled(d.commitButton, repoSelected && repo.Dirty && !busy)
	setEnabled(d.commitPushButton, repoSelected && repo.Dirty && !busy)
	setEnabled(d.pullButton, repoSelected && repo.Upstream != "" && !repo.UpstreamGone && !busy)
	setEnabled(d.pushButton, repoSelected && repo.Remote != "" && !busy)
	setEnabled(d.refreshButton, !busy)
	setEnabled(d.cloneButton, !busy)
	setEnabled(d.bulkCommitButton, hasDirtyRepos && !busy)
	setEnabled(d.pullAllButton, hasPullableRepos && !busy)
}

func (d *dashboard) startAutoRefresh() {
	interval := time.Duration(d.cfg.AutoRefreshSeconds) * time.Second
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(func() {
				if !d.isScanning && !d.isActing {
					d.refresh(false)
				}
			})
		}
	}()
}

func (d *dashboard) setStatus(text string) {
	d.rootLabel.SetText(valueOrDash(d.cfg.RootPath))
	d.statusLabel.SetText(text)
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

func (d *dashboard) cloneRootPath() (string, error) {
	root := strings.TrimSpace(d.cfg.RootPath)
	if root == "" {
		root = defaultRootPath()
	}
	if root == "" {
		return "", fmt.Errorf("unable to determine a repository root folder")
	}

	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	if d.cfg.RootPath != root {
		d.cfg.RootPath = root
		_ = config.Save(d.cfg)
	}
	d.rootLabel.SetText(root)

	return root, nil
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

func metadataRow(label string, value fyne.CanvasObject) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Resize(fyne.NewSize(140, title.MinSize().Height))
	return container.NewBorder(nil, nil, title, nil, value)
}

func repoListSummary(repo gitops.Repo) string {
	parts := []string{branchSummary(repo)}
	if repo.Dirty {
		parts = append(parts, fmt.Sprintf("%d changed", repo.ChangedCount))
	} else {
		parts = append(parts, "clean")
	}
	if repo.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("ahead %d", repo.Ahead))
	}
	if repo.Behind > 0 {
		parts = append(parts, fmt.Sprintf("behind %d", repo.Behind))
	}
	parts = append(parts, "activity "+formatTime(repo.LastActivity))
	return strings.Join(parts, " | ")
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

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04")
}

func formatLogBlock(title string, repo gitops.Repo, output string, err error) string {
	lines := []string{
		fmt.Sprintf("[%s] %s", title, repo.Name),
		repo.Path,
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

func setEnabled(button *widget.Button, enabled bool) {
	if button == nil {
		return
	}
	if enabled {
		button.Enable()
		return
	}
	button.Disable()
}
