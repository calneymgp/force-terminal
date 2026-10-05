// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package shellexec

import "testing"

func TestCurrentAgentBootIDIsStableAndNonEmpty(t *testing.T) {
	first, err := CurrentAgentBootID()
	if err != nil || first == "" {
		t.Fatalf("boot identity=%q err=%v", first, err)
	}
	second, err := CurrentAgentBootID()
	if err != nil || second != first {
		t.Fatalf("boot identity changed between reads: first=%q second=%q err=%v", first, second, err)
	}
}
