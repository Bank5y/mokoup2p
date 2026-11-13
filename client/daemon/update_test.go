package daemon_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Bank5y/mokoup2p/client/daemon"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/client/ipc"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

// release is the part of a GitHub release the daemon reads.
type release struct {
	TagName    string  `json:"tag_name"`
	HTMLURL    string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// fakeGitHub serves releases as GitHub's "list releases" endpoint does and
// returns that endpoint's URL.
func fakeGitHub(t *testing.T, releases ...release) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(releases)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/repos/owner/repo/releases"
}

func publishedRelease(version string) release {
	dl := "https://github.com/owner/repo/releases/download/v" + version + "/"
	return release{
		TagName: "v" + version,
		HTMLURL: "https://github.com/owner/repo/releases/tag/v" + version,
		Assets: []asset{
			{Name: "SHA256SUMS", URL: dl + "SHA256SUMS"},
			{Name: "mokoup2p-" + version + "-windows-x64-setup.exe", URL: dl + "mokoup2p-" + version + "-windows-x64-setup.exe"},
			{Name: "mokoup2p-" + version + "-windows-x64-portable.zip", URL: dl + "mokoup2p-" + version + "-windows-x64-portable.zip"},
			{Name: "mokoup2p-" + version + "-linux-amd64.tar.gz", URL: dl + "mokoup2p-" + version + "-linux-amd64.tar.gz"},
		},
	}
}

func startVersioned(t *testing.T, version string, edition ipc.Edition, releasesURL string) *daemontest.Daemon {
	t.Helper()
	return daemontest.StartWith(t, func(c *daemon.Config) {
		c.Version = version
		c.Edition = edition
		c.ReleasesURL = releasesURL
	})
}

func mustCheck(t *testing.T, d *daemontest.Daemon) ipc.UpdateInfo {
	t.Helper()
	info, err := d.IPC.CheckUpdate(ctx)
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	return info
}

func TestStatusReportsVersionAndEdition(t *testing.T) {
	d := startVersioned(t, "0.1.0", ipc.EditionLinux, "")

	st, err := d.IPC.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if st.Version != "0.1.0" || st.Edition != ipc.EditionLinux {
		t.Fatalf("want version 0.1.0 on linux, got %q %q", st.Version, st.Edition)
	}
}

func TestNewerReleaseIsOfferedWithTheWindowsInstaller(t *testing.T) {
	gh := fakeGitHub(t, publishedRelease("0.2.0"), publishedRelease("0.1.0"))
	d := startVersioned(t, "0.1.0", ipc.EditionWindowsInstaller, gh)

	info := mustCheck(t, d)

	want := ipc.UpdateInfo{
		CurrentVersion: "0.1.0",
		LatestVersion:  "0.2.0",
		Available:      true,
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v0.2.0",
		AssetName:      "mokoup2p-0.2.0-windows-x64-setup.exe",
		AssetURL:       "https://github.com/owner/repo/releases/download/v0.2.0/mokoup2p-0.2.0-windows-x64-setup.exe",
		ChecksumsURL:   "https://github.com/owner/repo/releases/download/v0.2.0/SHA256SUMS",
	}
	if info != want {
		t.Fatalf("want %+v\n got %+v", want, info)
	}
}

func TestPortableEditionIsOfferedThePortableZip(t *testing.T) {
	gh := fakeGitHub(t, publishedRelease("0.2.0"))
	d := startVersioned(t, "0.1.0", ipc.EditionWindowsPortable, gh)

	info := mustCheck(t, d)

	if info.AssetName != "mokoup2p-0.2.0-windows-x64-portable.zip" {
		t.Fatalf("want the portable zip, got %+v", info)
	}
}

func TestLinuxIsOfferedOnlyTheReleasePage(t *testing.T) {
	gh := fakeGitHub(t, publishedRelease("0.2.0"))
	d := startVersioned(t, "0.1.0", ipc.EditionLinux, gh)

	info := mustCheck(t, d)

	if !info.Available || info.AssetURL != "" || info.ReleaseURL == "" {
		t.Fatalf("want an available update with only a release page, got %+v", info)
	}
}

func TestNewestReleaseWinsWhateverTheListOrder(t *testing.T) {
	gh := fakeGitHub(t, publishedRelease("0.9.0"), publishedRelease("0.10.0"), publishedRelease("0.2.0"))
	d := startVersioned(t, "0.1.0", ipc.EditionLinux, gh)

	if info := mustCheck(t, d); info.LatestVersion != "0.10.0" {
		t.Fatalf("want 0.10.0, got %+v", info)
	}
}

func TestPrereleasesAndDraftsAreIgnored(t *testing.T) {
	pre := publishedRelease("0.3.0-rc.1")
	pre.Prerelease = true
	draft := publishedRelease("0.4.0")
	draft.Draft = true
	gh := fakeGitHub(t, draft, pre, publishedRelease("0.1.0"))
	d := startVersioned(t, "0.1.0", ipc.EditionLinux, gh)

	info := mustCheck(t, d)

	if info.Available || info.LatestVersion != "0.1.0" {
		t.Fatalf("want no update, got %+v", info)
	}
}

func TestCurrentVersionIsUpToDate(t *testing.T) {
	gh := fakeGitHub(t, publishedRelease("0.2.0"))
	d := startVersioned(t, "0.2.0", ipc.EditionLinux, gh)

	if info := mustCheck(t, d); info.Available {
		t.Fatalf("want up to date, got %+v", info)
	}
}

func TestDevelopmentBuildIsNeverOfferedAnUpdate(t *testing.T) {
	gh := fakeGitHub(t, publishedRelease("0.2.0"))
	d := startVersioned(t, "dev", ipc.EditionLinux, gh)

	info := mustCheck(t, d)

	if info.Available || info.LatestVersion != "0.2.0" {
		t.Fatalf("want latest reported but no update offered, got %+v", info)
	}
}

func TestUnreachableReleaseFeedIsReported(t *testing.T) {
	d := startVersioned(t, "0.1.0", ipc.EditionLinux, "http://127.0.0.1:1/releases")

	_, err := d.IPC.CheckUpdate(ctx)

	if code := servertest.ErrorCode(err); code != ipc.ErrUpdateCheckFailed {
		t.Fatalf("want %s, got %v", ipc.ErrUpdateCheckFailed, err)
	}
}

func TestUpdateSettingsDefaultToAutomaticCheck(t *testing.T) {
	d := startVersioned(t, "0.1.0", ipc.EditionLinux, "")

	s, err := d.IPC.UpdateSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !s.AutoCheck || s.IgnoredVersion != "" {
		t.Fatalf("want automatic check and nothing ignored, got %+v", s)
	}
}

func TestIgnoredVersionIsRememberedAcrossRestarts(t *testing.T) {
	gh := fakeGitHub(t, publishedRelease("0.2.0"))
	d := startVersioned(t, "0.1.0", ipc.EditionLinux, gh)
	if _, err := d.IPC.SetUpdateSettings(ctx, ipc.UpdateSettings{AutoCheck: false, IgnoredVersion: "0.2.0"}); err != nil {
		t.Fatal(err)
	}

	d.Restart(t)

	s, err := d.IPC.UpdateSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.AutoCheck || s.IgnoredVersion != "0.2.0" {
		t.Fatalf("want settings kept, got %+v", s)
	}
	if info := mustCheck(t, d); !info.Available || !info.Ignored {
		t.Fatalf("want 0.2.0 available but ignored, got %+v", info)
	}
}

func TestNewerReleaseThanTheIgnoredOneIsNotIgnored(t *testing.T) {
	gh := fakeGitHub(t, publishedRelease("0.3.0"))
	d := startVersioned(t, "0.1.0", ipc.EditionLinux, gh)
	if _, err := d.IPC.SetUpdateSettings(ctx, ipc.UpdateSettings{AutoCheck: true, IgnoredVersion: "0.2.0"}); err != nil {
		t.Fatal(err)
	}

	if info := mustCheck(t, d); info.Ignored {
		t.Fatalf("want 0.3.0 not ignored, got %+v", info)
	}
}
