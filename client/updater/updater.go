// Package updater downloads a release asset and checks it against the
// release's SHA256SUMS before anything runs it.
//
// The checksums come from the same release as the asset, so they catch a
// corrupt or truncated download, not a tampered release; signing is a
// separate concern (see #16, out of scope).
package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrChecksumMismatch = errors.New("downloaded file does not match SHA256SUMS")
	ErrNotInChecksums   = errors.New("file is not listed in SHA256SUMS")
)

// Asset is one file of a release.
type Asset struct {
	Name         string
	URL          string
	ChecksumsURL string
}

// Download saves a into dir as a.Name and returns its path, only once its
// SHA-256 matches SHA256SUMS; otherwise nothing is left in dir.
func Download(ctx context.Context, a Asset, dir string) (string, error) {
	if a.Name == "" || strings.ContainsAny(a.Name, `/\`) {
		return "", fmt.Errorf("invalid asset name %q", a.Name)
	}
	want, err := expectedSum(ctx, a)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, a.Name+".*.part")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	err = fetch(ctx, a.URL, io.MultiWriter(tmp, h))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return "", ErrChecksumMismatch
	}
	path := filepath.Join(dir, a.Name)
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// expectedSum reads a's line from SHA256SUMS ("<hex>  <name>", as written
// by sha256sum; a "*" before the name marks binary mode).
func expectedSum(ctx context.Context, a Asset) (string, error) {
	var buf strings.Builder
	if err := fetch(ctx, a.ChecksumsURL, &buf); err != nil {
		return "", err
	}
	sc := bufio.NewScanner(strings.NewReader(buf.String()))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == a.Name {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", ErrNotInChecksums
}

func fetch(ctx context.Context, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}
