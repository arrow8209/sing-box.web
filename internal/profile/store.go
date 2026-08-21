package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

type profileDocument struct {
	Version  int    `json:"version"`
	Profiles []Meta `json:"profiles"`
}

type store struct {
	profilesFile string
	draftsDir    string
}

func newStore(layout paths.Layout) *store {
	return &store{profilesFile: layout.ProfilesFile, draftsDir: layout.DraftsDir}
}

func (s *store) loadMetas() ([]Meta, error) {
	if _, err := os.Stat(s.profilesFile); os.IsNotExist(err) {
		return []Meta{}, nil
	} else if err != nil {
		return nil, err
	}
	document, err := storage.ReadJSON[profileDocument](s.profilesFile)
	if err != nil {
		return nil, err
	}
	if document.Version != 1 {
		return nil, fmt.Errorf("不支持的档案索引版本: %d", document.Version)
	}
	return cloneMetas(document.Profiles), nil
}

func (s *store) writeMetas(metas []Meta) error {
	return storage.WriteJSON(s.profilesFile, profileDocument{Version: 1, Profiles: cloneMetas(metas)}, 0o600)
}

func (s *store) writeDraft(draft Draft) error {
	return storage.WriteJSON(s.draftPath(draft.ProfileID), cloneDraft(draft), 0o600)
}

func (s *store) readDraft(profileID string) (Draft, error) {
	draft, err := storage.ReadJSON[Draft](s.draftPath(profileID))
	if err != nil {
		return Draft{}, err
	}
	if draft.ProfileID != profileID {
		return Draft{}, fmt.Errorf("草稿档案 ID 不匹配")
	}
	if err := validateDraftEnvelope(draft, maxModuleBytes, maxDraftBytes); err != nil {
		return Draft{}, err
	}
	return cloneDraft(draft), nil
}

func (s *store) create(meta Meta, draft Draft) error {
	metas, err := s.loadMetas()
	if err != nil {
		return err
	}
	for _, existing := range metas {
		if existing.ID == meta.ID {
			return fmt.Errorf("档案 ID 已存在")
		}
	}
	if err := s.writeDraft(draft); err != nil {
		return err
	}
	metas = append(metas, cloneMeta(meta))
	return s.writeMetas(metas)
}

func (s *store) draftPath(profileID string) string {
	return filepath.Join(s.draftsDir, profileID+".json")
}

func cloneDraft(source Draft) Draft {
	result := source
	result.Config = make(map[string]json.RawMessage, len(source.Config))
	for key, value := range source.Config {
		result.Config[key] = append(json.RawMessage(nil), value...)
	}
	result.Buffers = make(map[string]string, len(source.Buffers))
	for key, value := range source.Buffers {
		result.Buffers[key] = value
	}
	result.Bindings = append([]RuleBinding(nil), source.Bindings...)
	return result
}

func cloneMeta(source Meta) Meta {
	result := source
	if source.AppliedRevision != nil {
		revision := *source.AppliedRevision
		result.AppliedRevision = &revision
	}
	if source.AppliedAt != nil {
		appliedAt := *source.AppliedAt
		result.AppliedAt = &appliedAt
	}
	if source.LastValidation != nil {
		validation := *source.LastValidation
		result.LastValidation = &validation
	}
	result.AppliedResourceDigests = make(map[string]string, len(source.AppliedResourceDigests))
	for key, value := range source.AppliedResourceDigests {
		result.AppliedResourceDigests[key] = value
	}
	return result
}

func cloneMetas(source []Meta) []Meta {
	result := make([]Meta, len(source))
	for index := range source {
		result[index] = cloneMeta(source[index])
	}
	return result
}
