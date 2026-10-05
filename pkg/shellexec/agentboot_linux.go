// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package shellexec

import (
	"fmt"
	"os"
	"strings"
)

func CurrentAgentBootID() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	bootID := strings.TrimSpace(string(data))
	if bootID == "" {
		return "", fmt.Errorf("kernel boot id is empty")
	}
	return bootID, nil
}
