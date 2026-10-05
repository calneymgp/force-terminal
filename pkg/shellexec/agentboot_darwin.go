// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package shellexec

import (
	"fmt"
	"strings"
	"syscall"
)

func CurrentAgentBootID() (string, error) {
	bootID, err := syscall.Sysctl("kern.bootsessionuuid")
	if err != nil {
		return "", err
	}
	bootID = strings.TrimSpace(bootID)
	if bootID == "" {
		return "", fmt.Errorf("kernel boot session uuid is empty")
	}
	return bootID, nil
}
