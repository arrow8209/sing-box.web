package main

import (
	"os"
	"path/filepath"

	"singbox-web/internal/paths"
)

// installFakeCore creates the deterministic command surface used by later E2E
// stages. The visual suite itself never starts a proxy process.
func installFakeCore(workDir string) error {
	layout, err := paths.Resolve(workDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(layout.BinDir, 0o700); err != nil {
		return err
	}
	path := layout.CoreBinary
	script := "#!/bin/sh\ncase \"$1\" in version) echo 'sing-box version 1.0.0' ;; check) exit 0 ;; run) trap 'exit 0' TERM INT; while :; do sleep 1; done ;; esac\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workDir, "fixture.json"), []byte(`{"password":"visual-only"}`), 0o600)
}
