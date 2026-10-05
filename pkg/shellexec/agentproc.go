// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

package shellexec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/wavetermdev/waveterm/pkg/util/shellutil"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
)

// LocalAgentProcSpec is internal launch data, not a renderer-provided command.
// The service resolves the executable and canonical destination before launch.
type LocalAgentProcSpec struct {
	Executable string
	Args       []string
	Cwd        string
}

// StartLocalAgentProc starts the exact executable in the exact directory. It
// does not invoke a shell, source startup files, or fall back to another cwd.
// CLI home/history and user environment retain their existing values.
func StartLocalAgentProc(spec LocalAgentProcSpec, termSize waveobj.TermSize) (*ShellProc, error) {
	if !filepath.IsAbs(spec.Executable) || !filepath.IsAbs(spec.Cwd) || strings.ContainsRune(spec.Executable+spec.Cwd, 0) {
		return nil, fmt.Errorf("agent requires absolute executable and directory")
	}
	for _, arg := range spec.Args {
		if strings.ContainsRune(arg, 0) {
			return nil, fmt.Errorf("invalid agent argument")
		}
	}
	dir, err := os.Stat(spec.Cwd)
	if err != nil || !dir.IsDir() {
		return nil, fmt.Errorf("agent directory is unavailable")
	}
	canonical, err := filepath.EvalSymlinks(spec.Cwd)
	if err != nil || filepath.Clean(canonical) != filepath.Clean(spec.Cwd) {
		return nil, fmt.Errorf("agent directory must be canonical")
	}
	executable, err := os.Stat(spec.Executable)
	if err != nil || !executable.Mode().IsRegular() || executable.Mode().Perm()&0111 == 0 {
		return nil, fmt.Errorf("agent executable is unavailable")
	}
	if termSize.Rows == 0 && termSize.Cols == 0 {
		termSize = waveobj.TermSize{Rows: shellutil.DefaultTermRows, Cols: shellutil.DefaultTermCols}
	}
	if termSize.Rows <= 0 || termSize.Cols <= 0 || termSize.Rows > 65535 || termSize.Cols > 65535 {
		return nil, fmt.Errorf("invalid agent terminal size")
	}
	cmd := exec.Command(spec.Executable, spec.Args...)
	cmd.Dir = spec.Cwd
	cmd.Env = os.Environ()
	shellutil.UpdateCmdEnv(cmd, map[string]string{"TERM": shellutil.DefaultTermType, "COLORTERM": "truecolor"})
	cmdPty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(termSize.Rows), Cols: uint16(termSize.Cols)})
	if err != nil {
		return nil, fmt.Errorf("agent process could not start")
	}
	processGroupID, _ := AgentProcessGroupID(cmd.Process.Pid)
	return &ShellProc{
		Cmd: MakeCmdWrap(cmd, cmdPty, false), ConnName: "local",
		CloseOnce: &sync.Once{}, DoneCh: make(chan any), AgentProcessGroupID: processGroupID,
	}, nil
}
