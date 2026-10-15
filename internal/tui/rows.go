package tui

import (
	"errors"
	"sort"

	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/skills"
)

type row struct {
	name        string
	description string
	state       install.State
	version     string
}

func (r row) selectable() bool {
	return r.state != install.Conflict
}

func rank(s install.State) int {
	switch s {
	case install.Modified:
		return 7
	case install.Conflict:
		return 6
	case install.RemovedUpstream:
		return 5
	case install.UpdateAvailable:
		return 4
	case install.Legacy:
		return 3
	case install.NotInstalled:
		return 2
	default:
		return 1
	}
}

func relevant(mode install.Mode, s install.State) bool {
	switch mode {
	case install.ModeInstall:
		return s != install.RemovedUpstream
	default:
		switch s {
		case install.UpToDate, install.UpdateAvailable, install.Modified, install.RemovedUpstream:
			return true
		}
		return false
	}
}

func buildRows(mode install.Mode, dirs []string, avail map[string]skills.Skill) ([]row, error) {
	agg := map[string]row{}
	var errs []error
	for _, dir := range dirs {
		statuses, err := install.Inspect(dir, avail)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, st := range statuses {
			if !relevant(mode, st.State) {
				continue
			}
			cur, ok := agg[st.Name]
			if ok && rank(cur.state) >= rank(st.State) {
				continue
			}
			agg[st.Name] = row{name: st.Name, description: st.Description, state: st.State, version: st.Version}
		}
	}
	rows := make([]row, 0, len(agg))
	for _, r := range agg {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	return rows, errors.Join(errs...)
}

func badge(r row, latest string) string {
	switch r.state {
	case install.NotInstalled:
		return "not installed"
	case install.UpToDate:
		return "installed " + r.version
	case install.UpdateAvailable:
		if latest == "" {
			return "update available"
		}
		return "update available " + latest
	case install.Modified:
		return "modified locally"
	case install.Conflict:
		return "conflict"
	case install.Legacy:
		return "linked (legacy)"
	case install.RemovedUpstream:
		return "removed upstream"
	}
	return string(r.state)
}

func action(mode install.Mode, r row) string {
	switch {
	case mode == install.ModeRemove:
		return "remove"
	case mode == install.ModeUpdate && r.state == install.RemovedUpstream:
		return "remove (removed upstream)"
	}
	switch r.state {
	case install.NotInstalled:
		return "install"
	case install.Legacy:
		return "convert symlink to managed copy"
	case install.UpdateAvailable:
		return "update"
	case install.Modified:
		return "overwrite local edits"
	}
	return "keep (already up to date)"
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
