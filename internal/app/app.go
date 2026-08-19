package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"singbox-web/internal/assets"
	"singbox-web/internal/atomicfile"
	"singbox-web/internal/audit"
	"singbox-web/internal/auth"
	"singbox-web/internal/capabilities"
	"singbox-web/internal/configdoc"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/profile"
	"singbox-web/internal/publication"
	"singbox-web/internal/singbox"
	"singbox-web/internal/validation"
)

type Options struct {
	Listen            string
	DataDir           string
	ConfigRoot        string
	SingBoxPath       string
	ConfigGroup       string
	AllowPublicListen bool
	Development       bool
	PublicOrigin      string
	Version           string
	LogOutput         io.Writer
}

type schemaMetadata struct {
	TargetVersion string `json:"targetVersion"`
}

var (
	refreshCapabilities = func(ctx context.Context, service *capabilities.Service) { service.Refresh(ctx) }
	recoverPublication  = func(ctx context.Context, service *publication.Service) error { return service.Recover(ctx) }
	newHTTPHandler      = httpapi.New
	listenTCP           = net.Listen
)

func Run(ctx context.Context, options Options) error {
	if err := validateNetwork(options.Listen, options.PublicOrigin, options.AllowPublicListen, options.Development); err != nil {
		return err
	}
	group, err := user.LookupGroup(options.ConfigGroup)
	if err != nil {
		return fmt.Errorf("配置组无效")
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil || gid < 0 {
		return fmt.Errorf("配置组无效")
	}
	if err := prepareRoots(options.DataDir, options.ConfigRoot, gid); err != nil {
		return err
	}
	lockPath := filepath.Join(options.DataDir, "app.lock")
	if err := validateOptionalRegularFile(lockPath, 0o600); err != nil {
		return err
	}
	lock, err := atomicfile.Acquire(lockPath)
	if err != nil {
		return fmt.Errorf("应用锁已被占用: %s", lockPath)
	}
	defer lock.Close()

	now := time.Now
	authPath := filepath.Join(options.DataDir, "auth.json")
	if err := validateRequiredRegularFile(authPath, 0o600); err != nil {
		return err
	}
	store := auth.NewFileStore(authPath, now, rand.Reader)
	record, err := store.Load()
	if err != nil {
		return fmt.Errorf("%s: JSON 解析失败", authPath)
	}
	if record.Version != 1 {
		return fmt.Errorf("%s: 不支持的版本", authPath)
	}
	if record.PasswordHash == "" || !validSessionKey(record.SessionKey) {
		return fmt.Errorf("%s: 认证数据无效", authPath)
	}

	profilesPath := filepath.Join(options.DataDir, "profiles.json")
	if err := validateOptionalRegularFile(profilesPath, 0o600); err != nil {
		return err
	}
	if _, err := os.Lstat(profilesPath); err == nil {
		version, parseErr := readVersion(profilesPath)
		if parseErr != nil {
			return fmt.Errorf("%s: JSON 解析失败", profilesPath)
		}
		if version != 1 {
			return fmt.Errorf("%s: 不支持的版本", profilesPath)
		}
	}
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		return fmt.Errorf("内嵌默认配置无效")
	}
	schema, err := validation.NewSchemaValidator(assets.SingBoxSchema)
	if err != nil {
		return fmt.Errorf("内嵌 Schema 无效")
	}
	var metadata schemaMetadata
	if err := json.Unmarshal(assets.SchemaVersion, &metadata); err != nil || metadata.TargetVersion == "" {
		return fmt.Errorf("内嵌 Schema 元数据无效")
	}
	repository, err := profile.OpenRepository(options.DataDir, now)
	if err != nil {
		return fmt.Errorf("%s: 档案数据无效", profilesPath)
	}
	profiles := profile.NewService(repository, template, rand.Reader)
	preflight := validation.NewService(schema, metadata.TargetVersion, now)
	runner := singbox.NewRunner(options.SingBoxPath, 30*time.Second, 16*1024)
	capabilityService := capabilities.New(metadata.TargetVersion, runner)
	refreshCapabilities(ctx, capabilityService)
	publicationService := publication.NewService(publication.Options{
		ConfigRoot: options.ConfigRoot,
		Profiles:   repository,
		Preflight:  preflight,
		Checker:    runner,
		Now:        now,
		Random:     rand.Reader,
		FileMode:   0o640,
		DirMode:    0o750,
		GroupID:    &gid,
	})
	if err := recoverPublication(ctx, publicationService); err != nil {
		return fmt.Errorf("发布状态恢复失败")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.LogOutput == nil {
		options.LogOutput = io.Discard
	}
	auditor := audit.New(options.LogOutput, now)
	authService := auth.NewSessionManager(store, now, rand.Reader)
	handler := newHTTPHandler(httpapi.Dependencies{
		Auth:         authService,
		Capabilities: capabilityService,
		Profiles:     profiles,
		Preflight:    preflight,
		Publication:  publicationService,
		Auditor:      auditor,
		ConfigRoot:   options.ConfigRoot,
		PublicOrigin: options.PublicOrigin,
		Now:          now,
	})
	server := &http.Server{
		Addr:              options.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	listener, err := listenTCP("tcp", options.Listen)
	if err != nil {
		return fmt.Errorf("监听地址不可用")
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP 服务失败")
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			_ = server.Close()
		}
		err := <-serveErr
		if shutdownErr != nil {
			return fmt.Errorf("HTTP 服务关闭失败")
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP 服务失败")
		}
		return nil
	}
}

func validateNetwork(listen, publicOrigin string, allowPublic, development bool) error {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("监听地址无效")
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if !allowPublic && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("拒绝公开监听地址")
	}
	parsed, err := url.Parse(publicOrigin)
	if err != nil || publicOrigin == "" || parsed.String() != publicOrigin || parsed.User != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" || parsed.Scheme != strings.ToLower(parsed.Scheme) || parsed.Host != strings.ToLower(parsed.Host) || strings.HasSuffix(parsed.Hostname(), ".") {
		return fmt.Errorf("PublicOrigin 无效")
	}
	if (parsed.Scheme == "https" && parsed.Port() == "443") || (parsed.Scheme == "http" && parsed.Port() == "80") {
		return fmt.Errorf("PublicOrigin 无效")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if !development || parsed.Scheme != "http" {
		return fmt.Errorf("PublicOrigin 必须使用 HTTPS")
	}
	originIP := net.ParseIP(strings.Trim(parsed.Hostname(), "[]"))
	if originIP == nil || !originIP.IsLoopback() {
		return fmt.Errorf("开发模式 HTTP Origin 必须为回环地址")
	}
	return nil
}

func prepareRoots(dataDir, configRoot string, gid int) error {
	for _, root := range []string{dataDir, configRoot} {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return fmt.Errorf("根目录必须是清理后的绝对路径: %s", root)
		}
		if err := validateRootPath(root); err != nil {
			return err
		}
	}
	dataCreated, err := ensureRoot(dataDir, 0o700)
	if err != nil {
		return fmt.Errorf("无法创建数据目录: %s", dataDir)
	}
	configCreated, err := ensureRoot(configRoot, 0o750)
	if err != nil {
		return fmt.Errorf("无法创建配置目录: %s", configRoot)
	}
	if configCreated {
		if err := os.Chown(configRoot, -1, gid); err != nil {
			return fmt.Errorf("无法设置配置目录属组: %s", configRoot)
		}
	}
	_ = dataCreated
	if err := validateRootPath(dataDir); err != nil {
		return err
	}
	if err := validateRootPath(configRoot); err != nil {
		return err
	}
	uid := uint32(os.Geteuid())
	dataInfo, err := os.Lstat(dataDir)
	if err != nil || !dataInfo.IsDir() {
		return fmt.Errorf("数据目录无效: %s", dataDir)
	}
	dataStat, ok := dataInfo.Sys().(*syscall.Stat_t)
	if !ok || dataStat.Uid != uid {
		return fmt.Errorf("数据目录所有者无效: %s", dataDir)
	}
	if dataInfo.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("数据目录权限过宽: %s", dataDir)
	}
	configInfo, err := os.Lstat(configRoot)
	if err != nil || !configInfo.IsDir() {
		return fmt.Errorf("配置目录无效: %s", configRoot)
	}
	configStat, ok := configInfo.Sys().(*syscall.Stat_t)
	if !ok || configStat.Uid != uid || int(configStat.Gid) != gid {
		return fmt.Errorf("配置目录所有权无效: %s", configRoot)
	}
	if configInfo.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("配置目录权限不安全: %s", configRoot)
	}
	return nil
}

func ensureRoot(path string, mode os.FileMode) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return false, err
	}
	if err := os.Chmod(path, mode); err != nil {
		return false, err
	}
	return true, nil
}

func validateRootPath(path string) error {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	current := volume + string(os.PathSeparator)
	relative := strings.TrimPrefix(clean, current)
	for _, component := range strings.Split(relative, string(os.PathSeparator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("无法检查路径组件: %s", current)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("路径组件不得为符号链接: %s", current)
		}
	}
	return nil
}

func validateRequiredRegularFile(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("缺少必需文件: %s", path)
	}
	if err != nil {
		return fmt.Errorf("无法检查文件: %s", path)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != mode {
		return fmt.Errorf("文件类型或权限无效: %s", path)
	}
	return nil
}

func validateOptionalRegularFile(path string, mode os.FileMode) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return validateRequiredRegularFile(path, mode)
}

func validSessionKey(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func readVersion(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var header struct {
		Version int `json:"version"`
	}
	if err := decoder.Decode(&header); err != nil {
		return 0, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("trailing JSON")
	}
	return header.Version, nil
}
