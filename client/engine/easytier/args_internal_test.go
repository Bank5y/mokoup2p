package easytier

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Bank5y/mokoup2p/client/engine"
)

var testConfig = engine.Config{
	NetworkName: "mokoup2p-0123456789abcdef",
	Address:     netip.MustParsePrefix("10.208.3.1/24"),
	Hostname:    "pc",
}

func TestBroadcastRelayIsOnlyRequestedWhenEnabled(t *testing.T) {
	on := New(Options{UDPBroadcastRelay: true}).args(testConfig)
	off := New(Options{}).args(testConfig)

	if i := slices.Index(on, "--enable-udp-broadcast-relay"); i < 0 || on[i+1] != "true" {
		t.Fatalf("want --enable-udp-broadcast-relay true, got %q", on)
	}
	if slices.Contains(off, "--enable-udp-broadcast-relay") {
		t.Fatalf("relay requested although disabled: %q", off)
	}
}

func TestDLLSearchDirsArePrependedToThePath(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")

	env := New(Options{DLLSearchDirs: []string{`C:\Windows\System32\Npcap`}}).env(testConfig)

	want := "PATH=" + `C:\Windows\System32\Npcap` + string(os.PathListSeparator) + "/usr/bin"
	if !slices.Contains(env, want) {
		t.Fatalf("want %q in %q", want, env)
	}
}

func TestNoDLLSearchDirsLeavesThePathAlone(t *testing.T) {
	env := New(Options{}).env(testConfig)

	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			t.Fatalf("PATH overridden without DLLSearchDirs: %q", kv)
		}
	}
}

func TestMissingRequiredLibraryIsReportedBeforeStarting(t *testing.T) {
	dir := t.TempDir()
	e := New(Options{CorePath: filepath.Join(dir, "easytier-core"), RequiredLibraries: []string{"Packet.dll"}})

	err := e.Start(t.Context(), testConfig)

	if !errors.Is(err, ErrLibraryMissing) {
		t.Fatalf("want ErrLibraryMissing, got %v", err)
	}
}

func TestRequiredLibraryFoundInASearchDir(t *testing.T) {
	core, lib := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(lib, "Packet.dll"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(Options{CorePath: filepath.Join(core, "easytier-core"), RequiredLibraries: []string{"Packet.dll"}, DLLSearchDirs: []string{lib}})

	if err := e.checkLibraries(); err != nil {
		t.Fatalf("library in a search dir should be found: %v", err)
	}
}
