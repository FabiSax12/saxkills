package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/FabiSax12/saxkills/internal/cli"
	"github.com/FabiSax12/saxkills/internal/github"
	"github.com/FabiSax12/saxkills/internal/install"
	"github.com/FabiSax12/saxkills/internal/tui"
	"github.com/FabiSax12/saxkills/internal/update"
)

var version = ""

func currentVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

type lazyClient struct {
	once  sync.Once
	inner *github.HTTPClient
}

func (l *lazyClient) get() *github.HTTPClient {
	l.once.Do(func() {
		l.inner = github.NewHTTPClient(github.ResolveToken(os.Getenv, github.GhAuthToken))
	})
	return l.inner
}

func (l *lazyClient) LatestRelease(ctx context.Context) (github.Release, error) {
	return l.get().LatestRelease(ctx)
}

func (l *lazyClient) DownloadTarball(ctx context.Context, rel github.Release) (io.ReadCloser, error) {
	return l.get().DownloadTarball(ctx, rel)
}

func (l *lazyClient) DownloadAsset(ctx context.Context, a github.Asset) (io.ReadCloser, error) {
	return l.get().DownloadAsset(ctx, a)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	client := &lazyClient{}
	ver := currentVersion()

	env := cli.Env{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Client:  client,
		Home:    home,
		Cwd:     cwd,
		GitRoot: install.GitRoot,
		Version: ver,
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		ExePath: func() (string, error) {
			exe, err := os.Executable()
			if err != nil {
				return "", err
			}
			return filepath.EvalSymlinks(exe)
		},
	}
	env.RunTUI = func() error {
		root := install.ProjectRoot(cwd, home, install.GitRoot)
		return tui.Run(tui.Services{
			Targets: install.Targets(home, root),
			Home:    home,
			Version: ver,
			Latest:  client.LatestRelease,
			Load: func(ctx context.Context, rel github.Release) (update.Bundle, error) {
				return update.LoadRelease(ctx, client, rel)
			},
		})
	}
	os.Exit(cli.Run(ctx, os.Args[1:], env))
}
