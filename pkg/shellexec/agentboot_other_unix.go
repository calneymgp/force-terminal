// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build unix && !linux && !darwin

package shellexec

import "fmt"

func CurrentAgentBootID() (string, error) {
	return "", fmt.Errorf("agent boot identity is unsupported on this platform")
}
