package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPermissionConfigFailsClosed(t *testing.T) {
	content, digest, err := RenderPermissionConfig(TrialProfile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, required := range []string{
		`approval_policy = "never"`,
		`default_permissions = "mdbench-trial"`,
		`inherit = "none"`,
		`"/codex-home" = "deny"`,
		`"/host-home" = "deny"`,
		`"/proc/*/environ" = "deny"`,
		`"/out" = "deny"`,
		`".codex/skills" = "read"`,
		`[permissions.mdbench-trial.network]` + "\n" + `enabled = false`,
		`[permissions.mdbench-trial-network.network]` + "\n" + `enabled = true`,
	} {
		if !strings.Contains(text, required) {
			t.Errorf("config is missing %q", required)
		}
	}
	if strings.Contains(text, "sandbox_mode") {
		t.Fatal("legacy sandbox settings must not override permission profiles")
	}
	if len(digest) != 64 {
		t.Fatalf("digest length = %d, want 64", len(digest))
	}
}

func TestWritePermissionConfigIsPrivateAndAtomic(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "codex-home")
	path, firstDigest, err := WritePermissionConfig(directory, ControlProfile)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if permission := info.Mode().Perm(); permission != 0o600 {
		t.Fatalf("permission = %o, want 600", permission)
	}
	path, secondDigest, err := WritePermissionConfig(directory, TrialProfile)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest == secondDigest {
		t.Fatal("profile-specific config should have a distinct digest")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `default_permissions = "mdbench-trial"`) {
		t.Fatal("published config did not replace the previous profile")
	}
	matches, err := filepath.Glob(filepath.Join(directory, ".config-*.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary configs remain: %v", matches)
	}
}

func TestPermissionConfigRejectsUnknownProfile(t *testing.T) {
	if _, _, err := RenderPermissionConfig("full-access"); err == nil {
		t.Fatal("unknown profile should fail closed")
	}
}

func TestInstalledCodexAcceptsPermissionConfig(t *testing.T) {
	if os.Getenv("MDBENCH_LIVE_CODEX_CONFIG_TEST") != "1" {
		t.Skip("set MDBENCH_LIVE_CODEX_CONFIG_TEST=1 to check the installed Codex CLI")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("live permission check is supported on macOS and Linux")
	}
	binary, err := exec.LookPath("codex")
	if err != nil {
		t.Skip("Codex CLI is not installed")
	}
	codexHome := filepath.Join(t.TempDir(), "codex-home")
	if _, _, err := WritePermissionConfig(codexHome, TrialProfile); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	command := exec.Command(binary, "sandbox", "--permission-profile", string(TrialProfile), "--cd", workspace, "--", "/usr/bin/true")
	command.Env = append(os.Environ(), "CODEX_HOME="+codexHome)
	if output, err := command.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "sandbox_apply: Operation not permitted") {
			t.Skip("the current host already runs inside a sandbox and blocks nested macOS sandboxing")
		}
		t.Fatalf("Codex rejected generated permission config: %v\n%s", err, output)
	}
}
