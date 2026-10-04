package wavebase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemotePathsStayInForceHome(t *testing.T) {
	if RemoteFullWshBinPath != "~/.force-terminal/bin/wsh" {
		t.Fatalf("remote binary: %s", RemoteFullWshBinPath)
	}
	if RemoteFullDomainSocketPath != "~/.force-terminal/wave-remote.sock" {
		t.Fatalf("remote socket: %s", RemoteFullDomainSocketPath)
	}
	if got := GetPersistentRemoteSockName("client1"); got != "~/.force-terminal/client/client1/waveterm.sock" {
		t.Fatalf("client socket: %s", got)
	}
	if got := filepath.Join("/home/test", RemoteWaveHomeDirName, "jobs"); got != "/home/test/.force-terminal/jobs" {
		t.Fatalf("jobs: %s", got)
	}
	if got := GetRemoteJobSocketPath("job1"); got != fmt.Sprintf("/tmp/force-terminal-%d/job1.sock", os.Getuid()) {
		t.Fatalf("job socket: %s", got)
	}
	if got := GetRemoteConnectionSocketPath("a1b2"); got != "/tmp/force-terminal-a1b2.sock" {
		t.Fatalf("connection socket: %s", got)
	}
	if got := GetRemoteConnServerLogPath(1000); got != "/tmp/force-terminal-connserver-1000.log" {
		t.Fatalf("connection server log: %s", got)
	}
}

func TestCacheDefaultAndOverrideAreForceScoped(t *testing.T) {
	t.Setenv("HOME", "/tmp/force-test-home")
	t.Setenv("XDG_CACHE_HOME", "/tmp/force-test-cache")
	previousCache := CacheHome_VarCache
	previousDev := Dev_VarCache
	t.Cleanup(func() { CacheHome_VarCache = previousCache; Dev_VarCache = previousDev })
	CacheHome_VarCache = ""
	Dev_VarCache = ""
	if got := resolveWaveCachesDir(); !strings.Contains(got, "force-terminal") || strings.Contains(got, "waveterm") {
		t.Fatalf("production cache: %s", got)
	}
	Dev_VarCache = "1"
	if got := resolveWaveCachesDir(); !strings.Contains(got, "force-terminal-dev") {
		t.Fatalf("development cache: %s", got)
	}
	CacheHome_VarCache = "/tmp/force-test-override"
	if got := resolveWaveCachesDir(); got != "/tmp/force-test-override" {
		t.Fatalf("override cache: %s", got)
	}
}
