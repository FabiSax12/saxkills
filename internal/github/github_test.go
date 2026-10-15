package github

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(t *testing.T, gotAuth *[]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	record := func(r *http.Request) { *gotAuth = append(*gotAuth, r.Header.Get("Authorization")) }
	mux.HandleFunc("/repos/FabiSax12/saxkills/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		io.WriteString(w, `{"tag_name":"v1.2.3","tarball_url":"`+"http://"+r.Host+`/tarball/v1.2.3","assets":[{"name":"checksums.txt","url":"http://`+r.Host+`/assets/7","size":10}]}`)
	})
	mux.HandleFunc("/tarball/v1.2.3", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		io.WriteString(w, "TARBALL")
	})
	mux.HandleFunc("/assets/7", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Header.Get("Accept") != "application/octet-stream" {
			http.Error(w, "bad accept", http.StatusBadRequest)
			return
		}
		io.WriteString(w, "ASSET")
	})
	return httptest.NewServer(mux)
}

func TestLatestReleaseAndDownloads(t *testing.T) {
	var auth []string
	srv := newServer(t, &auth)
	defer srv.Close()
	c := &HTTPClient{BaseURL: srv.URL, Repo: "FabiSax12/saxkills", Token: "sekret", HTTP: srv.Client()}

	rel, err := c.LatestRelease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v1.2.3" || len(rel.Assets) != 1 || rel.Assets[0].Name != "checksums.txt" {
		t.Fatalf("unexpected release: %+v", rel)
	}

	body, err := c.DownloadTarball(context.Background(), rel)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(body)
	body.Close()
	if string(data) != "TARBALL" {
		t.Fatalf("tarball body = %q", data)
	}

	body, err = c.DownloadAsset(context.Background(), rel.Assets[0])
	if err != nil {
		t.Fatal(err)
	}
	data, _ = io.ReadAll(body)
	body.Close()
	if string(data) != "ASSET" {
		t.Fatalf("asset body = %q", data)
	}

	for _, a := range auth {
		if a != "Bearer sekret" {
			t.Fatalf("Authorization = %q", a)
		}
	}
}

func TestAnonymousSendsNoAuthorization(t *testing.T) {
	var auth []string
	srv := newServer(t, &auth)
	defer srv.Close()
	c := &HTTPClient{BaseURL: srv.URL, Repo: "FabiSax12/saxkills", HTTP: srv.Client()}
	if _, err := c.LatestRelease(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(auth) != 1 || auth[0] != "" {
		t.Fatalf("auth headers = %v", auth)
	}
}

func TestTokenNotSentToForeignHost(t *testing.T) {
	var foreign []string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreign = append(foreign, r.Header.Get("Authorization"))
	}))
	defer other.Close()
	c := &HTTPClient{BaseURL: "http://127.0.0.1:1", Repo: "x/y", Token: "sekret", HTTP: other.Client()}
	body, err := c.DownloadTarball(context.Background(), Release{TarballURL: other.URL + "/t"})
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
	if len(foreign) != 1 || foreign[0] != "" {
		t.Fatalf("token leaked to foreign host: %v", foreign)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantNF   bool
		contains string
	}{
		{"not found", http.StatusNotFound, true, "not found"},
		{"server error", http.StatusInternalServerError, false, "500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()
			c := &HTTPClient{BaseURL: srv.URL, Repo: "x/y", Token: "sekret", HTTP: srv.Client()}
			_, err := c.LatestRelease(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
			if (err == ErrNotFound || strings.Contains(err.Error(), ErrNotFound.Error())) != tt.wantNF {
				t.Fatalf("not-found mismatch: %v", err)
			}
			if !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("error %q missing %q", err, tt.contains)
			}
			if strings.Contains(err.Error(), "sekret") {
				t.Fatal("error leaks token")
			}
		})
	}
}

func TestResolveToken(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	ghOK := func() (string, error) { return " gh-token\n", nil }
	ghFail := func() (string, error) { return "", io.EOF }
	tests := []struct {
		name string
		env  map[string]string
		gh   func() (string, error)
		want string
	}{
		{"env wins", map[string]string{"GITHUB_TOKEN": "env-token"}, ghOK, "env-token"},
		{"gh fallback", nil, ghOK, "gh-token"},
		{"anonymous", nil, ghFail, ""},
		{"nil gh", nil, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveToken(env(tt.env), tt.gh); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
