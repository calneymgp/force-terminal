// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package shellexec

import (
	"fmt"
	"syscall"
)

type AgentProcessGroupState int

const (
	AgentProcessGroupUnknown AgentProcessGroupState = iota
	AgentProcessGroupAbsent
	AgentProcessGroupPresent
)

// AgentProcessGroupID reports the process group established for the PTY
// session. Agent launch requires the child to be its own session/group leader.
func AgentProcessGroupID(pid int) (int, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid agent process id")
	}
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return 0, err
	}
	if pgid != pid {
		return 0, fmt.Errorf("agent process is not its process group leader")
	}
	return pgid, nil
}

// InspectAgentProcessGroup treats any response other than ESRCH as evidence
// that the numeric group may still exist. Callers must never signal this PGID:
// Stop uses only the direct child process handle to avoid a stale-group race.
func InspectAgentProcessGroup(pgid int) (AgentProcessGroupState, error) {
	if pgid <= 0 {
		return AgentProcessGroupUnknown, fmt.Errorf("invalid agent process group id")
	}
	err := syscall.Kill(-pgid, 0)
	if err == nil || err == syscall.EPERM {
		return AgentProcessGroupPresent, nil
	}
	if err == syscall.ESRCH {
		return AgentProcessGroupAbsent, nil
	}
	return AgentProcessGroupUnknown, err
}
