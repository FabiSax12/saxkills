package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/FabiSax12/saxkills/internal/github"
	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/manifest"
	"github.com/FabiSax12/saxkills/internal/skills"
)

type fakeClient struct {
	rel       github.Release
	files     map[string]string
	err       error
	downloads int
}

func (f *fakeClient) LatestRelease(context.Context) (github.Release, error) {
	return f.rel, f.err
}

func (f *fakeClient) DownloadTarball(context.Context, github.Release) (io.ReadCloser, error) {
	f.downloads++
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range f.files {
		tw.WriteHeader(&tar.Header{Name: "root/" + name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return io.NopCloser(&buf), nil
}

func (f *fakeClient) DownloadAsset(context.Context, github.Asset) (io.ReadCloser, error) {
	return nil, errors.New("unused")
}

func target(dir string) install.Target {
	return install.Target{Scope: install.ScopeGlobal, Agent: install.AgentClaude, Dir: dir}
}

func seed(t *testing.T, dir, version, name, body string) {
	t.Helper()
	s := map[string]skills.File{"SKILL.md": {Data: []byte(body)}}
	_, err := install.Execute(dir, install.Request{Mode: install.ModeInstall, Names: []string{name}, Version: version,
		Available: map[string]skills.Skill{name: {Name: name, Files: s}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckReportsOnlyOutdatedManagedInstalls(t *testing.T) {
	old, current, empty := t.TempDir(), t.TempDir(), t.TempDir()
	seed(t, old, "v1.0.0", "solid", "a")
	seed(t, current, "v2.0.0", "solid", "a")
	c := &fakeClient{rel: github.Release{Tag: "v2.0.0"}, files: map[string]string{"skills/solid/SKILL.md": "new"}}

	a, err := Analyze(context.Background(), c, []install.Target{target(old), target(current), target(empty), {Unavailable: "x", Dir: "/nope"}})
	if err != nil {
		t.Fatal(err)
	}
	if a.Latest != "v2.0.0" || a.Managed != 2 || len(a.Outdated) != 1 || a.Outdated[0].Target.Dir != old || a.Outdated[0].Skills[0] != "solid" {
		t.Fatalf("got %+v", a)
	}
}

func TestAnalyzeSeparatesAttentionFromOutdated(t *testing.T) {
	edited, ghost, same := t.TempDir(), t.TempDir(), t.TempDir()
	seed(t, edited, "v1.0.0", "solid", "old")
	os.WriteFile(filepath.Join(edited, "solid/SKILL.md"), []byte("mine"), 0o644)
	seed(t, ghost, "v1.0.0", "ghost", "g")
	seed(t, same, "v1.0.0", "solid", "new")
	c := &fakeClient{rel: github.Release{Tag: "v2.0.0"}, files: map[string]string{"skills/solid/SKILL.md": "new"}}

	a, err := Analyze(context.Background(), c, []install.Target{target(edited), target(ghost), target(same)})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Outdated) != 0 {
		t.Fatalf("nothing here is actionable: %+v", a.Outdated)
	}
	states := map[string]install.State{}
	for _, at := range a.Attention {
		states[at.Skill] = at.State
	}
	if len(a.Attention) != 2 || states["solid"] != install.Modified || states["ghost"] != install.RemovedUpstream {
		t.Fatalf("attention = %+v", a.Attention)
	}
}

func TestCheckPropagatesNetworkError(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, "v1", "solid", "a")
	if _, err := Analyze(context.Background(), &fakeClient{err: errors.New("offline")}, []install.Target{target(dir)}); err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckWithNoManagedInstallsSkipsNetwork(t *testing.T) {
	c := &fakeClient{err: errors.New("must not be called")}
	a, err := Analyze(context.Background(), c, []install.Target{target(t.TempDir())})
	if err != nil || a.Managed != 0 || len(a.Outdated) != 0 {
		t.Fatalf("got %+v err %v", a, err)
	}
}

func TestRunUpdatesManagedInstalls(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	seed(t, a, "v1", "solid", "old")
	seed(t, b, "v1", "solid", "old")
	os.MkdirAll(filepath.Join(b, "mine"), 0o755)
	c := &fakeClient{rel: github.Release{Tag: "v2"}, files: map[string]string{"skills/solid/SKILL.md": "new"}}

	reports, err := Run(context.Background(), c, []install.Target{target(a), target(b)})
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || c.downloads != 1 {
		t.Fatalf("reports=%d downloads=%d", len(reports), c.downloads)
	}
	for _, dir := range []string{a, b} {
		data, _ := os.ReadFile(filepath.Join(dir, "solid/SKILL.md"))
		if string(data) != "new" {
			t.Fatalf("%s not updated", dir)
		}
	}
	if m, _ := manifest.Read(a); m.Version != "v2" {
		t.Fatalf("manifest %+v", m)
	}
}

func TestRunNothingToDoDoesNotDownload(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, "v2", "solid", "a")
	c := &fakeClient{rel: github.Release{Tag: "v2"}}
	reports, err := Run(context.Background(), c, []install.Target{target(dir)})
	if err != nil || len(reports) != 0 || c.downloads != 0 {
		t.Fatalf("reports=%v err=%v downloads=%d", reports, err, c.downloads)
	}
}

func TestRunBumpsIdenticalContentSilently(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, "v1", "solid", "new")
	c := &fakeClient{rel: github.Release{Tag: "v2"}, files: map[string]string{"skills/solid/SKILL.md": "new"}}
	a, _ := Analyze(context.Background(), c, []install.Target{target(dir)})
	if len(a.Outdated) != 0 {
		t.Fatalf("identical content must not count as outdated: %+v", a)
	}
	if _, err := Run(context.Background(), c, []install.Target{target(dir)}); err != nil {
		t.Fatal(err)
	}
	if m, _ := manifest.Read(dir); m.Skills["solid"].Version != "v2" {
		t.Fatalf("manifest %+v", m)
	}
}

func TestRunContinuesPastBrokenTarget(t *testing.T) {
	broken, good := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(broken, ".saxkills.json"), []byte("{nope"), 0o644)
	seed(t, good, "v1", "solid", "old")
	c := &fakeClient{rel: github.Release{Tag: "v2"}, files: map[string]string{"skills/solid/SKILL.md": "new"}}
	_, err := Run(context.Background(), c, []install.Target{target(broken), target(good)})
	if err == nil {
		t.Fatal("expected the broken target to be reported")
	}
	data, _ := os.ReadFile(filepath.Join(good, "solid/SKILL.md"))
	if string(data) != "new" {
		t.Fatal("healthy target must still be updated")
	}
}

func TestRunReportsSkippedSkills(t *testing.T) {
	dir := t.TempDir()
	seed(t, dir, "v1", "solid", "old")
	os.WriteFile(filepath.Join(dir, "solid/SKILL.md"), []byte("mine"), 0o644)
	c := &fakeClient{rel: github.Release{Tag: "v2"}, files: map[string]string{"skills/solid/SKILL.md": "new"}}
	reports, err := Run(context.Background(), c, []install.Target{target(dir)})
	if err != nil || len(reports) != 1 || reports[0].Results[0].Outcome != install.Skipped {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
}

func TestLoadRejectsEmptyRelease(t *testing.T) {
	c := &fakeClient{rel: github.Release{Tag: "v3"}, files: map[string]string{"README.md": "x"}}
	if _, err := Load(context.Background(), c); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadReturnsSkillsAndVersion(t *testing.T) {
	c := &fakeClient{rel: github.Release{Tag: "v3"}, files: map[string]string{
		"skills/solid/SKILL.md": "---\nname: solid\ndescription: D\n---\n",
		"README.md":             "x",
	}}
	b, err := Load(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if b.Release.Tag != "v3" || b.Skills["solid"].Description != "D" {
		t.Fatalf("%+v", b)
	}
}
