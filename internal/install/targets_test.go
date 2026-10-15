package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargets(t *testing.T) {
	home := "/home/u"
	got := Targets(home, "/work/proj")
	want := map[string]string{
		"global/claude":  "/home/u/.claude/skills",
		"global/agents":  "/home/u/.agents/skills",
		"project/claude": "/work/proj/.claude/skills",
		"project/agents": "/work/proj/.agents/skills",
	}
	if len(got) != 4 {
		t.Fatalf("got %d targets", len(got))
	}
	for _, tg := range got {
		key := string(tg.Scope) + "/" + string(tg.Agent)
		if tg.Dir != filepath.FromSlash(want[key]) || tg.Unavailable != "" {
			t.Errorf("%s: %+v", key, tg)
		}
	}
}

func TestTargetsWithoutProject(t *testing.T) {
	for _, tg := range Targets("/home/u", "") {
		if tg.Scope == ScopeProject && (tg.Unavailable == "" || tg.Dir != "") {
			t.Errorf("project target should be unavailable: %+v", tg)
		}
		if tg.Scope == ScopeGlobal && tg.Unavailable != "" {
			t.Errorf("global target should be available: %+v", tg)
		}
	}
}

func TestProjectRoot(t *testing.T) {
	gitOK := func(string) (string, error) { return "/work/proj", nil }
	gitHome := func(string) (string, error) { return "/home/u", nil }
	gitFail := func(string) (string, error) { return "", errors.New("not a repo") }
	tests := []struct {
		name string
		cwd  string
		git  func(string) (string, error)
		want string
	}{
		{"git root", "/work/proj/sub", gitOK, "/work/proj"},
		{"fallback to cwd", "/work/other", gitFail, "/work/other"},
		{"git root is home", "/home/u/sub", gitHome, ""},
		{"cwd is home without git", "/home/u", gitFail, ""},
		{"cwd is home with trailing slash", "/home/u/", gitFail, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProjectRoot(tt.cwd, "/home/u", tt.git); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestProjectTargetsRefuseSymlinksOutsideRoot(t *testing.T) {
	outside := t.TempDir()
	inside := t.TempDir()
	root := t.TempDir()
	os.Symlink(outside, filepath.Join(root, ".claude"))
	os.MkdirAll(filepath.Join(root, ".agents"), 0o755)
	os.MkdirAll(filepath.Join(inside, "real"), 0o755)
	os.Symlink(outside, filepath.Join(root, ".agents", "skills"))
	for _, tg := range Targets(t.TempDir(), root) {
		if tg.Scope != ScopeProject {
			if tg.Unavailable != "" {
				t.Errorf("global target flagged: %+v", tg)
			}
			continue
		}
		if tg.Unavailable == "" || !strings.Contains(tg.Unavailable, "outside the project") || tg.Dir == "" {
			t.Errorf("expected refusal for %+v", tg)
		}
	}
}

func TestProjectTargetsAllowSymlinksInsideRoot(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "shared"), 0o755)
	os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	os.Symlink(filepath.Join(root, "shared"), filepath.Join(root, ".claude", "skills"))
	for _, tg := range Targets(t.TempDir(), root) {
		if tg.Unavailable != "" {
			t.Errorf("unexpected refusal: %+v", tg)
		}
	}
}
