package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FabiSax12/saxkills/internal/manifest"
	"github.com/FabiSax12/saxkills/internal/skills"
)

func mkSkill(name, body string, extra map[string]string) skills.Skill {
	files := map[string]skills.File{"SKILL.md": {Data: []byte(body)}}
	for p, c := range extra {
		files[p] = skills.File{Data: []byte(c)}
	}
	return skills.Skill{Name: name, Description: "d-" + name, Files: files}
}

func avail(ss ...skills.Skill) map[string]skills.Skill {
	m := map[string]skills.Skill{}
	for _, s := range ss {
		m[s.Name] = s
	}
	return m
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func resultFor(t *testing.T, rs []Result, name string) Result {
	t.Helper()
	for _, r := range rs {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no result for %s in %+v", name, rs)
	return Result{}
}

func run(t *testing.T, dir string, req Request) []Result {
	t.Helper()
	rs, err := Execute(dir, req)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestInstallFresh(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	s := mkSkill("solid", "v1", map[string]string{"references/a.md": "ref"})
	s.Files["run.sh"] = skills.File{Data: []byte("#!/bin/sh"), Exec: true}
	rs := run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1.0.0", Available: avail(s)})
	if r := resultFor(t, rs, "solid"); r.Outcome != Installed {
		t.Fatalf("outcome = %+v", r)
	}
	if read(t, filepath.Join(dir, "solid/references/a.md")) != "ref" {
		t.Fatal("nested file missing")
	}
	info, _ := os.Stat(filepath.Join(dir, "solid/run.sh"))
	if info.Mode()&0o111 == 0 {
		t.Fatal("exec bit lost")
	}
	m, _ := manifest.Read(dir)
	if m.Version != "v1.0.0" || m.Skills["solid"].Version != "v1.0.0" || m.Skills["solid"].Hash != skills.Hash(s.Files) {
		t.Fatalf("manifest = %+v", m)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".saxkills-new") || strings.HasSuffix(e.Name(), ".old") {
			t.Fatalf("temp leftover %s", e.Name())
		}
	}
}

func TestInstallAllWhenNoNames(t *testing.T) {
	dir := t.TempDir()
	rs := run(t, dir, Request{Mode: ModeInstall, Version: "v1", Available: avail(mkSkill("a", "1", nil), mkSkill("b", "1", nil))})
	if len(rs) != 2 || rs[0].Name != "a" || rs[1].Name != "b" {
		t.Fatalf("results = %+v", rs)
	}
}

func TestInstallSkipsUnmanagedAndLeavesOthersAlone(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "solid"), 0o755)
	os.WriteFile(filepath.Join(dir, "solid/SKILL.md"), []byte("mine"), 0o644)
	os.MkdirAll(filepath.Join(dir, "other"), 0o755)
	os.WriteFile(filepath.Join(dir, "other/SKILL.md"), []byte("other"), 0o644)
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(dir, "linked"))

	rs := run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid", "linked"}, Version: "v1",
		Available: avail(mkSkill("solid", "new", nil), mkSkill("linked", "new", nil)), Force: true})
	for _, name := range []string{"solid", "linked"} {
		r := resultFor(t, rs, name)
		if r.Outcome != Skipped || !strings.Contains(r.Reason, "exists and is not managed by saxkills") {
			t.Fatalf("%s: %+v", name, r)
		}
	}
	if read(t, filepath.Join(dir, "solid/SKILL.md")) != "mine" || read(t, filepath.Join(dir, "other/SKILL.md")) != "other" {
		t.Fatal("unmanaged content modified")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("symlink target modified")
	}
}

func TestInstallUnknownSkillSkipped(t *testing.T) {
	rs := run(t, t.TempDir(), Request{Mode: ModeInstall, Names: []string{"ghost"}, Version: "v1", Available: avail()})
	if r := resultFor(t, rs, "ghost"); r.Outcome != Skipped || !strings.Contains(r.Reason, "not found in release") {
		t.Fatalf("%+v", r)
	}
}

func TestInvalidNamesAreRejected(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	victim := filepath.Join(filepath.Dir(dir), "victim")
	os.MkdirAll(victim, 0o755)
	for _, mode := range []Mode{ModeInstall, ModeUpdate, ModeRemove} {
		rs := run(t, dir, Request{Mode: mode, Names: []string{"../victim"}, Version: "v1", Available: avail(), Force: true})
		if r := resultFor(t, rs, "../victim"); r.Outcome != Skipped || r.Reason != "invalid skill name" {
			t.Fatalf("mode %d: %+v", mode, r)
		}
	}
	if !exists(victim) {
		t.Fatal("victim removed")
	}
}

func makeClone(t *testing.T, name string) string {
	t.Helper()
	clone := t.TempDir()
	os.MkdirAll(filepath.Join(clone, "skills", name), 0o755)
	os.MkdirAll(filepath.Join(clone, "scripts"), 0o755)
	os.WriteFile(filepath.Join(clone, "scripts/install.sh"), []byte("#!/bin/sh"), 0o755)
	os.WriteFile(filepath.Join(clone, "skills", name, "SKILL.md"), []byte("clone"), 0o644)
	return clone
}

func TestLegacySymlinkIsConverted(t *testing.T) {
	dir := t.TempDir()
	clone := makeClone(t, "solid")
	os.Symlink(filepath.Join(clone, "skills/solid"), filepath.Join(dir, "solid"))

	st, _ := Inspect(dir, avail(mkSkill("solid", "new", nil)))
	if st[0].State != Legacy {
		t.Fatalf("state = %v", st[0].State)
	}

	rs := run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "new", nil))})
	if r := resultFor(t, rs, "solid"); r.Outcome != Converted {
		t.Fatalf("%+v", r)
	}
	info, _ := os.Lstat(filepath.Join(dir, "solid"))
	if info.Mode()&os.ModeSymlink != 0 || read(t, filepath.Join(dir, "solid/SKILL.md")) != "new" {
		t.Fatal("not converted to real copy")
	}
	if read(t, filepath.Join(clone, "skills/solid/SKILL.md")) != "clone" {
		t.Fatal("clone content was touched")
	}
}

func TestUpdateManagedSkill(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", map[string]string{"gone.md": "x"}))})

	rs := run(t, dir, Request{Mode: ModeUpdate, Version: "v2", Available: avail(mkSkill("solid", "new", nil))})
	if r := resultFor(t, rs, "solid"); r.Outcome != Updated {
		t.Fatalf("%+v", r)
	}
	if read(t, filepath.Join(dir, "solid/SKILL.md")) != "new" || exists(filepath.Join(dir, "solid/gone.md")) {
		t.Fatal("content not replaced cleanly")
	}
	m, _ := manifest.Read(dir)
	if m.Version != "v2" || m.Skills["solid"].Version != "v2" {
		t.Fatalf("manifest = %+v", m)
	}
}

func TestUpdateSameContentBumpsVersionOnly(t *testing.T) {
	dir := t.TempDir()
	s := mkSkill("solid", "same", nil)
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(s)})
	rs := run(t, dir, Request{Mode: ModeUpdate, Version: "v2", Available: avail(s)})
	if r := resultFor(t, rs, "solid"); r.Outcome != Unchanged {
		t.Fatalf("%+v", r)
	}
	if m, _ := manifest.Read(dir); m.Skills["solid"].Version != "v2" {
		t.Fatalf("manifest = %+v", m)
	}
}

func TestUpdateDoesNotInstallOrTouchUnmanaged(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
	os.MkdirAll(filepath.Join(dir, "mine"), 0o755)
	os.WriteFile(filepath.Join(dir, "mine/SKILL.md"), []byte("mine"), 0o644)

	rs := run(t, dir, Request{Mode: ModeUpdate, Version: "v2", Available: avail(
		mkSkill("solid", "new", nil), mkSkill("mine", "new", nil), mkSkill("fresh", "new", nil))})
	if len(rs) != 1 {
		t.Fatalf("results = %+v", rs)
	}
	if exists(filepath.Join(dir, "fresh")) || read(t, filepath.Join(dir, "mine/SKILL.md")) != "mine" {
		t.Fatal("update touched things it does not manage")
	}
}

func TestModifiedLocallyBlocksUpdateAndRemoveUnlessForced(t *testing.T) {
	setup := func() string {
		dir := t.TempDir()
		run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
		os.WriteFile(filepath.Join(dir, "solid/SKILL.md"), []byte("edited"), 0o644)
		return dir
	}
	tests := []struct {
		name  string
		mode  Mode
		force bool
		want  Outcome
		after func(dir string) bool
	}{
		{"update blocked", ModeUpdate, false, Skipped, func(d string) bool { return read(t, filepath.Join(d, "solid/SKILL.md")) == "edited" }},
		{"update forced", ModeUpdate, true, Updated, func(d string) bool { return read(t, filepath.Join(d, "solid/SKILL.md")) == "new" }},
		{"remove blocked", ModeRemove, false, Skipped, func(d string) bool { return exists(filepath.Join(d, "solid")) }},
		{"remove forced", ModeRemove, true, Removed, func(d string) bool { return !exists(filepath.Join(d, "solid")) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setup()
			rs := run(t, dir, Request{Mode: tt.mode, Names: []string{"solid"}, Version: "v2", Available: avail(mkSkill("solid", "new", nil)), Force: tt.force})
			r := resultFor(t, rs, "solid")
			if r.Outcome != tt.want {
				t.Fatalf("%+v", r)
			}
			if tt.want == Skipped && !strings.Contains(r.Reason, "modified locally") {
				t.Fatalf("reason = %q", r.Reason)
			}
			if !tt.after(dir) {
				t.Fatal("unexpected on-disk state")
			}
		})
	}
}

func TestExtraFileCountsAsModified(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
	os.WriteFile(filepath.Join(dir, "solid/notes.md"), []byte("n"), 0o644)
	st, _ := Inspect(dir, avail(mkSkill("solid", "old", nil)))
	if st[0].State != Modified {
		t.Fatalf("state = %v", st[0].State)
	}
}

func TestRemovedUpstream(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
	st, _ := Inspect(dir, avail(mkSkill("other", "x", nil)))
	if st[1].Name != "solid" || st[1].State != RemovedUpstream {
		t.Fatalf("states = %+v", st)
	}
	rs := run(t, dir, Request{Mode: ModeUpdate, Version: "v2", Available: avail(mkSkill("other", "x", nil))})
	if r := resultFor(t, rs, "solid"); r.Outcome != Skipped || r.Reason != "removed upstream" {
		t.Fatalf("%+v", r)
	}
	if !exists(filepath.Join(dir, "solid")) {
		t.Fatal("update removed a skill")
	}
	rs = run(t, dir, Request{Mode: ModeRemove, Names: []string{"solid"}})
	if r := resultFor(t, rs, "solid"); r.Outcome != Removed {
		t.Fatalf("%+v", r)
	}
}

func TestRemoveManagedKeepsOthersAndDropsEmptyManifest(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "mine"), 0o755)
	run(t, dir, Request{Mode: ModeInstall, Version: "v1", Available: avail(mkSkill("a", "1", nil), mkSkill("b", "1", nil))})

	run(t, dir, Request{Mode: ModeRemove, Names: []string{"a"}})
	if exists(filepath.Join(dir, "a")) || !exists(filepath.Join(dir, "b")) || !exists(filepath.Join(dir, "mine")) {
		t.Fatal("wrong directories removed")
	}
	run(t, dir, Request{Mode: ModeRemove, Names: []string{"b"}})
	if exists(filepath.Join(dir, manifest.FileName)) {
		t.Fatal("empty manifest should be deleted")
	}
	if !exists(filepath.Join(dir, "mine")) {
		t.Fatal("unmanaged dir removed")
	}
}

func TestRemoveUnmanagedSkipped(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "mine"), 0o755)
	rs := run(t, dir, Request{Mode: ModeRemove, Names: []string{"mine"}, Force: true})
	if r := resultFor(t, rs, "mine"); r.Outcome != Skipped || !strings.Contains(r.Reason, "not managed by saxkills") {
		t.Fatalf("%+v", r)
	}
	if !exists(filepath.Join(dir, "mine")) {
		t.Fatal("unmanaged dir removed")
	}
}

func TestInspectStates(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"current", "outdated", "gone"}, Version: "v1",
		Available: avail(mkSkill("current", "1", nil), mkSkill("outdated", "1", nil), mkSkill("gone", "1", nil))})
	os.MkdirAll(filepath.Join(dir, "conflict"), 0o755)

	st, err := Inspect(dir, avail(mkSkill("current", "1", nil), mkSkill("outdated", "2", nil), mkSkill("conflict", "1", nil), mkSkill("fresh", "1", nil)))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]State{}
	for _, s := range st {
		got[s.Name] = s.State
	}
	want := map[string]State{"current": UpToDate, "outdated": UpdateAvailable, "gone": RemovedUpstream, "conflict": Conflict, "fresh": NotInstalled}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: got %v want %v", name, got[name], w)
		}
	}
	for i := 1; i < len(st); i++ {
		if st[i-1].Name > st[i].Name {
			t.Fatal("not sorted")
		}
	}
}

func TestInspectOffline(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"a"}, Version: "v1", Available: avail(mkSkill("a", "1", nil))})
	st, _ := Inspect(dir, nil)
	if len(st) != 1 || st[0].State != UpToDate || st[0].Version != "v1" {
		t.Fatalf("%+v", st)
	}
}

func TestInspectMissingDirectoryForManagedEntry(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"a"}, Version: "v1", Available: avail(mkSkill("a", "1", nil))})
	os.RemoveAll(filepath.Join(dir, "a"))
	st, _ := Inspect(dir, avail(mkSkill("a", "1", nil)))
	if st[0].State != NotInstalled {
		t.Fatalf("%+v", st)
	}
	rs := run(t, dir, Request{Mode: ModeInstall, Names: []string{"a"}, Version: "v1", Available: avail(mkSkill("a", "1", nil))})
	if r := resultFor(t, rs, "a"); r.Outcome != Installed {
		t.Fatalf("%+v", r)
	}
}

func TestRemovedUpstreamNeverComputedFromEmptyRelease(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
	st, _ := Inspect(dir, avail())
	if st[0].State != UpToDate {
		t.Fatalf("state = %v", st[0].State)
	}
	rs := run(t, dir, Request{Mode: ModeUpdate, Version: "v2", Available: avail()})
	if r := resultFor(t, rs, "solid"); r.Outcome != Skipped || !strings.Contains(r.Reason, "no skills") {
		t.Fatalf("%+v", r)
	}
}

func TestReplaceIgnoresCleanupFailureAfterSwap(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
	orig := removeAll
	removeAll = func(string) error { return errors.New("busy") }
	defer func() { removeAll = orig }()
	rs, err := Execute(dir, Request{Mode: ModeUpdate, Version: "v2", Available: avail(mkSkill("solid", "new", nil))})
	if err != nil {
		t.Fatal(err)
	}
	if r := resultFor(t, rs, "solid"); r.Outcome != Updated {
		t.Fatalf("%+v", r)
	}
	if read(t, filepath.Join(dir, "solid/SKILL.md")) != "new" {
		t.Fatal("swap did not happen")
	}
}

func TestStaleTempDirsAreCleaned(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-3 * time.Hour)
	for _, n := range []string{".saxkills-new-x-1", ".saxkills-new-x-2.old"} {
		os.MkdirAll(filepath.Join(dir, n), 0o755)
		os.Chtimes(filepath.Join(dir, n), old, old)
	}
	os.MkdirAll(filepath.Join(dir, ".saxkills-new-fresh-3"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"a"}, Version: "v1", Available: avail(mkSkill("a", "1", nil))})
	if exists(filepath.Join(dir, ".saxkills-new-x-1")) || exists(filepath.Join(dir, ".saxkills-new-x-2.old")) {
		t.Fatal("stale temp dirs not cleaned")
	}
	if !exists(filepath.Join(dir, ".saxkills-new-fresh-3")) || !exists(filepath.Join(dir, ".hidden")) {
		t.Fatal("cleaned something it should not have")
	}
}

func TestDiskMatchingNewReleaseRepairsManifest(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
	os.WriteFile(filepath.Join(dir, "solid/SKILL.md"), []byte("new"), 0o644)
	next := avail(mkSkill("solid", "new", nil))
	if st, _ := Inspect(dir, next); st[0].State != UpToDate {
		t.Fatalf("state = %v", st[0].State)
	}
	rs := run(t, dir, Request{Mode: ModeUpdate, Version: "v2", Available: next})
	if r := resultFor(t, rs, "solid"); r.Outcome != Unchanged {
		t.Fatalf("%+v", r)
	}
	m, _ := manifest.Read(dir)
	if m.Skills["solid"].Hash != skills.Hash(next["solid"].Files) || m.Skills["solid"].Version != "v2" {
		t.Fatalf("manifest = %+v", m)
	}
}

func TestExecuteContinuesPastFailingSkill(t *testing.T) {
	dir := t.TempDir()
	bad := mkSkill("bad", "x", map[string]string{"../evil": "x"})
	rs, err := Execute(dir, Request{Mode: ModeInstall, Names: []string{"bad", "good"}, Version: "v1", Available: avail(bad, mkSkill("good", "g", nil))})
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	if r := resultFor(t, rs, "bad"); r.Outcome != Failed || r.Reason == "" {
		t.Fatalf("%+v", r)
	}
	if r := resultFor(t, rs, "good"); r.Outcome != Installed {
		t.Fatalf("%+v", r)
	}
	if m, _ := manifest.Read(dir); m.Skills["good"].Version != "v1" || len(m.Skills) != 1 {
		t.Fatalf("manifest = %+v", m)
	}
	if exists(filepath.Join(filepath.Dir(dir), "evil")) {
		t.Fatal("path escaped")
	}
}

func TestEditorAndOSNoiseDoesNotCountAsModified(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
	for _, n := range []string{".DS_Store", "SKILL.md~", ".SKILL.md.swp"} {
		os.WriteFile(filepath.Join(dir, "solid", n), []byte("n"), 0o644)
	}
	os.MkdirAll(filepath.Join(dir, "solid/emptydir"), 0o755)
	if st, _ := Inspect(dir, avail(mkSkill("solid", "old", nil))); st[0].State != UpToDate {
		t.Fatalf("state = %v", st[0].State)
	}
}

func TestUnreadableFileSurfacesAsError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read everything")
	}
	dir := t.TempDir()
	run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: avail(mkSkill("solid", "old", nil))})
	f := filepath.Join(dir, "solid/SKILL.md")
	os.Chmod(f, 0)
	defer os.Chmod(f, 0o644)
	if _, err := Inspect(dir, avail(mkSkill("solid", "new", nil))); err == nil {
		t.Fatal("Inspect should report unreadable file")
	}
	rs, err := Execute(dir, Request{Mode: ModeUpdate, Version: "v2", Available: avail(mkSkill("solid", "new", nil))})
	if err == nil || resultFor(t, rs, "solid").Outcome != Failed {
		t.Fatalf("rs=%+v err=%v", rs, err)
	}
}

func TestDanglingLegacySymlinkIsRepairable(t *testing.T) {
	dir := t.TempDir()
	os.Symlink("/nonexistent/old-clone/skills/solid", filepath.Join(dir, "solid"))
	os.Symlink(t.TempDir(), filepath.Join(dir, "other"))
	a := avail(mkSkill("solid", "new", nil), mkSkill("other", "new", nil))
	st, _ := Inspect(dir, a)
	got := map[string]State{}
	for _, s := range st {
		got[s.Name] = s.State
	}
	if got["solid"] != Legacy || got["other"] != Conflict {
		t.Fatalf("states = %v", got)
	}
	rs := run(t, dir, Request{Mode: ModeInstall, Names: []string{"solid"}, Version: "v1", Available: a})
	if r := resultFor(t, rs, "solid"); r.Outcome != Converted {
		t.Fatalf("%+v", r)
	}
}
