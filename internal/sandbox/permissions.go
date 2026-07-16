package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type PermissionProfile string

const (
	ControlProfile      PermissionProfile = "mdbench-control"
	TrialProfile        PermissionProfile = "mdbench-trial"
	TrialNetworkProfile PermissionProfile = "mdbench-trial-network"
)

func RenderPermissionConfig(defaultProfile PermissionProfile) ([]byte, string, error) {
	if !validPermissionProfile(defaultProfile) {
		return nil, "", fmt.Errorf("unsupported permission profile %q", defaultProfile)
	}
	content := []byte(fmt.Sprintf(`approval_policy = "never"
default_permissions = %q
allow_login_shell = false
web_search = "disabled"

[shell_environment_policy]
inherit = "none"
ignore_default_excludes = false
set = { PATH = "/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin", HOME = "/home/mdbench", TMPDIR = "/tmp", LANG = "C.UTF-8" }

[permissions.mdbench-control]
description = "Read benchmark control inputs without workspace or credential access."
extends = ":read-only"

[permissions.mdbench-control.filesystem]
":minimal" = "read"
"/control" = "read"
"/artifact" = "read"
"/codex-home" = "deny"
"/home" = "deny"
"/host-home" = "deny"
"/out" = "deny"
"/proc" = "deny"
"/work" = "deny"

[permissions.mdbench-control.network]
enabled = false

[permissions.mdbench-trial]
description = "Edit only the trial workspace; credentials, host paths, output, and network stay unavailable."
extends = ":workspace"

[permissions.mdbench-trial.filesystem]
":minimal" = "read"
"/control" = "read"
"/artifact" = "read"
"/codex-home" = "deny"
"/home" = "deny"
"/host-home" = "deny"
"/out" = "deny"
"/proc" = "deny"
glob_scan_max_depth = 5

[permissions.mdbench-trial.filesystem.":workspace_roots"]
"." = "write"
".codex/skills" = "read"
"**/*.env" = "deny"

[permissions.mdbench-trial.network]
enabled = false

[permissions.mdbench-trial-network]
description = "Edit only the trial workspace with reviewed command network access."
extends = ":workspace"

[permissions.mdbench-trial-network.filesystem]
":minimal" = "read"
"/control" = "read"
"/artifact" = "read"
"/codex-home" = "deny"
"/home" = "deny"
"/host-home" = "deny"
"/out" = "deny"
"/proc" = "deny"
glob_scan_max_depth = 5

[permissions.mdbench-trial-network.filesystem.":workspace_roots"]
"." = "write"
".codex/skills" = "read"
"**/*.env" = "deny"

[permissions.mdbench-trial-network.network]
enabled = true
`, defaultProfile))
	digest := sha256.Sum256(content)
	return content, hex.EncodeToString(digest[:]), nil
}

func WritePermissionConfig(codexHome string, defaultProfile PermissionProfile) (string, string, error) {
	if codexHome == "" {
		return "", "", errors.New("Codex home is required")
	}
	content, digest, err := RenderPermissionConfig(defaultProfile)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return "", "", fmt.Errorf("create isolated Codex home: %w", err)
	}
	temporary, err := os.CreateTemp(codexHome, ".config-*.toml")
	if err != nil {
		return "", "", fmt.Errorf("create temporary Codex config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("protect temporary Codex config: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("write temporary Codex config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("flush temporary Codex config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", "", fmt.Errorf("close temporary Codex config: %w", err)
	}
	path := filepath.Join(codexHome, "config.toml")
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", "", fmt.Errorf("publish Codex config: %w", err)
	}
	return path, digest, nil
}

func validPermissionProfile(profile PermissionProfile) bool {
	return profile == ControlProfile || profile == TrialProfile || profile == TrialNetworkProfile
}
