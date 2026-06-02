package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/soriano/nota/internal/domain"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	itemStyle     = lipgloss.NewStyle().PaddingLeft(4)
	selectedStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#7C3AED")).Bold(true)
	tagStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	scoreStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1)
)

type docItem struct {
	doc   *domain.Document
	score float32
}

func (d docItem) Title() string { return d.doc.Title }
func (d docItem) Description() string {
	meta := ""
	if len(d.doc.Tags) > 0 {
		meta += strings.Join(d.doc.Tags, ", ")
	}
	if d.doc.Notebook != "" {
		if meta != "" {
			meta += " · "
		}
		meta += d.doc.Notebook
	}
	if d.doc.Category != "" {
		if meta != "" {
			meta += " · "
		}
		meta += d.doc.Category
	}
	preview := truncate(d.doc.Content, 60)
	return fmt.Sprintf("%s  %s", tagStyle.Render(meta), dimStyle.Render(preview))
}
func (d docItem) FilterValue() string {
	return d.doc.Title + " " + strings.Join(d.doc.Tags, " ") + " " + d.doc.Content
}

type FuzzyModel struct {
	list     list.Model
	choice   *domain.Document
	quitting bool
	mode     string
}

func NewFuzzyPicker(docs []*domain.Document, mode string) FuzzyModel {
	items := make([]list.Item, len(docs))
	for i, d := range docs {
		items[i] = docItem{doc: d}
	}

	l := list.New(items, list.NewDefaultDelegate(), 80, 20)
	l.Title = "Nota - " + mode
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	return FuzzyModel{list: l, mode: mode}
}

func (m FuzzyModel) Init() tea.Cmd { return nil }

func (m FuzzyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if i, ok := m.list.SelectedItem().(docItem); ok {
				m.choice = i.doc
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc, tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m FuzzyModel) View() string {
	if m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

func (m FuzzyModel) Selected() *domain.Document {
	return m.choice
}

func truncate(s string, max int) string {
	clean := strings.ReplaceAll(s, "\n", " ")
	clean = strings.TrimSpace(clean)
	if len(clean) > max {
		return clean[:max-3] + "..."
	}
	return clean
}

func RunFuzzyPicker(docs []*domain.Document, mode string) (*domain.Document, error) {
	p := tea.NewProgram(NewFuzzyPicker(docs, mode), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return nil, err
	}
	if fm, ok := m.(FuzzyModel); ok {
		return fm.Selected(), nil
	}
	return nil, nil
}
