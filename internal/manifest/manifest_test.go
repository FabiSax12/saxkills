package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMissingReturnsEmpty(t *testing.T) {
	m, err := Read(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "" || len(m.Skills) != 0 || m.Skills == nil {
		t.Fatalf("unexpected %+v", m)
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	want := Manifest{Version: "v1.0.0", Skills: map[string]Entry{"solid": {Version: "v1.0.0", Hash: "abc"}}}
	if err := Write(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != want.Version || got.Skills["solid"] != want.Skills["solid"] {
		t.Fatalf("got %+v", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != FileName {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestReadCorrupt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, FileName), []byte("{nope"), 0o644)
	if _, err := Read(dir); err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveDeletesFile(t *testing.T) {
	dir := t.TempDir()
	Write(dir, Manifest{Skills: map[string]Entry{}})
	if err := Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); err != nil {
		t.Fatalf("second remove should be a no-op: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
		t.Fatal("manifest still present")
	}
}
