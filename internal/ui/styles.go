package ui

import "github.com/charmbracelet/lipgloss"

type uiStyles struct {
	title          lipgloss.Style
	headerDivider  lipgloss.Style
	headerText     lipgloss.Style
	panel          lipgloss.Style
	panelTitle     lipgloss.Style
	statCard       lipgloss.Style
	statLabel      lipgloss.Style
	statValue      lipgloss.Style
	actionButton   lipgloss.Style
	actionSelected lipgloss.Style
	actionActive   lipgloss.Style
	actionDisabled lipgloss.Style
	repoRow        lipgloss.Style
	repoSelected   lipgloss.Style
	repoTitle      lipgloss.Style
	mutedText      lipgloss.Style
	sectionInfo    lipgloss.Style
	emptyState     lipgloss.Style
	detailLabel    lipgloss.Style
	detailValue    lipgloss.Style
	badgeDirty     lipgloss.Style
	badgeClean     lipgloss.Style
	badgeAhead     lipgloss.Style
	badgeBehind    lipgloss.Style
	badgeBusy      lipgloss.Style
	badgeMuted     lipgloss.Style
	modalBox       lipgloss.Style
	modalTitle     lipgloss.Style
	modalText      lipgloss.Style
	modalInput     lipgloss.Style
	help           lipgloss.Style
	spinner        lipgloss.Style
}

var styles = newUIStyles()

func newUIStyles() uiStyles {
	panelBorder := lipgloss.AdaptiveColor{Light: "#A7B5C7", Dark: "#38536B"}
	panelFill := lipgloss.AdaptiveColor{Light: "#F6F9FC", Dark: "#10202D"}
	textColor := lipgloss.AdaptiveColor{Light: "#102A43", Dark: "#E6EEF7"}
	mutedColor := lipgloss.AdaptiveColor{Light: "#627D98", Dark: "#94A3B8"}
	accent := lipgloss.AdaptiveColor{Light: "#0B699B", Dark: "#7DD3FC"}
	accentStrong := lipgloss.AdaptiveColor{Light: "#F8FBFD", Dark: "#0B1620"}
	accentSoft := lipgloss.AdaptiveColor{Light: "#DCEEF8", Dark: "#143042"}
	successSoft := lipgloss.AdaptiveColor{Light: "#E6F7EE", Dark: "#153428"}
	successText := lipgloss.AdaptiveColor{Light: "#1C6B3D", Dark: "#86EFAC"}
	warnSoft := lipgloss.AdaptiveColor{Light: "#FFF2D8", Dark: "#493816"}
	warnText := lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#FACC15"}
	dangerSoft := lipgloss.AdaptiveColor{Light: "#FDE7E8", Dark: "#441C23"}
	dangerText := lipgloss.AdaptiveColor{Light: "#A11B2B", Dark: "#FCA5A5"}

	return uiStyles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),
		headerDivider: lipgloss.NewStyle().
			Foreground(mutedColor).
			PaddingLeft(1),
		headerText: lipgloss.NewStyle().
			Foreground(textColor),
		panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(panelBorder).
			Background(panelFill).
			Padding(0, 1).
			MarginBottom(1),
		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			MarginBottom(1),
		statCard: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(panelBorder).
			Background(panelFill).
			Padding(0, 1).
			MarginRight(1),
		statLabel: lipgloss.NewStyle().
			Foreground(mutedColor),
		statValue: lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor),
		actionButton: lipgloss.NewStyle().
			Foreground(textColor).
			Background(accentSoft).
			Padding(0, 1),
		actionSelected: lipgloss.NewStyle().
			Foreground(accent).
			Background(panelFill).
			Underline(true).
			Padding(0, 1),
		actionActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(accentStrong).
			Background(accent).
			Padding(0, 1),
		actionDisabled: lipgloss.NewStyle().
			Foreground(mutedColor).
			Background(panelFill).
			Padding(0, 1).
			Faint(true),
		repoRow: lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1).
			MarginBottom(1),
		repoSelected: lipgloss.NewStyle().
			Foreground(textColor).
			Background(accentSoft).
			BorderStyle(lipgloss.ThickBorder()).
			BorderLeft(true).
			BorderTop(false).
			BorderRight(false).
			BorderBottom(false).
			BorderForeground(accent).
			Padding(0, 1).
			MarginBottom(1),
		repoTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			MarginBottom(1),
		mutedText: lipgloss.NewStyle().
			Foreground(mutedColor),
		sectionInfo: lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginBottom(1),
		emptyState: lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true),
		detailLabel: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),
		detailValue: lipgloss.NewStyle().
			Foreground(textColor),
		badgeDirty: lipgloss.NewStyle().
			Foreground(dangerText).
			Background(dangerSoft).
			Padding(0, 1),
		badgeClean: lipgloss.NewStyle().
			Foreground(successText).
			Background(successSoft).
			Padding(0, 1),
		badgeAhead: lipgloss.NewStyle().
			Foreground(successText).
			Background(successSoft).
			Padding(0, 1),
		badgeBehind: lipgloss.NewStyle().
			Foreground(warnText).
			Background(warnSoft).
			Padding(0, 1),
		badgeBusy: lipgloss.NewStyle().
			Foreground(accentStrong).
			Background(accent).
			Padding(0, 1),
		badgeMuted: lipgloss.NewStyle().
			Foreground(textColor).
			Background(accentSoft).
			Padding(0, 1),
		modalBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(accent).
			Background(panelFill).
			Padding(1, 2),
		modalTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			MarginBottom(1),
		modalText: lipgloss.NewStyle().
			Foreground(textColor).
			MarginBottom(1),
		modalInput: lipgloss.NewStyle().
			Foreground(textColor).
			MarginBottom(1),
		help: lipgloss.NewStyle().
			Foreground(mutedColor),
		spinner: lipgloss.NewStyle().
			Foreground(accent),
	}
}
