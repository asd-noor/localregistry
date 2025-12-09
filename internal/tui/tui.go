package tui

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"localregistry/internal/api"
)

// ViewState represents the current view in the TUI.
type ViewState int

const (
	ViewRepositories ViewState = iota
	ViewTags
	ViewManifest
	ViewConfirmDelete
)

// Model is the main TUI model.
type Model struct {
	client *api.Client
	ctx    context.Context

	state        ViewState
	width        int
	height       int
	err          error
	loading      bool
	statusMsg    string
	confirmMsg   string
	confirmYes   bool
	deleteTarget deleteTarget

	repoList    list.Model
	tagList     list.Model
	spinner     spinner.Model
	currentRepo string

	manifest *api.Manifest
}

type deleteTarget struct {
	repo   string
	tag    string
	digest string
}

// Item types for the list.
type repoItem string

func (r repoItem) Title() string       { return string(r) }
func (r repoItem) Description() string { return "" }
func (r repoItem) FilterValue() string { return string(r) }

type tagItem struct {
	name   string
	digest string
}

func (t tagItem) Title() string       { return t.name }
func (t tagItem) Description() string { return digestStyle.Render(truncateDigest(t.digest)) }
func (t tagItem) FilterValue() string { return t.name }

func truncateDigest(d string) string {
	if len(d) > 19 {
		return d[:19] + "..."
	}
	return d
}

// Messages for async operations.
type (
	reposLoadedMsg    []string
	tagsLoadedMsg     []tagItem
	manifestLoadedMsg *api.Manifest
	deleteSuccessMsg  string
	errMsg            error
)

// KeyMap defines the keybindings for the TUI.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Delete  key.Binding
	Refresh key.Binding
	Quit    key.Binding
	Help    key.Binding
	Confirm key.Binding
	Cancel  key.Binding
}

var keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "delete"),
		key.WithHelp("d", "delete"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y", "yes"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("n", "N", "esc"),
		key.WithHelp("n/esc", "no"),
	),
}

// New creates a new TUI model.
func New(client *api.Client) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedStyle
	delegate.Styles.SelectedDesc = dimmedStyle

	repoList := list.New([]list.Item{}, delegate, 0, 0)
	repoList.Title = "Repositories"
	repoList.SetShowStatusBar(true)
	repoList.SetFilteringEnabled(true)
	repoList.Styles.Title = titleStyle

	tagList := list.New([]list.Item{}, delegate, 0, 0)
	tagList.Title = "Tags"
	tagList.SetShowStatusBar(true)
	tagList.SetFilteringEnabled(true)
	tagList.Styles.Title = titleStyle

	return Model{
		client:   client,
		ctx:      context.Background(),
		state:    ViewRepositories,
		spinner:  s,
		repoList: repoList,
		tagList:  tagList,
		loading:  true,
	}
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadRepositories(),
	)
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state == ViewConfirmDelete {
			return m.handleConfirmDelete(msg)
		}
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h := msg.Height - 4
		m.repoList.SetSize(msg.Width, h)
		m.tagList.SetSize(msg.Width, h)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case reposLoadedMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, r := range msg {
			items[i] = repoItem(r)
		}
		m.repoList.SetItems(items)
		m.statusMsg = ""
		return m, nil

	case tagsLoadedMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, t := range msg {
			items[i] = t
		}
		m.tagList.SetItems(items)
		m.statusMsg = ""
		return m, nil

	case manifestLoadedMsg:
		m.loading = false
		m.manifest = msg
		m.state = ViewManifest
		m.statusMsg = ""
		return m, nil

	case deleteSuccessMsg:
		m.loading = false
		m.statusMsg = successStyle.Render(string(msg))
		if m.state == ViewTags || m.state == ViewConfirmDelete {
			m.state = ViewTags
			return m, m.loadTags(m.currentRepo)
		}
		return m, m.loadRepositories()

	case errMsg:
		m.loading = false
		m.err = msg
		m.statusMsg = errorStyle.Render("Error: " + msg.Error())
		return m, nil
	}

	switch m.state {
	case ViewRepositories:
		var cmd tea.Cmd
		m.repoList, cmd = m.repoList.Update(msg)
		cmds = append(cmds, cmd)
	case ViewTags:
		var cmd tea.Cmd
		m.tagList, cmd = m.tagList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Refresh):
		m.loading = true
		m.statusMsg = "Refreshing..."
		switch m.state {
		case ViewRepositories:
			return m, tea.Batch(m.spinner.Tick, m.loadRepositories())
		case ViewTags:
			return m, tea.Batch(m.spinner.Tick, m.loadTags(m.currentRepo))
		}

	case key.Matches(msg, keys.Back):
		switch m.state {
		case ViewTags:
			m.state = ViewRepositories
			m.currentRepo = ""
			return m, nil
		case ViewManifest:
			m.state = ViewTags
			m.manifest = nil
			return m, nil
		}

	case key.Matches(msg, keys.Enter):
		switch m.state {
		case ViewRepositories:
			if item := m.repoList.SelectedItem(); item != nil {
				m.currentRepo = string(item.(repoItem))
				m.state = ViewTags
				m.loading = true
				m.statusMsg = "Loading tags..."
				return m, tea.Batch(m.spinner.Tick, m.loadTags(m.currentRepo))
			}
		case ViewTags:
			if item := m.tagList.SelectedItem(); item != nil {
				tag := item.(tagItem)
				m.loading = true
				m.statusMsg = "Loading manifest..."
				return m, tea.Batch(m.spinner.Tick, m.loadManifest(m.currentRepo, tag.name))
			}
		}

	case key.Matches(msg, keys.Delete):
		switch m.state {
		case ViewTags:
			if item := m.tagList.SelectedItem(); item != nil {
				tag := item.(tagItem)
				m.deleteTarget = deleteTarget{
					repo:   m.currentRepo,
					tag:    tag.name,
					digest: tag.digest,
				}
				m.state = ViewConfirmDelete
				m.confirmYes = false
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	switch m.state {
	case ViewRepositories:
		m.repoList, cmd = m.repoList.Update(msg)
	case ViewTags:
		m.tagList, cmd = m.tagList.Update(msg)
	}
	return m, cmd
}

func (m Model) handleConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Confirm):
		m.state = ViewTags
		m.loading = true
		m.statusMsg = "Deleting..."
		return m, tea.Batch(
			m.spinner.Tick,
			m.deleteManifest(m.deleteTarget.repo, m.deleteTarget.digest),
		)
	case key.Matches(msg, keys.Cancel):
		m.state = ViewTags
		return m, nil
	case msg.String() == "left" || msg.String() == "right" || msg.String() == "tab":
		m.confirmYes = !m.confirmYes
		return m, nil
	case msg.String() == "enter":
		if m.confirmYes {
			m.state = ViewTags
			m.loading = true
			m.statusMsg = "Deleting..."
			return m, tea.Batch(
				m.spinner.Tick,
				m.deleteManifest(m.deleteTarget.repo, m.deleteTarget.digest),
			)
		}
		m.state = ViewTags
		return m, nil
	}
	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var content string

	switch m.state {
	case ViewRepositories:
		content = m.repoList.View()
	case ViewTags:
		content = m.tagList.View()
	case ViewManifest:
		content = m.renderManifest()
	case ViewConfirmDelete:
		content = m.tagList.View() + "\n" + m.renderConfirmDialog()
	}

	if m.loading {
		content = m.spinner.View() + " " + m.statusMsg + "\n\n" + content
	}

	statusBar := m.renderStatusBar()
	return content + "\n" + statusBar
}

func (m Model) renderManifest() string {
	if m.manifest == nil {
		return "No manifest loaded"
	}

	title := titleStyle.Render("Manifest: " + m.currentRepo)
	info := lipgloss.JoinVertical(lipgloss.Left,
		subtitleStyle.Render("Content-Type: "+m.manifest.ContentType),
		subtitleStyle.Render("Digest: "+m.manifest.Digest),
	)
	body := string(m.manifest.Body)
	if len(body) > 500 {
		body = body[:500] + "\n..."
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, info, "\n", body)
}

func (m Model) renderConfirmDialog() string {
	question := warningStyle.Render("Delete tag '" + m.deleteTarget.tag + "'?")
	hint := dimmedStyle.Render("This will delete the manifest by digest.")

	yesBtn := buttonStyle.Render("[ Yes ]")
	noBtn := buttonStyle.Render("[ No ]")
	if m.confirmYes {
		yesBtn = activeButtonStyle.Render("[ Yes ]")
	} else {
		noBtn = activeButtonStyle.Render("[ No ]")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesBtn, noBtn)
	content := lipgloss.JoinVertical(lipgloss.Center, question, hint, "", buttons)

	return dialogBoxStyle.Render(content)
}

func (m Model) renderStatusBar() string {
	var status string
	switch m.state {
	case ViewRepositories:
		status = "Repositories"
	case ViewTags:
		status = m.currentRepo + " > Tags"
	case ViewManifest:
		status = m.currentRepo + " > Manifest"
	case ViewConfirmDelete:
		status = "Confirm Delete"
	}

	help := helpStyle.Render("↑/↓: navigate • enter: select • d: delete • r: refresh • esc: back • q: quit")
	bar := statusBarStyle.Width(m.width).Render(status)

	return bar + "\n" + help
}

// Commands for async operations.
func (m Model) loadRepositories() tea.Cmd {
	return func() tea.Msg {
		repos, err := m.client.ListAllRepositories(m.ctx)
		if err != nil {
			return errMsg(err)
		}
		return reposLoadedMsg(repos)
	}
}

func (m Model) loadTags(repo string) tea.Cmd {
	return func() tea.Msg {
		tags, err := m.client.ListAllTags(m.ctx, repo)
		if err != nil {
			return errMsg(err)
		}

		items := make([]tagItem, 0, len(tags))
		for _, tag := range tags {
			manifest, err := m.client.HeadManifest(m.ctx, repo, tag)
			digest := ""
			if err == nil && manifest != nil {
				digest = manifest.Digest
			}
			items = append(items, tagItem{name: tag, digest: digest})
		}
		return tagsLoadedMsg(items)
	}
}

func (m Model) loadManifest(repo, reference string) tea.Cmd {
	return func() tea.Msg {
		manifest, err := m.client.GetManifest(m.ctx, repo, reference)
		if err != nil {
			return errMsg(err)
		}
		return manifestLoadedMsg(manifest)
	}
}

func (m Model) deleteManifest(repo, digest string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.DeleteManifest(m.ctx, repo, digest); err != nil {
			return errMsg(err)
		}
		return deleteSuccessMsg("Manifest deleted successfully")
	}
}

// Run starts the TUI application.
func Run(client *api.Client) error {
	slog.Debug("starting tui application")
	p := tea.NewProgram(New(client), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		slog.Error("tui application error", "error", err)
	}
	slog.Debug("tui application exited")
	return err
}
