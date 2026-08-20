package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/assets"
	"singbox-web/internal/atomicfile"
	"singbox-web/internal/auth"
	"singbox-web/internal/capabilities"
	"singbox-web/internal/configdoc"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/profile"
	"singbox-web/internal/publication"
)

func validOptions(t *testing.T) Options {
	t.Helper()
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	group, err := user.LookupGroupId(current.Gid)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{Listen: "127.0.0.1:0", DataDir: filepath.Join(t.TempDir(), "data"), ConfigRoot: filepath.Join(t.TempDir(), "config"), ConfigGroup: group.Name, SingBoxPath: "/definitely/missing/sing-box"}
	store := auth.NewFileStore(filepath.Join(options.DataDir, "auth.json"), time.Now, bytes.NewReader(bytes.Repeat([]byte{7}, 4096)))
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	return options
}

func withAppHooks(t *testing.T) {
	t.Helper()
	oldRefresh, oldRecover, oldHandler, oldListen := refreshCapabilities, recoverPublication, newHTTPHandler, listenTCP
	t.Cleanup(func() {
		refreshCapabilities, recoverPublication, newHTTPHandler, listenTCP = oldRefresh, oldRecover, oldHandler, oldListen
	})
}

func TestRunRejectsMissingAuthBeforeListen(t *testing.T) {
	group, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	configuredGroup, err := user.LookupGroupId(group.Gid)
	if err != nil {
		t.Fatal(err)
	}
	data, config := filepath.Join(t.TempDir(), "data"), filepath.Join(t.TempDir(), "config")
	err = Run(context.Background(), Options{Listen: "127.0.0.1:0", DataDir: data, ConfigRoot: config, ConfigGroup: configuredGroup.Name, SingBoxPath: "/missing"})
	if err == nil {
		t.Fatal("Run() succeeded without auth.json")
	}
}

func TestValidateEndpoint(t *testing.T) {
	for _, test := range []struct {
		name, listen string
		public, ok   bool
	}{
		{"loopback", "127.0.0.1:8080", false, true},
		{"public denied", "0.0.0.0:8080", false, false},
		{"public allowed", "0.0.0.0:8080", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateNetwork(test.listen, test.public)
			if (err == nil) != test.ok {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestValidateRootRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(root, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := validateRootPath(filepath.Join(root, "linked", "child")); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestRunRejectsHeldAppLock(t *testing.T) {
	options := validOptions(t)
	lock, err := atomicfile.Acquire(filepath.Join(options.DataDir, "app.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := Run(context.Background(), options); err == nil || !strings.Contains(err.Error(), "应用锁") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunRejectsMalformedAndUnknownVersionFilesWithoutChangingBytes(t *testing.T) {
	for _, test := range []struct {
		name, file string
		content    []byte
		class      string
	}{
		{"auth malformed", "auth.json", []byte(`{"version":1,"raw-secret":"do-not-log"`), "JSON 解析失败"},
		{"auth version", "auth.json", []byte(`{"version":2,"raw-secret":"do-not-log"}`), "不支持的版本"},
		{"profiles malformed", "profiles.json", []byte(`{"version":1,"raw-secret":"do-not-log"`), "JSON 解析失败"},
		{"profiles version", "profiles.json", []byte(`{"version":2,"raw-secret":"do-not-log"}`), "不支持的版本"},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := validOptions(t)
			path := filepath.Join(options.DataDir, test.file)
			if err := os.WriteFile(path, test.content, 0o600); err != nil {
				t.Fatal(err)
			}
			err := Run(context.Background(), options)
			after, readErr := os.ReadFile(path)
			if err == nil || readErr != nil || !bytes.Equal(after, test.content) || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), test.class) || strings.Contains(err.Error(), "do-not-log") {
				t.Fatalf("err=%v bytesChanged=%v readErr=%v", err, !bytes.Equal(after, test.content), readErr)
			}
		})
	}
}

func TestRunRejectsUnsafeRootAndFileModes(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, options *Options)
	}{
		{"data mode", func(t *testing.T, options *Options) {
			if err := os.Chmod(options.DataDir, 0o750); err != nil {
				t.Fatal(err)
			}
		}},
		{"config mode", func(t *testing.T, options *Options) {
			if err := os.MkdirAll(options.ConfigRoot, 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(options.ConfigRoot, 0o770); err != nil {
				t.Fatal(err)
			}
			current, _ := user.Current()
			value, _ := strconv.Atoi(current.Gid)
			_ = os.Chown(options.ConfigRoot, -1, value)
		}},
		{"auth mode", func(t *testing.T, options *Options) {
			if err := os.Chmod(filepath.Join(options.DataDir, "auth.json"), 0o640); err != nil {
				t.Fatal(err)
			}
		}},
		{"config symlink", func(t *testing.T, options *Options) {
			target := t.TempDir()
			options.ConfigRoot = filepath.Join(t.TempDir(), "linked")
			if err := os.Symlink(target, options.ConfigRoot); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := validOptions(t)
			test.mutate(t, &options)
			if err := Run(context.Background(), options); err == nil {
				t.Fatal("unsafe startup succeeded")
			}
		})
	}
}

func TestRunRefreshRecoverHandlerBeforeListen(t *testing.T) {
	options := validOptions(t)
	withAppHooks(t)
	var order []string
	refreshCapabilities = func(ctx context.Context, service *capabilities.Service) {
		order = append(order, "refresh")
		service.Refresh(ctx)
	}
	recoverPublication = func(context.Context, *publication.Service) error { order = append(order, "recover"); return nil }
	newHTTPHandler = func(httpapi.Dependencies) http.Handler {
		order = append(order, "handler")
		return http.NotFoundHandler()
	}
	listenTCP = func(string, string) (net.Listener, error) {
		order = append(order, "listen")
		return nil, errors.New("finite stop")
	}
	if err := Run(context.Background(), options); err == nil {
		t.Fatal("Run() unexpectedly succeeded")
	}
	if want := []string{"refresh", "recover", "handler", "listen"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order=%v want=%v", order, want)
	}
}

func TestUnavailableRunnerStillBuildsHandlerWithPublishDisabled(t *testing.T) {
	options := validOptions(t)
	withAppHooks(t)
	var publish bool
	newHTTPHandler = func(deps httpapi.Dependencies) http.Handler {
		publish = deps.Capabilities.Snapshot(context.Background()).Capabilities["config.publish"]
		return http.NotFoundHandler()
	}
	listenTCP = func(string, string) (net.Listener, error) { return nil, errors.New("finite stop") }
	if err := Run(context.Background(), options); err == nil {
		t.Fatal("Run() unexpectedly succeeded")
	}
	if publish {
		t.Fatal("publish capability stayed enabled")
	}
}

func TestCorruptDraftDoesNotBlockHealthyProfileAssembly(t *testing.T) {
	options := validOptions(t)
	repository, err := profile.OpenRepository(options.DataDir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	profiles := profile.NewService(repository, template, rand.Reader)
	broken, _, err := profiles.Create(context.Background(), profile.CreateInput{Name: "broken", Slug: "broken"})
	if err != nil {
		t.Fatal(err)
	}
	healthy, _, err := profiles.Create(context.Background(), profile.CreateInput{Name: "healthy", Slug: "healthy"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repository.DraftPath(broken.ID), []byte(`{"secret":"broken"`), 0o600); err != nil {
		t.Fatal(err)
	}
	withAppHooks(t)
	var brokenStatus, healthyStatus profile.DraftStatus
	newHTTPHandler = func(deps httpapi.Dependencies) http.Handler {
		brokenStatus = deps.Profiles.DraftStatus(context.Background(), broken.ID)
		healthyStatus = deps.Profiles.DraftStatus(context.Background(), healthy.ID)
		return http.NotFoundHandler()
	}
	listenTCP = func(string, string) (net.Listener, error) { return nil, errors.New("finite stop") }
	if err := Run(context.Background(), options); err == nil {
		t.Fatal("Run() unexpectedly succeeded")
	}
	if !brokenStatus.ReadOnly || healthyStatus.ReadOnly {
		t.Fatalf("broken=%#v healthy=%#v", brokenStatus, healthyStatus)
	}
}
