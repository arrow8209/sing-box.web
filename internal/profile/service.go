package profile

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"singbox-web/internal/configdoc"
)

type Service struct {
	repo     *Repository
	template *configdoc.Template
	random   io.Reader
	randomMu sync.Mutex
}

func NewService(repo *Repository, template *configdoc.Template, random io.Reader) *Service {
	return &Service{repo: repo, template: template, random: random}
}

func (service *Service) Create(ctx context.Context, input CreateInput) (Profile, Draft, error) {
	if service == nil || service.repo == nil {
		return Profile{}, Draft{}, fmt.Errorf("profile repository is nil")
	}
	if service.template == nil {
		return Profile{}, Draft{}, fmt.Errorf("profile template is nil")
	}
	if service.random == nil {
		return Profile{}, Draft{}, fmt.Errorf("random reader is nil")
	}
	if err := ctx.Err(); err != nil {
		return Profile{}, Draft{}, err
	}
	name, err := validateName(input.Name)
	if err != nil {
		return Profile{}, Draft{}, err
	}
	if !slugPattern.MatchString(input.Slug) {
		return Profile{}, Draft{}, fmt.Errorf("invalid profile slug %q", input.Slug)
	}

	service.randomMu.Lock()
	id, err := randomID(service.random)
	if err != nil {
		service.randomMu.Unlock()
		return Profile{}, Draft{}, err
	}
	config, err := service.template.New(service.random)
	service.randomMu.Unlock()
	if err != nil {
		return Profile{}, Draft{}, fmt.Errorf("create profile config: %w", err)
	}

	item := Profile{
		ID:            id,
		Name:          name,
		Slug:          input.Slug,
		DraftRevision: 1,
	}
	draft := Draft{
		ProfileID: id,
		Revision:  1,
		Content: DraftContent{
			Config:        json.RawMessage(config),
			EditorBuffers: map[string]EditorBuffer{},
		},
	}
	draft, err = service.repo.insert(ctx, item, draft)
	if err != nil {
		return Profile{}, Draft{}, err
	}
	return cloneProfile(item), cloneDraft(draft), nil
}

func (service *Service) List(ctx context.Context) ([]Profile, error) {
	if service == nil || service.repo == nil {
		return nil, fmt.Errorf("profile repository is nil")
	}
	return service.repo.List(ctx)
}

func (service *Service) Get(ctx context.Context, id string) (Profile, error) {
	if service == nil || service.repo == nil {
		return Profile{}, fmt.Errorf("profile repository is nil")
	}
	return service.repo.Get(ctx, id)
}

func (service *Service) Rename(ctx context.Context, id, name string) (Profile, error) {
	if service == nil || service.repo == nil {
		return Profile{}, fmt.Errorf("profile repository is nil")
	}
	return service.repo.Rename(ctx, id, name)
}

func (service *Service) RemoveMetadata(ctx context.Context, id string) error {
	if service == nil || service.repo == nil {
		return fmt.Errorf("profile repository is nil")
	}
	return service.repo.RemoveMetadata(ctx, id)
}

func randomID(random io.Reader) (string, error) {
	bytes := make([]byte, 16)
	if _, err := io.ReadFull(random, bytes); err != nil {
		return "", fmt.Errorf("generate profile ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func cloneDraft(draft Draft) Draft {
	draft.Content.Config = append(json.RawMessage(nil), draft.Content.Config...)
	if draft.Content.EditorBuffers != nil {
		buffers := make(map[string]EditorBuffer, len(draft.Content.EditorBuffers))
		for key, value := range draft.Content.EditorBuffers {
			buffers[key] = value
		}
		draft.Content.EditorBuffers = buffers
	}
	return draft
}
