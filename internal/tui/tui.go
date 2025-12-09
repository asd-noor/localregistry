package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"localregistry/internal/api"
	"localregistry/internal/registry"
)

// ViewState represents the current view in the TUI.
type ViewState int

const (
	ViewRepositories ViewState = iota
	ViewTags
	ViewManifest
	ViewConfirmDelete
	ViewLogs
	ViewImageDetails
	ViewHelp
	ViewGarbageCollect
	ViewServerActions
)

// Model is the main TUI model.
type Model struct {
	client *api.Client
	ctx    context.Context

	state         ViewState
	previousState ViewState // For overlay views like help
	width         int
	height        int
	err           error
	loading       bool
	statusMsg     string
	confirmMsg    string
	confirmYes    bool
	deleteTarget  deleteTarget

	repoList    list.Model
	tagList     list.Model
	spinner     spinner.Model
	currentRepo string
	currentTag  string

	manifest *api.Manifest

	// Log viewing
	logsDocker    registry.DockerClient
	logsContainer string
	logsViewport  viewport.Model
	logsContent   strings.Builder
	logsFollowing bool
	logsCh        chan string
	logsCtx       context.Context
	logsCancelFn  context.CancelFunc

	// Server/Registry management
	registry       *registry.Registry
	serverStatus   *registry.RegistryInfo
	serverStatusCh chan *registry.RegistryInfo

	// Image details
	imageDetails *ImageDetails

	// GC state
	gcResult     *registry.GarbageCollectResult
	gcInProgress bool

	// Server action selection
	serverActionIdx int
}

type deleteTarget struct {
	repo   string
	tag    string
	digest string
}

// ImageDetails contains detailed information about an image.
type ImageDetails struct {
	Repository   string
	Tag          string
	Digest       string
	ContentType  string
	Architecture string
	OS           string
	Created      string
	Size         int64
	Layers       []LayerInfo
	Labels       map[string]string
}

// LayerInfo contains information about a single layer.
type LayerInfo struct {
	Digest    string
	Size      int64
	MediaType string
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
	reposLoadedMsg      []string
	tagsLoadedMsg       []tagItem
	manifestLoadedMsg   *api.Manifest
	deleteSuccessMsg    string
	errMsg              error
	logLineMsg          string
	logsStoppedMsg      struct{}
	serverStatusMsg     *registry.RegistryInfo
	imageDetailsMsg     *ImageDetails
	gcCompleteMsg       *registry.GarbageCollectResult
	serverActionDoneMsg string
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
	Logs    key.Binding
	Details key.Binding
	GC      key.Binding
	Server  key.Binding
	Tab     key.Binding
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
	Logs: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "logs"),
	),
	Details: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "info/details"),
	),
	GC: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "garbage collect"),
	),
	Server: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "server actions"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next option"),
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
		// Update viewport size for logs view
		if m.state == ViewLogs {
			m.logsViewport.Width = msg.Width - 4
			m.logsViewport.Height = h - 2
		}
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

	case logLineMsg:
		m.logsContent.WriteString(string(msg) + "\n")
		m.logsViewport.SetContent(m.logsContent.String())
		if m.logsFollowing {
			m.logsViewport.GotoBottom()
		}
		// Continue listening for more log lines
		return m, m.waitForLogLine()

	case logsStoppedMsg:
		m.logsFollowing = false
		m.statusMsg = "Log streaming stopped"
		return m, nil

	case serverStatusMsg:
		m.serverStatus = msg
		return m, nil

	case imageDetailsMsg:
		m.loading = false
		m.imageDetails = msg
		m.state = ViewImageDetails
		m.statusMsg = ""
		return m, nil

	case gcCompleteMsg:
		m.loading = false
		m.gcInProgress = false
		m.gcResult = msg
		if msg.ExitCode == 0 {
			m.statusMsg = successStyle.Render("Garbage collection completed")
		} else {
			m.statusMsg = errorStyle.Render("Garbage collection failed")
		}
		m.state = ViewGarbageCollect
		return m, nil

	case serverActionDoneMsg:
		m.loading = false
		m.statusMsg = successStyle.Render(string(msg))
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
	case ViewLogs:
		var cmd tea.Cmd
		m.logsViewport, cmd = m.logsViewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle special views with their own key handlers
	switch m.state {
	case ViewLogs:
		return m.handleLogsKeyMsg(msg)
	case ViewHelp:
		return m.handleHelpKeyMsg(msg)
	case ViewServerActions:
		return m.handleServerActionsKeyMsg(msg)
	}

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
		case ViewImageDetails:
			m.state = ViewTags
			m.imageDetails = nil
			return m, nil
		case ViewHelp:
			m.state = m.previousState
			return m, nil
		case ViewGarbageCollect:
			m.state = ViewRepositories
			m.gcResult = nil
			return m, nil
		case ViewServerActions:
			m.state = m.previousState
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

	case key.Matches(msg, keys.Logs):
		if m.logsDocker != nil && m.logsContainer != "" {
			return m.startLogsView()
		}

	case key.Matches(msg, keys.Help):
		m.previousState = m.state
		m.state = ViewHelp
		return m, nil

	case key.Matches(msg, keys.Details):
		// Show image details when in tags view
		if m.state == ViewTags {
			if item := m.tagList.SelectedItem(); item != nil {
				tag := item.(tagItem)
				m.currentTag = tag.name
				m.loading = true
				m.statusMsg = "Loading image details..."
				return m, tea.Batch(m.spinner.Tick, m.loadImageDetails(m.currentRepo, tag.name))
			}
		}

	case key.Matches(msg, keys.GC):
		// Trigger garbage collection
		if m.registry != nil && !m.gcInProgress {
			m.gcInProgress = true
			m.loading = true
			m.statusMsg = "Running garbage collection..."
			return m, tea.Batch(m.spinner.Tick, m.runGarbageCollect())
		}

	case key.Matches(msg, keys.Server):
		// Show server actions
		if m.registry != nil {
			m.previousState = m.state
			m.state = ViewServerActions
			m.serverActionIdx = 0
			return m, nil
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

	// Render server status panel at top if available
	serverPanel := m.renderServerStatus()

	switch m.state {
	case ViewRepositories:
		content = m.repoList.View()
	case ViewTags:
		content = m.tagList.View()
	case ViewManifest:
		content = m.renderManifest()
	case ViewConfirmDelete:
		content = m.tagList.View() + "\n" + m.renderConfirmDialog()
	case ViewLogs:
		content = m.renderLogs()
	case ViewImageDetails:
		content = m.renderImageDetails()
	case ViewHelp:
		// Render help as overlay on previous view
		var baseContent string
		switch m.previousState {
		case ViewRepositories:
			baseContent = m.repoList.View()
		case ViewTags:
			baseContent = m.tagList.View()
		default:
			baseContent = ""
		}
		content = baseContent + "\n" + m.renderHelp()
	case ViewGarbageCollect:
		content = m.renderGarbageCollect()
	case ViewServerActions:
		// Render server actions as overlay
		var baseContent string
		switch m.previousState {
		case ViewRepositories:
			baseContent = m.repoList.View()
		case ViewTags:
			baseContent = m.tagList.View()
		default:
			baseContent = ""
		}
		content = baseContent + "\n" + m.renderServerActions()
	}

	if m.loading {
		content = m.spinner.View() + " " + m.statusMsg + "\n\n" + content
	}

	statusBar := m.renderStatusBar()

	// Combine all parts
	if serverPanel != "" {
		return serverPanel + "\n" + content + "\n" + statusBar
	}
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
	case ViewLogs:
		status = "Container Logs"
		if m.logsFollowing {
			status += " (following)"
		}
	case ViewImageDetails:
		status = m.currentRepo + ":" + m.currentTag + " > Details"
	case ViewHelp:
		status = "Help"
	case ViewGarbageCollect:
		status = "Garbage Collection"
	case ViewServerActions:
		status = "Server Actions"
	}

	var help string
	switch m.state {
	case ViewLogs:
		help = helpStyle.Render("↑/↓/pgup/pgdn: scroll • f: toggle follow • esc: back • q: quit")
	case ViewHelp:
		help = helpStyle.Render("Press ? or Esc to close help")
	case ViewServerActions:
		help = helpStyle.Render("↑/↓: select • enter: execute • esc: cancel")
	case ViewImageDetails, ViewGarbageCollect:
		help = helpStyle.Render("esc: back • q: quit")
	default:
		help = helpStyle.Render("?: help • i: details • s: server • g: gc • d: delete • r: refresh • q: quit")
	}
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

// RunWithLogs starts the TUI application with log viewing capability.
func RunWithLogs(client *api.Client, docker registry.DockerClient, containerID string) error {
	slog.Debug("starting tui application with logs", "container_id", containerID)
	m := New(client)
	m.logsDocker = docker
	m.logsContainer = containerID
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		slog.Error("tui application error", "error", err)
	}
	slog.Debug("tui application exited")
	return err
}

// RunWithRegistry starts the TUI application with full registry management capabilities.
func RunWithRegistry(client *api.Client, reg *registry.Registry, docker registry.DockerClient, containerID string) error {
	slog.Debug("starting tui application with registry management", "container_id", containerID)
	m := New(client)
	m.registry = reg
	if docker != nil {
		m.logsDocker = docker
		m.logsContainer = containerID
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		slog.Error("tui application error", "error", err)
	}
	slog.Debug("tui application exited")
	return err
}

// SetLogsSource configures the Docker client and container for log viewing.
func (m *Model) SetLogsSource(docker registry.DockerClient, containerID string) {
	m.logsDocker = docker
	m.logsContainer = containerID
}

// handleLogsKeyMsg handles key events in the logs view.
func (m Model) handleLogsKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.stopLogs()
		return m, tea.Quit

	case key.Matches(msg, keys.Back):
		m.stopLogs()
		m.state = ViewRepositories
		m.logsContent.Reset()
		return m, nil

	case msg.String() == "f":
		m.logsFollowing = !m.logsFollowing
		if m.logsFollowing {
			m.logsViewport.GotoBottom()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.logsViewport, cmd = m.logsViewport.Update(msg)
	return m, cmd
}

// startLogsView initializes and switches to the logs view.
func (m Model) startLogsView() (tea.Model, tea.Cmd) {
	// Initialize viewport
	h := m.height - 6
	if h < 5 {
		h = 5
	}
	m.logsViewport = viewport.New(m.width-4, h)
	m.logsViewport.Style = logViewportStyle
	m.logsContent.Reset()
	m.logsFollowing = true
	m.state = ViewLogs
	m.statusMsg = "Streaming logs..."

	// Create channel and context for log streaming
	m.logsCh = make(chan string, 100)
	m.logsCtx, m.logsCancelFn = context.WithCancel(context.Background())

	// Start background log streaming
	go m.streamLogsBackground()

	return m, m.waitForLogLine()
}

// stopLogs cancels the log streaming goroutine.
func (m *Model) stopLogs() {
	if m.logsCancelFn != nil {
		m.logsCancelFn()
		m.logsCancelFn = nil
	}
}

// renderLogs renders the logs view.
func (m Model) renderLogs() string {
	title := titleStyle.Render("Container Logs: " + truncateID(m.logsContainer))

	var followIndicator string
	if m.logsFollowing {
		followIndicator = successStyle.Render(" [FOLLOWING]")
	} else {
		followIndicator = dimmedStyle.Render(" [PAUSED]")
	}

	header := lipgloss.JoinHorizontal(lipgloss.Left, title, followIndicator)
	return lipgloss.JoinVertical(lipgloss.Left, header, "", m.logsViewport.View())
}

// truncateID truncates a container ID for display.
func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// streamLogs returns a command that streams container logs.
func (m Model) streamLogs() tea.Cmd {
	return func() tea.Msg {
		if m.logsDocker == nil || m.logsContainer == "" {
			return errMsg(io.EOF)
		}

		ctx, cancel := context.WithCancel(context.Background())
		// Store cancel function - note: this requires pointer receiver to persist
		// We'll handle cancellation via context

		reader, err := m.logsDocker.StreamLogs(ctx, m.logsContainer, registry.LogsOptions{
			Follow: true,
			Tail:   "100",
		})
		if err != nil {
			cancel()
			return errMsg(err)
		}

		// Start goroutine to read logs
		go func() {
			defer reader.Close()
			defer cancel()
			streamLogsToProgram(ctx, reader, m.logsContainer, m.logsDocker)
		}()

		return nil
	}
}

// waitForLogLine returns a command that waits for the next log line from the channel.
func (m Model) waitForLogLine() tea.Cmd {
	return func() tea.Msg {
		if m.logsCh == nil {
			return logsStoppedMsg{}
		}
		select {
		case line, ok := <-m.logsCh:
			if !ok {
				return logsStoppedMsg{}
			}
			return logLineMsg(line)
		case <-m.logsCtx.Done():
			return logsStoppedMsg{}
		}
	}
}

// streamLogsBackground reads logs in the background and sends them to the channel.
func (m Model) streamLogsBackground() {
	if m.logsDocker == nil || m.logsContainer == "" || m.logsCh == nil {
		return
	}
	defer close(m.logsCh)

	reader, err := m.logsDocker.StreamLogs(m.logsCtx, m.logsContainer, registry.LogsOptions{
		Follow: true,
		Tail:   "100",
	})
	if err != nil {
		slog.Error("failed to start log streaming", "error", err)
		return
	}
	defer reader.Close()

	// Check if container uses TTY
	inspect, err := m.logsDocker.InspectContainer(m.logsCtx, m.logsContainer)
	if err != nil {
		slog.Error("failed to inspect container for log streaming", "error", err)
		return
	}

	if inspect.Config.Tty {
		// TTY mode: read lines directly
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			select {
			case <-m.logsCtx.Done():
				return
			case m.logsCh <- logPrefixStdoutStyle.Render("[STDOUT]: ") + scanner.Text():
			}
		}
	} else {
		// Non-TTY: demultiplex
		m.demuxLogsToChannel(reader)
	}
}

// demuxLogsToChannel reads Docker's multiplexed log stream and sends to channel.
func (m Model) demuxLogsToChannel(r io.Reader) {
	header := make([]byte, 8)

	for {
		select {
		case <-m.logsCtx.Done():
			return
		default:
		}

		_, err := io.ReadFull(r, header)
		if err != nil {
			return
		}

		streamType := header[0]
		size := uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7])

		payload := make([]byte, size)
		_, err = io.ReadFull(r, payload)
		if err != nil {
			return
		}

		// Process lines with appropriate styling
		var prefix string
		switch streamType {
		case 1:
			prefix = logPrefixStdoutStyle.Render("[STDOUT]: ")
		case 2:
			prefix = logPrefixStderrStyle.Render("[STDERR]: ")
		default:
			prefix = dimmedStyle.Render("[UNKNOWN]: ")
		}

		scanner := bufio.NewScanner(strings.NewReader(string(payload)))
		for scanner.Scan() {
			var styledLine string
			if streamType == 2 {
				styledLine = prefix + logStderrStyle.Render(scanner.Text())
			} else {
				styledLine = prefix + logStdoutStyle.Render(scanner.Text())
			}

			select {
			case <-m.logsCtx.Done():
				return
			case m.logsCh <- styledLine:
			}
		}
	}
}

// streamLogsToProgram reads logs and would send them to the program.
// Note: In a real implementation, you'd pass the *tea.Program to send messages.
// For now, this demonstrates the pattern - actual integration requires Program.Send().
func streamLogsToProgram(ctx context.Context, reader io.ReadCloser, containerID string, docker registry.DockerClient) {
	// Check if container uses TTY by inspecting it
	inspect, err := docker.InspectContainer(ctx, containerID)
	if err != nil {
		slog.Error("failed to inspect container for log streaming", "error", err)
		return
	}

	if inspect.Config.Tty {
		// TTY mode: read lines directly
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Text()
				slog.Debug("log line (tty)", "line", line)
				// Would send: program.Send(logLineMsg("[STDOUT]: " + line + "\n"))
			}
		}
	} else {
		// Non-TTY: demultiplex
		demuxLogsToChannel(ctx, reader)
	}
}

// demuxLogsToChannel reads Docker's multiplexed log stream.
func demuxLogsToChannel(ctx context.Context, r io.Reader) {
	header := make([]byte, 8)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, err := io.ReadFull(r, header)
		if err != nil {
			return
		}

		streamType := header[0]
		size := uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7])

		var prefix string
		switch streamType {
		case 1:
			prefix = "[STDOUT]: "
		case 2:
			prefix = "[STDERR]: "
		default:
			prefix = "[UNKNOWN]: "
		}

		payload := make([]byte, size)
		_, err = io.ReadFull(r, payload)
		if err != nil {
			return
		}

		// Process lines
		scanner := bufio.NewScanner(strings.NewReader(string(payload)))
		for scanner.Scan() {
			line := prefix + scanner.Text() + "\n"
			slog.Debug("log line (demux)", "line", line)
			// Would send: program.Send(logLineMsg(line))
		}
	}
}

// loadImageDetails loads detailed information about an image.
func (m Model) loadImageDetails(repo, tag string) tea.Cmd {
	return func() tea.Msg {
		manifest, err := m.client.GetManifest(m.ctx, repo, tag)
		if err != nil {
			return errMsg(err)
		}

		details := &ImageDetails{
			Repository:  repo,
			Tag:         tag,
			Digest:      manifest.Digest,
			ContentType: manifest.ContentType,
		}

		// Parse manifest to extract layer info
		details.Layers = parseManifestLayers(manifest.Body)

		return imageDetailsMsg(details)
	}
}

// parseManifestLayers extracts layer information from manifest JSON.
func parseManifestLayers(body []byte) []LayerInfo {
	var layers []LayerInfo

	// Simple JSON parsing to extract layers
	type manifestJSON struct {
		Layers []struct {
			MediaType string `json:"mediaType"`
			Size      int64  `json:"size"`
			Digest    string `json:"digest"`
		} `json:"layers"`
		Config struct {
			MediaType string `json:"mediaType"`
			Size      int64  `json:"size"`
			Digest    string `json:"digest"`
		} `json:"config"`
	}

	var mj manifestJSON
	if err := json.Unmarshal(body, &mj); err != nil {
		return layers
	}

	for _, l := range mj.Layers {
		layers = append(layers, LayerInfo{
			Digest:    l.Digest,
			Size:      l.Size,
			MediaType: l.MediaType,
		})
	}

	return layers
}

// runGarbageCollect triggers garbage collection on the registry.
func (m Model) runGarbageCollect() tea.Cmd {
	return func() tea.Msg {
		if m.registry == nil {
			return errMsg(fmt.Errorf("registry not configured"))
		}

		result, err := m.registry.GarbageCollect(m.ctx, true)
		if err != nil {
			return errMsg(err)
		}

		return gcCompleteMsg(result)
	}
}

// loadServerStatus fetches the current registry container status.
func (m Model) loadServerStatus() tea.Cmd {
	return func() tea.Msg {
		if m.registry == nil {
			return nil
		}

		info, err := m.registry.Info(m.ctx)
		if err != nil {
			slog.Debug("failed to get server status", "error", err)
			return nil
		}

		return serverStatusMsg(info)
	}
}

// serverAction performs a server action (start, stop, restart).
func (m Model) serverAction(action string) tea.Cmd {
	return func() tea.Msg {
		if m.registry == nil {
			return errMsg(fmt.Errorf("registry not configured"))
		}

		var err error
		switch action {
		case "start":
			err = m.registry.Start(m.ctx)
		case "stop":
			err = m.registry.Stop(m.ctx)
		case "restart":
			err = m.registry.Restart(m.ctx)
		default:
			return errMsg(fmt.Errorf("unknown action: %s", action))
		}

		if err != nil {
			return errMsg(err)
		}

		return serverActionDoneMsg(fmt.Sprintf("Server %sed successfully", action))
	}
}

// handleServerActionsKeyMsg handles key events in the server actions view.
func (m Model) handleServerActionsKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	actions := []string{"start", "stop", "restart"}

	switch {
	case key.Matches(msg, keys.Back), key.Matches(msg, keys.Cancel):
		m.state = m.previousState
		return m, nil

	case key.Matches(msg, keys.Up), msg.String() == "k":
		if m.serverActionIdx > 0 {
			m.serverActionIdx--
		}
		return m, nil

	case key.Matches(msg, keys.Down), msg.String() == "j":
		if m.serverActionIdx < len(actions)-1 {
			m.serverActionIdx++
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		action := actions[m.serverActionIdx]
		m.loading = true
		m.statusMsg = fmt.Sprintf("Executing %s...", action)
		m.state = m.previousState
		return m, tea.Batch(m.spinner.Tick, m.serverAction(action))
	}

	return m, nil
}

// handleHelpKeyMsg handles key events in the help view.
func (m Model) handleHelpKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back), key.Matches(msg, keys.Help), key.Matches(msg, keys.Quit):
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		m.state = m.previousState
		return m, nil
	}
	return m, nil
}

// SetRegistry configures the registry manager for server operations.
func (m *Model) SetRegistry(reg *registry.Registry) {
	m.registry = reg
}

// renderImageDetails renders the image details view.
func (m Model) renderImageDetails() string {
	if m.imageDetails == nil {
		return "No image details loaded"
	}

	d := m.imageDetails
	title := titleStyle.Render(fmt.Sprintf("Image: %s:%s", d.Repository, d.Tag))

	var lines []string
	lines = append(lines, subtitleStyle.Render("Digest: "+truncateDigest(d.Digest)))
	lines = append(lines, subtitleStyle.Render("Content-Type: "+d.ContentType))

	if len(d.Layers) > 0 {
		lines = append(lines, "")
		lines = append(lines, titleStyle.Render("Layers:"))

		var totalSize int64
		for i, l := range d.Layers {
			totalSize += l.Size
			sizeStr := formatBytes(l.Size)
			digestShort := l.Digest
			if len(digestShort) > 30 {
				digestShort = digestShort[:30] + "..."
			}
			line := fmt.Sprintf("  %d. %s  %s", i+1, sizeStyle.Render(sizeStr), layerStyle.Render(digestShort))
			lines = append(lines, line)
		}
		lines = append(lines, "")
		lines = append(lines, sizeStyle.Render(fmt.Sprintf("Total Size: %s (%d layers)", formatBytes(totalSize), len(d.Layers))))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{title, ""}, lines...)...)
	return content
}

// renderHelp renders the help overlay.
func (m Model) renderHelp() string {
	title := titleStyle.Render("Keyboard Shortcuts")

	sections := []struct {
		name  string
		binds []struct{ key, desc string }
	}{
		{
			name: "Navigation",
			binds: []struct{ key, desc string }{
				{"↑/k", "Move up"},
				{"↓/j", "Move down"},
				{"Enter", "Select / Open"},
				{"Esc", "Go back"},
				{"/", "Filter list"},
			},
		},
		{
			name: "Actions",
			binds: []struct{ key, desc string }{
				{"d", "Delete selected tag"},
				{"i", "Image details"},
				{"r", "Refresh"},
				{"l", "View container logs"},
			},
		},
		{
			name: "Server",
			binds: []struct{ key, desc string }{
				{"s", "Server actions (start/stop/restart)"},
				{"g", "Run garbage collection"},
			},
		},
		{
			name: "General",
			binds: []struct{ key, desc string }{
				{"?", "Toggle help"},
				{"q", "Quit"},
			},
		},
	}

	var lines []string
	for _, section := range sections {
		lines = append(lines, helpSectionStyle.Render(section.name))
		for _, b := range section.binds {
			line := helpKeyStyle.Render(b.key) + helpDescStyle.Render(b.desc)
			lines = append(lines, line)
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return helpOverlayStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, "", content))
}

// renderGarbageCollect renders the garbage collection result view.
func (m Model) renderGarbageCollect() string {
	title := titleStyle.Render("Garbage Collection")

	if m.gcResult == nil {
		return title + "\n\nNo results yet."
	}

	var status string
	if m.gcResult.ExitCode == 0 {
		status = actionSuccessStyle.Render("✓ Completed successfully")
	} else {
		status = actionErrorStyle.Render(fmt.Sprintf("✗ Failed (exit code: %d)", m.gcResult.ExitCode))
	}

	output := m.gcResult.Output
	if len(output) > 1000 {
		output = output[:1000] + "\n..."
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		status,
		"",
		subtitleStyle.Render("Output:"),
		output,
	)
}

// renderServerActions renders the server actions menu.
func (m Model) renderServerActions() string {
	title := titleStyle.Render("Server Actions")

	actions := []struct {
		name string
		desc string
	}{
		{"start", "Start the registry container"},
		{"stop", "Stop the registry container"},
		{"restart", "Restart the registry container"},
	}

	var lines []string
	for i, a := range actions {
		prefix := "  "
		style := normalStyle
		if i == m.serverActionIdx {
			prefix = "> "
			style = selectedStyle
		}
		line := style.Render(fmt.Sprintf("%s%s - %s", prefix, a.name, a.desc))
		lines = append(lines, line)
	}

	// Show current server status if available
	var statusLine string
	if m.serverStatus != nil {
		if m.serverStatus.Running {
			statusLine = statusRunningStyle.Render("● Running")
		} else {
			statusLine = statusStoppedStyle.Render("○ Stopped")
		}
		statusLine = "\n" + subtitleStyle.Render("Status: ") + statusLine
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return dialogBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, statusLine, "", content))
}

// renderServerStatus renders a compact server status bar.
func (m Model) renderServerStatus() string {
	if m.serverStatus == nil {
		return ""
	}

	var status string
	if m.serverStatus.Running {
		status = statusRunningStyle.Render("● Running")
	} else {
		status = statusStoppedStyle.Render("○ Stopped")
	}

	info := fmt.Sprintf("Registry: %s  %s", m.serverStatus.Address, status)
	return statusPanelStyle.Width(m.width - 4).Render(info)
}

// formatBytes formats a byte size to human readable format.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
