package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Scope string

type Agent string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
	AgentClaude  Agent = "claude"
	AgentAgents  Agent = "agents"
)

type Target struct {
	Scope       Scope
	Agent       Agent
	Dir         string
	Unavailable string
}

func (t Target) Label() string {
	return string(t.Scope) + "/" + string(t.Agent)
}

func agentDir(a Agent) string {
	return "." + string(a) + "/skills"
}

func Targets(home, projectRoot string) []Target {
	var out []Target
	for _, scope := range []Scope{ScopeGlobal, ScopeProject} {
		for _, agent := range []Agent{AgentClaude, AgentAgents} {
			t := Target{Scope: scope, Agent: agent}
			base := home
			if scope == ScopeProject {
				base = projectRoot
			}
			if base == "" {
				t.Unavailable = "no project detected (not inside a git repository or project folder)"
			} else {
				t.Dir = filepath.Join(base, filepath.FromSlash(agentDir(agent)))
				if scope == ScopeProject {
					if err := checkInsideRoot(base, t.Dir); err != nil {
						t.Unavailable = err.Error()
					}
				}
			}
			out = append(out, t)
		}
	}
	return out
}

func ProjectRoot(cwd, home string, gitRoot func(dir string) (string, error)) string {
	home = filepath.Clean(home)
	root := filepath.Clean(cwd)
	if gitRoot != nil {
		if r, err := gitRoot(cwd); err == nil && strings.TrimSpace(r) != "" {
			root = filepath.Clean(strings.TrimSpace(r))
		}
	}
	if root == home {
		return ""
	}
	return root
}

func GitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func checkInsideRoot(root, dir string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil
	}
	for _, p := range []string{filepath.Dir(dir), dir} {
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(realRoot, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("refusing %s: it resolves outside the project (%s)", p, root)
		}
	}
	return nil
}
