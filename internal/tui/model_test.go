package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FabiSax12/saxkills/internal/github"
	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/skills"
	"github.com/FabiSax12/saxkills/internal/update"
)

type env struct {
	home, project string
	rel           github.Release
	latestErr     error
	skills        map[string]skills.Skill
}

func sk(name, body string) skills.Skill {
	return skills.Skill{Name: name, Description: "desc of " + name, Files: map[string]skills.File{"SKILL.md": {Data: []byte(body)}}}
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{
		home:    filepath.Join(root, "home"),
		project: filepath.Join(root, "proj"),
		rel:     github.Release{Tag: "v2.0.0"},
		skills:  map[string]skills.Skill{"alpha": sk("alpha", "a2"), "beta": sk("beta", "b2")},
	}
	os.MkdirAll(e.home, 0o755)
	os.MkdirAll(e.project, 0o755)
	return e
}

func (e *env) services() Services {
	return Services{
		Targets: install.Targets(e.home, e.project),
		Home:    e.home,
		Version: "v1.0.0",
		Latest:  func(context.Context) (github.Release, error) { return e.rel, e.latestErr },
		Load: func(_ context.Context, rel github.Release) (update.Bundle, error) {
			return update.Bundle{Release: rel, Skills: e.skills}, nil
		},
	}
}

func (e *env) globalClaude() string { return filepath.Join(e.home, ".claude/skills") }

func (e *env) seed(t *testing.T, dir, version string, avail ...skills.Skill) {
	t.Helper()
	m := map[string]skills.Skill{}
	var names []string
	for _, s := range avail {
		m[s.Name] = s
		names = append(names, s.Name)
	}
	if _, err := install.Execute(dir, install.Request{Mode: install.ModeInstall, Names: names, Version: version, Available: m}); err != nil {
		t.Fatal(err)
	}
}

func settle(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		switch x := msg.(type) {
		case spinner.TickMsg, nil:
			continue
		case tea.BatchMsg:
			queue = append(queue, x...)
			continue
		}
		var next tea.Model
		next, c = m.Update(msg)
		m = next.(Model)
		queue = append(queue, c)
	}
	return m
}

func send(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	return settle(t, next.(Model), cmd)
}

func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		m = send(t, m, keyMsg(k))
	}
	return m
}

func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func start(t *testing.T, e *env) Model {
	t.Helper()
	m := New(e.services())
	m = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	return settle(t, m, m.Init())
}

func assertView(t *testing.T, m Model, wants ...string) {
	t.Helper()
	v := m.View()
	for _, w := range wants {
		if !strings.Contains(v, w) {
			t.Fatalf("view missing %q:\n%s", w, v)
		}
	}
}

func assertNoView(t *testing.T, m Model, nots ...string) {
	t.Helper()
	v := m.View()
	for _, w := range nots {
		if strings.Contains(v, w) {
			t.Fatalf("view should not contain %q:\n%s", w, v)
		}
	}
}

func assertFits(t *testing.T, m Model, width int) {
	t.Helper()
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("line wider than %d (%d): %q", width, w, line)
		}
	}
}

func TestBannerShowsNewBinaryAndOutdatedInstalls(t *testing.T) {
	e := newEnv(t)
	e.seed(t, e.globalClaude(), "v1.0.0", sk("alpha", "a1"))
	m := start(t, e)
	assertView(t, m, "saxkills v2.0.0 is available", "self-update", "1 install outdated")
}

func TestBannerHiddenWhenCurrent(t *testing.T) {
	e := newEnv(t)
	e.rel.Tag = "v1.0.0"
	m := start(t, e)
	assertNoView(t, m, "is available", "outdated")
}

func TestOfflineShowsNoticeAndBlocksInstall(t *testing.T) {
	e := newEnv(t)
	e.latestErr = errors.New("dial tcp: no route")
	m := start(t, e)
	assertView(t, m, "offline")
	m = press(t, m, "enter")
	if m.screen != screenHome {
		t.Fatalf("screen = %v, install must be blocked offline", m.screen)
	}
	assertView(t, m, "cannot install new skills while offline")
}

func TestHomeNavigationAndQuit(t *testing.T) {
	e := newEnv(t)
	m := start(t, e)
	m = press(t, m, "down", "down", "up")
	if m.cursor != 1 {
		t.Fatalf("cursor = %d", m.cursor)
	}
	_, cmd := m.Update(keyMsg("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should quit")
	}
	_, cmd = m.Update(keyMsg("ctrl+c"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c should quit")
	}
}

func TestInstallFlowEndToEnd(t *testing.T) {
	e := newEnv(t)
	m := start(t, e)
	m = press(t, m, "enter")
	if m.screen != screenTargets {
		t.Fatalf("screen = %v", m.screen)
	}
	assertView(t, m, "global", "claude", ".claude/skills", "project", ".agents/skills")

	m = press(t, m, "enter")
	assertView(t, m, "select at least one")

	m = press(t, m, "space", "enter")
	if m.screen != screenSkills {
		t.Fatalf("screen = %v", m.screen)
	}
	assertView(t, m, "alpha", "desc of alpha", "not installed", "beta")

	m = press(t, m, "a", "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v", m.screen)
	}
	assertView(t, m, "alpha", "beta", "~/.claude/skills")

	m = press(t, m, "enter")
	if m.screen != screenResult {
		t.Fatalf("screen = %v", m.screen)
	}
	assertView(t, m, "installed")
	for _, n := range []string{"alpha", "beta"} {
		if _, err := os.Stat(filepath.Join(e.globalClaude(), n, "SKILL.md")); err != nil {
			t.Fatalf("%s not installed: %v", n, err)
		}
	}

	m = press(t, m, "enter")
	if m.screen != screenHome {
		t.Fatalf("screen = %v", m.screen)
	}
}

func TestEscBackChain(t *testing.T) {
	e := newEnv(t)
	m := start(t, e)
	m = press(t, m, "enter", "space", "enter", "a", "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v", m.screen)
	}
	m = press(t, m, "esc")
	if m.screen != screenSkills {
		t.Fatalf("after esc from confirm: %v", m.screen)
	}
	m = press(t, m, "esc")
	if m.screen != screenTargets {
		t.Fatalf("after esc from skills: %v", m.screen)
	}
	m = press(t, m, "esc")
	if m.screen != screenHome {
		t.Fatalf("after esc from targets: %v", m.screen)
	}
}

func TestUnavailableProjectTargetCannotBeSelected(t *testing.T) {
	e := newEnv(t)
	svc := e.services()
	svc.Targets = install.Targets(e.home, "")
	m := New(svc)
	m = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m = settle(t, m, m.Init())
	m = press(t, m, "enter", "down", "down", "space")
	assertView(t, m, "no project detected")
	m = press(t, m, "enter")
	assertView(t, m, "select at least one")
}

func TestConflictRowsAreNotSelectable(t *testing.T) {
	e := newEnv(t)
	os.MkdirAll(filepath.Join(e.globalClaude(), "alpha"), 0o755)
	os.WriteFile(filepath.Join(e.globalClaude(), "alpha/SKILL.md"), []byte("mine"), 0o644)
	m := start(t, e)
	m = press(t, m, "enter", "space", "enter", "a")
	assertView(t, m, "conflict")
	m = press(t, m, "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v", m.screen)
	}
	assertNoView(t, m, "alpha")
	m = press(t, m, "enter")
	if string(mustRead(t, filepath.Join(e.globalClaude(), "alpha/SKILL.md"))) != "mine" {
		t.Fatal("unmanaged skill overwritten")
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUpdateFlowPreselectsAndForceConfirmation(t *testing.T) {
	setup := func() (*env, Model) {
		e := newEnv(t)
		e.seed(t, e.globalClaude(), "v1.0.0", sk("alpha", "a1"), sk("beta", "b2"))
		os.WriteFile(filepath.Join(e.globalClaude(), "alpha/SKILL.md"), []byte("edited"), 0o644)
		m := start(t, e)
		m = press(t, m, "down", "enter", "space", "enter")
		return e, m
	}

	e, m := setup()
	if m.screen != screenSkills {
		t.Fatalf("screen = %v", m.screen)
	}
	assertView(t, m, "modified locally", "installed v1.0.0")
	m = press(t, m, "a", "enter")
	assertView(t, m, "locally modified", "alpha", "irreversible", "enter skips")
	m = press(t, m, "enter")
	if string(mustRead(t, filepath.Join(e.globalClaude(), "alpha/SKILL.md"))) != "edited" {
		t.Fatal("enter must keep local edits")
	}
	assertView(t, m, "skipped")

	e, m = setup()
	m = press(t, m, "a", "enter", "y")
	if string(mustRead(t, filepath.Join(e.globalClaude(), "alpha/SKILL.md"))) != "a2" {
		t.Fatal("explicit y should overwrite")
	}
}

func TestUpdateOffersRemovalOfSkillsRemovedUpstream(t *testing.T) {
	e := newEnv(t)
	e.seed(t, e.globalClaude(), "v1.0.0", sk("alpha", "a1"), sk("ghost", "g"))
	m := start(t, e)
	m = press(t, m, "down", "enter", "space", "enter")
	assertView(t, m, "removed upstream")
	m = press(t, m, "a", "enter")
	assertView(t, m, "remove")
	m = press(t, m, "enter")
	if _, err := os.Stat(filepath.Join(e.globalClaude(), "ghost")); !os.IsNotExist(err) {
		t.Fatal("ghost should be removed")
	}
	if string(mustRead(t, filepath.Join(e.globalClaude(), "alpha/SKILL.md"))) != "a2" {
		t.Fatal("alpha should be updated")
	}
}

func TestRemoveWorksOffline(t *testing.T) {
	e := newEnv(t)
	e.latestErr = errors.New("offline")
	e.seed(t, e.globalClaude(), "v1.0.0", sk("alpha", "a1"), sk("beta", "b1"))
	m := start(t, e)
	m = press(t, m, "down", "down", "enter", "space", "enter")
	if m.screen != screenSkills {
		t.Fatalf("screen = %v", m.screen)
	}
	assertView(t, m, "alpha", "beta")
	m = press(t, m, "space", "enter", "enter")
	if _, err := os.Stat(filepath.Join(e.globalClaude(), "alpha")); !os.IsNotExist(err) {
		t.Fatal("alpha should be removed")
	}
	if _, err := os.Stat(filepath.Join(e.globalClaude(), "beta")); err != nil {
		t.Fatal("beta should remain")
	}
}

func TestStatusScreenShowsManifestState(t *testing.T) {
	e := newEnv(t)
	e.latestErr = errors.New("offline")
	e.seed(t, e.globalClaude(), "v1.0.0", sk("alpha", "a1"))
	m := start(t, e)
	m = press(t, m, "down", "down", "down", "enter")
	if m.screen != screenStatus {
		t.Fatalf("screen = %v", m.screen)
	}
	assertView(t, m, "alpha", "v1.0.0", "~/.claude/skills")
	m = press(t, m, "esc")
	if m.screen != screenHome {
		t.Fatalf("screen = %v", m.screen)
	}
}

func TestLoadErrorIsShownAndEscRecovers(t *testing.T) {
	e := newEnv(t)
	svc := e.services()
	svc.Load = func(context.Context, github.Release) (update.Bundle, error) {
		return update.Bundle{}, errors.New("tarball failed")
	}
	m := New(svc)
	m = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m = settle(t, m, m.Init())
	m = press(t, m, "enter", "space", "enter")
	assertView(t, m, "tarball failed")
	m = press(t, m, "esc")
	if m.screen != screenTargets {
		t.Fatalf("screen = %v", m.screen)
	}
}

func TestViewsFitEightyColumns(t *testing.T) {
	e := newEnv(t)
	e.skills["alpha"] = skills.Skill{Name: "alpha", Description: strings.Repeat("very long description ", 12), Files: sk("alpha", "x").Files}
	e.seed(t, e.globalClaude(), "v1.0.0", sk("beta", "b1"))
	m := start(t, e)
	assertFits(t, m, 80)
	m = press(t, m, "enter")
	assertFits(t, m, 80)
	m = press(t, m, "space", "enter")
	assertFits(t, m, 80)
	m = press(t, m, "a", "enter")
	assertFits(t, m, 80)
	m = press(t, m, "enter")
	assertFits(t, m, 80)
}

func TestSkillListScrollsWithinHeight(t *testing.T) {
	e := newEnv(t)
	e.skills = map[string]skills.Skill{}
	for _, n := range []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9"} {
		e.skills[n] = sk(n, "x")
	}
	m := New(e.services())
	m = send(t, m, tea.WindowSizeMsg{Width: 80, Height: 14})
	m = settle(t, m, m.Init())
	m = press(t, m, "enter", "space", "enter")
	for i := 0; i < 8; i++ {
		m = press(t, m, "down")
	}
	assertView(t, m, "a9")
	if h := strings.Count(m.View(), "\n") + 1; h > 14 {
		t.Fatalf("view is %d lines, window is 14", h)
	}
}

func TestEnterEnterOnRemoveNeverDeletesModifiedSkills(t *testing.T) {
	e := newEnv(t)
	e.seed(t, e.globalClaude(), "v1.0.0", sk("alpha", "a1"))
	os.WriteFile(filepath.Join(e.globalClaude(), "alpha/SKILL.md"), []byte("edited"), 0o644)
	m := start(t, e)
	m = press(t, m, "down", "down", "enter", "space", "enter", "space", "enter", "enter")
	if m.screen != screenResult {
		t.Fatalf("screen = %v", m.screen)
	}
	if string(mustRead(t, filepath.Join(e.globalClaude(), "alpha/SKILL.md"))) != "edited" {
		t.Fatal("modified skill was deleted without an explicit y")
	}
	assertView(t, m, "skipped")
}

func TestBannerCountsOnlyActionableInstalls(t *testing.T) {
	e := newEnv(t)
	e.seed(t, e.globalClaude(), "v1.0.0", sk("alpha", "a1"), sk("beta", "b2"), sk("ghost", "g"))
	os.WriteFile(filepath.Join(e.globalClaude(), "alpha/SKILL.md"), []byte("edited"), 0o644)
	m := start(t, e)
	assertNoView(t, m, "outdated")
	m = press(t, m, "down", "enter", "space", "enter")
	for name, picked := range m.picked {
		if picked {
			t.Fatalf("%s preselected although nothing is actionable", name)
		}
	}
}

func TestBannerAndUpdateScreenAgree(t *testing.T) {
	e := newEnv(t)
	e.seed(t, e.globalClaude(), "v1.0.0", sk("alpha", "a1"), sk("beta", "b2"))
	m := start(t, e)
	assertView(t, m, "1 install outdated")
	m = press(t, m, "down", "enter", "space", "enter")
	if !m.picked["alpha"] || m.picked["beta"] {
		t.Fatalf("picked = %v", m.picked)
	}
}

func TestRetryKeyRefreshesOnlineState(t *testing.T) {
	e := newEnv(t)
	e.latestErr = errors.New("offline")
	m := start(t, e)
	assertView(t, m, "offline", "r retry")
	e.latestErr = nil
	m = press(t, m, "r")
	assertNoView(t, m, "offline")
	m = press(t, m, "enter")
	if m.screen != screenTargets {
		t.Fatalf("screen = %v after retry", m.screen)
	}
}

func TestStaleBundleIsIgnored(t *testing.T) {
	e := newEnv(t)
	m := start(t, e)
	next, _ := m.Update(bundleMsg{tag: "v0.0.1", bundle: update.Bundle{Release: github.Release{Tag: "v0.0.1"}, Skills: map[string]skills.Skill{"old": sk("old", "x")}}})
	m = next.(Model)
	if m.bundle == nil || m.bundle.Release.Tag != "v2.0.0" {
		t.Fatalf("bundle = %+v", m.bundle)
	}
	if _, ok := m.bundle.Skills["old"]; ok {
		t.Fatal("stale bundle replaced the current one")
	}
}

func TestApplyContinuesPastBrokenTarget(t *testing.T) {
	e := newEnv(t)
	os.MkdirAll(e.globalClaude(), 0o755)
	os.WriteFile(filepath.Join(e.globalClaude(), ".saxkills.json"), []byte("{nope"), 0o644)
	m := start(t, e)
	m = press(t, m, "enter", "space", "down", "space", "enter")
	m = press(t, m, "a", "enter", "enter")
	if m.screen != screenResult {
		t.Fatalf("screen = %v", m.screen)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".agents/skills/alpha/SKILL.md")); err != nil {
		t.Fatalf("healthy target not applied: %v", err)
	}
	assertView(t, m, "error")
}
