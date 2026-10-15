package skills

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

type entry struct {
	name     string
	body     string
	typeflag byte
	link     string
	mode     int64
}

func buildTarball(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		flag := e.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{Name: e.name, Typeflag: flag, Mode: mode, Linkname: e.link}
		if flag == tar.TypeXGlobalHeader {
			hdr = &tar.Header{Typeflag: flag, Name: "pax_global_header", PAXRecords: map[string]string{"comment": "abc"}}
		}
		if flag == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if flag == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

const rootDir = "FabiSax12-saxkills-abc123/"

func TestExtractHappyPath(t *testing.T) {
	data := buildTarball(t, []entry{
		{name: "pax_global_header", typeflag: tar.TypeXGlobalHeader},
		{name: rootDir, typeflag: tar.TypeDir},
		{name: rootDir + "README.md", body: "readme"},
		{name: rootDir + "skills/", typeflag: tar.TypeDir},
		{name: rootDir + "skills/solid/", typeflag: tar.TypeDir},
		{name: rootDir + "skills/solid/SKILL.md", body: "---\nname: solid\ndescription: SOLID as a guide.\n---\nbody"},
		{name: rootDir + "skills/solid/references/a.md", body: "ref"},
		{name: rootDir + "skills/solid/scripts/run.sh", body: "#!/bin/sh", mode: 0o755},
		{name: rootDir + "skills/empty/notes.txt", body: "no skill file"},
		{name: rootDir + "scripts/install.sh", body: "x"},
	})
	got, err := Extract(bytes.NewReader(data), DefaultMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only solid, got %v", keys(got))
	}
	s := got["solid"]
	if s.Description != "SOLID as a guide." {
		t.Fatalf("description = %q", s.Description)
	}
	if string(s.Files["references/a.md"].Data) != "ref" {
		t.Fatalf("files = %v", s.Files)
	}
	if !s.Files["scripts/run.sh"].Exec || s.Files["SKILL.md"].Exec {
		t.Fatal("exec bit not preserved correctly")
	}
}

func keys(m map[string]Skill) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestExtractRejectsHostileEntries(t *testing.T) {
	tests := []struct {
		name  string
		entry entry
	}{
		{"dotdot", entry{name: rootDir + "skills/solid/../../evil", body: "x"}},
		{"dotdot at start", entry{name: "../evil", body: "x"}},
		{"absolute", entry{name: "/etc/passwd", body: "x"}},
		{"symlink in skills", entry{name: rootDir + "skills/solid/link", typeflag: tar.TypeSymlink, link: "/etc/passwd"}},
		{"hardlink in skills", entry{name: rootDir + "skills/solid/hl", typeflag: tar.TypeLink, link: "x"}},
		{"bad skill name", entry{name: rootDir + "skills/.hidden/SKILL.md", body: "x"}},
		{"backslash", entry{name: rootDir + "skills/solid/a\\..\\b", body: "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildTarball(t, []entry{
				{name: rootDir + "skills/solid/SKILL.md", body: "ok"},
				tt.entry,
			})
			if _, err := Extract(bytes.NewReader(data), DefaultMaxBytes); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestExtractIgnoresSymlinksOutsideSkills(t *testing.T) {
	data := buildTarball(t, []entry{
		{name: rootDir + "CLAUDE.md", typeflag: tar.TypeSymlink, link: "AGENTS.md"},
		{name: rootDir + "skills/solid/SKILL.md", body: "ok"},
	})
	got, err := Extract(bytes.NewReader(data), DefaultMaxBytes)
	if err != nil || len(got) != 1 {
		t.Fatalf("err=%v got=%v", err, keys(got))
	}
}

func TestExtractEnforcesSizeCap(t *testing.T) {
	data := buildTarball(t, []entry{
		{name: rootDir + "skills/solid/SKILL.md", body: strings.Repeat("a", 2048)},
	})
	if _, err := Extract(bytes.NewReader(data), 1024); err == nil {
		t.Fatal("expected size cap error")
	}
}

func TestExtractRejectsNonGzip(t *testing.T) {
	if _, err := Extract(strings.NewReader("not gzip"), DefaultMaxBytes); err == nil {
		t.Fatal("expected error")
	}
}

func TestHashIsDeterministicAndSensitive(t *testing.T) {
	a := map[string]File{"SKILL.md": {Data: []byte("x")}, "b/c.md": {Data: []byte("y")}}
	b := map[string]File{"b/c.md": {Data: []byte("y")}, "SKILL.md": {Data: []byte("x")}}
	if Hash(a) != Hash(b) {
		t.Fatal("hash depends on map order")
	}
	tests := map[string]map[string]File{
		"content": {"SKILL.md": {Data: []byte("z")}, "b/c.md": {Data: []byte("y")}},
		"exec":    {"SKILL.md": {Data: []byte("x"), Exec: true}, "b/c.md": {Data: []byte("y")}},
		"extra":   {"SKILL.md": {Data: []byte("x")}, "b/c.md": {Data: []byte("y")}, "d": {Data: nil}},
		"rename":  {"SKILL.md": {Data: []byte("x")}, "b/d.md": {Data: []byte("y")}},
	}
	for name, files := range tests {
		if Hash(files) == Hash(a) {
			t.Fatalf("hash unchanged for %s", name)
		}
	}
}

func TestParseDescription(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "---\nname: x\ndescription: Does things.\n---\n", "Does things."},
		{"quoted", "---\nname: x\ndescription: \"Does: things.\"\n---\n", "Does: things."},
		{"crlf", "---\r\nname: x\r\ndescription: Hi\r\n---\r\n", "Hi"},
		{"missing", "---\nname: x\n---\n", ""},
		{"no frontmatter", "# title\ndescription: nope\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseDescription([]byte(tt.in)); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestValidName(t *testing.T) {
	for _, n := range []string{"solid", "clean-code", "a1", "x.y"} {
		if !ValidName(n) {
			t.Errorf("%q should be valid", n)
		}
	}
	for _, n := range []string{"", ".", "..", ".hidden", "a/b", "a\\b", "-x", "a b"} {
		if ValidName(n) {
			t.Errorf("%q should be invalid", n)
		}
	}
}

func TestExtractRejectsEmptyRelease(t *testing.T) {
	data := buildTarball(t, []entry{
		{name: rootDir + "README.md", body: "x"},
		{name: rootDir + "skills/nofile/notes.txt", body: "x"},
	})
	if _, err := Extract(bytes.NewReader(data), DefaultMaxBytes); err == nil {
		t.Fatal("expected error for a release without skills")
	}
}

func TestHashIgnoresNoiseFiles(t *testing.T) {
	base := map[string]File{"SKILL.md": {Data: []byte("x")}}
	noisy := map[string]File{"SKILL.md": {Data: []byte("x")}, "sub/.DS_Store": {Data: []byte("a")}, "SKILL.md~": {Data: []byte("b")}, ".SKILL.md.swp": {Data: []byte("c")}}
	if Hash(base) != Hash(noisy) {
		t.Fatal("noise files must not affect the hash")
	}
}
