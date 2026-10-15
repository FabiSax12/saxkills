package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/FabiSax12/saxkills/internal/manifest"
	"github.com/FabiSax12/saxkills/internal/skills"
)

type Mode int

const (
	ModeInstall Mode = iota
	ModeUpdate
	ModeRemove
)

type Outcome string

const (
	Installed Outcome = "installed"
	Updated   Outcome = "updated"
	Converted Outcome = "converted"
	Removed   Outcome = "removed"
	Unchanged Outcome = "unchanged"
	Skipped   Outcome = "skipped"
	Failed    Outcome = "failed"
)

var (
	removeAll = os.RemoveAll
	rename    = os.Rename
)

const staleAge = time.Hour

type State string

const (
	NotInstalled    State = "not installed"
	UpToDate        State = "up to date"
	UpdateAvailable State = "update available"
	Modified        State = "modified locally"
	Conflict        State = "conflict"
	Legacy          State = "linked (legacy)"
	RemovedUpstream State = "removed upstream"
)

const (
	reasonUnmanaged = "exists and is not managed by saxkills"
	reasonModified  = "modified locally (use force to overwrite)"
)

type Request struct {
	Mode      Mode
	Names     []string
	Version   string
	Available map[string]skills.Skill
	Force     bool
}

type Result struct {
	Name    string
	Outcome Outcome
	Reason  string
}

type SkillStatus struct {
	Name        string
	State       State
	Version     string
	Description string
}

func Inspect(dir string, available map[string]skills.Skill) ([]SkillStatus, error) {
	m, err := manifest.Read(dir)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for n := range available {
		names[n] = true
	}
	for n := range m.Skills {
		names[n] = true
	}
	var out []SkillStatus
	for name := range names {
		if !skills.ValidName(name) {
			continue
		}
		st := SkillStatus{Name: name, Description: available[name].Description}
		entry, managed := m.Skills[name]
		up, inRelease := available[name]
		path := filepath.Join(dir, name)
		_, lerr := os.Lstat(path)
		present := lerr == nil
		switch {
		case managed && present:
			st.Version = entry.Version
			dh, err := diskHash(path)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			known := entry.Hash
			if inRelease && dh == skills.Hash(up.Files) {
				known = dh
			}
			switch {
			case dh != known:
				st.State = Modified
			case len(available) > 0 && !inRelease:
				st.State = RemovedUpstream
			case inRelease && skills.Hash(up.Files) != known:
				st.State = UpdateAvailable
			default:
				st.State = UpToDate
			}
		case managed:
			if !inRelease {
				continue
			}
			st.State = NotInstalled
		case !inRelease:
			continue
		case present && isLegacyLink(dir, name):
			st.State = Legacy
		case present:
			st.State = Conflict
		default:
			st.State = NotInstalled
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Execute(dir string, req Request) ([]Result, error) {
	m, err := manifest.Read(dir)
	if err != nil {
		return nil, err
	}
	names := req.Names
	if len(names) == 0 {
		switch req.Mode {
		case ModeInstall:
			for n := range req.Available {
				names = append(names, n)
			}
		case ModeUpdate:
			for n := range m.Skills {
				names = append(names, n)
			}
		}
		sort.Strings(names)
	}

	cleanStale(dir)
	var results []Result
	var errs []error
	for _, name := range names {
		r, changed, err := executeOne(dir, &m, name, req)
		if err != nil {
			results = append(results, Result{Name: name, Outcome: Failed, Reason: err.Error()})
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		results = append(results, r)
		if changed {
			if err := saveManifest(dir, m); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return results, errors.Join(errs...)
}

func cleanStale(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".saxkills-new-") {
			continue
		}
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < staleAge {
			continue
		}
		removeAll(filepath.Join(dir, e.Name()))
	}
}

func saveManifest(dir string, m manifest.Manifest) error {
	if len(m.Skills) == 0 {
		return manifest.Remove(dir)
	}
	return manifest.Write(dir, m)
}

func executeOne(dir string, m *manifest.Manifest, name string, req Request) (Result, bool, error) {
	skip := func(reason string) (Result, bool, error) {
		return Result{Name: name, Outcome: Skipped, Reason: reason}, false, nil
	}
	if !skills.ValidName(name) {
		return skip("invalid skill name")
	}
	path := filepath.Join(dir, name)
	entry, managed := m.Skills[name]
	_, lerr := os.Lstat(path)
	present := lerr == nil

	if req.Mode == ModeRemove {
		if !managed {
			return skip(reasonUnmanaged)
		}
		if present {
			dh, err := diskHash(path)
			if err != nil {
				return Result{}, false, err
			}
			if dh != entry.Hash && !req.Force {
				return skip(reasonModified)
			}
		}
		if err := removeAll(path); err != nil {
			return Result{}, false, err
		}
		delete(m.Skills, name)
		return Result{Name: name, Outcome: Removed}, true, nil
	}

	rel, inRelease := req.Available[name]
	if req.Mode == ModeUpdate && !managed {
		return skip(reasonUnmanaged)
	}
	if !inRelease {
		if managed && len(req.Available) == 0 {
			return skip("release contains no skills")
		}
		if managed {
			return skip("removed upstream")
		}
		return skip("not found in release")
	}
	newHash := skills.Hash(rel.Files)

	outcome := Installed
	switch {
	case managed && present:
		dh, err := diskHash(path)
		if err != nil {
			return Result{}, false, err
		}
		modified := dh != entry.Hash
		if modified && dh == newHash {
			m.Skills[name] = manifest.Entry{Version: req.Version, Hash: newHash}
			m.Version = req.Version
			return Result{Name: name, Outcome: Unchanged}, true, nil
		}
		if modified && !req.Force {
			return skip(reasonModified)
		}
		if !modified && entry.Hash == newHash {
			if entry.Version == req.Version {
				return Result{Name: name, Outcome: Unchanged}, false, nil
			}
			m.Skills[name] = manifest.Entry{Version: req.Version, Hash: newHash}
			m.Version = req.Version
			return Result{Name: name, Outcome: Unchanged}, true, nil
		}
		outcome = Updated
	case present && !managed && isLegacyLink(dir, name):
		outcome = Converted
	case present && !managed:
		return skip(reasonUnmanaged)
	}

	if err := replaceDir(dir, name, rel.Files); err != nil {
		return Result{}, false, err
	}
	m.Skills[name] = manifest.Entry{Version: req.Version, Hash: newHash}
	m.Version = req.Version
	return Result{Name: name, Outcome: outcome}, true, nil
}

func replaceDir(parent, name string, files map[string]skills.File) error {
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".saxkills-new-"+name+"-")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			removeAll(tmp)
		}
	}()
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	for rel, f := range files {
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return fmt.Errorf("unsafe file path %q", rel)
		}
		dst := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if f.Exec {
			mode = 0o755
		}
		if err := os.WriteFile(dst, f.Data, mode); err != nil {
			return err
		}
		if err := os.Chmod(dst, mode); err != nil {
			return err
		}
	}

	dest := filepath.Join(parent, name)
	if _, err := os.Lstat(dest); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := rename(tmp, dest); err != nil {
			return err
		}
		cleanup = false
		return nil
	}
	old := tmp + ".old"
	if err := rename(dest, old); err != nil {
		return err
	}
	if err := rename(tmp, dest); err != nil {
		rename(old, dest)
		return err
	}
	cleanup = false
	removeAll(old)
	return nil
}

func diskFiles(path string) (map[string]skills.File, error) {
	files := map[string]skills.File{}
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(path, p)
		rel = filepath.ToSlash(rel)
		if skills.Ignored(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			files[rel] = skills.File{Data: []byte("symlink:" + target)}
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[rel] = skills.File{Data: data, Exec: info.Mode()&0o111 != 0}
		return nil
	})
	return files, err
}

func diskHash(path string) (string, error) {
	files, err := diskFiles(path)
	if err != nil {
		return "", err
	}
	return skills.Hash(files), nil
}

func isLegacyLink(dir, name string) bool {
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	target = filepath.Clean(target)
	skillsDir := filepath.Dir(target)
	if filepath.Base(target) != name || filepath.Base(skillsDir) != "skills" {
		return false
	}
	if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) {
		return true
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(skillsDir), "scripts", "install.sh"))
	return err == nil
}
