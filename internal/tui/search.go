package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/soriano/nota/internal/usecase"
)

type searchItem struct {
	doc   usecase.SearchResult
	index int
}

func (s searchItem) Title() string {
	return fmt.Sprintf("#%d %s", s.index+1, s.doc.Document.Title)
}
func (s searchItem) Description() string {
	meta := ""
	if len(s.doc.Document.Tags) > 0 {
		meta += strings.Join(s.doc.Document.Tags, ", ")
	}
	if s.doc.Document.Grupo != "" {
		if meta != "" {
			meta += " · "
		}
		meta += s.doc.Document.Grupo
	}
	if s.doc.Document.Categoria != "" {
		if meta != "" {
			meta += " · "
		}
		meta += s.doc.Document.Categoria
	}
	preview := truncate(s.doc.Document.Content, 50)
	scoreStr := scoreStyle.Render(fmt.Sprintf("%d%%", int(s.doc.Score*100)))
	return fmt.Sprintf("%s  %s  %s", meta, dimStyle.Render(preview), scoreStr)
}
func (s searchItem) FilterValue() string {
	return s.doc.Document.Title + " " + strings.Join(s.doc.Document.Tags, " ")
}

type SearchModel struct {
	list     list.Model
	results  []*usecase.SearchResult
	choice   *usecase.SearchResult
	quitting bool
	action   string
}

func NewSearchModel(results []*usecase.SearchResult, query string) SearchModel {
	items := make([]list.Item, len(results))
	for i, r := range results {
		items[i] = searchItem{doc: *r, index: i}
	}

	l := list.New(items, list.NewDefaultDelegate(), 80, 20)
	l.Title = fmt.Sprintf("Nota Search: \"%s\"", query)
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	return SearchModel{list: l, results: results}
}

func (m SearchModel) Init() tea.Cmd { return nil }

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEnter:
			if i, ok := m.list.SelectedItem().(searchItem); ok {
				m.choice = &i.doc
			}
			m.quitting = true
			return m, tea.Quit
		case msg.String() == "q" || msg.Type == tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m SearchModel) View() string {
	if m.quitting {
		return ""
	}
	footer := dimStyle.Render("  ↑↓ navigate  ↵ open  e edit  d delete  q back")
	return "\n" + m.list.View() + "\n" + footer
}

func (m SearchModel) Selected() *usecase.SearchResult { return m.choice }
func (m SearchModel) Action() string                  { return m.action }

func RunSearchTUI(results []*usecase.SearchResult, query string) (*usecase.SearchResult, error) {
	p := tea.NewProgram(NewSearchModel(results, query), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return nil, err
	}
	if sm, ok := m.(SearchModel); ok {
		return sm.Selected(), nil
	}
	return nil, nil
}
