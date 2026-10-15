package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.github.com"
	DefaultRepo    = "FabiSax12/saxkills"
)

var ErrNotFound = errors.New("not found (a private repository needs GITHUB_TOKEN or gh auth login)")

type Asset struct {
	Name string
	URL  string
	Size int64
}

type Release struct {
	Tag        string
	TarballURL string
	Assets     []Asset
}

func (r Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

type Client interface {
	LatestRelease(ctx context.Context) (Release, error)
	DownloadTarball(ctx context.Context, rel Release) (io.ReadCloser, error)
	DownloadAsset(ctx context.Context, asset Asset) (io.ReadCloser, error)
}

type HTTPClient struct {
	BaseURL string
	Repo    string
	Token   string
	HTTP    *http.Client
}

func NewHTTPClient(token string) *HTTPClient {
	return &HTTPClient{
		BaseURL: DefaultBaseURL,
		Repo:    DefaultRepo,
		Token:   token,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

type releaseJSON struct {
	Tag        string `json:"tag_name"`
	TarballURL string `json:"tarball_url"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

func (c *HTTPClient) LatestRelease(ctx context.Context) (Release, error) {
	body, err := c.get(ctx, c.BaseURL+"/repos/"+c.Repo+"/releases/latest", "application/vnd.github+json")
	if err != nil {
		return Release{}, err
	}
	defer body.Close()
	var raw releaseJSON
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return Release{}, fmt.Errorf("github: decode release: %w", err)
	}
	rel := Release{Tag: raw.Tag, TarballURL: raw.TarballURL}
	for _, a := range raw.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.URL, Size: a.Size})
	}
	return rel, nil
}

func (c *HTTPClient) DownloadTarball(ctx context.Context, rel Release) (io.ReadCloser, error) {
	return c.get(ctx, rel.TarballURL, "application/vnd.github+json")
}

func (c *HTTPClient) DownloadAsset(ctx context.Context, asset Asset) (io.ReadCloser, error) {
	return c.get(ctx, asset.URL, "application/octet-stream")
}

func (c *HTTPClient) get(ctx context.Context, rawURL, accept string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" && c.sameHost(req.URL) {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: request failed: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("github: unexpected status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *HTTPClient) sameHost(u *url.URL) bool {
	base, err := url.Parse(c.BaseURL)
	return err == nil && strings.EqualFold(base.Host, u.Host)
}

func ResolveToken(getenv func(string) string, ghToken func() (string, error)) string {
	if t := strings.TrimSpace(getenv("GITHUB_TOKEN")); t != "" {
		return t
	}
	if ghToken == nil {
		return ""
	}
	t, err := ghToken()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(t)
}

func GhAuthToken() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	return string(out), err
}
