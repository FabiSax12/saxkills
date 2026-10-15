package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const FileName = ".saxkills.json"

type Entry struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

type Manifest struct {
	Version string           `json:"version"`
	Skills  map[string]Entry `json:"skills"`
}

func Read(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return Manifest{Skills: map[string]Entry{}}, nil
	}
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: parse %s: %w", filepath.Join(dir, FileName), err)
	}
	if m.Skills == nil {
		m.Skills = map[string]Entry{}
	}
	return m, nil
}

func Write(dir string, m Manifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".saxkills-manifest-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, FileName))
}

func Remove(dir string) error {
	err := os.Remove(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
