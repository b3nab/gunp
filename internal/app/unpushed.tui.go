package app

import (
	"fmt"
	"gunp/internal/gunp"
	logger "gunp/internal/log"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rmhubbert/bubbletea-overlay"
)

func StartUnpushedApp() {
	m := NewUnpushedModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logger.Get().Error("StartUnpushedApp", "err", err)
		os.Exit(1)
	}
}

type status int

const (
	loading status = iota
	errorStatus
	scanning
	finished
)

type unpushedAppModel struct {
	state        status
	width        int
	height       int
	errorMessage string
	showDetail   bool
	cursorRepo   int
	cursorCommit int
	// ui elements
	stopwatch    stopwatch.Model
	spinner      spinner.Model
	progress     progress.Model
	table        table.Model
	tableCommits table.Model

	// data
	walkedCounter *gunp.Counter
	unpushedCount int
	gitPaths      []string
	gunpRepos     []*gunp.GunpRepo

	// channels
	discoveryDoneCh <-chan bool
	scanningDoneCh  <-chan bool
	gitPathsCh      <-chan string
	gunpReposCh     <-chan *gunp.GunpRepo
}

func NewUnpushedModel() unpushedAppModel {
	rootDir, discoveryDoneCh, scanningDoneCh, walkedCounter, gitPathsCh, gunpReposCh, err := gunp.GunpTUI()
	if err != nil {
		logger.Get().Error("GunpTUI", "rootDir", rootDir, "err", err)
		return unpushedAppModel{
			state:        errorStatus,
			errorMessage: err.Error(),
		}
	}

	uiTable := table.New(
		table.WithColumns([]table.Column{
			{Title: "ID"},
			{Title: "Repository"},
			{Title: "Unpushed Commits"},
		}),
		table.WithFocused(true),
		table.WithStyles(TableStyle()),
	)
	uiTableCommits := table.New(
		table.WithColumns([]table.Column{
			{Title: "Hash"},
			{Title: "Author"},
			{Title: "Message"},
		}),
		table.WithFocused(true),
		table.WithStyles(TableStyle()),
	)

	return unpushedAppModel{
		state:        loading,
		width:        0,
		height:       0,
		showDetail:   false,
		cursorRepo:   0,
		cursorCommit: 0,
		// ui elements
		stopwatch:    stopwatch.NewWithInterval(time.Millisecond),
		progress:     progress.New(progress.WithDefaultGradient()),
		spinner:      spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("69")))),
		table:        uiTable,
		tableCommits: uiTableCommits,
		// data
		walkedCounter: walkedCounter,
		gitPaths:      []string{},
		gunpRepos:     []*gunp.GunpRepo{},
		// channels
		discoveryDoneCh: discoveryDoneCh,
		scanningDoneCh:  scanningDoneCh,
		gitPathsCh:      gitPathsCh,
		gunpReposCh:     gunpReposCh,
	}
}

type uiUpdateMsg struct{}

func uiUpdateCmd() tea.Cmd {
	return nil
}

type discoveryProgressMsg struct {
	gitPath string
}
type discoveryDoneMsg struct{}

func discoveryCmd(discoveryDoneCh <-chan bool, gitPathsCh <-chan string) tea.Cmd {
	return func() tea.Msg {
		select {
		case _, ok := <-discoveryDoneCh:
			if !ok {
				return discoveryDoneMsg{}
			}
			return discoveryDoneMsg{}
		case gitPath, ok := <-gitPathsCh:
			if !ok {
				return discoveryDoneMsg{}
			}
			return discoveryProgressMsg{gitPath: gitPath}
		}
	}
}

func counterCmd(walkedCounter *gunp.Counter) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-walkedCounter.UpdatedCh:
			// let it run to flush the counter
			return nil
		default:
			return nil
		}
	}
}

type scanningProgressMsg struct {
	gunpRepo *gunp.GunpRepo
}
type scanningDoneMsg struct{}

func scanningCmd(scanningDoneCh <-chan bool, gunpReposCh <-chan *gunp.GunpRepo) tea.Cmd {
	return func() tea.Msg {
		select {
		case _, ok := <-scanningDoneCh:
			if !ok {
				return scanningDoneMsg{}
			}
			return scanningDoneMsg{}
		case gunpRepo, ok := <-gunpReposCh:
			if !ok {
				return scanningDoneMsg{}
			}
			return scanningProgressMsg{gunpRepo: gunpRepo}
		}
	}
}

func (m unpushedAppModel) getProgressPercent() float64 {
	if len(m.gitPaths) == 0 {
		return 0
	}
	return float64(len(m.gunpRepos)) / float64(len(m.gitPaths))
}

// ------------------------------------------------------------
// BUBBLETEA FUNCTIONS
// ------------------------------------------------------------

func (m unpushedAppModel) Init() tea.Cmd {
	return tea.Batch(
		m.stopwatch.Init(),
		m.spinner.Tick,
		counterCmd(m.walkedCounter),
		discoveryCmd(m.discoveryDoneCh, m.gitPathsCh),
		scanningCmd(m.scanningDoneCh, m.gunpReposCh),
	)
}

func (m unpushedAppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// always pass msg to the table
	var tableCmd tea.Cmd
	if !m.showDetail {
		m.table, tableCmd = m.table.Update(msg)
		cmds = append(cmds, tableCmd)
	} else {
		m.tableCommits, tableCmd = m.tableCommits.Update(msg)
		cmds = append(cmds, tableCmd)
	}
	var stopwatchCmd tea.Cmd
	m.stopwatch, stopwatchCmd = m.stopwatch.Update(msg)
	cmds = append(cmds, stopwatchCmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress.Width = msg.Width - 4

	case discoveryProgressMsg:
		if msg.gitPath != "" {
			m.gitPaths = append(m.gitPaths, msg.gitPath)
		}
		cmds = append(cmds, discoveryCmd(m.discoveryDoneCh, m.gitPathsCh))
		cmds = append(cmds, m.progress.SetPercent(m.getProgressPercent()))

	case discoveryDoneMsg:
		if m.state == loading {
			m.state = scanning
			cmds = append(cmds, scanningCmd(m.scanningDoneCh, m.gunpReposCh))
		}

	case scanningProgressMsg:
		m.gunpRepos = append(m.gunpRepos, msg.gunpRepo)
		m.unpushedCount += len(msg.gunpRepo.UnpushedCommits)
		cmds = append(cmds, scanningCmd(m.scanningDoneCh, m.gunpReposCh))
		cmds = append(cmds, m.progress.SetPercent(m.getProgressPercent()))

	case scanningDoneMsg:
		rows := []table.Row{}
		unpushedCount := 0
		for i, repo := range m.gunpRepos {
			unpushedCount += len(repo.UnpushedCommits)
			if len(repo.UnpushedCommits) > 0 {
				rows = append(rows, table.Row{strconv.Itoa(i), repo.Path, strconv.Itoa(len(repo.UnpushedCommits))})
			}
		}
		m.unpushedCount = unpushedCount
		m.table.SetRows(rows)
		m.state = finished
		cmds = append(cmds, m.stopwatch.Stop())

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		}
		switch m.state {
		case finished:
			switch msg.String() {
			case "down", "up", "j", "k":
				selectedStrIndex := m.table.SelectedRow()[0]
				if selectedIndex, err := strconv.Atoi(selectedStrIndex); err == nil {
					m.cursorRepo = selectedIndex
				}
			case "enter", "v":
				if m.showDetail {
					m.showDetail = false
				} else {
					m.showDetail = true
					rows := []table.Row{}
					for _, cmt := range m.gunpRepos[m.cursorRepo].UnpushedCommits {
						rows = append(rows, table.Row{cmt.Hash.String(), cmt.Author.String(), cmt.Message})
					}
					m.tableCommits.SetRows(rows)
					cmds = append(cmds, uiUpdateCmd())
				}
			}
		}
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m unpushedAppModel) View() string {
	var content string

	if m.width == 0 || m.height == 0 {
		return "Loading TUI..."
	}

	// container := lipgloss.NewStyle().
	// 	Border(lipgloss.RoundedBorder(), true).
	// 	Height(m.height - 2).
	// 	Width(m.width - 2).
	// 	AlignHorizontal(lipgloss.Center).
	// 	AlignVertical(lipgloss.Center)

	// Calculate max width for each column:
	// - longest item in column
	// - longest column title
	// - max width is container width / number of columns
	m.table.SetColumns(updateWidthColumns(m.table, m.width))
	m.tableCommits.SetColumns(updateWidthColumns(m.tableCommits, m.width))

	switch m.state {
	case loading:
		// show progress and text
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			m.uiTitle(),
			"",
			m.progress.View(),
		)
	case scanning:
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			m.uiTitle(),
			"",
			m.progress.View(),
		)
	case finished:
		// show the table with results
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			m.uiTitle(),
			"",
			m.uiTable(),
			"\n\n\n",
			m.uiHelpText(),
		)
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		m.uiOverlay(
			content,
		),
	)
}

func updateWidthColumns(t table.Model, w int) []table.Column {
	columns := t.Columns()
	rows := t.Rows()
	takenWidth := 0
	for i := range columns {
		if i == 0 {
			columns[i].Width = 5
			continue
		}
		// default maxWidth to m.width / number of columns
		maxWidth := (w - takenWidth) / len(columns)
		for _, r := range rows {
			minWidthRow := len(r[i])
			if minWidthRow > maxWidth {
				maxWidth = minWidthRow
			}

			if len(columns[i].Title) > maxWidth {
				maxWidth = len(columns[i].Title)
			}

			if maxWidth > (w / len(columns)) {
				maxWidth = (w / len(columns)) - 2
			}
		}
		takenWidth += maxWidth
		columns[i].Width = maxWidth
	}
	return columns
}

func (m unpushedAppModel) uiTitle() string {
	// titleGunp := fmt.Sprintf("%d-%v\nGitUNPushed by b3nab", m.cursorRepo, m.showDetail)
	titleGunp := "GitUNPushed by b3nab"
	titleWalked := fmt.Sprintf("Walked Directories: %d", m.walkedCounter.Get())
	titleDiscovery := fmt.Sprintf("👀 Discovering Repositories... %d", len(m.gitPaths))
	titleDiscoveryDone := fmt.Sprintf("👀 Repository Discovered: %d", len(m.gitPaths))
	titleScanning := fmt.Sprintf("🔍 Scanning Repositories... (%d/%d)", len(m.gunpRepos), len(m.gitPaths))
	titleScanningDone := fmt.Sprintf("🔎 Repository Scanned: %d", len(m.gunpRepos))
	titleUnpushed := fmt.Sprintf("🐙 Unpushed Commits: %d", m.unpushedCount)

	switch m.state {
	case loading:
		return fmt.Sprintf("%s%s\n%s\n%s %s\n%s %s\n%s", titleGunp, m.uiStopwatch(), titleWalked, m.uiSpinner(), titleDiscovery, m.uiSpinner(), titleScanning, titleUnpushed)
	case scanning:
		return fmt.Sprintf("%s%s\n%s\n%s\n%s %s\n%s", titleGunp, m.uiStopwatch(), titleWalked, titleDiscoveryDone, m.uiSpinner(), titleScanning, titleUnpushed)
	case finished:
		return fmt.Sprintf("%s%s\n%s\n%s\n%s\n%s", titleGunp, m.uiStopwatch(), titleWalked, titleDiscoveryDone, titleScanningDone, titleUnpushed)
	}
	return ""
}

func (m unpushedAppModel) uiHelpText() string {
	switch m.state {
	case loading:
		return "Press 'q' to quit, 'h' for help"
	case scanning:
		return "Press 'q' to quit, 'h' for help"
	case finished:
		return "Press 'q' to quit, 'j'/'k'/'up'/'down' to navigate, 'v'/'enter' to toggle detail"
	}
	return ""
}

func (m unpushedAppModel) uiStopwatch() string {
	if m.stopwatch.Running() {
		return fmt.Sprintf("\n%s\n", m.stopwatch.View())
	} else {
		return fmt.Sprintf("\nCompleted in %s\n", m.stopwatch.View())
	}
}

func (m unpushedAppModel) uiSpinner() string {
	return m.spinner.View()
}

func (m unpushedAppModel) uiTable() string {
	m.table.SetStyles(TableStyle())
	return TableWrapperStyle().Render(m.table.View())
}

func (m unpushedAppModel) uiOverlay(content string) string {
	if m.showDetail {
		selectedRepo := m.gunpRepos[m.cursorRepo]
		m.tableCommits.SetStyles(TableStyle())
		detailContent := fmt.Sprintf("Path: %s\nUnpushed Commits: %d\n%s", selectedRepo.Path, len(selectedRepo.UnpushedCommits), TableWrapperStyle().Render(m.tableCommits.View()))
		detailView := TableWrapperStyle().Render(detailContent)
		return overlay.Composite(detailView, content, overlay.Center, overlay.Center, 0.0, 0.0)
	}
	return content
}
