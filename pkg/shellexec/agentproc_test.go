// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

package shellexec

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/waveobj"
)

func TestLocalAgentProcUsesExactArgvAndCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("first agent target is macOS arm64")
	}
	dir := filepath.Join(t.TempDir(), "projeto com espaços 🌙")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "fake claude")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf 'ARG:%s\\nCWD:%s\\n' \"$1\" \"$PWD\"\nIFS= read -r line\nprintf 'INPUT:%s\\n' \"$line\"\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	literal := "space ' quote ; $(must-never-run) 🌙"
	proc, err := StartLocalAgentProc(LocalAgentProcSpec{Executable: executable, Args: []string{literal}, Cwd: dir}, waveobj.TermSize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { proc.Close() })
	if err := proc.Cmd.SetSize(100, 32); err != nil {
		t.Fatal(err)
	}
	if _, err := proc.Cmd.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	readDone := make(chan string, 1)
	go func() {
		var output strings.Builder
		buf := make([]byte, 1024)
		for {
			n, err := proc.Cmd.Read(buf)
			output.Write(buf[:n])
			if err != nil {
				readDone <- output.String()
				return
			}
		}
	}()
	select {
	case output := <-readDone:
		for _, expected := range []string{"ARG:" + literal, "CWD:" + dir, "INPUT:hello"} {
			if !strings.Contains(output, expected) {
				t.Fatalf("fake output missing %q: %q", expected, output)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fake PTY output timed out")
	}
	if err := proc.Cmd.Wait(); err == nil || proc.Cmd.ExitCode() != 7 {
		t.Fatalf("expected observed exit 7, got err=%v code=%d", err, proc.Cmd.ExitCode())
	}
}

func TestLocalAgentProcRejectsInvalidDestinationWithoutFallback(t *testing.T) {
	root := t.TempDir()
	cases := []LocalAgentProcSpec{
		{Executable: "/bin/sh", Cwd: filepath.Join(root, "missing")},
		{Executable: "/bin/sh", Cwd: "relative"},
		{Executable: "sh", Cwd: root},
		{Executable: "/bin/sh", Args: []string{"bad\x00arg"}, Cwd: root},
	}
	for _, spec := range cases {
		if proc, err := StartLocalAgentProc(spec, waveobj.TermSize{Rows: 24, Cols: 80}); err == nil {
			proc.Close()
			t.Fatalf("invalid destination/argv launched a process: %#v", spec)
		}
	}
}
