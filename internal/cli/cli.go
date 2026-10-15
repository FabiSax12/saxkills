package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/FabiSax12/saxkills/internal/github"
	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/selfupdate"
	"github.com/FabiSax12/saxkills/internal/update"
	"github.com/FabiSax12/saxkills/internal/version"
)

const (
	ExitOK       = 0
	ExitFailure  = 1
	ExitUsage    = 2
	ExitSkipped  = 3
	ExitOutdated = 10
)

type Env struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Client  github.Client
	Home    string
	Cwd     string
	GitRoot func(dir string) (string, error)
	Version string
	GOOS    string
	GOARCH  string
	ExePath func() (string, error)
	RunTUI  func() error
}

const usage = `Usage:
  saxkills                       open the interactive installer
  saxkills update [--all] [--scope global|project] [--target claude|agents] [--quiet]
  saxkills check [--quiet]       exit 10 when an install can be updated, 0 otherwise
  saxkills self-update           replace this binary with the latest release
  saxkills version
`

func Run(ctx context.Context, args []string, env Env) int {
	if len(args) == 0 {
		if err := env.RunTUI(); err != nil {
			fmt.Fprintln(env.Stderr, "error:", err)
			return ExitFailure
		}
		return ExitOK
	}
	if (args[0] == "version" || args[0] == "self-update") && len(args) > 1 {
		fmt.Fprintf(env.Stderr, "unexpected argument %q\n", args[1])
		return ExitUsage
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(env.Stdout, "saxkills", env.Version)
		return ExitOK
	case "help", "--help", "-h":
		fmt.Fprint(env.Stdout, usage)
		return ExitOK
	case "check":
		return runCheck(ctx, args[1:], env)
	case "update":
		return runUpdate(ctx, args[1:], env)
	case "self-update":
		return runSelfUpdate(ctx, env)
	default:
		fmt.Fprintf(env.Stderr, "unknown command %q\n\n%s", args[0], usage)
		return ExitUsage
	}
}

func targetsFor(env Env) []install.Target {
	root := install.ProjectRoot(env.Cwd, env.Home, env.GitRoot)
	return install.Targets(env.Home, root)
}

type selector struct {
	all    bool
	scope  string
	target string
}

func (s selector) filter(targets []install.Target) (usable, refused []install.Target) {
	for _, t := range targets {
		if t.Dir == "" {
			continue
		}
		if s.scope != "" && string(t.Scope) != s.scope {
			continue
		}
		if s.target != "" && string(t.Agent) != s.target {
			continue
		}
		if t.Unavailable != "" {
			refused = append(refused, t)
			continue
		}
		usable = append(usable, t)
	}
	return usable, refused
}

func reportRefused(env Env, refused []install.Target) bool {
	for _, t := range refused {
		fmt.Fprintf(env.Stderr, "error: %s: %s\n", t.Label(), t.Unavailable)
	}
	return len(refused) > 0
}

func parseSelector(name string, args []string, env Env, requireSelector bool) (selector, bool, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	var sel selector
	var quiet bool
	fs.BoolVar(&sel.all, "all", false, "every location: global and project, claude and agents")
	fs.StringVar(&sel.scope, "scope", "", "global or project")
	fs.StringVar(&sel.target, "target", "", "claude or agents")
	fs.BoolVar(&quiet, "quiet", false, "print errors only")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return sel, false, ExitOK
		}
		return sel, false, ExitUsage
	}
	if sel.scope != "" && sel.scope != "global" && sel.scope != "project" {
		fmt.Fprintf(env.Stderr, "invalid --scope %q (use global or project)\n", sel.scope)
		return sel, false, ExitUsage
	}
	if sel.target != "" && sel.target != "claude" && sel.target != "agents" {
		fmt.Fprintf(env.Stderr, "invalid --target %q (use claude or agents)\n", sel.target)
		return sel, false, ExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(env.Stderr, "unexpected argument %q\n", fs.Arg(0))
		return sel, false, ExitUsage
	}
	if requireSelector && !sel.all && sel.scope == "" && sel.target == "" {
		fmt.Fprintln(env.Stderr, "specify --all, --scope or --target")
		return sel, false, ExitUsage
	}
	return sel, quiet, -1
}

func runCheck(ctx context.Context, args []string, env Env) int {
	sel, quiet, code := parseSelector("check", args, env, false)
	if code >= 0 {
		return code
	}
	targets, refused := sel.filter(targetsFor(env))
	failed := reportRefused(env, refused)
	a, err := update.Analyze(ctx, env.Client, targets)
	if err != nil {
		fmt.Fprintln(env.Stderr, "error:", err)
		failed = true
	}
	if !quiet {
		printCheck(env, a)
	}
	switch {
	case failed:
		return ExitFailure
	case len(a.Outdated) > 0:
		return ExitOutdated
	}
	return ExitOK
}

func printCheck(env Env, a update.Analysis) {
	if a.Managed == 0 {
		fmt.Fprintln(env.Stdout, "no managed installs found")
		return
	}
	for _, o := range a.Outdated {
		fmt.Fprintf(env.Stdout, "%s (%s): outdated, latest is %s: %s\n", o.Target.Label(), o.Target.Dir, a.Latest, strings.Join(o.Skills, ", "))
	}
	for _, at := range a.Attention {
		fmt.Fprintf(env.Stdout, "needs attention: %s: %s is %s\n", at.Target.Label(), at.Skill, at.State)
	}
	if len(a.Outdated) == 0 && len(a.Attention) == 0 {
		fmt.Fprintln(env.Stdout, "all managed installs are up to date")
	}
}

func runUpdate(ctx context.Context, args []string, env Env) int {
	sel, quiet, code := parseSelector("update", args, env, true)
	if code >= 0 {
		return code
	}
	targets, refused := sel.filter(targetsFor(env))
	failed := reportRefused(env, refused)
	reports, err := update.Run(ctx, env.Client, targets)
	skipped := false
	for _, r := range reports {
		if r.Err != nil {
			failed = true
		}
		for _, res := range r.Results {
			if res.Outcome == install.Skipped {
				skipped = true
			}
			if res.Outcome == install.Unchanged || (quiet && res.Outcome != install.Skipped) {
				continue
			}
			line := fmt.Sprintf("%s: %s %s", r.Target.Label(), res.Name, res.Outcome)
			if res.Reason != "" {
				line += " (" + res.Reason + ")"
			}
			if res.Outcome == install.Skipped || res.Outcome == install.Failed {
				fmt.Fprintln(env.Stderr, "warning:", line)
			} else {
				fmt.Fprintln(env.Stdout, line)
			}
		}
	}
	switch {
	case err != nil:
		fmt.Fprintln(env.Stderr, "error:", err)
		return ExitFailure
	case failed:
		return ExitFailure
	case skipped:
		return ExitSkipped
	}
	return ExitOK
}

func runSelfUpdate(ctx context.Context, env Env) int {
	rel, err := env.Client.LatestRelease(ctx)
	if err != nil {
		fmt.Fprintln(env.Stderr, "error:", err)
		return ExitFailure
	}
	if env.Version != "dev" && !version.Newer(rel.Tag, env.Version) {
		fmt.Fprintf(env.Stdout, "saxkills %s is up to date\n", env.Version)
		return ExitOK
	}
	exe, err := env.ExePath()
	if err != nil {
		fmt.Fprintln(env.Stderr, "error:", err)
		return ExitFailure
	}
	if err := selfupdate.Apply(ctx, env.Client, rel, env.GOOS, env.GOARCH, exe); err != nil {
		fmt.Fprintln(env.Stderr, "error:", err)
		return ExitFailure
	}
	fmt.Fprintf(env.Stdout, "updated saxkills to %s\n", rel.Tag)
	return ExitOK
}
