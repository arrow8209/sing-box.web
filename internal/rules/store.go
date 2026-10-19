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
	"strings"
	"sync"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

type document struct {
	Resources []Resource `json:"resources"`
}

var resourceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

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
		if err := migrateResources(file, layout); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return &Store{file: file, layout: layout, clock: clock}, nil
}

// migrateResources persists portable paths and drops obsolete remote Source
// payloads, preserving resource IDs and references. Legacy absolute paths are
// only used to recognize the old format; all file operations use this layout.
func migrateResources(file string, layout paths.Layout) error {
	data, legacyPaths, err := readDocument(file, layout)
	if err != nil {
		return err
	}
	obsolete := make(map[string]struct{})
	changed := legacyPaths
	for index := range data.Resources {
		resource := &data.Resources[index]
		if effectiveSourceType(resource.SourceType) != SourceRemote {
			continue
		}
		sourcePath, _, err := layout.RuleSetPath(resource.ID, ".json")
		if err != nil {
			return err
		}
		obsolete[sourcePath] = struct{}{}
		if resource.Format != FormatSource {
			continue
		}
		binaryPath, _, err := layout.RuleSetPath(resource.ID, ".srs")
		if err != nil {
			return err
		}
		resource.Format, resource.LocalPath = FormatBinary, binaryPath
		resource.LastAttemptAt, resource.LastSuccessAt = nil, nil
		resource.LastDigest, resource.LastError = "", ""
		resource.NextUpdateAt, resource.RuleCount = nil, nil
		changed = true
	}
	if changed {
		if err := validateDocument(data, layout); err != nil {
			return err
		}
		if legacyPaths {
			original, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			id, err := randomID(rand.Reader)
			if err != nil {
				return err
			}
			if err := storage.WriteFile(file+".before-relative-paths-"+id+".bak", original, 0o600); err != nil {
				return fmt.Errorf("备份旧规则资源路径失败: %w", err)
			}
		}
		if err := writeDocument(file, data, layout); err != nil {
			return err
		}
	}
	for path := range obsolete {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除旧 Source 规则文件失败: %w", err)
		}
	}
	return nil
}

// ValidateManagedState performs the read-only rule-resource startup gate.
func ValidateManagedState(file string, layout paths.Layout) error {
	_, _, err := readDocument(file, layout)
	return err
}

// readDocument resolves persisted paths for runtime use without consulting the
// old location or changing the file. The startup gate uses this same reader.
func readDocument(file string, layout paths.Layout) (document, bool, error) {
	data, err := storage.ReadJSON[document](file)
	if err != nil {
		return document{}, false, err
	}
	if err := validateDocument(data, layout); err != nil {
		return document{}, false, &storage.CorruptFileError{Path: file, Err: err}
	}
	legacyPaths := false
	for index := range data.Resources {
		resource := &data.Resources[index]
		absolute, relative, _ := layout.RuleSetPath(resource.ID, extension(resource.Format))
		legacyPaths = legacyPaths || resource.LocalPath != relative
		resource.LocalPath = absolute
	}
	return data, legacyPaths, nil
}

// writeDocument keeps runtime absolute paths out of the managed JSON. Copy the
// resource slice so callers can keep using its resolved paths after a save.
func writeDocument(file string, data document, layout paths.Layout) error {
	if err := validateDocument(data, layout); err != nil {
		return err
	}
	resources := make([]Resource, len(data.Resources))
	copy(resources, data.Resources)
	for index := range resources {
		resource := &resources[index]
		_, relative, _ := layout.RuleSetPath(resource.ID, extension(resource.Format))
		resource.LocalPath = relative
	}
	return storage.WriteJSON(file, document{Resources: resources}, 0o600)
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
		if resource.CatalogKey != "" && !validCatalogKey(resource.CatalogKey) {
			return fmt.Errorf("resources[%d].catalogKey 无效", index)
		}
		if resource.Format != FormatBinary && resource.Format != FormatSource {
			return fmt.Errorf("resources[%d].format 无效", index)
		}
		if resource.RuleCount != nil && *resource.RuleCount < 0 {
			return fmt.Errorf("resources[%d].ruleCount 无效", index)
		}
		switch effectiveSourceType(resource.SourceType) {
		case SourceRemote:
			if err := validateRemoteInput(resource.Name, resource.URL, resource.Format, resource.UpdateInterval); err != nil {
				return fmt.Errorf("resources[%d] 无效: %w", index, err)
			}
		case SourceManual:
			if err := validateName(resource.Name); err != nil {
				return fmt.Errorf("resources[%d] 无效: %w", index, err)
			}
			if resource.URL != "" || resource.UpdateInterval != "" || resource.Format != FormatSource {
				return fmt.Errorf("resources[%d] 的手工规则字段无效", index)
			}
			if resource.LastSuccessAt == nil || resource.LastDigest == "" {
				return fmt.Errorf("resources[%d] 的手工规则缺少内容状态", index)
			}
		default:
			return fmt.Errorf("resources[%d].sourceType 无效", index)
		}
		_, relative, err := layout.RuleSetPath(resource.ID, extension(resource.Format))
		if err != nil {
			return fmt.Errorf("resources[%d].id 派生的规则路径无效", index)
		}
		// Only canonical relative paths or the exact fixed suffix of a legacy
		// absolute path are accepted. Never resolve traversal or use its root.
		path := resource.LocalPath
		legacy := filepath.IsAbs(path) && filepath.Clean(path) == path &&
			!strings.ContainsRune(path, '\x00') &&
			strings.HasSuffix(path, string(filepath.Separator)+filepath.FromSlash(relative))
		if path != relative && !legacy {
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
		data, _, readErr = readDocument(s.file, s.layout)
		if readErr != nil {
			return nil, readErr
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	ids := make(map[string]string, len(templates))
	changed := false
	for _, template := range templates {
		if template.TemplateKey == "" || template.Name == "" || template.URL == "" || template.Format != FormatBinary {
			return nil, fmt.Errorf("无效默认规则模板: %s", template.TemplateKey)
		}
		for index := range data.Resources {
			resource := data.Resources[index]
			if resource.TemplateKey == template.TemplateKey {
				ids[template.TemplateKey] = resource.ID
				if resource.CatalogKey == "" && template.CatalogKey != "" {
					data.Resources[index].CatalogKey = template.CatalogKey
					changed = true
				}
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
		localPath, _, err := s.layout.RuleSetPath(id, extension(template.Format))
		if err != nil {
			return nil, err
		}
		data.Resources = append(data.Resources, Resource{ID: id, TemplateKey: template.TemplateKey, CatalogKey: template.CatalogKey, SourceType: SourceRemote, Name: template.Name, URL: template.URL, Format: template.Format, UpdateInterval: template.UpdateInterval, LocalPath: localPath})
		ids[template.TemplateKey] = id
		changed = true
	}
	if changed || os.IsNotExist(statErr(s.file)) {
		if err := writeDocument(s.file, data, s.layout); err != nil {
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
	return hex.EncodeToString(bytes), nil
}

func extension(format Format) string {
	if format == FormatSource {
		return ".json"
	}
	return ".srs"
}
func statErr(path string) error { _, err := os.Stat(path); return err }

func (s *Store) list() ([]Resource, error) {
	data := document{Resources: []Resource{}}
	if _, err := os.Stat(s.file); err == nil {
		var readErr error
		data, _, readErr = readDocument(s.file, s.layout)
		if readErr != nil {
			return nil, readErr
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	resources := make([]Resource, len(data.Resources))
	copy(resources, data.Resources)
	for index := range resources {
		resources[index].SourceType = effectiveSourceType(resources[index].SourceType)
	}
	return resources, nil
}

func (s *Store) write(resources []Resource) error {
	copied := make([]Resource, len(resources))
	copy(copied, resources)
	data := document{Resources: copied}
	for index := range data.Resources {
		data.Resources[index].SourceType = effectiveSourceType(data.Resources[index].SourceType)
	}
	return writeDocument(s.file, data, s.layout)
}
