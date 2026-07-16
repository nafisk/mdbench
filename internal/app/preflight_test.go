package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nafiskhan/mdbench/internal/sandbox"
)

func TestPreflightRunsAndCachesBoundaryCanary(t *testing.T) {
	runtime := &fakePreflightRuntime{}
	auth := &AuthStatus{Kind: "api-key"}
	preflight := Preflight{
		CacheDir: t.TempDir(), Image: sandbox.DefaultImageTag, Runtime: runtime, Auth: auth,
		Now: func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) },
	}
	report, err := preflight.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready || runtime.starts != 1 || runtime.stops != 1 || runtime.removes != 1 {
		t.Fatalf("unexpected first preflight: report=%#v runtime=%#v", report, runtime)
	}
	if got, want := checkStatuses(report.Checks), []CheckStatus{CheckPassed, CheckPassed, CheckPassed, CheckPassed, CheckPassed, CheckPassed}; !reflect.DeepEqual(got, want) {
		t.Fatalf("check statuses = %v, want %v", got, want)
	}

	cached, err := preflight.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !cached.Ready || runtime.starts != 1 {
		t.Fatalf("cached preflight started another container: report=%#v starts=%d", cached, runtime.starts)
	}
	if got, want := checkStatuses(cached.Checks), []CheckStatus{CheckPassed, CheckPassed, CheckPassed, CheckCached, CheckCached, CheckCached}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cached statuses = %v, want %v", got, want)
	}
	cachePath := filepath.Join(preflight.CacheDir, "preflight-canaries", report.CanaryCacheKey+".json")
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cache permission = %o, want 600", info.Mode().Perm())
	}
}

func TestPreflightBlocksCredentialBoundaryFailure(t *testing.T) {
	runtime := &fakePreflightRuntime{credentialReadable: true}
	preflight := Preflight{CacheDir: t.TempDir(), Image: sandbox.DefaultImageTag, Runtime: runtime, Auth: &AuthStatus{Kind: "api-key"}}
	report, err := preflight.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("error = %v, want credential denial", err)
	}
	if report.Ready {
		t.Fatal("failed boundary must not be ready")
	}
	if _, err := os.Stat(filepath.Join(preflight.CacheDir, "preflight-canaries", report.CanaryCacheKey+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed canary was cached: %v", err)
	}
}

func TestDetectCodexAuthDoesNotReadAPIKey(t *testing.T) {
	t.Setenv("CODEX_API_KEY", "test-value-is-never-returned")
	status, err := DetectCodexAuth()
	if err != nil {
		t.Fatal(err)
	}
	if status.Kind != "api-key" || status.Path != "" {
		t.Fatalf("unexpected auth status: %#v", status)
	}
}

func TestCodexVersionLineIgnoresBoundedWarnings(t *testing.T) {
	output := "WARNING: read-only home\ncodex-cli 0.144.3\n"
	if got := codexVersionLine(output); got != "codex-cli 0.144.3" {
		t.Fatalf("version = %q", got)
	}
}

func TestParseBoundaryProbeIgnoresBoundedWarnings(t *testing.T) {
	output := "WARNING: read-only home\n{\"workspace_write\":true,\"control_read\":true}\n"
	var probe boundaryProbe
	if err := parseBoundaryProbe(output, &probe); err != nil {
		t.Fatal(err)
	}
	if !probe.WorkspaceWrite || !probe.ControlRead {
		t.Fatalf("unexpected probe: %#v", probe)
	}
}

func TestLiveContainerPreflight(t *testing.T) {
	base := os.Getenv("MDBENCH_LIVE_PREFLIGHT_CACHE")
	if base == "" {
		t.Skip("set MDBENCH_LIVE_PREFLIGHT_CACHE to a host directory shared with Docker or Podman")
	}
	cacheDir, err := os.MkdirTemp(base, "mdbench-live-preflight-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })
	runtimeClient, err := sandbox.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	preflight := Preflight{CacheDir: cacheDir, Image: sandbox.DefaultImageTag, Runtime: runtimeClient}
	report, err := preflight.Run(context.Background())
	if err != nil {
		t.Fatalf("live preflight failed: %v\nreport: %#v", err, report)
	}
	if !report.Ready {
		t.Fatalf("live preflight is not ready: %#v", report)
	}
	cached, err := preflight.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !cached.Ready || checkStatus(cached.Checks, "boundary-canary") != CheckCached {
		t.Fatalf("live canary was not reused: %#v", cached)
	}
}

func TestLiveStrictContainerBoundary(t *testing.T) {
	base := os.Getenv("MDBENCH_LIVE_PREFLIGHT_CACHE")
	if base == "" {
		t.Skip("set MDBENCH_LIVE_PREFLIGHT_CACHE to a host directory shared with Docker or Podman")
	}
	root, err := os.MkdirTemp(base, "mdbench-live-strict-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	controlDir := filepath.Join(root, "control")
	if err := os.MkdirAll(controlDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir, "public.txt"), []byte("strict boundary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtimeClient, err := sandbox.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	image, err := runtimeClient.InspectImage(context.Background(), sandbox.DefaultImageTag)
	if err != nil {
		t.Fatal(err)
	}
	immutable, err := image.ImmutableReference()
	if err != nil {
		t.Fatal(err)
	}
	spec, err := sandbox.DefaultContainerSpec(immutable)
	if err != nil {
		t.Fatal(err)
	}
	spec.Name = "mdbench-live-strict-" + time.Now().Format("150405.000000")
	spec.Network = sandbox.NetworkNone
	spec.Mounts = append(spec.Mounts, sandbox.BindMount{Source: controlDir, Target: "/control", ReadOnly: true})
	var probe boundaryProbe
	err = sandbox.WithContainer(context.Background(), runtimeClient, spec, func(ctx context.Context, containerID string) error {
		result, err := runtimeClient.Exec(ctx, containerID, sandbox.ProcessSpec{Argv: []string{"mdbench-boundary-probe"}})
		if err != nil {
			return err
		}
		return parseBoundaryProbe(result.Output, &probe)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := probe.validate(); err != nil {
		t.Fatal(err)
	}
}

func checkStatus(checks []PreflightCheck, name string) CheckStatus {
	for _, check := range checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}

func checkStatuses(checks []PreflightCheck) []CheckStatus {
	statuses := make([]CheckStatus, len(checks))
	for index, check := range checks {
		statuses[index] = check.Status
	}
	return statuses
}

type fakePreflightRuntime struct {
	starts             int
	stops              int
	removes            int
	credentialReadable bool
}

func (f *fakePreflightRuntime) Name() string { return "docker" }
func (f *fakePreflightRuntime) Version(context.Context) (string, error) {
	return "Docker version 29.5.2", nil
}
func (f *fakePreflightRuntime) ImageExists(context.Context, string) (bool, error) { return true, nil }
func (f *fakePreflightRuntime) InspectImage(context.Context, string) (sandbox.ImageInfo, error) {
	return sandbox.ImageInfo{
		ID: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Labels: map[string]string{
			"io.mdbench.image.version": sandbox.ImageLabelVersion,
			"io.mdbench.codex.version": sandbox.ExpectedCodexVersion,
		},
	}, nil
}
func (f *fakePreflightRuntime) Start(_ context.Context, spec sandbox.ContainerSpec) (string, error) {
	f.starts++
	if spec.Image != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || spec.Network != sandbox.NetworkBridge {
		return "", errors.New("unsafe preflight spec")
	}
	return "preflight-container", nil
}
func (f *fakePreflightRuntime) Exec(_ context.Context, _ string, spec sandbox.ProcessSpec) (sandbox.ProcessResult, error) {
	if reflect.DeepEqual(spec.Argv, []string{"mdbench-boundary-probe", "network"}) {
		return sandbox.ProcessResult{}, nil
	}
	if reflect.DeepEqual(spec.Argv, []string{"codex", "--version"}) {
		return sandbox.ProcessResult{Output: "codex-cli " + sandbox.ExpectedCodexVersion + "\n"}, nil
	}
	if len(spec.Argv) > 2 && spec.Argv[0] == "codex" && spec.Argv[1] == "sandbox" {
		probe := boundaryProbe{WorkspaceWrite: true, ControlRead: true, CredentialRead: f.credentialReadable}
		content, _ := json.Marshal(probe)
		return sandbox.ProcessResult{Output: string(content)}, nil
	}
	return sandbox.ProcessResult{}, errors.New("unexpected exec")
}
func (f *fakePreflightRuntime) Stop(context.Context, string, time.Duration) error {
	f.stops++
	return nil
}
func (f *fakePreflightRuntime) Kill(context.Context, string) error { return nil }
func (f *fakePreflightRuntime) Remove(context.Context, string) error {
	f.removes++
	return nil
}
