// SPDX-License-Identifier: Apache-2.0
package forceservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/wavetermdev/waveterm/pkg/remote"
	"github.com/wavetermdev/waveterm/pkg/remote/conncontroller"
	"github.com/wavetermdev/waveterm/pkg/tsgen/tsgenmeta"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

var ErrConflict = errors.New("catalog version conflict")

type ForceService struct{}

type ForceCatalog struct {
	Projects []*waveobj.ForceProject      `json:"projects"`
	Profiles []*waveobj.ForceAgentProfile `json:"profiles"`
}

type ForceProjectInput struct {
	ID              string `json:"id"`
	ExpectedVersion int    `json:"expectedversion"`
	CreationKey     string `json:"creationkey"`
	Name            string `json:"name"`
	Icon            string `json:"icon"`
	Connection      string `json:"connection"`
	RootPath        string `json:"rootpath"`
}

type ForceProfileInput struct {
	ID              string `json:"id"`
	ExpectedVersion int    `json:"expectedversion"`
	CreationKey     string `json:"creationkey"`
	Title           string `json:"title"`
	Icon            string `json:"icon"`
	SystemPrompt    string `json:"systemprompt"`
	Adapter         string `json:"adapter"`
}

var allowedIcons = map[string]bool{
	"bolt": true, "code": true, "database": true, "bullhorn": true,
	"wrench": true, "robot": true, "server": true, "folder": true,
}

var allowedAdapters = map[string]bool{"codex": true, "claude-code": true}

func validateIdentity(id string, version int) error {
	if id == "" {
		if version != 0 {
			return fmt.Errorf("new catalog entry must have expectedversion 0")
		}
		return nil
	}
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("invalid catalog id")
	}
	if version < 1 {
		return fmt.Errorf("editing a catalog entry requires its current version")
	}
	return nil
}

func validateCreationKey(id string, creationKey *string) error {
	if id != "" {
		if *creationKey != "" {
			return fmt.Errorf("creationkey can only be used when creating a catalog entry")
		}
		return nil
	}
	if *creationKey == "" {
		return nil
	}
	parsed, err := uuid.Parse(*creationKey)
	if err != nil || parsed.String() != strings.ToLower(*creationKey) {
		return fmt.Errorf("invalid creationkey")
	}
	*creationKey = parsed.String()
	return nil
}

func validateLabel(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	n := utf8.RuneCountInString(value)
	if n < 1 || n > 120 {
		return "", fmt.Errorf("%s must contain 1 to 120 characters", field)
	}
	return value, nil
}

func validateIcon(icon string) error {
	if !allowedIcons[icon] {
		return fmt.Errorf("unsupported icon")
	}
	return nil
}

func validateProject(input *ForceProjectInput) error {
	if err := validateIdentity(input.ID, input.ExpectedVersion); err != nil {
		return err
	}
	if err := validateCreationKey(input.ID, &input.CreationKey); err != nil {
		return err
	}
	if len(input.Connection) > 1024 || len(input.RootPath) > 4096 {
		return fmt.Errorf("connection or rootpath exceeds size limit")
	}
	var err error
	input.Name, err = validateLabel(input.Name, "name")
	if err != nil {
		return err
	}
	if err := validateIcon(input.Icon); err != nil {
		return err
	}
	input.Connection = strings.TrimSpace(input.Connection)
	input.RootPath = strings.TrimSpace(input.RootPath)
	if input.Connection == "" {
		if !filepath.IsAbs(input.RootPath) {
			return fmt.Errorf("local rootpath must be absolute")
		}
		info, err := os.Stat(input.RootPath)
		if err != nil {
			return fmt.Errorf("local rootpath: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("local rootpath must be a directory")
		}
	} else {
		if conncontroller.IsLocalConnName(input.Connection) || conncontroller.IsWslConnName(input.Connection) {
			return fmt.Errorf("connection must be an SSH target")
		}
		if _, err := remote.ParseOpts(input.Connection); err != nil {
			return fmt.Errorf("invalid SSH connection: %w", err)
		}
		if !strings.HasPrefix(input.RootPath, "/") && !strings.HasPrefix(input.RootPath, "~/") {
			return fmt.Errorf("SSH rootpath must be absolute or start with ~/")
		}
		if strings.ContainsAny(input.RootPath, "\x00\r\n") {
			return fmt.Errorf("SSH rootpath contains invalid characters")
		}
	}
	return nil
}

func validateProfile(input *ForceProfileInput) error {
	if err := validateIdentity(input.ID, input.ExpectedVersion); err != nil {
		return err
	}
	if err := validateCreationKey(input.ID, &input.CreationKey); err != nil {
		return err
	}
	var err error
	input.Title, err = validateLabel(input.Title, "title")
	if err != nil {
		return err
	}
	if err := validateIcon(input.Icon); err != nil {
		return err
	}
	if utf8.RuneCountInString(input.SystemPrompt) > 32000 {
		return fmt.Errorf("systemprompt exceeds 32000 characters")
	}
	if !allowedAdapters[input.Adapter] {
		return fmt.Errorf("unsupported adapter")
	}
	return nil
}

func (svc *ForceService) GetCatalog_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx"}}
}

func (svc *ForceService) GetCatalog(ctx context.Context) (*ForceCatalog, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*ForceCatalog, error) {
		projects, err := wstore.DBGetAllObjsByType[*waveobj.ForceProject](tx.Context(), waveobj.OType_ForceProject)
		if err != nil {
			return nil, err
		}
		profiles, err := wstore.DBGetAllObjsByType[*waveobj.ForceAgentProfile](tx.Context(), waveobj.OType_ForceAgentProfile)
		if err != nil {
			return nil, err
		}
		sort.Slice(projects, func(i, j int) bool {
			left, right := strings.ToLower(projects[i].Name), strings.ToLower(projects[j].Name)
			if left == right {
				return projects[i].OID < projects[j].OID
			}
			return left < right
		})
		sort.Slice(profiles, func(i, j int) bool {
			left, right := strings.ToLower(profiles[i].Title), strings.ToLower(profiles[j].Title)
			if left == right {
				return profiles[i].OID < profiles[j].OID
			}
			return left < right
		})
		return &ForceCatalog{Projects: projects, Profiles: profiles}, nil
	})
}

func (svc *ForceService) SaveProject_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "input"}}
}

func (svc *ForceService) SaveProject(ctx context.Context, input ForceProjectInput) (*waveobj.ForceProject, waveobj.UpdatesRtnType, error) {
	if err := validateProject(&input); err != nil {
		return nil, nil, err
	}
	ctx = waveobj.ContextWithUpdates(ctx)
	idempotentRetry := false
	project, err := wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*waveobj.ForceProject, error) {
		var project *waveobj.ForceProject
		if input.ID == "" {
			oid := input.CreationKey
			if oid != "" {
				existing, err := wstore.DBGet[*waveobj.ForceProject](tx.Context(), oid)
				if err != nil {
					return nil, err
				}
				if existing != nil {
					if existing.Version == 1 && !existing.Archived && existing.Name == input.Name && existing.Icon == input.Icon && existing.Connection == input.Connection && existing.RootPath == input.RootPath {
						idempotentRetry = true
						return existing, nil
					}
					return nil, ErrConflict
				}
			} else {
				oid = uuid.NewString()
			}
			project = &waveobj.ForceProject{OID: oid, CreatedAt: time.Now().UnixMilli()}
		} else {
			var err error
			project, err = wstore.DBGet[*waveobj.ForceProject](tx.Context(), input.ID)
			if err != nil {
				return nil, err
			}
			if project == nil {
				return nil, wstore.ErrNotFound
			}
			if project.Version != input.ExpectedVersion {
				return nil, ErrConflict
			}
		}
		project.Name, project.Icon = input.Name, input.Icon
		project.Connection, project.RootPath = input.Connection, input.RootPath
		project.UpdatedAt = time.Now().UnixMilli()
		if input.ID == "" {
			err := wstore.DBInsert(tx.Context(), project)
			return project, err
		}
		err := wstore.DBUpdate(tx.Context(), project)
		return project, err
	})
	if err != nil {
		return nil, nil, err
	}
	if idempotentRetry {
		return project, nil, nil
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return project, updates, nil
}

func (svc *ForceService) SaveProfile_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "input"}}
}

func (svc *ForceService) SaveProfile(ctx context.Context, input ForceProfileInput) (*waveobj.ForceAgentProfile, waveobj.UpdatesRtnType, error) {
	if err := validateProfile(&input); err != nil {
		return nil, nil, err
	}
	ctx = waveobj.ContextWithUpdates(ctx)
	idempotentRetry := false
	profile, err := wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*waveobj.ForceAgentProfile, error) {
		var profile *waveobj.ForceAgentProfile
		if input.ID == "" {
			oid := input.CreationKey
			if oid != "" {
				existing, err := wstore.DBGet[*waveobj.ForceAgentProfile](tx.Context(), oid)
				if err != nil {
					return nil, err
				}
				if existing != nil {
					if existing.Version == 1 && !existing.Archived && existing.Title == input.Title && existing.Icon == input.Icon && existing.SystemPrompt == input.SystemPrompt && existing.Adapter == input.Adapter {
						idempotentRetry = true
						return existing, nil
					}
					return nil, ErrConflict
				}
			} else {
				oid = uuid.NewString()
			}
			profile = &waveobj.ForceAgentProfile{OID: oid, CreatedAt: time.Now().UnixMilli()}
		} else {
			var err error
			profile, err = wstore.DBGet[*waveobj.ForceAgentProfile](tx.Context(), input.ID)
			if err != nil {
				return nil, err
			}
			if profile == nil {
				return nil, wstore.ErrNotFound
			}
			if profile.Version != input.ExpectedVersion {
				return nil, ErrConflict
			}
		}
		profile.Title, profile.Icon = input.Title, input.Icon
		profile.SystemPrompt, profile.Adapter = input.SystemPrompt, input.Adapter
		profile.UpdatedAt = time.Now().UnixMilli()
		if input.ID == "" {
			err := wstore.DBInsert(tx.Context(), profile)
			return profile, err
		}
		err := wstore.DBUpdate(tx.Context(), profile)
		return profile, err
	})
	if err != nil {
		return nil, nil, err
	}
	if idempotentRetry {
		return profile, nil, nil
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return profile, updates, nil
}

func (svc *ForceService) ArchiveProject_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "id", "expectedVersion", "archived"}}
}

func (svc *ForceService) ArchiveProject(ctx context.Context, id string, expectedVersion int, archived bool) (waveobj.UpdatesRtnType, error) {
	if err := validateIdentity(id, expectedVersion); err != nil || id == "" {
		return nil, fmt.Errorf("archive project requires an id and version")
	}
	ctx = waveobj.ContextWithUpdates(ctx)
	err := wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		project, err := wstore.DBGet[*waveobj.ForceProject](tx.Context(), id)
		if err != nil {
			return err
		}
		if project == nil {
			return wstore.ErrNotFound
		}
		if project.Version != expectedVersion {
			return ErrConflict
		}
		project.Archived = archived
		project.UpdatedAt = time.Now().UnixMilli()
		return wstore.DBUpdate(tx.Context(), project)
	})
	if err != nil {
		return nil, err
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return updates, nil
}

func (svc *ForceService) ArchiveProfile_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{ArgNames: []string{"ctx", "id", "expectedVersion", "archived"}}
}

func (svc *ForceService) ArchiveProfile(ctx context.Context, id string, expectedVersion int, archived bool) (waveobj.UpdatesRtnType, error) {
	if err := validateIdentity(id, expectedVersion); err != nil || id == "" {
		return nil, fmt.Errorf("archive profile requires an id and version")
	}
	ctx = waveobj.ContextWithUpdates(ctx)
	err := wstore.WithTx(ctx, func(tx *wstore.TxWrap) error {
		profile, err := wstore.DBGet[*waveobj.ForceAgentProfile](tx.Context(), id)
		if err != nil {
			return err
		}
		if profile == nil {
			return wstore.ErrNotFound
		}
		if profile.Version != expectedVersion {
			return ErrConflict
		}
		profile.Archived = archived
		profile.UpdatedAt = time.Now().UnixMilli()
		return wstore.DBUpdate(tx.Context(), profile)
	})
	if err != nil {
		return nil, err
	}
	updates := waveobj.ContextGetUpdatesRtn(ctx)
	wps.Broker.SendUpdateEvents(updates)
	return updates, nil
}
