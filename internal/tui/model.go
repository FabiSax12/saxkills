package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/FabiSax12/saxkills/internal/github"
	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/skills"
	"github.com/FabiSax12/saxkills/internal/update"
	"github.com/FabiSax12/saxkills/internal/version"
)

type screen int

const (
	screenHome screen = iota
	screenTargets
	screenSkills
	screenConfirm
	screenProgress
	screenResult
	screenStatus
)

type Services struct {
	Targets []install.Target
	Home    string
	Version string
	Latest  func(ctx context.Context) (github.Release, error)
	Load    func(ctx context.Context, rel github.Release) (update.Bundle, error)
}

type latestMsg struct {
	rel github.Release
	err error
}

type bundleMsg struct {
	tag    string
	bundle update.Bundle
	err    error
}

type appliedMsg struct {
	reports []update.Report
	err     error
}

var homeItems = []string{"Install", "Update", "Remove", "Status", "Quit"}

type Model struct {
	svc    Services
	width  int
	height int
	screen screen
	cursor int
	spin   spinner.Model

	checked  bool
	online   bool
	latest   github.Release
	outdated []update.Outdated
	notice   string

	mode         install.Mode
	targetCursor int
	targetSel    map[int]bool

	bundle    *update.Bundle
	loading   bool
	bundleErr error
	loadErr   error
	rows      []row
	rowCursor int
	rowOffset int
	picked    map[string]bool

	reports  []update.Report
	applyErr error
}

func New(svc Services) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleCursor
	return Model{svc: svc, spin: sp, targetSel: map[int]bool{}, picked: map[string]bool{}}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.fetchLatest())
}

func (m Model) fetchLatest() tea.Cmd {
	latest := m.svc.Latest
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		rel, err := latest(ctx)
		return latestMsg{rel: rel, err: err}
	}
}

func (m Model) loadBundle() tea.Cmd {
	load, rel := m.svc.Load, m.latest
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		b, err := load(ctx, rel)
		return bundleMsg{tag: rel.Tag, bundle: b, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.adjustScroll()
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case latestMsg:
		m.checked = true
		m.online = msg.err == nil
		if !m.online {
			m.bundle, m.loading = nil, false
			m.refreshOutdated()
			return m, nil
		}
		if m.bundle != nil && m.bundle.Release.Tag != msg.rel.Tag {
			m.bundle = nil
		}
		m.latest = msg.rel
		m.refreshOutdated()
		if m.bundle == nil && !m.loading {
			m.loading, m.bundleErr = true, nil
			return m, m.loadBundle()
		}
		return m, nil
	case bundleMsg:
		if !m.online || msg.tag != m.latest.Tag {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.bundleErr = msg.err
			if m.screen == screenSkills {
				m.loadErr = msg.err
			}
			return m, nil
		}
		m.bundle, m.bundleErr = &msg.bundle, nil
		m.refreshOutdated()
		if m.screen == screenSkills && m.mode != install.ModeRemove {
			m.rebuildRows()
		}
		return m, nil
	case appliedMsg:
		m.reports, m.applyErr = msg.reports, msg.err
		m.screen = screenResult
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.screen == screenProgress {
		return m, nil
	}
	if key == "q" {
		return m, tea.Quit
	}
	if key == "r" && m.screen == screenHome {
		m.checked, m.notice = false, ""
		return m, m.fetchLatest()
	}
	switch m.screen {
	case screenHome:
		return m.keyHome(key)
	case screenTargets:
		return m.keyTargets(key)
	case screenSkills:
		return m.keySkills(key)
	case screenConfirm:
		return m.keyConfirm(key)
	case screenResult:
		if key == "enter" || key == "esc" {
			return m.goHome(), nil
		}
	case screenStatus:
		if key == "enter" || key == "esc" {
			return m.goHome(), nil
		}
	}
	return m, nil
}

func (m Model) goHome() Model {
	m.screen = screenHome
	m.notice = ""
	m.reports, m.applyErr = nil, nil
	m.refreshOutdated()
	return m
}

func (m *Model) refreshOutdated() {
	m.outdated = nil
	if !m.online || m.bundle == nil {
		return
	}
	managed, _ := update.FindManaged(m.svc.Targets)
	c, _ := update.Classify(managed, m.bundle.Skills, m.latest.Tag)
	m.outdated = c.Outdated
}

func moveCursor(cursor int, key string, n int) int {
	switch key {
	case "up", "k":
		if cursor > 0 {
			return cursor - 1
		}
	case "down", "j":
		if cursor < n-1 {
			return cursor + 1
		}
	}
	return cursor
}

func (m Model) keyHome(key string) (tea.Model, tea.Cmd) {
	m.cursor = moveCursor(m.cursor, key, len(homeItems))
	if key != "enter" {
		return m, nil
	}
	m.notice = ""
	switch homeItems[m.cursor] {
	case "Install", "Update":
		if !m.checked {
			m.notice = "still checking for releases, try again in a moment"
			return m, nil
		}
		if !m.online {
			if m.cursor == 0 {
				m.notice = "offline: cannot install new skills while offline"
			} else {
				m.notice = "offline: cannot download updates while offline"
			}
			return m, nil
		}
		m.mode = install.ModeInstall
		if m.cursor == 1 {
			m.mode = install.ModeUpdate
		}
		return m.enterTargets(), nil
	case "Remove":
		m.mode = install.ModeRemove
		return m.enterTargets(), nil
	case "Status":
		m.screen = screenStatus
	case "Quit":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) enterTargets() Model {
	m.screen = screenTargets
	m.targetCursor = 0
	m.targetSel = map[int]bool{}
	m.loadErr = nil
	return m
}

func (m Model) keyTargets(key string) (tea.Model, tea.Cmd) {
	m.targetCursor = moveCursor(m.targetCursor, key, len(m.svc.Targets))
	switch key {
	case "esc":
		return m.goHome(), nil
	case " ", "space":
		if m.svc.Targets[m.targetCursor].Unavailable == "" {
			m.targetSel[m.targetCursor] = !m.targetSel[m.targetCursor]
		}
	case "enter":
		if len(m.selectedTargets()) == 0 {
			m.notice = "select at least one location"
			return m, nil
		}
		m.notice = ""
		return m.enterSkills()
	}
	return m, nil
}

func (m Model) selectedTargets() []install.Target {
	var out []install.Target
	for i, t := range m.svc.Targets {
		if m.targetSel[i] && t.Dir != "" {
			out = append(out, t)
		}
	}
	return out
}

func (m Model) enterSkills() (tea.Model, tea.Cmd) {
	m.screen = screenSkills
	m.loadErr = nil
	m.rows, m.rowCursor, m.rowOffset = nil, 0, 0
	m.picked = map[string]bool{}
	if m.mode == install.ModeRemove || m.bundle != nil {
		m.rebuildRows()
		return m, nil
	}
	if m.loading {
		return m, m.spin.Tick
	}
	m.loading, m.bundleErr = true, nil
	return m, tea.Batch(m.spin.Tick, m.loadBundle())
}

func (m *Model) rebuildRows() {
	var dirs []string
	for _, t := range m.selectedTargets() {
		dirs = append(dirs, t.Dir)
	}
	rows, err := buildRows(m.mode, dirs, m.available())
	if err != nil {
		m.notice = "some locations could not be read: " + err.Error()
	}
	m.rows = rows
	m.picked = map[string]bool{}
	if m.mode == install.ModeUpdate {
		for _, r := range rows {
			if r.state == install.UpdateAvailable {
				m.picked[r.name] = true
			}
		}
	}
}

func (m Model) keySkills(key string) (tea.Model, tea.Cmd) {
	if key == "esc" {
		m.screen = screenTargets
		m.loadErr = nil
		m.notice = ""
		return m, nil
	}
	if m.loading || m.loadErr != nil {
		return m, nil
	}
	m.rowCursor = moveCursor(m.rowCursor, key, len(m.rows))
	m.adjustScroll()
	switch key {
	case " ", "space":
		if len(m.rows) > 0 && m.rows[m.rowCursor].selectable() {
			name := m.rows[m.rowCursor].name
			m.picked[name] = !m.picked[name]
		}
	case "a":
		for _, r := range m.rows {
			if r.selectable() {
				m.picked[r.name] = true
			}
		}
	case "n":
		m.picked = map[string]bool{}
	case "enter":
		if len(m.pickedRows()) == 0 {
			m.notice = "select at least one skill"
			return m, nil
		}
		m.notice = ""
		m.screen = screenConfirm
	}
	return m, nil
}

func (m Model) pickedRows() []row {
	var out []row
	for _, r := range m.rows {
		if m.picked[r.name] && r.selectable() {
			out = append(out, r)
		}
	}
	return out
}

func (m Model) modifiedCount() int {
	n := 0
	for _, r := range m.pickedRows() {
		if r.state == install.Modified {
			n++
		}
	}
	return n
}

func (m Model) keyConfirm(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenSkills
	case "enter":
		return m.startApply(false)
	case "y":
		if m.modifiedCount() > 0 {
			return m.startApply(true)
		}
		return m.startApply(false)
	}
	return m, nil
}

func (m Model) startApply(force bool) (tea.Model, tea.Cmd) {
	m.screen = screenProgress
	targets := m.selectedTargets()
	rows := m.pickedRows()
	mode := m.mode
	tag := ""
	if m.bundle != nil {
		tag = m.bundle.Release.Tag
	}
	avail := m.available()
	cmd := func() tea.Msg {
		var main, removals []string
		for _, r := range rows {
			if mode == install.ModeUpdate && r.state == install.RemovedUpstream {
				removals = append(removals, r.name)
			} else {
				main = append(main, r.name)
			}
		}
		var reports []update.Report
		var errs []error
		for _, t := range targets {
			rep := update.Report{Target: t}
			reqs := []install.Request{{Mode: mode, Names: main, Version: tag, Available: avail, Force: force}}
			if len(removals) > 0 {
				reqs = append(reqs, install.Request{Mode: install.ModeRemove, Names: removals, Force: force})
			}
			var repErrs []error
			for _, req := range reqs {
				if len(req.Names) == 0 {
					continue
				}
				res, err := install.Execute(t.Dir, req)
				rep.Results = append(rep.Results, res...)
				if err != nil {
					repErrs = append(repErrs, err)
				}
			}
			rep.Err = errors.Join(repErrs...)
			if rep.Err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", t.Label(), rep.Err))
			}
			reports = append(reports, rep)
		}
		return appliedMsg{reports: reports, err: errors.Join(errs...)}
	}
	return m, tea.Batch(m.spin.Tick, cmd)
}

func (m *Model) adjustScroll() {
	visible := m.visibleRows()
	if m.rowCursor < m.rowOffset {
		m.rowOffset = m.rowCursor
	}
	if m.rowCursor >= m.rowOffset+visible {
		m.rowOffset = m.rowCursor - visible + 1
	}
}

func (m Model) visibleRows() int {
	if m.height == 0 {
		return len(m.rows)
	}
	v := m.height - 8
	if v < 3 {
		v = 3
	}
	return v
}

func (m Model) available() map[string]skills.Skill {
	if m.bundle == nil || m.mode == install.ModeRemove {
		return nil
	}
	return m.bundle.Skills
}

func (m Model) newerBinary() bool {
	return m.online && version.Newer(m.latest.Tag, m.svc.Version)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (m Model) display(path string) string {
	if m.svc.Home != "" && strings.HasPrefix(path, m.svc.Home) {
		return "~" + strings.TrimPrefix(path, m.svc.Home)
	}
	return path
}
