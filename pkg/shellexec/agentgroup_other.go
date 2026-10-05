// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package shellexec

import "fmt"

type AgentProcessGroupState int

const (
	AgentProcessGroupUnknown AgentProcessGroupState = iota
	AgentProcessGroupAbsent
	AgentProcessGroupPresent
)

func AgentProcessGroupID(int) (int, error) {
	return 0, fmt.Errorf("agent process groups are unsupported on this platform")
}

func InspectAgentProcessGroup(int) (AgentProcessGroupState, error) {
	return AgentProcessGroupUnknown, fmt.Errorf("agent process groups are unsupported on this platform")
}

func CurrentAgentBootID() (string, error) {
	return "", fmt.Errorf("agent boot identity is unsupported on this platform")
}
