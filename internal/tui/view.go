package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/manifest"
	"github.com/FabiSax12/saxkills/internal/version"
)

func (m Model) contentWidth() int {
	w := m.width
	if w == 0 {
		w = 80
	}
	if w < 40 {
		w = 40
	}
	if w > 100 {
		w = 100
	}
	return w
}

func (m Model) View() string {
	w := m.contentWidth()
	var lines []string
	switch m.screen {
	case screenHome:
		lines = m.viewHome(w)
	case screenTargets:
		lines = m.viewTargets(w)
	case screenSkills:
		lines = m.viewSkills(w)
	case screenConfirm:
		lines = m.viewConfirm(w)
	case screenProgress:
		lines = []string{m.title(), "", m.spin.View() + " Applying changes..."}
	case screenResult:
		lines = m.viewResult(w)
	case screenStatus:
		lines = m.viewStatus(w)
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, w, "…")
	}
	if m.height > 0 && len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) title() string {
	v := m.svc.Version
	if v == "" {
		v = "dev"
	}
	return styleTitle.Render("saxkills") + " " + styleMuted.Render(v)
}

func (m Model) noticeLine() string {
	if m.notice == "" {
		return ""
	}
	return styleWarn.Render(m.notice)
}

func help(parts ...string) string {
	return styleMuted.Render(strings.Join(parts, "  ·  "))
}

func (m Model) viewHome(w int) []string {
	lines := []string{m.title(), ""}
	switch {
	case !m.checked:
		lines = append(lines, styleMuted.Render("checking for updates..."))
	case !m.online:
		lines = append(lines, styleWarn.Render("offline: showing local state, installing new skills is disabled (r retry)"))
	default:
		if m.newerBinary() {
			lines = append(lines, styleWarn.Render(fmt.Sprintf("saxkills %s is available, run `saxkills self-update`", m.latest.Tag)))
		}
		if n := len(m.outdated); n > 0 {
			lines = append(lines, styleWarn.Render(plural(n, "install", "installs")+" outdated, choose Update"))
		}
	}
	lines = append(lines, "")
	for i, item := range homeItems {
		if i == m.cursor {
			lines = append(lines, styleCursor.Render("> "+item))
		} else {
			lines = append(lines, "  "+item)
		}
	}
	lines = append(lines, "", m.noticeLine(), help("↑/↓ move", "enter select", "q quit"))
	return lines
}

func (m Model) modeName() string {
	switch m.mode {
	case install.ModeUpdate:
		return "Update"
	case install.ModeRemove:
		return "Remove"
	}
	return "Install"
}

func (m Model) viewTargets(w int) []string {
	lines := []string{m.title(), "", styleBold.Render(m.modeName() + ": choose locations"), ""}
	for i, t := range m.svc.Targets {
		cursor := "  "
		if i == m.targetCursor {
			cursor = styleCursor.Render("> ")
		}
		if t.Unavailable != "" {
			lines = append(lines, cursor+styleMuted.Render(fmt.Sprintf("[-] %-8s %-7s %s", t.Scope, t.Agent, t.Unavailable)))
			continue
		}
		box := "[ ]"
		if m.targetSel[i] {
			box = styleOK.Render("[x]")
		}
		lines = append(lines, fmt.Sprintf("%s%s %-8s %-7s %s", cursor, box, t.Scope, t.Agent, styleMuted.Render(m.display(t.Dir))))
	}
	lines = append(lines, "", m.noticeLine(), help("space toggle", "enter continue", "esc back", "q quit"))
	return lines
}

func (m Model) badgeStyle(s install.State) func(...string) string {
	switch s {
	case install.UpToDate:
		return styleOK.Render
	case install.NotInstalled:
		return styleMuted.Render
	case install.Conflict:
		return styleErr.Render
	default:
		return styleWarn.Render
	}
}

func (m Model) viewSkills(w int) []string {
	head := []string{m.title(), "", styleBold.Render(m.modeName()+": choose skills") + " " + styleMuted.Render(m.targetSummary()), ""}
	switch {
	case m.loadErr != nil:
		return append(head, styleErr.Render("error: "+m.loadErr.Error()), "", help("esc back", "q quit"))
	case m.loading && m.mode != install.ModeRemove:
		return append(head, m.spin.View()+" Downloading skills from "+m.latest.Tag+"...")
	case len(m.rows) == 0:
		return append(head, styleMuted.Render("nothing to show here"), "", help("esc back", "q quit"))
	}

	nameW, badgeW := 0, 0
	for _, r := range m.rows {
		nameW = max(nameW, lipgloss.Width(r.name))
		badgeW = max(badgeW, lipgloss.Width(badge(r, m.latest.Tag)))
	}
	nameW = min(nameW, 24)
	descW := w - 6 - nameW - 2 - badgeW - 2
	end := min(len(m.rows), m.rowOffset+m.visibleRows())
	lines := head
	for i := m.rowOffset; i < end; i++ {
		r := m.rows[i]
		cursor := "  "
		if i == m.rowCursor {
			cursor = styleCursor.Render("> ")
		}
		box := "[ ]"
		switch {
		case !r.selectable():
			box = styleMuted.Render("[-]")
		case m.picked[r.name]:
			box = styleOK.Render("[x]")
		}
		name := ansi.Truncate(r.name, nameW, "…")
		line := cursor + box + " " + name + strings.Repeat(" ", nameW-lipgloss.Width(name)) + "  "
		b := badge(r, m.latest.Tag)
		bpad := strings.Repeat(" ", badgeW-lipgloss.Width(b))
		if descW >= 12 {
			line += styleMuted.Render(padTo(ansi.Truncate(r.description, descW, "…"), descW)) + "  "
		}
		line += m.badgeStyle(r.state)(b) + bpad
		lines = append(lines, line)
	}
	lines = append(lines, "", m.noticeLine(),
		help("space toggle", "a all", "n none", "enter confirm", "esc back", fmt.Sprintf("%d/%d", m.rowCursor+1, len(m.rows))))
	return lines
}

func padTo(s string, n int) string {
	if d := n - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func (m Model) targetSummary() string {
	var labels []string
	for _, t := range m.selectedTargets() {
		labels = append(labels, t.Label())
	}
	return strings.Join(labels, ", ")
}

func (m Model) viewConfirm(w int) []string {
	lines := []string{m.title(), "", styleBold.Render(m.modeName() + ": confirm"), ""}
	for _, t := range m.selectedTargets() {
		lines = append(lines, "  "+t.Label()+"  "+styleMuted.Render(m.display(t.Dir)))
	}
	lines = append(lines, "")
	for _, r := range m.pickedRows() {
		lines = append(lines, fmt.Sprintf("  %-24s %s", ansi.Truncate(r.name, 24, "…"), styleMuted.Render(action(m.mode, r))))
	}
	lines = append(lines, "")
	if n := m.modifiedCount(); n > 0 {
		var names []string
		for _, r := range m.pickedRows() {
			if r.state == install.Modified {
				names = append(names, r.name)
			}
		}
		lines = append(lines, styleWarn.Render("locally modified: "+strings.Join(names, ", ")),
			styleWarn.Render("enter skips them. y overwrites or deletes them: irreversible."), "")
		lines = append(lines, help("enter apply, enter skips modified", "y include modified", "esc back"))
		return lines
	}
	return append(lines, help("enter apply", "esc back"))
}

func (m Model) viewResult(w int) []string {
	lines := []string{m.title(), "", styleBold.Render(m.modeName() + ": result"), ""}
	for _, rep := range m.reports {
		lines = append(lines, rep.Target.Label()+"  "+styleMuted.Render(m.display(rep.Target.Dir)))
		if rep.Err != nil {
			lines = append(lines, "  "+styleErr.Render("error: "+rep.Err.Error()))
		}
		for _, r := range rep.Results {
			status := string(r.Outcome)
			style := styleOK.Render
			if r.Outcome == install.Skipped {
				style = styleWarn.Render
			}
			if r.Outcome == install.Failed {
				style = styleErr.Render
			}
			line := fmt.Sprintf("  %-24s %s", ansi.Truncate(r.Name, 24, "…"), style(status))
			if r.Reason != "" {
				line += styleMuted.Render(": " + r.Reason)
			}
			lines = append(lines, line)
		}
	}
	if m.applyErr != nil {
		lines = append(lines, "", styleErr.Render("error: "+m.applyErr.Error()))
	}
	return append(lines, "", help("enter home", "q quit"))
}

func (m Model) viewStatus(w int) []string {
	lines := []string{m.title(), "", styleBold.Render("Status"), ""}
	for _, t := range m.svc.Targets {
		if t.Dir == "" {
			lines = append(lines, styleMuted.Render(t.Label()+"  "+t.Unavailable))
			continue
		}
		lines = append(lines, t.Label()+"  "+styleMuted.Render(m.display(t.Dir)))
		mf, err := manifest.Read(t.Dir)
		switch {
		case err != nil:
			lines = append(lines, "  "+styleErr.Render(err.Error()))
		case len(mf.Skills) == 0:
			lines = append(lines, "  "+styleMuted.Render("nothing installed by saxkills"))
		default:
			for _, name := range sortedKeys(mf.Skills) {
				e := mf.Skills[name]
				line := fmt.Sprintf("  %-24s %s", ansi.Truncate(name, 24, "…"), e.Version)
				if m.online && version.Outdated(e.Version, m.latest.Tag) {
					line += "  " + styleWarn.Render("update available "+m.latest.Tag)
				}
				lines = append(lines, line)
			}
		}
	}
	return append(lines, "", help("esc back", "q quit"))
}
