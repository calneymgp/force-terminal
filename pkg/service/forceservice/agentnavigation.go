// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

package forceservice

import (
	"context"
	"fmt"
	"time"

	"github.com/wavetermdev/waveterm/pkg/tsgen/tsgenmeta"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wcore"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
	"github.com/wavetermdev/waveterm/pkg/wshutil"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

type agentNavigation struct {
	WindowID    string
	WorkspaceID string
	TabID       string
	BlockID     string
}

func (svc *ForceService) FocusAgentTerminal_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "instanceID"}}
}

// Focus is navigation only. It uses the original terminal and workspace and
// never invokes Start, Reconnect, or a generic terminal restart.
func (svc *ForceService) FocusAgentTerminal(ctx context.Context, instanceID string) (waveobj.UpdatesRtnType, error) {
	return focusAgentTerminal(ctx, instanceID, navigateAgentTerminal)
}

func focusAgentTerminal(ctx context.Context, instanceID string, navigate func(context.Context, agentNavigation) error) (waveobj.UpdatesRtnType, error) {
	if !validAgentID(instanceID) {
		return nil, fmt.Errorf("invalid agent identity")
	}
	ctx = waveobj.ContextWithUpdates(ctx)
	nav, err := wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (agentNavigation, error) {
		v, err := wstore.DBMustGet[*waveobj.ForceAgentInstance](tx.Context(), instanceID)
		if err != nil {
			return agentNavigation{}, err
		}
		block, err := wstore.DBMustGet[*waveobj.Block](tx.Context(), v.BlockID)
		if err != nil || block.Meta.GetString("force:agentinstanceid", "") != v.OID || block.ParentORef != waveobj.MakeORef(waveobj.OType_Tab, v.TabID).String() {
			return agentNavigation{}, fmt.Errorf("agent terminal is unavailable")
		}
		tab, err := wstore.DBMustGet[*waveobj.Tab](tx.Context(), v.TabID)
		if err != nil {
			return agentNavigation{}, fmt.Errorf("agent tab is unavailable")
		}
		layout, err := wstore.DBMustGet[*waveobj.LayoutState](tx.Context(), tab.LayoutState)
		if err != nil {
			return agentNavigation{}, fmt.Errorf("agent layout is unavailable")
		}
		workspaceID, err := wstore.DBFindWorkspaceForTabId(tx.Context(), tab.OID)
		if err != nil || workspaceID == "" {
			return agentNavigation{}, fmt.Errorf("agent workspace is unavailable")
		}
		workspace, err := wstore.DBMustGet[*waveobj.Workspace](tx.Context(), workspaceID)
		if err != nil {
			return agentNavigation{}, err
		}
		if !agentLayoutHasBlock(layout, block.OID) {
			if err := wcore.QueueLayoutAction(tx.Context(), layout.OID, waveobj.LayoutActionData{ActionType: wcore.LayoutActionDataType_Insert, BlockId: block.OID, Focused: true}); err != nil {
				return agentNavigation{}, err
			}
		}
		workspace.ActiveTabId = tab.OID
		if workspace.Meta == nil {
			workspace.Meta = make(waveobj.MetaMapType)
		}
		workspace.Meta["force:projectid"] = v.ProjectID
		if err := wstore.DBUpdate(tx.Context(), workspace); err != nil {
			return agentNavigation{}, err
		}
		windowID, err := wstore.DBFindWindowForWorkspaceId(tx.Context(), workspaceID)
		if err != nil {
			return agentNavigation{}, err
		}
		if windowID == "" {
			window, err := wcore.CreateWindow(tx.Context(), nil, workspaceID)
			if err != nil {
				return agentNavigation{}, err
			}
			windowID = window.OID
		}
		return agentNavigation{WindowID: windowID, WorkspaceID: workspaceID, TabID: tab.OID, BlockID: block.OID}, nil
	})
	if err != nil {
		return nil, err
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	if err := navigate(ctx, nav); err != nil {
		return updates, fmt.Errorf("agent terminal navigation failed; try opening it again")
	}
	return updates, nil
}

func agentLayoutHasBlock(layout *waveobj.LayoutState, blockID string) bool {
	if layout.LeafOrder != nil {
		for _, leaf := range *layout.LeafOrder {
			if leaf.BlockId == blockID {
				return true
			}
		}
	}
	if layout.PendingBackendActions != nil {
		for _, action := range *layout.PendingBackendActions {
			if action.BlockId == blockID && (action.ActionType == wcore.LayoutActionDataType_Insert || action.ActionType == wcore.LayoutActionDataType_InsertAtIndex) {
				return true
			}
		}
	}
	return agentTreeHasBlock(layout.RootNode, blockID)
}

func agentTreeHasBlock(node any, blockID string) bool {
	switch value := node.(type) {
	case map[string]any:
		if value["blockId"] == blockID || value["blockid"] == blockID {
			return true
		}
		for _, child := range value {
			if agentTreeHasBlock(child, blockID) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if agentTreeHasBlock(child, blockID) {
				return true
			}
		}
	}
	return false
}

func navigateAgentTerminal(ctx context.Context, nav agentNavigation) error {
	client := wshclient.GetBareRpcClient()
	if err := wshclient.FocusWindowCommand(client, nav.WindowID, &wshrpc.RpcOpts{Route: wshutil.ElectronRoute, Timeout: 10000}); err != nil {
		return err
	}
	wcore.SendActiveTabUpdate(ctx, nav.WorkspaceID, nav.TabID)
	route := wshutil.MakeTabRouteId(nav.TabID)
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := wshutil.DefaultRouter.WaitForRegister(waitCtx, route); err != nil {
		return err
	}
	return wshclient.SetBlockFocusCommand(client, nav.BlockID, &wshrpc.RpcOpts{Route: route, Timeout: 5000})
}
