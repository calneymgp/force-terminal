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

type Agent struct {
	BlockID        string
	Title          string
	State          string
	Phase          string
	WriterLeaseKey string
}

// A restored agent can retain an uncertain attempt without a loaded terminal
// controller. Its durable checkpoint and lease must still prevent a restart.
func AgentBlockers(agents []Agent) []string {
	reasons := make([]string, 0)
	for _, agent := range agents {
		settled := agent.Phase == "" || agent.Phase == "prepare_failed"
		if settled && agent.WriterLeaseKey == "" {
			switch agent.State {
			case "prepared", "unavailable", "exited", "resume_failed":
				continue
			}
		}
		title := agent.Title
		if title == "" {
			title = agent.BlockID
		}
		reasons = append(reasons, fmt.Sprintf("Agente %s: sessão ativa ou estado não confirmado", title))
	}
	return reasons
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
