package rules

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

type document struct {
	Resources []Resource `json:"resources"`
}
type Store struct {
	file   string
	layout paths.Layout
	clock  func() time.Time
	mu     sync.Mutex
}

func OpenStore(file string, layout paths.Layout, clock func() time.Time) (*Store, error) {
	if file == "" {
		return nil, fmt.Errorf("规则资源文件不能为空")
	}
	if clock == nil {
		clock = time.Now
	}
	if _, err := os.Stat(file); err == nil {
		if _, err := storage.ReadJSON[document](file); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return &Store{file: file, layout: layout, clock: clock}, nil
}

func (s *Store) EnsureTemplateResources(ctx context.Context, templates []ResourceTemplate, randomSource io.Reader) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if randomSource == nil {
		randomSource = rand.Reader
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data := document{}
	if _, err := os.Stat(s.file); err == nil {
		var readErr error
		data, readErr = storage.ReadJSON[document](s.file)
		if readErr != nil {
			return nil, readErr
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	ids := make(map[string]string, len(templates))
	changed := false
	for _, template := range templates {
		if template.TemplateKey == "" || template.Name == "" || template.URL == "" || (template.Format != FormatBinary && template.Format != FormatSource) {
			return nil, fmt.Errorf("无效默认规则模板: %s", template.TemplateKey)
		}
		for _, resource := range data.Resources {
			if resource.TemplateKey == template.TemplateKey {
				ids[template.TemplateKey] = resource.ID
				break
			}
		}
		if ids[template.TemplateKey] != "" {
			continue
		}
		id, err := randomID(randomSource)
		if err != nil {
			return nil, err
		}
		data.Resources = append(data.Resources, Resource{ID: id, TemplateKey: template.TemplateKey, Name: template.Name, URL: template.URL, Format: template.Format, UpdateInterval: template.UpdateInterval, LocalPath: filepath.Join(s.layout.RuleSetsDir, id+".srs")})
		ids[template.TemplateKey] = id
		changed = true
	}
	if changed || os.IsNotExist(statErr(s.file)) {
		if err := storage.WriteJSON(s.file, data, 0o600); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func randomID(source io.Reader) (string, error) {
	bytes := make([]byte, 16)
	if _, err := io.ReadFull(source, bytes); err != nil {
		return "", err
	}
	return "ID_" + hex.EncodeToString(bytes), nil
}
func statErr(path string) error { _, err := os.Stat(path); return err }
