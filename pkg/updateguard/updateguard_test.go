// SPDX-License-Identifier: Apache-2.0
package updateguard

import "testing"

func TestControllerBlockers(t *testing.T) {
	for _, tc := range []struct {
		name       string
		controller Controller
		verified   []string
		blocked    bool
	}{
		{"idle shell with fresh renderer confirmation", Controller{"a", "shell", "running", "ready"}, []string{"a"}, false},
		{"stale idle shell in unloaded tab", Controller{"a", "shell", "running", "ready"}, nil, true},
		{"active command despite renderer confirmation", Controller{"a", "shell", "running", "running-command"}, []string{"a"}, true},
		{"unknown integration", Controller{"a", "shell", "running", ""}, []string{"a"}, true},
		{"command controller cannot claim idle", Controller{"a", "cmd", "running", "ready"}, []string{"a"}, true},
		{"starting controller", Controller{"a", "shell", "init", "ready"}, []string{"a"}, true},
		{"finished controller", Controller{"a", "cmd", "done", ""}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Blockers([]Controller{tc.controller}, nil, tc.verified)
			if (len(got) > 0) != tc.blocked {
				t.Fatalf("blocked=%v, reasons=%v", tc.blocked, got)
			}
		})
	}
}

func TestJobsIncludeUnattachedAndStartingWork(t *testing.T) {
	jobs := []Job{
		{ID: "task", Kind: "task", State: "running", PID: 20},
		{ID: "starting", Kind: "task", State: "init"},
		{ID: "unknown-shell", Kind: "shell", State: "running", PID: 30},
		{ID: "idle-shell", BlockID: "a", Kind: "shell", State: "running", PID: 40},
		{ID: "done", Kind: "task", State: "done", PID: 50, ExitTS: 123},
	}
	got := Blockers([]Controller{{"a", "shell", "running", "ready"}}, jobs, []string{"a"})
	if len(got) != 3 {
		t.Fatalf("expected active task, starting task and unattached shell blockers; got %v", got)
	}
}

func TestUnknownVerifiedIdDoesNotExemptJob(t *testing.T) {
	got := Blockers(nil, []Job{{ID: "shell", BlockID: "missing", Kind: "shell", State: "running", PID: 40}}, []string{"missing"})
	if len(got) != 1 {
		t.Fatalf("unverified controller must block: %v", got)
	}
}
