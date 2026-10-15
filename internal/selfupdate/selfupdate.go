package selfupdate

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/FabiSax12/saxkills/internal/github"
)

const (
	binaryName    = "saxkills"
	checksumsName = "checksums.txt"
	maxArchive    = 200 << 20
)

func AssetName(goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s.tar.gz", binaryName, goos, goarch)
}

func Apply(ctx context.Context, c github.Client, rel github.Release, goos, goarch, exePath string) error {
	name := AssetName(goos, goarch)
	asset, ok := rel.Asset(name)
	if !ok {
		return fmt.Errorf("selfupdate: no asset %s in release %s", name, rel.Tag)
	}
	sumsAsset, ok := rel.Asset(checksumsName)
	if !ok {
		return fmt.Errorf("selfupdate: no %s in release %s", checksumsName, rel.Tag)
	}

	sums, err := download(ctx, c, sumsAsset, 1<<20)
	if err != nil {
		return err
	}
	want, err := findChecksum(sums, name)
	if err != nil {
		return err
	}
	archive, err := download(ctx, c, asset, maxArchive)
	if err != nil {
		return err
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("selfupdate: checksum mismatch for %s", name)
	}

	binary, err := extractBinary(archive)
	if err != nil {
		return err
	}
	return replaceFile(exePath, binary)
}

func download(ctx context.Context, c github.Client, a github.Asset, max int64) ([]byte, error) {
	body, err := c.DownloadAsset(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: download %s: %w", a.Name, err)
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, max+1))
	if err != nil {
		return nil, fmt.Errorf("selfupdate: read %s: %w", a.Name, err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("selfupdate: %s exceeds size cap", a.Name)
	}
	return data, nil
}

func findChecksum(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("selfupdate: no checksum listed for %s", name)
}

func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("selfupdate: open archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("selfupdate: %s not found in archive", binaryName)
		}
		if err != nil {
			return nil, fmt.Errorf("selfupdate: read archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != binaryName {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxArchive+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxArchive {
			return nil, fmt.Errorf("selfupdate: binary exceeds size cap")
		}
		return data, nil
	}
}

func replaceFile(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".saxkills-self-*")
	if err != nil {
		return fmt.Errorf("selfupdate: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}
