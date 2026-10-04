// SPDX-License-Identifier: Apache-2.0
// Package updateguard checks work which would be interrupted by an app restart.
package updateguard

import "fmt"

type Controller struct {
	BlockID    string
	Kind       string
	State      string
	ShellState string
}

type Job struct {
	ID      string
	BlockID string
	Kind    string
	State   string
	PID     int
	ExitTS  int64
}

// A cached ready state alone is insufficient: only a fresh renderer snapshot
// plus the current server state can exempt a running interactive shell.
func Blockers(controllers []Controller, jobs []Job, verifiedIdleBlocks []string) []string {
	verified := make(map[string]bool, len(verifiedIdleBlocks))
	for _, id := range verifiedIdleBlocks {
		verified[id] = true
	}
	idle := make(map[string]bool)
	blockers := make([]string, 0)
	for _, controller := range controllers {
		if controller.State == "done" {
			continue
		}
		if controller.State == "running" && controller.Kind == "shell" && controller.ShellState == "ready" && verified[controller.BlockID] {
			idle[controller.BlockID] = true
			continue
		}
		blockers = append(blockers, fmt.Sprintf("Terminal %s: comando ativo ou estado não confirmado", controller.BlockID))
	}
	for _, job := range jobs {
		if job.ExitTS > 0 || job.State == "done" {
			continue
		}
		if job.Kind == "shell" && job.State == "running" && job.PID > 0 && idle[job.BlockID] {
			continue
		}
		blockers = append(blockers, fmt.Sprintf("Execução %s: finalize ou confirme o estado antes de reiniciar", job.ID))
	}
	return blockers
}
