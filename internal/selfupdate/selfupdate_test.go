package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FabiSax12/saxkills/internal/github"
)

type fakeClient struct {
	assets map[string][]byte
}

func (f *fakeClient) LatestRelease(context.Context) (github.Release, error) {
	return github.Release{}, errors.New("unused")
}

func (f *fakeClient) DownloadTarball(context.Context, github.Release) (io.ReadCloser, error) {
	return nil, errors.New("unused")
}

func (f *fakeClient) DownloadAsset(_ context.Context, a github.Asset) (io.ReadCloser, error) {
	data, ok := f.assets[a.Name]
	if !ok {
		return nil, errors.New("no such asset")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func archive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(body))})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func setup(t *testing.T, files map[string]string, checksums func(archive []byte) string) (*fakeClient, github.Release, string) {
	t.Helper()
	const name = "saxkills_linux_amd64.tar.gz"
	data := archive(t, files)
	c := &fakeClient{assets: map[string][]byte{name: data, "checksums.txt": []byte(checksums(data))}}
	rel := github.Release{Tag: "v2", Assets: []github.Asset{{Name: name}, {Name: "checksums.txt"}}}
	exe := filepath.Join(t.TempDir(), "saxkills")
	os.WriteFile(exe, []byte("OLD"), 0o755)
	return c, rel, exe
}

func goodChecksums(data []byte) string {
	return fmt.Sprintf("%s  other.tar.gz\n%s  saxkills_linux_amd64.tar.gz\n", strings.Repeat("0", 64), sum(data))
}

func TestApplyReplacesBinary(t *testing.T) {
	c, rel, exe := setup(t, map[string]string{"saxkills": "NEW", "README.md": "x"}, goodChecksums)
	if err := Apply(context.Background(), c, rel, "linux", "amd64", exe); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(exe)
	info, _ := os.Stat(exe)
	if string(data) != "NEW" || info.Mode()&0o111 == 0 {
		t.Fatalf("content=%q mode=%v", data, info.Mode())
	}
	entries, _ := os.ReadDir(filepath.Dir(exe))
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestApplyFailuresLeaveBinaryUntouched(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		checksums func([]byte) string
		goos      string
		want      string
	}{
		{"checksum mismatch", map[string]string{"saxkills": "NEW"}, func([]byte) string {
			return strings.Repeat("a", 64) + "  saxkills_linux_amd64.tar.gz\n"
		}, "linux", "checksum mismatch"},
		{"checksum entry missing", map[string]string{"saxkills": "NEW"}, func([]byte) string {
			return strings.Repeat("a", 64) + "  other.tar.gz\n"
		}, "linux", "no checksum"},
		{"binary missing in archive", map[string]string{"README.md": "x"}, goodChecksums, "linux", "not found in archive"},
		{"unsupported platform", map[string]string{"saxkills": "NEW"}, goodChecksums, "windows", "no asset"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rel, exe := setup(t, tt.files, tt.checksums)
			err := Apply(context.Background(), c, rel, tt.goos, "amd64", exe)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if data, _ := os.ReadFile(exe); string(data) != "OLD" {
				t.Fatalf("binary modified: %q", data)
			}
			if entries, _ := os.ReadDir(filepath.Dir(exe)); len(entries) != 1 {
				t.Fatalf("temp files left behind: %v", entries)
			}
		})
	}
}

func TestAssetName(t *testing.T) {
	if got := AssetName("darwin", "arm64"); got != "saxkills_darwin_arm64.tar.gz" {
		t.Fatal(got)
	}
}
