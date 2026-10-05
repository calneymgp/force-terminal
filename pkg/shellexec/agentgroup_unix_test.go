// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package shellexec

import (
	"os"
	"syscall"
	"testing"
)

func TestInspectAgentProcessGroupUsesKernelExistence(t *testing.T) {
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	state, err := InspectAgentProcessGroup(pgid)
	if err != nil || state != AgentProcessGroupPresent {
		t.Fatalf("current process group state=%v err=%v", state, err)
	}
	state, err = InspectAgentProcessGroup(2147483647)
	if err != nil || state != AgentProcessGroupAbsent {
		t.Fatalf("unused process group state=%v err=%v", state, err)
	}
	actualPGID, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	actual, err := AgentProcessGroupID(os.Getpid())
	if actualPGID == os.Getpid() {
		if err != nil || actual != actualPGID {
			t.Fatalf("group leader proof=%d err=%v, want %d", actual, err, actualPGID)
		}
	} else if err == nil {
		t.Fatalf("non-leader PID was accepted as PGID %d", actual)
	}
}
