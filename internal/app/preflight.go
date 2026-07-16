package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nafiskhan/mdbench/internal/sandbox"
)

type CheckStatus string

const (
	CheckPassed CheckStatus = "passed"
	CheckFailed CheckStatus = "failed"
	CheckCached CheckStatus = "cached"
)

type PreflightCheck struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Detail string      `json:"detail"`
}

type PreflightReport struct {
	Ready          bool             `json:"ready"`
	RuntimeName    string           `json:"runtime_name"`
	RuntimeVersion string           `json:"runtime_version"`
	Image          string           `json:"image"`
	CodexVersion   string           `json:"codex_version"`
	PermissionSHA  string           `json:"permission_sha"`
	RuntimePolicy  string           `json:"runtime_policy"`
	Authentication string           `json:"authentication"`
	CanaryCacheKey string           `json:"canary_cache_key"`
	Checks         []PreflightCheck `json:"checks"`
}

type AuthStatus struct {
	Kind string
	Path string
}

type Preflight struct {
	CacheDir string
	Image    string
	Runtime  sandbox.Runtime
	Auth     *AuthStatus
	Now      func() time.Time
}

type canaryRecord struct {
	SchemaVersion int           `json:"schema_version"`
	Key           string        `json:"key"`
	CodexVersion  string        `json:"codex_version"`
	PassedAt      time.Time     `json:"passed_at"`
	Probe         boundaryProbe `json:"probe"`
}

type boundaryProbe struct {
	WorkspaceWrite bool `json:"workspace_write"`
	RootWrite      bool `json:"root_write"`
	ControlRead    bool `json:"control_read"`
	CredentialRead bool `json:"credential_read"`
	HostRead       bool `json:"host_read"`
	NetworkConnect bool `json:"network_connect"`
}

func NewPreflight(config Config) Preflight {
	return Preflight{CacheDir: config.CacheDir, Image: sandbox.DefaultImageTag}
}

func (p Preflight) Run(ctx context.Context) (PreflightReport, error) {
	report := PreflightReport{}
	runtimeClient := p.Runtime
	if runtimeClient == nil {
		var err error
		runtimeClient, err = sandbox.Detect(ctx)
		if err != nil {
			return failPreflight(report, "runtime", "Docker or Podman is not available.", err)
		}
	}
	report.RuntimeName = runtimeClient.Name()
	runtimeVersion, err := runtimeClient.Version(ctx)
	if err != nil {
		return failPreflight(report, "runtime", "The container runtime is installed but unavailable.", err)
	}
	report.RuntimeVersion = runtimeVersion
	report.Checks = append(report.Checks, passedCheck("runtime", runtimeClient.Name()+" is available"))

	image := p.Image
	if image == "" {
		image = sandbox.DefaultImageTag
	}
	imageInfo, err := runtimeClient.InspectImage(ctx, image)
	if err != nil {
		return failPreflight(report, "image", "Build the mdbench evaluation image before running an evaluation.", err)
	}
	immutableImage, err := imageInfo.ImmutableReference()
	if err != nil {
		return failPreflight(report, "image", "The evaluation image could not be resolved to an immutable digest.", err)
	}
	if imageInfo.Labels["io.mdbench.image.version"] != sandbox.ImageLabelVersion || imageInfo.Labels["io.mdbench.codex.version"] != sandbox.ExpectedCodexVersion {
		return failPreflight(report, "image", "The evaluation image labels do not match this mdbench build.", errors.New("evaluation image version mismatch"))
	}
	report.Image = immutableImage
	report.Checks = append(report.Checks, passedCheck("image", "immutable image and labels match"))

	auth := p.Auth
	if auth == nil {
		detected, detectErr := DetectCodexAuth()
		if detectErr != nil {
			return failPreflight(report, "authentication", "Sign in with Codex or set CODEX_API_KEY for this run.", detectErr)
		}
		auth = &detected
	}
	if auth.Kind != "api-key" && auth.Kind != "auth-file" {
		return failPreflight(report, "authentication", "Use a supported Codex authentication method.", fmt.Errorf("unsupported authentication kind %q", auth.Kind))
	}
	if auth.Kind == "auth-file" {
		if err := validateAuthFile(auth.Path); err != nil {
			return failPreflight(report, "authentication", "Saved Codex authentication is unavailable or not private.", err)
		}
	}
	report.Authentication = auth.Kind
	report.Checks = append(report.Checks, passedCheck("authentication", "Codex authentication is available"))

	_, permissionSHA, err := sandbox.RenderPermissionConfig(sandbox.TrialProfile)
	if err != nil {
		return failPreflight(report, "permission-profile", "The mdbench permission profile is invalid.", err)
	}
	report.PermissionSHA = permissionSHA
	report.RuntimePolicy = sandbox.CodexRuntimePolicyVersion
	report.CodexVersion = sandbox.ExpectedCodexVersion
	// ponytail: the policy is fixed for the MVP; bump its version whenever the trusted launcher flags change.
	report.CanaryCacheKey = preflightCacheKey(runtimeClient.Name(), runtimeVersion, immutableImage, sandbox.ExpectedCodexVersion, permissionSHA, sandbox.CodexRuntimePolicyVersion)

	record, cached, err := loadCanary(p.CacheDir, report.CanaryCacheKey)
	if err != nil {
		return failPreflight(report, "boundary-canary", "The saved boundary check could not be read safely.", err)
	}
	if cached {
		report.CodexVersion = record.CodexVersion
		report.Checks = append(report.Checks,
			cachedCheck("codex-version", "validated by the matching cached boundary check"),
			cachedCheck("permission-profile", "matching permission profile already passed"),
			cachedCheck("boundary-canary", "matching isolation boundary already passed"),
		)
		report.Ready = true
		return report, nil
	}

	probeRoot := filepath.Join(p.CacheDir, "preflight", report.CanaryCacheKey)
	codexHome := filepath.Join(probeRoot, "codex-home")
	controlDir := filepath.Join(probeRoot, "control")
	hostDir := filepath.Join(probeRoot, "host-home")
	for _, directory := range []string{codexHome, controlDir, filepath.Join(hostDir, ".ssh")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return failPreflight(report, "permission-profile", "The preflight workspace could not be created.", err)
		}
	}
	if _, _, err := sandbox.WritePermissionConfig(codexHome, sandbox.TrialProfile); err != nil {
		return failPreflight(report, "permission-profile", "The isolated Codex config could not be created.", err)
	}
	if err := writePrivateFile(filepath.Join(codexHome, "auth.json"), []byte("{}\n")); err != nil {
		return failPreflight(report, "permission-profile", "The credential-denial probe could not be prepared.", err)
	}
	if err := writePrivateFile(filepath.Join(controlDir, "public.txt"), []byte("mdbench boundary probe\n")); err != nil {
		return failPreflight(report, "permission-profile", "The control-read probe could not be prepared.", err)
	}

	spec, err := sandbox.CodexContainerSpec(immutableImage)
	if err != nil {
		return failPreflight(report, "image", "The immutable image could not be applied.", err)
	}
	spec.Name = "mdbench-preflight-" + report.CanaryCacheKey[:12]
	spec.Command = []string{"mdbench-boundary-probe", "serve"}
	filteredTmpfs := make([]sandbox.TmpfsMount, 0, len(spec.Tmpfs))
	for _, mount := range spec.Tmpfs {
		if mount.Target != "/codex-home" {
			filteredTmpfs = append(filteredTmpfs, mount)
		}
	}
	spec.Tmpfs = filteredTmpfs
	spec.Mounts = append(spec.Mounts,
		sandbox.BindMount{Source: codexHome, Target: "/codex-home", ReadOnly: true},
		sandbox.BindMount{Source: controlDir, Target: "/control", ReadOnly: true},
		sandbox.BindMount{Source: hostDir, Target: "/host-home", ReadOnly: true},
	)

	var probe boundaryProbe
	failingCheck := "codex-version"
	err = sandbox.WithContainer(ctx, runtimeClient, spec, func(ctx context.Context, containerID string) error {
		if err := waitForProbeListener(ctx, runtimeClient, containerID); err != nil {
			return fmt.Errorf("start canary listener: %w", err)
		}
		versionResult, err := runtimeClient.Exec(ctx, containerID, sandbox.ProcessSpec{Argv: []string{"codex", "--version"}})
		if err != nil {
			return fmt.Errorf("read container Codex version: %w", err)
		}
		expectedVersion := "codex-cli " + sandbox.ExpectedCodexVersion
		actualVersion := codexVersionLine(versionResult.Output)
		if actualVersion != expectedVersion {
			return fmt.Errorf("container Codex version is %q, want %q", actualVersion, expectedVersion)
		}
		report.Checks = append(report.Checks, passedCheck("codex-version", expectedVersion))

		failingCheck = "permission-profile"
		profileResult, err := runtimeClient.Exec(ctx, containerID, sandbox.ProcessSpec{
			Argv: []string{"codex", "sandbox", "--permission-profile", string(sandbox.TrialProfile), "--cd", "/work", "--", "mdbench-boundary-probe"},
		})
		if err != nil {
			return fmt.Errorf("run Codex permission profile: %w", err)
		}
		report.Checks = append(report.Checks, passedCheck("permission-profile", "Codex accepted and enforced mdbench-trial"))
		failingCheck = "boundary-canary"
		if err := parseBoundaryProbe(profileResult.Output, &probe); err != nil {
			return fmt.Errorf("parse boundary probe: %w", err)
		}
		return probe.validate()
	})
	if err != nil {
		detail := "The local isolation canary failed; model calls remain blocked."
		if failingCheck == "codex-version" {
			detail = "The Codex CLI inside the image does not match this mdbench build."
		} else if failingCheck == "permission-profile" {
			detail = "Codex could not enforce the mdbench permission profile; model calls remain blocked."
		}
		return failPreflight(report, failingCheck, detail, err)
	}
	report.Checks = append(report.Checks, passedCheck("boundary-canary", "writes are bounded; credentials, host paths, and command network are denied"))

	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}
	record = canaryRecord{SchemaVersion: 1, Key: report.CanaryCacheKey, CodexVersion: sandbox.ExpectedCodexVersion, PassedAt: now, Probe: probe}
	if err := saveCanary(p.CacheDir, record); err != nil {
		return failPreflight(report, "boundary-canary", "The successful boundary check could not be cached.", err)
	}
	report.Ready = true
	return report, nil
}

func codexVersionLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "codex-cli ") {
			return line
		}
	}
	return strings.TrimSpace(output)
}

func parseBoundaryProbe(output string, probe *boundaryProbe) error {
	var lastErr error
	lines := strings.Split(output, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		if err := json.Unmarshal([]byte(line), probe); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("boundary probe returned no JSON object")
}

func DetectCodexAuth() (AuthStatus, error) {
	if value, ok := os.LookupEnv("CODEX_API_KEY"); ok && value != "" {
		return AuthStatus{Kind: "api-key"}, nil
	}
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return AuthStatus{}, fmt.Errorf("resolve home directory: %w", err)
		}
		codexHome = filepath.Join(home, ".codex")
	}
	authPath := filepath.Join(codexHome, "auth.json")
	if err := validateAuthFile(authPath); err != nil {
		return AuthStatus{}, err
	}
	return AuthStatus{Kind: "auth-file", Path: authPath}, nil
}

func validateAuthFile(authPath string) error {
	if !filepath.IsAbs(authPath) {
		return errors.New("saved Codex authentication path must be absolute")
	}
	info, err := os.Lstat(authPath)
	if err != nil {
		return fmt.Errorf("find saved Codex authentication: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("saved Codex authentication must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("saved Codex authentication must not be readable by group or other users")
	}
	return nil
}

func (p boundaryProbe) validate() error {
	if !p.WorkspaceWrite {
		return errors.New("trial workspace is not writable")
	}
	if p.RootWrite {
		return errors.New("read-only container root accepted a write")
	}
	if !p.ControlRead {
		return errors.New("read-only control input is unavailable")
	}
	if p.CredentialRead {
		return errors.New("model-run command read the Codex credential path")
	}
	if p.HostRead {
		return errors.New("model-run command read the host-path probe")
	}
	if p.NetworkConnect {
		return errors.New("model-run command reached the canary network listener")
	}
	return nil
}

func waitForProbeListener(ctx context.Context, runtimeClient sandbox.Runtime, containerID string) error {
	var lastErr error
	for attempt := 0; attempt < 20; attempt++ {
		if _, err := runtimeClient.Exec(ctx, containerID, sandbox.ProcessSpec{Argv: []string{"mdbench-boundary-probe", "network"}}); err == nil {
			return nil
		} else {
			lastErr = err
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func preflightCacheKey(values ...string) string {
	digest := sha256.New()
	for _, value := range append(values, runtime.GOOS, runtime.GOARCH) {
		_, _ = io.WriteString(digest, value)
		_, _ = digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func loadCanary(cacheDir, key string) (canaryRecord, bool, error) {
	path := filepath.Join(cacheDir, "preflight-canaries", key+".json")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return canaryRecord{}, false, nil
	}
	if err != nil {
		return canaryRecord{}, false, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	var record canaryRecord
	if err := decoder.Decode(&record); err != nil {
		return canaryRecord{}, false, err
	}
	if record.SchemaVersion != 1 || record.Key != key || record.CodexVersion != sandbox.ExpectedCodexVersion {
		return canaryRecord{}, false, errors.New("boundary canary cache does not match the current environment")
	}
	if err := record.Probe.validate(); err != nil {
		return canaryRecord{}, false, fmt.Errorf("cached boundary canary is invalid: %w", err)
	}
	return record, true, nil
}

func saveCanary(cacheDir string, record canaryRecord) error {
	directory := filepath.Join(cacheDir, "preflight-canaries")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	temporary, err := os.CreateTemp(directory, ".canary-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, filepath.Join(directory, record.Key+".json"))
}

func writePrivateFile(path string, content []byte) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func failPreflight(report PreflightReport, name, detail string, err error) (PreflightReport, error) {
	report.Checks = append(report.Checks, PreflightCheck{Name: name, Status: CheckFailed, Detail: detail})
	return report, fmt.Errorf("preflight %s: %w", name, err)
}

func passedCheck(name, detail string) PreflightCheck {
	return PreflightCheck{Name: name, Status: CheckPassed, Detail: detail}
}

func cachedCheck(name, detail string) PreflightCheck {
	return PreflightCheck{Name: name, Status: CheckCached, Detail: detail}
}
