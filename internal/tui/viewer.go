package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/soriano/nota/internal/domain"
)

type ViewerModel struct {
	viewport viewport.Model
	doc      *domain.Document
	linked   []*domain.Document
	ready    bool
	quitting bool
	action   string
}

func NewViewer(doc *domain.Document, linked []*domain.Document) ViewerModel {
	return ViewerModel{doc: doc, linked: linked}
}

func (m ViewerModel) Init() tea.Cmd { return nil }

func (m ViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 4
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.viewport.SetContent(m.renderContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc"))):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
			m.action = "edit"
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			m.action = "delete"
			m.quitting = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m ViewerModel) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return "\n  Loading..."
	}

	header := titleStyle.Render(m.doc.Title)
	meta := m.renderMeta()
	linked := m.renderLinked()
	footer := dimStyle.Render("  ↑↓ scroll  e edit  d delete  q back")

	content := lipgloss.JoinVertical(lipgloss.Left,
		"  "+header,
		"  "+meta,
		linked,
		"",
		m.viewport.View(),
		"",
		footer,
	)
	return "\n" + content
}

func (m ViewerModel) renderContent() string {
	var sb strings.Builder
	sb.WriteString(m.doc.Content)
	if len(m.linked) > 0 {
		sb.WriteString("\n\n---\n## Linked Notes\n")
		for _, l := range m.linked {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", l.Title, l.ID))
		}
	}
	return sb.String()
}

func (m ViewerModel) renderMeta() string {
	parts := []string{}
	if len(m.doc.Tags) > 0 {
		parts = append(parts, "tags: "+strings.Join(m.doc.Tags, ", "))
	}
	if m.doc.Notebook != "" {
		parts = append(parts, "notebook: "+m.doc.Notebook)
	}
	if m.doc.Category != "" {
		parts = append(parts, "cat: "+m.doc.Category)
	}
	parts = append(parts, fmt.Sprintf("accessed: %d", m.doc.Accessed))
	parts = append(parts, m.doc.CreatedAt.Format("2006-01-02 15:04"))
	return dimStyle.Render(strings.Join(parts, "  ·  "))
}

func (m ViewerModel) renderLinked() string {
	if len(m.linked) == 0 {
		return ""
	}
	names := make([]string, len(m.linked))
	for i, l := range m.linked {
		names[i] = l.Title
	}
	return tagStyle.Render("  linked: " + strings.Join(names, ", "))
}

func (m ViewerModel) Action() string { return m.action }
func (m ViewerModel) DocID() string  { return m.doc.ID }

func RunViewer(doc *domain.Document, linked []*domain.Document) (string, error) {
	p := tea.NewProgram(NewViewer(doc, linked), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", err
	}
	if vm, ok := m.(ViewerModel); ok {
		return vm.Action(), nil
	}
	return "", nil
}
