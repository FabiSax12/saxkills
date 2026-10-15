package update

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/FabiSax12/saxkills/internal/github"
	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/manifest"
	"github.com/FabiSax12/saxkills/internal/skills"
	"github.com/FabiSax12/saxkills/internal/version"
)

type Bundle struct {
	Release github.Release
	Skills  map[string]skills.Skill
}

type Managed struct {
	Target   install.Target
	Manifest manifest.Manifest
}

type Outdated struct {
	Target install.Target
	Skills []string
}

type Attention struct {
	Target install.Target
	Skill  string
	State  install.State
}

type Report struct {
	Target  install.Target
	Results []install.Result
	Err     error
}

func Load(ctx context.Context, c github.Client) (Bundle, error) {
	rel, err := c.LatestRelease(ctx)
	if err != nil {
		return Bundle{}, err
	}
	return LoadRelease(ctx, c, rel)
}

func LoadRelease(ctx context.Context, c github.Client, rel github.Release) (Bundle, error) {
	body, err := c.DownloadTarball(ctx, rel)
	if err != nil {
		return Bundle{}, err
	}
	defer body.Close()
	found, err := skills.Extract(body, skills.DefaultMaxBytes)
	if err != nil {
		return Bundle{}, err
	}
	return Bundle{Release: rel, Skills: found}, nil
}

func FindManaged(targets []install.Target) ([]Managed, error) {
	var out []Managed
	var errs []error
	for _, t := range targets {
		if t.Dir == "" || t.Unavailable != "" {
			continue
		}
		m, err := manifest.Read(t.Dir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(m.Skills) > 0 {
			out = append(out, Managed{Target: t, Manifest: m})
		}
	}
	return out, errors.Join(errs...)
}

func OutdatedIn(managed []Managed, latest string) []Outdated {
	var out []Outdated
	for _, m := range managed {
		var names []string
		for name, e := range m.Manifest.Skills {
			if version.Outdated(e.Version, latest) {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			sort.Strings(names)
			out = append(out, Outdated{Target: m.Target, Skills: names})
		}
	}
	return out
}

type Classification struct {
	Pending   []Outdated
	Outdated  []Outdated
	Attention []Attention
}

func Classify(managed []Managed, avail map[string]skills.Skill, latest string) (Classification, error) {
	var c Classification
	var errs []error
	c.Pending = OutdatedIn(managed, latest)
	for _, p := range c.Pending {
		statuses, err := install.Inspect(p.Target.Dir, avail)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Target.Dir, err))
			continue
		}
		byName := map[string]install.State{}
		for _, st := range statuses {
			byName[st.Name] = st.State
		}
		var actionable []string
		for _, name := range p.Skills {
			switch state := byName[name]; state {
			case install.UpdateAvailable:
				actionable = append(actionable, name)
			case install.Modified, install.RemovedUpstream:
				c.Attention = append(c.Attention, Attention{Target: p.Target, Skill: name, State: state})
			}
		}
		if len(actionable) > 0 {
			c.Outdated = append(c.Outdated, Outdated{Target: p.Target, Skills: actionable})
		}
	}
	return c, errors.Join(errs...)
}

type Analysis struct {
	Classification
	Latest  string
	Managed int
	bundle  *Bundle
}

func Analyze(ctx context.Context, c github.Client, targets []install.Target) (Analysis, error) {
	managed, findErr := FindManaged(targets)
	a := Analysis{Managed: len(managed)}
	if len(managed) == 0 {
		return a, findErr
	}
	rel, err := c.LatestRelease(ctx)
	if err != nil {
		return a, errors.Join(findErr, err)
	}
	a.Latest = rel.Tag
	a.Pending = OutdatedIn(managed, rel.Tag)
	if len(a.Pending) == 0 {
		return a, findErr
	}
	bundle, err := LoadRelease(ctx, c, rel)
	if err != nil {
		return a, errors.Join(findErr, err)
	}
	a.bundle = &bundle
	cl, err := Classify(managed, bundle.Skills, rel.Tag)
	a.Classification = cl
	return a, errors.Join(findErr, err)
}

func Run(ctx context.Context, c github.Client, targets []install.Target) ([]Report, error) {
	a, err := Analyze(ctx, c, targets)
	if a.bundle == nil {
		return nil, err
	}
	errs := []error{err}
	var reports []Report
	for _, p := range a.Pending {
		results, execErr := install.Execute(p.Target.Dir, install.Request{
			Mode:      install.ModeUpdate,
			Names:     p.Skills,
			Version:   a.Latest,
			Available: a.bundle.Skills,
		})
		reports = append(reports, Report{Target: p.Target, Results: results, Err: execErr})
		if execErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Target.Dir, execErr))
		}
	}
	return reports, errors.Join(errs...)
}
