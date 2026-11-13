package updater_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Bank5y/mokoup2p/client/updater"
)

const assetName = "mokoup2p-0.2.0-windows-x64-setup.exe"

var installer = []byte("pretend this is an installer")

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// release serves the installer and a SHA256SUMS file with the given body.
func release(t *testing.T, sums string) (assetURL, sumsURL string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/"+assetName, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(installer) })
	mux.HandleFunc("/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(sums)) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL + "/" + assetName, srv.URL + "/SHA256SUMS"
}

func TestVerifiedDownloadIsSavedUnderItsName(t *testing.T) {
	assetURL, sumsURL := release(t,
		sum([]byte("other"))+"  mokoup2p-0.2.0-linux-amd64.tar.gz\n"+
			sum(installer)+"  "+assetName+"\n")
	dir := t.TempDir()

	path, err := updater.Download(context.Background(), updater.Asset{Name: assetName, URL: assetURL, ChecksumsURL: sumsURL}, dir)
	if err != nil {
		t.Fatalf("download: %v", err)
	}

	if path != filepath.Join(dir, assetName) {
		t.Fatalf("unexpected path %s", path)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(installer) {
		t.Fatalf("want the installer saved, got %q %v", got, err)
	}
}

func TestChecksumMismatchIsRejectedAndNothingIsLeft(t *testing.T) {
	assetURL, sumsURL := release(t, sum([]byte("tampered"))+"  "+assetName+"\n")
	dir := t.TempDir()

	_, err := updater.Download(context.Background(), updater.Asset{Name: assetName, URL: assetURL, ChecksumsURL: sumsURL}, dir)

	if !errors.Is(err, updater.ErrChecksumMismatch) {
		t.Fatalf("want checksum mismatch, got %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("want nothing left behind, got %v", entries)
	}
}

func TestAssetMissingFromChecksumsIsRejected(t *testing.T) {
	assetURL, sumsURL := release(t, sum(installer)+"  something-else.exe\n")
	dir := t.TempDir()

	_, err := updater.Download(context.Background(), updater.Asset{Name: assetName, URL: assetURL, ChecksumsURL: sumsURL}, dir)

	if !errors.Is(err, updater.ErrNotInChecksums) {
		t.Fatalf("want not-in-checksums, got %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("want nothing left behind, got %v", entries)
	}
}
