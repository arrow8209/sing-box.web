package rules

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

type document struct {
	Resources []Resource `json:"resources"`
}

var resourceIDPattern = regexp.MustCompile(`^ID_[0-9a-f]{32}$`)

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
		if err := ValidateManagedState(file, layout); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return &Store{file: file, layout: layout, clock: clock}, nil
}

// ValidateManagedState performs the read-only rule-resource startup gate.
func ValidateManagedState(file string, layout paths.Layout) error {
	document, err := storage.ReadJSON[document](file)
	if err != nil {
		return err
	}
	if err := validateDocument(document, layout); err != nil {
		return &storage.CorruptFileError{Path: file, Err: err}
	}
	return nil
}

func validateDocument(data document, layout paths.Layout) error {
	if data.Resources == nil {
		return fmt.Errorf("resources 不能为 null")
	}
	ids := make(map[string]struct{}, len(data.Resources))
	templateKeys := make(map[string]struct{}, len(data.Resources))
	for index, resource := range data.Resources {
		if !resourceIDPattern.MatchString(resource.ID) {
			return fmt.Errorf("resources[%d].id 无效", index)
		}
		if _, exists := ids[resource.ID]; exists {
			return fmt.Errorf("resources[%d].id 重复: %s", index, resource.ID)
		}
		ids[resource.ID] = struct{}{}
		if resource.TemplateKey != "" {
			if _, exists := templateKeys[resource.TemplateKey]; exists {
				return fmt.Errorf("resources[%d].templateKey 重复: %s", index, resource.TemplateKey)
			}
			templateKeys[resource.TemplateKey] = struct{}{}
		}
		if resource.Format != FormatBinary && resource.Format != FormatSource {
			return fmt.Errorf("resources[%d].format 无效", index)
		}
		derived := filepath.Join(layout.RuleSetsDir, resource.ID+".srs")
		relative, err := filepath.Rel(layout.RuleSetsDir, derived)
		if err != nil || relative != resource.ID+".srs" || filepath.IsAbs(relative) {
			return fmt.Errorf("resources[%d].id 派生的规则路径无效", index)
		}
		if filepath.Clean(resource.LocalPath) != filepath.Clean(derived) {
			return fmt.Errorf("resources[%d].localPath 不是固定派生路径", index)
		}
	}
	return nil
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
	data := document{Resources: []Resource{}}
	if _, err := os.Stat(s.file); err == nil {
		var readErr error
		data, readErr = storage.ReadJSON[document](s.file)
		if readErr != nil {
			return nil, readErr
		}
		if validateErr := validateDocument(data, s.layout); validateErr != nil {
			return nil, &storage.CorruptFileError{Path: s.file, Err: validateErr}
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
		if err := validateDocument(data, s.layout); err != nil {
			return nil, err
		}
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
