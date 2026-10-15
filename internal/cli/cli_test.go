package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FabiSax12/saxkills/internal/github"
	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/skills"
)

type fakeClient struct {
	rel      github.Release
	err      error
	skills   map[string]string
	assets   map[string][]byte
	tarballs int
}

func (f *fakeClient) LatestRelease(context.Context) (github.Release, error) { return f.rel, f.err }

func (f *fakeClient) DownloadTarball(context.Context, github.Release) (io.ReadCloser, error) {
	f.tarballs++
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range f.skills {
		tw.WriteHeader(&tar.Header{Name: "root/skills/" + name + "/SKILL.md", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return io.NopCloser(&buf), nil
}

func (f *fakeClient) DownloadAsset(_ context.Context, a github.Asset) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.assets[a.Name])), nil
}

type harness struct {
	home, project, exe string
	out, errOut        *bytes.Buffer
	client             *fakeClient
	tuiCalled          bool
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root := t.TempDir()
	h := &harness{
		home:    filepath.Join(root, "home"),
		project: filepath.Join(root, "proj"),
		exe:     filepath.Join(root, "saxkills"),
		out:     &bytes.Buffer{},
		errOut:  &bytes.Buffer{},
		client:  &fakeClient{rel: github.Release{Tag: "v2.0.0"}, skills: map[string]string{"solid": "new"}},
	}
	os.MkdirAll(h.home, 0o755)
	os.MkdirAll(h.project, 0o755)
	os.WriteFile(h.exe, []byte("OLD"), 0o755)
	return h
}

func (h *harness) env(cwd string) Env {
	return Env{
		Stdout: h.out, Stderr: h.errOut, Client: h.client,
		Home: h.home, Cwd: cwd,
		GitRoot: func(string) (string, error) { return "", errors.New("no git") },
		Version: "v1.0.0", GOOS: "linux", GOARCH: "amd64",
		ExePath: func() (string, error) { return h.exe, nil },
		RunTUI:  func() error { h.tuiCalled = true; return nil },
	}
}

func (h *harness) seed(t *testing.T, dir, version string) {
	t.Helper()
	_, err := install.Execute(dir, install.Request{Mode: install.ModeInstall, Names: []string{"solid"}, Version: version,
		Available: map[string]skills.Skill{"solid": {Name: "solid", Files: map[string]skills.File{"SKILL.md": {Data: []byte("old")}}}}})
	if err != nil {
		t.Fatal(err)
	}
}

func (h *harness) globalClaude() string  { return filepath.Join(h.home, ".claude/skills") }
func (h *harness) projectAgents() string { return filepath.Join(h.project, ".agents/skills") }

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNoArgsRunsTUI(t *testing.T) {
	h := newHarness(t)
	if code := Run(context.Background(), nil, h.env(h.project)); code != 0 || !h.tuiCalled {
		t.Fatalf("code=%d tui=%v", code, h.tuiCalled)
	}
}

func TestVersionAndUnknown(t *testing.T) {
	h := newHarness(t)
	if code := Run(context.Background(), []string{"version"}, h.env(h.project)); code != 0 || strings.TrimSpace(h.out.String()) != "saxkills v1.0.0" {
		t.Fatalf("code=%d out=%q", code, h.out.String())
	}
	if code := Run(context.Background(), []string{"bogus"}, h.env(h.project)); code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func TestUpdateFlagValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no selector", []string{"update"}},
		{"bad scope", []string{"update", "--scope", "galaxy"}},
		{"bad target", []string{"update", "--target", "cursor"}},
		{"unknown flag", []string{"update", "--nope"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if code := Run(context.Background(), tt.args, h.env(h.project)); code != 2 {
				t.Fatalf("code=%d stderr=%q", code, h.errOut.String())
			}
		})
	}
}

func TestCheckExitCodes(t *testing.T) {
	h := newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	code := Run(context.Background(), []string{"check"}, h.env(h.project))
	if code != 10 || !strings.Contains(h.out.String(), "solid") || !strings.Contains(h.out.String(), "v2.0.0") {
		t.Fatalf("code=%d out=%q", code, h.out.String())
	}

	h = newHarness(t)
	h.seed(t, h.globalClaude(), "v2.0.0")
	if code := Run(context.Background(), []string{"check"}, h.env(h.project)); code != 0 {
		t.Fatalf("up to date: code=%d", code)
	}

	h = newHarness(t)
	if code := Run(context.Background(), []string{"check"}, h.env(h.project)); code != 0 {
		t.Fatalf("no installs: code=%d", code)
	}

	h = newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	h.client.err = errors.New("offline")
	if code := Run(context.Background(), []string{"check"}, h.env(h.project)); code != 1 || !strings.Contains(h.errOut.String(), "offline") {
		t.Fatalf("offline: code=%d stderr=%q", code, h.errOut.String())
	}
}

func TestUpdateAllQuiet(t *testing.T) {
	h := newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	h.seed(t, h.projectAgents(), "v1.0.0")
	code := Run(context.Background(), []string{"update", "--all", "--quiet"}, h.env(h.project))
	if code != 0 || h.out.Len() != 0 || h.errOut.Len() != 0 {
		t.Fatalf("code=%d out=%q err=%q", code, h.out.String(), h.errOut.String())
	}
	for _, d := range []string{h.globalClaude(), h.projectAgents()} {
		if readFile(t, filepath.Join(d, "solid/SKILL.md")) != "new" {
			t.Fatalf("%s not updated", d)
		}
	}
}

func TestUpdateReportsWhenNotQuiet(t *testing.T) {
	h := newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	if code := Run(context.Background(), []string{"update", "--all"}, h.env(h.project)); code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(h.out.String(), "updated") || !strings.Contains(h.out.String(), "solid") {
		t.Fatalf("out=%q", h.out.String())
	}
}

func TestUpdateFiltersByScopeAndTarget(t *testing.T) {
	h := newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	h.seed(t, h.projectAgents(), "v1.0.0")
	args := []string{"update", "--scope", "project", "--target", "agents", "--quiet"}
	if code := Run(context.Background(), args, h.env(h.project)); code != 0 {
		t.Fatalf("code=%d", code)
	}
	if readFile(t, filepath.Join(h.projectAgents(), "solid/SKILL.md")) != "new" {
		t.Fatal("project/agents not updated")
	}
	if readFile(t, filepath.Join(h.globalClaude(), "solid/SKILL.md")) != "old" {
		t.Fatal("global/claude should be untouched")
	}
}

func TestUpdateFromHomeNeverTreatsHomeAsProject(t *testing.T) {
	h := newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	if code := Run(context.Background(), []string{"update", "--all", "--quiet"}, h.env(h.home)); code != 0 {
		t.Fatalf("code=%d err=%q", code, h.errOut.String())
	}
	if _, err := os.Stat(filepath.Join(h.home, ".claude/skills/solid")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateNetworkFailureIsNonZero(t *testing.T) {
	h := newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	h.client.err = errors.New("boom")
	if code := Run(context.Background(), []string{"update", "--all", "--quiet"}, h.env(h.project)); code != 1 {
		t.Fatalf("code=%d", code)
	}
}

func TestSelfUpdate(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "saxkills", Typeflag: tar.TypeReg, Mode: 0o755, Size: 3})
	tw.Write([]byte("NEW"))
	tw.Close()
	gz.Close()
	sum := sha256.Sum256(buf.Bytes())

	h := newHarness(t)
	h.client.rel = github.Release{Tag: "v2.0.0", Assets: []github.Asset{{Name: "saxkills_linux_amd64.tar.gz"}, {Name: "checksums.txt"}}}
	h.client.assets = map[string][]byte{
		"saxkills_linux_amd64.tar.gz": buf.Bytes(),
		"checksums.txt":               []byte(fmt.Sprintf("%s  saxkills_linux_amd64.tar.gz\n", hex.EncodeToString(sum[:]))),
	}
	if code := Run(context.Background(), []string{"self-update"}, h.env(h.project)); code != 0 {
		t.Fatalf("code=%d err=%q", code, h.errOut.String())
	}
	if readFile(t, h.exe) != "NEW" {
		t.Fatal("binary not replaced")
	}

	h.out.Reset()
	env := h.env(h.project)
	env.Version = "v2.0.0"
	os.WriteFile(h.exe, []byte("OLD"), 0o755)
	if code := Run(context.Background(), []string{"self-update"}, env); code != 0 || !strings.Contains(h.out.String(), "up to date") {
		t.Fatalf("code=%d out=%q", code, h.out.String())
	}
	if readFile(t, h.exe) != "OLD" {
		t.Fatal("binary replaced although up to date")
	}
}

func TestUpdateWithSkippedSkillsExits3AndWarnsEvenWhenQuiet(t *testing.T) {
	h := newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	os.WriteFile(filepath.Join(h.globalClaude(), "solid/SKILL.md"), []byte("mine"), 0o644)
	code := Run(context.Background(), []string{"update", "--all", "--quiet"}, h.env(h.project))
	if code != ExitSkipped || !strings.Contains(h.errOut.String(), "solid") || !strings.Contains(h.errOut.String(), "modified locally") {
		t.Fatalf("code=%d stderr=%q", code, h.errOut.String())
	}
	if h.out.Len() != 0 {
		t.Fatalf("quiet stdout = %q", h.out.String())
	}
	if readFile(t, filepath.Join(h.globalClaude(), "solid/SKILL.md")) != "mine" {
		t.Fatal("local edit lost")
	}
}

func TestCheckSeparatesNeedsAttentionFromOutdated(t *testing.T) {
	h := newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	os.WriteFile(filepath.Join(h.globalClaude(), "solid/SKILL.md"), []byte("mine"), 0o644)
	code := Run(context.Background(), []string{"check"}, h.env(h.project))
	if code != 0 || !strings.Contains(h.out.String(), "needs attention") || !strings.Contains(h.out.String(), "modified locally") {
		t.Fatalf("code=%d out=%q", code, h.out.String())
	}
}

func TestCheckQuietAndNoInstalls(t *testing.T) {
	h := newHarness(t)
	if code := Run(context.Background(), []string{"check"}, h.env(h.project)); code != 0 || !strings.Contains(h.out.String(), "no managed installs found") {
		t.Fatalf("code=%d out=%q", code, h.out.String())
	}
	h = newHarness(t)
	h.seed(t, h.globalClaude(), "v1.0.0")
	if code := Run(context.Background(), []string{"check", "--quiet"}, h.env(h.project)); code != ExitOutdated || h.out.Len() != 0 {
		t.Fatalf("code=%d out=%q", code, h.out.String())
	}
}

func TestPositionalArgsRejected(t *testing.T) {
	for _, args := range [][]string{{"check", "extra"}, {"update", "--all", "extra"}, {"self-update", "now"}, {"version", "x"}} {
		h := newHarness(t)
		if code := Run(context.Background(), args, h.env(h.project)); code != ExitUsage {
			t.Fatalf("%v: code=%d", args, code)
		}
	}
}

func TestUpdateRefusesProjectDirResolvingOutside(t *testing.T) {
	h := newHarness(t)
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(h.project, ".claude"))
	h.seed(t, h.globalClaude(), "v1.0.0")
	code := Run(context.Background(), []string{"update", "--all", "--quiet"}, h.env(h.project))
	if code != ExitFailure || !strings.Contains(h.errOut.String(), "outside the project") {
		t.Fatalf("code=%d stderr=%q", code, h.errOut.String())
	}
	if readFile(t, filepath.Join(h.globalClaude(), "solid/SKILL.md")) != "new" {
		t.Fatal("safe targets should still be updated")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("wrote outside the project")
	}
}

func TestUpdateContinuesPastBrokenInstall(t *testing.T) {
	h := newHarness(t)
	os.MkdirAll(h.globalClaude(), 0o755)
	os.WriteFile(filepath.Join(h.globalClaude(), ".saxkills.json"), []byte("{nope"), 0o644)
	h.seed(t, h.projectAgents(), "v1.0.0")
	code := Run(context.Background(), []string{"update", "--all", "--quiet"}, h.env(h.project))
	if code != ExitFailure {
		t.Fatalf("code=%d", code)
	}
	if readFile(t, filepath.Join(h.projectAgents(), "solid/SKILL.md")) != "new" {
		t.Fatal("healthy install not updated")
	}
}
