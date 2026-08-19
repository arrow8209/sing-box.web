package app

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

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
	err = Run(context.Background(), Options{Listen: "127.0.0.1:0", DataDir: data, ConfigRoot: config, ConfigGroup: configuredGroup.Name, PublicOrigin: "https://admin.example", SingBoxPath: "/missing"})
	if err == nil {
		t.Fatal("Run() succeeded without auth.json")
	}
}

func TestValidateEndpointAndOrigin(t *testing.T) {
	for _, test := range []struct {
		name, listen, origin string
		public, dev, ok      bool
	}{
		{"loopback production", "127.0.0.1:8080", "https://admin.example", false, false, true},
		{"public denied", "0.0.0.0:8080", "https://admin.example", false, false, false},
		{"public allowed", "0.0.0.0:8080", "https://admin.example", true, false, true},
		{"trailing slash", "127.0.0.1:8080", "https://admin.example/", false, false, false},
		{"http production", "127.0.0.1:8080", "http://127.0.0.1:8080", false, false, false},
		{"http dev loopback", "127.0.0.1:8080", "http://127.0.0.1:8080", false, true, true},
		{"http dev public origin", "127.0.0.1:8080", "http://example.com", false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateNetwork(test.listen, test.origin, test.public, test.dev)
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
