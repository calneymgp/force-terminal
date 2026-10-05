package wstore

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/wavetermdev/waveterm/pkg/waveobj"
)

const forceAgentInstanceKey = "force:agentinstanceid"

var ErrForceAgentProtected = errors.New("Force agent context is protected")

// A durable agent owns its block and tab. Generic object operations must not
// change or delete those identities; the Force service owns their lifecycle.
func ForceAgentForBlock(ctx context.Context, blockID string) (string, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (string, error) {
		id := tx.GetString("SELECT oid FROM db_forceagentinstance WHERE json_extract(data, '$.blockid') = ? LIMIT 1", blockID)
		block, err := DBGet[*waveobj.Block](tx.Context(), blockID)
		if err != nil {
			return "", err
		}
		if block != nil && block.Meta.GetString(forceAgentInstanceKey, "") != "" {
			return block.Meta.GetString(forceAgentInstanceKey, ""), nil
		}
		return id, nil
	})
}

func RejectForceBlockDeletion(ctx context.Context, blockID string) error {
	id, err := ForceAgentForBlock(ctx, blockID)
	if err != nil {
		return err
	}
	if id != "" {
		return fmt.Errorf("%w: terminal cannot be deleted by a generic action", ErrForceAgentProtected)
	}
	return nil
}

func RejectForceTabDeletion(ctx context.Context, tabID string) error {
	return WithTx(ctx, func(tx *TxWrap) error {
		id := tx.GetString("SELECT oid FROM db_forceagentinstance WHERE json_extract(data, '$.tabid') = ? LIMIT 1", tabID)
		if id != "" {
			return fmt.Errorf("%w: tab cannot be deleted by a generic action", ErrForceAgentProtected)
		}
		tab, err := DBGet[*waveobj.Tab](tx.Context(), tabID)
		if err != nil {
			return err
		}
		if tab != nil {
			for _, blockID := range tab.BlockIds {
				if err := RejectForceBlockDeletion(tx.Context(), blockID); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func RejectForceWorkspaceDeletion(ctx context.Context, workspaceID string) error {
	ws, err := DBGet[*waveobj.Workspace](ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws == nil {
		return nil
	}
	for _, tabID := range ws.TabIds {
		if err := RejectForceTabDeletion(ctx, tabID); err != nil {
			return err
		}
	}
	return nil
}

func RejectGenericForceBlockCreate(meta waveobj.MetaMapType) error {
	for key := range meta {
		if strings.HasPrefix(key, "force:") {
			return fmt.Errorf("Force agent binding cannot be copied into a generic block")
		}
	}
	return nil
}

func protectedForceMetaKey(key string) bool {
	return strings.HasPrefix(key, "force:") || strings.HasPrefix(key, "cmd:") ||
		key == waveobj.MetaKey_View || key == waveobj.MetaKey_Controller ||
		key == waveobj.MetaKey_Connection || key == "file"
}

func rejectForceBlockMetaChange(ctx context.Context, old *waveobj.Block, next waveobj.MetaMapType) error {
	id, err := ForceAgentForBlock(ctx, old.OID)
	if err != nil {
		return err
	}
	if id == "" {
		for key := range next {
			if strings.HasPrefix(key, "force:") {
				return fmt.Errorf("Force agent binding cannot be assigned by a generic action")
			}
		}
		return nil
	}
	for key, oldValue := range old.Meta {
		if protectedForceMetaKey(key) && !reflect.DeepEqual(oldValue, next[key]) {
			return fmt.Errorf("Force agent terminal destination and binding are immutable")
		}
	}
	for key, nextValue := range next {
		if protectedForceMetaKey(key) && !reflect.DeepEqual(nextValue, old.Meta[key]) {
			return fmt.Errorf("Force agent terminal destination and binding are immutable")
		}
	}
	return nil
}

// ValidateGenericObjectUpdate is for public whole-object updates. Internal
// Force service transactions intentionally use DBUpdate directly.
func ValidateGenericObjectUpdate(ctx context.Context, next waveobj.WaveObj) error {
	switch v := next.(type) {
	case *waveobj.ForceAgentInstance:
		return fmt.Errorf("Force agent instance cannot be updated by a generic action")
	case *waveobj.Block:
		old, err := DBMustGet[*waveobj.Block](ctx, v.OID)
		if err != nil {
			return err
		}
		if err := rejectForceBlockMetaChange(ctx, old, v.Meta); err != nil {
			return err
		}
		id, err := ForceAgentForBlock(ctx, v.OID)
		if err != nil {
			return err
		}
		if id != "" && (old.ParentORef != v.ParentORef || old.JobId != v.JobId || !reflect.DeepEqual(old.SubBlockIds, v.SubBlockIds)) {
			return fmt.Errorf("Force agent terminal structure is immutable")
		}
	case *waveobj.Tab:
		old, err := DBMustGet[*waveobj.Tab](ctx, v.OID)
		if err != nil {
			return err
		}
		if old.LayoutState != v.LayoutState {
			if err := RejectForceTabDeletion(ctx, v.OID); err != nil {
				return err
			}
		}
		for _, blockID := range old.BlockIds {
			if !containsID(v.BlockIds, blockID) {
				if err := RejectForceBlockDeletion(ctx, blockID); err != nil {
					return err
				}
			}
		}
	case *waveobj.Workspace:
		old, err := DBMustGet[*waveobj.Workspace](ctx, v.OID)
		if err != nil {
			return err
		}
		for _, tabID := range old.TabIds {
			if !containsID(v.TabIds, tabID) {
				if err := RejectForceTabDeletion(ctx, tabID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func containsID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}
