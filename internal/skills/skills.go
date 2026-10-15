package skills

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
)

const DefaultMaxBytes int64 = 64 << 20

var (
	ErrTooLarge = errors.New("skills: archive exceeds size cap")
	nameRe      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

type File struct {
	Data []byte
	Exec bool
}

type Skill struct {
	Name        string
	Description string
	Files       map[string]File
}

func ValidName(name string) bool {
	return nameRe.MatchString(name)
}

func Ignored(rel string) bool {
	base := path.Base(rel)
	return base == ".DS_Store" || strings.HasSuffix(base, "~") || (strings.HasPrefix(base, ".") && strings.HasSuffix(base, ".swp"))
}

func Hash(files map[string]File) string {
	paths := make([]string, 0, len(files))
	for p := range files {
		if !Ignored(p) {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		f := files[p]
		exec := "0"
		if f.Exec {
			exec = "1"
		}
		fmt.Fprintf(h, "%d:%s\n%s\n%d\n", len(p), p, exec, len(f.Data))
		h.Write(f.Data)
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ParseDescription(skillMD []byte) string {
	text := strings.ReplaceAll(string(skillMD), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return ""
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		value, ok := strings.CutPrefix(line, "description:")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		return value
	}
	return ""
}

type capReader struct {
	r    io.Reader
	left int64
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.left < 0 {
		return 0, ErrTooLarge
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	if c.left < 0 {
		return n, ErrTooLarge
	}
	return n, err
}

func Extract(r io.Reader, maxBytes int64) (map[string]Skill, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("skills: open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(&capReader{r: gz, left: maxBytes})

	found := map[string]*Skill{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("skills: read tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader || hdr.Typeflag == tar.TypeXHeader {
			continue
		}
		if err := checkRawName(hdr.Name); err != nil {
			return nil, err
		}
		rel, ok := stripRoot(hdr.Name)
		if !ok {
			continue
		}
		parts := strings.Split(rel, "/")
		if parts[0] != "skills" || len(parts) < 2 {
			continue
		}
		name := parts[1]
		if !ValidName(name) {
			return nil, fmt.Errorf("skills: invalid skill name %q", name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return nil, fmt.Errorf("skills: unsupported entry type %q for %s", string(hdr.Typeflag), hdr.Name)
		}
		if len(parts) < 3 {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("skills: read %s: %w", hdr.Name, err)
		}
		s := found[name]
		if s == nil {
			s = &Skill{Name: name, Files: map[string]File{}}
			found[name] = s
		}
		s.Files[strings.Join(parts[2:], "/")] = File{Data: data, Exec: hdr.Mode&0o111 != 0}
	}

	out := map[string]Skill{}
	for name, s := range found {
		md, ok := s.Files["SKILL.md"]
		if !ok {
			continue
		}
		s.Description = ParseDescription(md.Data)
		out[name] = *s
	}
	if len(out) == 0 {
		return nil, errors.New("skills: release contains no skills")
	}
	return out, nil
}

func checkRawName(name string) error {
	if strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) {
		return fmt.Errorf("skills: unsafe path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return fmt.Errorf("skills: unsafe path %q", name)
		}
	}
	return nil
}

func stripRoot(name string) (string, bool) {
	_, rel, ok := strings.Cut(strings.TrimSuffix(name, "/"), "/")
	if !ok || rel == "" {
		return "", false
	}
	return rel, true
}
