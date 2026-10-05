package wshremote

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

func TestRemoteJobConversionCarriesTypedCwdAndLegacyEmpty(t *testing.T) {
	base := t.TempDir()
	cwd := filepath.Join(base, "checkout ü com espaço")
	args := []string{"--resume", "uuid-123", "--append-system-prompt-file", filepath.Join(cwd, "prompt ü.txt")}
	impl := &ServerImpl{InitialEnv: map[string]string{"HOME": "/existing/home", "CLAUDE_CONFIG_DIR": "/existing/history"}}
	data := wshrpc.CommandRemoteStartJobData{Cmd: "/fake/bin/claude", Args: args, Cwd: cwd, Env: map[string]string{"WAVETERM_JOBID": "job-1"}}
	converted := impl.makeCommandStartJobData(data)
	if converted.Cwd != cwd || converted.Cmd != data.Cmd || !reflect.DeepEqual(converted.Args, args) {
		t.Fatalf("remote typed command changed: %#v", converted)
	}
	if converted.Env["HOME"] != "/existing/home" || converted.Env["CLAUDE_CONFIG_DIR"] != "/existing/history" {
		t.Fatal("existing CLI context was not inherited")
	}
	legacy := impl.makeCommandStartJobData(wshrpc.CommandRemoteStartJobData{Cmd: "/fake/bin/sh"})
	if legacy.Cwd != "" {
		t.Fatalf("legacy cwd changed: %q", legacy.Cwd)
	}
}
