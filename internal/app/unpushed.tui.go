package app

import (
	"fmt"
	"gunp/internal/gunp"
	logger "gunp/internal/log"
	"os"
	"strconv"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	state            status
	width            int
	height           int
	errorMessage     string
	showDetail       bool
	selectedRowIndex int
	// ui elements
	spinner  spinner.Model
	progress progress.Model
	table    table.Model
	// table *table.Table

	// data
	walkedCounter *gunp.Counter
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
	return unpushedAppModel{
		state:            loading,
		width:            0,
		height:           0,
		showDetail:       false,
		selectedRowIndex: 0,
		// ui elements
		progress: progress.New(progress.WithDefaultGradient()),
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("69")))),
		// table: table.New().
		// 	Headers([]string{"Repository", "Unpushed Commits"}...).
		// 	Border(lipgloss.NormalBorder()).
		// 	BorderStyle(re.NewStyle().Foreground(lipgloss.Color("238"))).
		// 	Border(lipgloss.ThickBorder()),
		table: table.New(
			table.WithColumns([]table.Column{
				{Title: "ID"},
				{Title: "Repository"},
				{Title: "Unpushed Commits"},
			}),
			table.WithFocused(true),
		),
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
		m.spinner.Tick,
		counterCmd(m.walkedCounter),
		discoveryCmd(m.discoveryDoneCh, m.gitPathsCh),
		scanningCmd(m.scanningDoneCh, m.gunpReposCh),
	)
}

func (m unpushedAppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

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
		cmds = append(cmds, scanningCmd(m.scanningDoneCh, m.gunpReposCh))
		cmds = append(cmds, m.progress.SetPercent(m.getProgressPercent()))

	case scanningDoneMsg:
		rows := []table.Row{}
		for i, repo := range m.gunpRepos {
			if len(repo.UnpushedCommits) > 0 {
				rows = append(rows, table.Row{strconv.Itoa(i), repo.Path, strconv.Itoa(len(repo.UnpushedCommits))})
			}
		}
		m.table.SetRows(rows)
		m.state = finished

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
					m.selectedRowIndex = selectedIndex
				}
			case "enter", " ":
				if m.showDetail {
					m.showDetail = false
				} else {
					m.showDetail = true
				}
			}
		}
	}

	// always pass msg to the table
	var tableCmd tea.Cmd
	m.table, tableCmd = m.table.Update(msg)
	cmds = append(cmds, tableCmd)

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

	// mini components
	uiTitle := m.uiTitle(m.state)

	container := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		Height(m.height - 2).
		Width(m.width - 2).
		AlignHorizontal(lipgloss.Center).
		AlignVertical(lipgloss.Center)

	// Calculate max width for each column:
	// - longest item in column
	// - longest column title
	// - max width is container width / number of columns
	columns := m.table.Columns()
	rows := m.table.Rows()
	for i := range columns {
		maxWidth := 0
		for _, r := range rows {
			if len(r[i]) > maxWidth {
				maxWidth = len(r[i])
			}

			if len(columns[i].Title) > maxWidth {
				maxWidth = len(columns[i].Title)
			}

			if maxWidth > (container.GetWidth() / len(columns)) {
				maxWidth = (container.GetWidth() / len(columns)) - 2
			}
		}
		columns[i].Width = maxWidth
	}
	m.table.SetColumns(columns)

	switch m.state {
	case loading:
		// show progress and text
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			uiTitle,
			"",
			m.progress.View(),
		)
	case scanning:
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			uiTitle,
			"",
			m.progress.View(),
		)
	case finished:
		// show the table with results
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			uiTitle,
			"",
			m.table.View(),
		)
	}

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

func (m unpushedAppModel) uiTitle(status status) string {
	titleGunp := "GitUNPushed by b3nab"
	titleDiscovery := fmt.Sprintf("👀 Discovering Repositories... %d (walked directories: %d)", len(m.gitPaths), m.walkedCounter.Get())
	titleDiscoveryDone := fmt.Sprintf("👀 Repository Discovered: %d (walked directories: %d)", len(m.gitPaths), m.walkedCounter.Get())
	titleScanning := fmt.Sprintf("🔍 Scanning Repositories... (%d/%d)", len(m.gunpRepos), len(m.gitPaths))
	titleScanningDone := fmt.Sprintf("🔎 Repository Scanned: %d", len(m.gunpRepos))

	switch status {
	case loading:
		return fmt.Sprintf("%s\n%s %s\n%s %s", titleGunp, m.spinner.View(), titleDiscovery, m.spinner.View(), titleScanning)
	case scanning:
		return fmt.Sprintf("%s\n%s\n%s %s", titleGunp, titleDiscoveryDone, m.spinner.View(), titleScanning)
	case finished:
		return fmt.Sprintf("%s\n%s\n%s", titleGunp, titleDiscoveryDone, titleScanningDone)
	}
	return ""
}
