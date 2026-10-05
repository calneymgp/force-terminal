package jobmanager

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
)

func TestJobCommandCwdIsTypedAndDoesNotSpawn(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(base, "projeto ü com espaço")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(base, "fake claude")
	if err := os.WriteFile(cli, []byte("not a runnable program"), 0700); err != nil {
		t.Fatal(err)
	}
	argv := []string{"--resume", "uuid-123", "--append-system-prompt-file", filepath.Join(cwd, "prompt ü.txt")}
	data := wshrpc.CommandStartJobData{Cmd: cli, Args: argv, Cwd: cwd}
	cmdDef := makeJobCmdDef(data)
	cmd, err := makeJobExecCommand(cmdDef)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Dir != cwd || cmd.Path != cli || !reflect.DeepEqual(cmd.Args, append([]string{cli}, argv...)) {
		t.Fatalf("typed argv/cwd changed: path=%q args=%q dir=%q", cmd.Path, cmd.Args, cmd.Dir)
	}
	if cmd.Process != nil {
		t.Fatal("builder unexpectedly started a process")
	}
	if _, err := os.Stat(filepath.Join(cwd, "prompt ü.txt")); !os.IsNotExist(err) {
		t.Fatal("builder unexpectedly wrote or ran a process")
	}

	legacy, err := makeJobExecCommand(makeJobCmdDef(wshrpc.CommandStartJobData{Cmd: cli, Args: []string{"legacy"}}))
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Dir != "" {
		t.Fatalf("legacy working directory changed: %q", legacy.Dir)
	}
}

func TestJobCommandRejectsUnsafeCwdBeforeSpawn(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(base, "valid")
	if err := os.Mkdir(valid, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(valid, alias); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, cwd := range []string{"relative", "bad\x00path", "bad\rpath", "bad\npath", alias, file, filepath.Join(base, "missing")} {
		if _, err := makeJobExecCommand(CmdDef{Cmd: filepath.Join(base, "fake"), Cwd: cwd}); err == nil {
			t.Fatalf("unsafe cwd accepted: %q", cwd)
		}
	}
	if _, err := MakeJobCmd("fake-job", CmdDef{Cmd: filepath.Join(base, "fake"), Cwd: alias, TermSize: waveobj.TermSize{Rows: 24, Cols: 80}}); err == nil || !strings.Contains(err.Error(), "working directory") {
		t.Fatalf("MakeJobCmd did not reject noncanonical cwd before PTY start: %v", err)
	}
}
