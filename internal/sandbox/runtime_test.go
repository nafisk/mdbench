package sandbox

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStartArgumentsApplyContainerBoundary(t *testing.T) {
	args, err := startArgs(ContainerSpec{
		Name: "mdbench-trial", Image: "example@sha256:abc", WorkDir: "/work", User: "10001:10001",
		Hostname: "mdbench", Network: NetworkBridge, MemoryBytes: 1 << 30, CPUs: "1", PidsLimit: 128,
		Tmpfs:       []TmpfsMount{{Target: "/work", SizeBytes: 64 << 20, UID: 10001, GID: 10001}},
		Mounts:      []BindMount{{Source: "/safe/control", Target: "/control", ReadOnly: true}},
		Environment: []string{"CODEX_HOME=/codex-home"}, Command: []string{"sleep", "infinity"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"run", "--detach", "--name", "mdbench-trial", "--read-only", "--cap-drop", "ALL",
		"--network", "bridge", "--memory", "1073741824", "--cpus", "1", "--pids-limit", "128",
		"--security-opt", "no-new-privileges", "--workdir", "/work", "--user", "10001:10001",
		"--hostname", "mdbench", "--tmpfs", "/work:rw,nosuid,nodev,size=67108864,mode=0700,uid=10001,gid=10001",
		"--mount", "type=bind,src=/safe/control,dst=/control,readonly", "--env", "CODEX_HOME=/codex-home",
		"example@sha256:abc", "sleep", "infinity",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args:\n got %#v\nwant %#v", args, want)
	}
}

func TestCodexContainerArgumentsScopeNestedSandboxException(t *testing.T) {
	image := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	spec, err := CodexContainerSpec(image)
	if err != nil {
		t.Fatal(err)
	}
	args, err := startArgs(spec)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, required := range []string{
		"--init", "--cap-add SYS_ADMIN", "--cap-add SYS_CHROOT", "--cap-add SETUID",
		"--cap-add SETGID", "--cap-add SYS_PTRACE", "--cap-add NET_ADMIN", "--security-opt seccomp=unconfined",
		"--security-opt apparmor=unconfined",
	} {
		if !strings.Contains(joined, required) {
			t.Errorf("nested Codex args are missing %q: %s", required, joined)
		}
	}
	if strings.Contains(joined, "no-new-privileges") {
		t.Fatal("no-new-privileges would prevent the approved setuid Bubblewrap launcher")
	}

	strict, err := DefaultContainerSpec(image)
	if err != nil {
		t.Fatal(err)
	}
	strictArgs, err := startArgs(strict)
	if err != nil {
		t.Fatal(err)
	}
	strictJoined := strings.Join(strictArgs, " ")
	if !strings.Contains(strictJoined, "no-new-privileges") || strings.Contains(strictJoined, "--cap-add") || strings.Contains(strictJoined, "unconfined") {
		t.Fatalf("strict container inherited Codex exception: %s", strictJoined)
	}
}

func TestDefaultContainerSpecRequiresAndUsesImmutableImage(t *testing.T) {
	metadata, err := EvaluationImageMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if metadata.LocalTag != DefaultImageTag || metadata.User != "10001:10001" {
		t.Fatalf("unexpected image metadata: %#v", metadata)
	}
	if _, err := DefaultContainerSpec(DefaultImageTag); err == nil {
		t.Fatal("mutable image tag should be rejected")
	}
	image := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	spec, err := DefaultContainerSpec(image)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Image != image || spec.Network != NetworkBridge || spec.User != "10001:10001" {
		t.Fatalf("unexpected default spec: %#v", spec)
	}
	if len(spec.Tmpfs) != 4 {
		t.Fatalf("tmpfs mounts = %d, want 4", len(spec.Tmpfs))
	}
}

func TestImmutableReferencePrefersRepositoryDigest(t *testing.T) {
	digest := "registry.example/mdbench@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	value, err := (ImageInfo{ID: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", RepoDigests: []string{digest}}).ImmutableReference()
	if err != nil {
		t.Fatal(err)
	}
	if value != digest {
		t.Fatalf("reference = %q, want %q", value, digest)
	}
}

func TestWithContainerCleansUpAfterCancellation(t *testing.T) {
	runtime := &fakeRuntime{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := WithContainer(ctx, runtime, ContainerSpec{StopTimeout: time.Second}, func(ctx context.Context, containerID string) error {
		if containerID != "container-1" {
			t.Fatalf("container ID = %q", containerID)
		}
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
	if want := []string{"start", "stop", "remove"}; !reflect.DeepEqual(runtime.calls, want) {
		t.Fatalf("calls = %v, want %v", runtime.calls, want)
	}
}

func TestCleanupKillsWhenGracefulStopFails(t *testing.T) {
	runtime := &fakeRuntime{stopErr: errors.New("stuck")}
	err := Cleanup(context.Background(), runtime, "container-1", time.Second)
	if err == nil {
		t.Fatal("expected cleanup error")
	}
	if want := []string{"stop", "kill", "remove"}; !reflect.DeepEqual(runtime.calls, want) {
		t.Fatalf("calls = %v, want %v", runtime.calls, want)
	}
}

type fakeRuntime struct {
	calls   []string
	stopErr error
}

func (f *fakeRuntime) Name() string                                      { return "fake" }
func (f *fakeRuntime) Version(context.Context) (string, error)           { return "fake 1", nil }
func (f *fakeRuntime) ImageExists(context.Context, string) (bool, error) { return true, nil }
func (f *fakeRuntime) InspectImage(context.Context, string) (ImageInfo, error) {
	return ImageInfo{ID: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, nil
}
func (f *fakeRuntime) Start(context.Context, ContainerSpec) (string, error) {
	f.calls = append(f.calls, "start")
	return "container-1", nil
}
func (f *fakeRuntime) Exec(context.Context, string, ProcessSpec) (ProcessResult, error) {
	f.calls = append(f.calls, "exec")
	return ProcessResult{}, nil
}
func (f *fakeRuntime) Stop(context.Context, string, time.Duration) error {
	f.calls = append(f.calls, "stop")
	return f.stopErr
}
func (f *fakeRuntime) Kill(context.Context, string) error {
	f.calls = append(f.calls, "kill")
	return nil
}
func (f *fakeRuntime) Remove(context.Context, string) error {
	f.calls = append(f.calls, "remove")
	return nil
}
