package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Layout struct {
	WorkDir              string
	DataDir              string
	AuthFile             string
	ProfilesFile         string
	CoreFile             string
	SettingsFile         string
	SubscriptionsFile    string
	InboundResourcesFile string
	RulesFile            string
	RuleCatalogFile      string
	DraftsDir            string
	RuleSetsDir          string
	BinDir               string
	CoreBinary           string
	CoreBackup           string
	LogsDir              string
	CoreLog              string
	AppLock              string
	ConfigDir            string
}

// StagingConfigDir returns a new, fixed-name staging directory directly below
// WorkDir. Keeping it outside data prevents a checked configuration from
// accidentally resolving data/rulesets relative to a different directory.
func (layout Layout) StagingConfigDir(token string) (string, error) {
	if len(token) != 32 {
		return "", fmt.Errorf("临时配置令牌无效")
	}
	for _, character := range token {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return "", fmt.Errorf("临时配置令牌无效")
		}
	}
	path := filepath.Join(layout.WorkDir, "config.staging-"+token)
	if !isWithin(layout.WorkDir, path) || filepath.Dir(path) != layout.WorkDir {
		return "", fmt.Errorf("临时配置路径越出工作目录")
	}
	return path, nil
}

func Resolve(workDir string) (Layout, error) {
	root, err := filepath.Abs(workDir)
	if err != nil {
		return Layout{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Layout{}, err
	}
	data := filepath.Join(root, "data")
	layout := Layout{
		WorkDir: root, DataDir: data,
		AuthFile:             filepath.Join(data, "auth.json"),
		ProfilesFile:         filepath.Join(data, "profiles.json"),
		CoreFile:             filepath.Join(data, "core.json"),
		SettingsFile:         filepath.Join(data, "settings.json"),
		SubscriptionsFile:    filepath.Join(data, "subscriptions.json"),
		InboundResourcesFile: filepath.Join(data, "inbound-resources.json"),
		RulesFile:            filepath.Join(data, "rule-resources.json"),
		RuleCatalogFile:      filepath.Join(data, "rule-catalog.json"),
		DraftsDir:            filepath.Join(data, "drafts"),
		RuleSetsDir:          filepath.Join(data, "rulesets"),
		BinDir:               filepath.Join(data, "bin"),
		CoreBinary:           filepath.Join(data, "bin", "sing-box"),
		CoreBackup:           filepath.Join(data, "bin", "sing-box.bak"),
		LogsDir:              filepath.Join(data, "logs"),
		CoreLog:              filepath.Join(data, "logs", "sing-box.log"),
		AppLock:              filepath.Join(data, "app.lock"),
		ConfigDir:            filepath.Join(root, "config"),
	}
	for _, path := range layout.allManagedPaths() {
		if !isWithin(root, path) {
			return Layout{}, fmt.Errorf("固定路径越出工作目录: %s", path)
		}
		if err := rejectExistingSymlinkComponents(root, path); err != nil {
			return Layout{}, err
		}
	}
	return layout, nil
}

func (layout Layout) allManagedPaths() []string {
	return []string{layout.DataDir, layout.AuthFile, layout.ProfilesFile, layout.CoreFile, layout.SettingsFile, layout.SubscriptionsFile, layout.InboundResourcesFile, layout.RulesFile, layout.RuleCatalogFile, layout.DraftsDir, layout.RuleSetsDir, layout.BinDir, layout.CoreBinary, layout.CoreBackup, layout.LogsDir, layout.CoreLog, layout.AppLock, layout.ConfigDir}
}

// RuleSetPath derives the sole permitted local rule-set location from a
// server-generated resource ID and a supported ruleset extension.
func (layout Layout) RuleSetPath(resourceID, extension string) (string, string, error) {
	if len(resourceID) != 32 {
		return "", "", fmt.Errorf("规则资源 ID 无效")
	}
	for _, character := range resourceID {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return "", "", fmt.Errorf("规则资源 ID 无效")
		}
	}
	if extension != ".json" && extension != ".srs" {
		return "", "", fmt.Errorf("规则文件扩展名无效")
	}
	relative := filepath.ToSlash(filepath.Join("data", "rulesets", resourceID+extension))
	absolute := filepath.Join(layout.WorkDir, filepath.FromSlash(relative))
	if !isWithin(layout.WorkDir, absolute) {
		return "", "", fmt.Errorf("规则文件路径越出工作目录")
	}
	return absolute, relative, nil
}

func isWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func rejectExistingSymlinkComponents(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || !isWithin(root, path) {
		return fmt.Errorf("固定路径越出工作目录: %s", path)
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("受管路径包含符号链接: %s", current)
		}
	}
	return nil
}
