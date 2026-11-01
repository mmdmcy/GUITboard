package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"guitboard/internal/gitops"
)

// The dashboard must fit the terminal it is given: a line wider than the
// terminal wraps and a view taller than it scrolls its top off screen.
func TestDashboardFitsTheTerminal(t *testing.T) {
	repos := make([]gitops.Repo, 0, 40)
	for i := 0; i < 40; i++ {
		repos = append(repos, gitops.Repo{
			Name:              fmt.Sprintf("repository-%02d", i),
			Path:              fmt.Sprintf("/home/someone/Documents/github/repository-%02d", i),
			Branch:            "main",
			Upstream:          "origin/main",
			Remote:            "https://github.com/someone/repository.git",
			LastCommitMessage: strings.Repeat("a long commit message ", 6),
			ChangedFiles:      []string{"a.go", "b.go"},
			ChangedCount:      2,
			Ahead:             i % 3,
			Dirty:             i%2 == 0,
		})
	}
	for _, size := range [][2]int{{96, 28}, {120, 40}, {140, 40}, {200, 50}, {80, 24}, {60, 20}} {
		model := newDashboardModel(configStub())
		model.repos = repos
		model.filtered = repos
		next, _ := model.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := next.(dashboardModel).View()
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Errorf("%dx%d: %d lines, taller than the terminal", size[0], size[1], len(lines))
		}
		for number, line := range lines {
			if width := lipgloss.Width(line); width > size[0] {
				t.Errorf("%dx%d: line %d is %d wide", size[0], size[1], number+1, width)
				break
			}
		}
	}
}
