// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

package forceservice

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

func TestAgentFocusUsesOriginalWorkspaceTabAndBlockWithoutDuplicateInsertion(t *testing.T) {
	ctx, svc, project, profile, tab := agentFixture(t, t.TempDir())
	v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID, CreationKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	workspace := &waveobj.Workspace{OID: uuid.NewString(), TabIds: []string{tab.OID}, ActiveTabId: ""}
	if err := wstore.DBInsert(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	window := &waveobj.Window{OID: uuid.NewString(), WorkspaceId: workspace.OID}
	if err := wstore.DBInsert(ctx, window); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		_, err := focusAgentTerminal(ctx, v.OID, func(_ context.Context, nav agentNavigation) error {
			if nav.WindowID != window.OID || nav.WorkspaceID != workspace.OID || nav.TabID != tab.OID || nav.BlockID != v.BlockID {
				t.Fatalf("navigation changed persistent destination: %#v", nav)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	layout, err := wstore.DBMustGet[*waveobj.LayoutState](ctx, tab.LayoutState)
	if err != nil || layout.PendingBackendActions == nil || len(*layout.PendingBackendActions) != 1 {
		t.Fatalf("focus queued duplicate terminal insertions: %#v %v", layout, err)
	}
	current, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](ctx, v.OID)
	if err != nil || current.Generation != 0 || current.AttemptID != "" || current.Status != "prepared" {
		t.Fatal("focusing an agent changed or launched its execution")
	}
	workspace, _ = wstore.DBMustGet[*waveobj.Workspace](ctx, workspace.OID)
	if workspace.ActiveTabId != tab.OID {
		t.Fatal("original workspace did not select the original tab")
	}
}

func TestAgentFocusMissingTerminalFailsWithoutRecreatingOrLaunching(t *testing.T) {
	ctx, svc, project, profile, tab := agentFixture(t, t.TempDir())
	v, _, err := svc.CreateAgentInstance(ctx, ForceAgentInstanceInput{ProjectID: project.OID, ProjectVersion: project.Version, ProfileID: profile.OID, ProfileVersion: profile.Version, TabID: tab.OID})
	if err != nil {
		t.Fatal(err)
	}
	if err := wstore.DBDelete(ctx, waveobj.OType_Block, v.BlockID); err != nil {
		t.Fatal(err)
	}
	navigated := false
	if _, err := focusAgentTerminal(ctx, v.OID, func(_ context.Context, _ agentNavigation) error { navigated = true; return nil }); err == nil {
		t.Fatal("missing terminal was silently replaced")
	}
	if navigated {
		t.Fatal("invalid binding navigated to another terminal")
	}
}
