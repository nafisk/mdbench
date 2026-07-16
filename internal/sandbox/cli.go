package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxRuntimeOutput = 1 << 20

type CLI struct {
	kind   string
	binary string
	run    commandRunner
}

type commandRunner interface {
	Run(context.Context, string, []string, ProcessSpec) (ProcessResult, error)
}

type execRunner struct{}

func NewCLI(kind, binary string) (*CLI, error) {
	if kind != "docker" && kind != "podman" {
		return nil, fmt.Errorf("unsupported container runtime %q", kind)
	}
	if binary == "" {
		var err error
		binary, err = exec.LookPath(kind)
		if err != nil {
			return nil, fmt.Errorf("find %s: %w", kind, err)
		}
	}
	return &CLI{kind: kind, binary: binary, run: execRunner{}}, nil
}

func Detect(ctx context.Context) (*CLI, error) {
	var unavailable []error
	for _, kind := range []string{"docker", "podman"} {
		runtime, err := NewCLI(kind, "")
		if err != nil {
			unavailable = append(unavailable, err)
			continue
		}
		if _, err := runtime.Version(ctx); err == nil {
			return runtime, nil
		} else {
			unavailable = append(unavailable, err)
		}
	}
	return nil, fmt.Errorf("no usable Docker or Podman runtime: %w", errors.Join(unavailable...))
}

func (c *CLI) Name() string { return c.kind }

func (c *CLI) Version(ctx context.Context) (string, error) {
	result, err := c.run.Run(ctx, c.binary, []string{"--version"}, ProcessSpec{})
	if err != nil {
		return "", fmt.Errorf("read %s version: %w", c.kind, err)
	}
	return strings.TrimSpace(result.Output), nil
}

func (c *CLI) ImageExists(ctx context.Context, image string) (bool, error) {
	if image == "" {
		return false, errors.New("image is required")
	}
	_, err := c.InspectImage(ctx, image)
	if err == nil {
		return true, nil
	}
	var exitErr *CommandError
	if errors.As(err, &exitErr) && missingImageOutput(exitErr.Output) {
		return false, nil
	}
	return false, fmt.Errorf("inspect image: %w", err)
}

func (c *CLI) InspectImage(ctx context.Context, image string) (ImageInfo, error) {
	if image == "" {
		return ImageInfo{}, errors.New("image is required")
	}
	result, err := c.run.Run(ctx, c.binary, []string{"image", "inspect", image}, ProcessSpec{})
	if err != nil {
		return ImageInfo{}, err
	}
	var records []struct {
		ID          string   `json:"Id"`
		Digest      string   `json:"Digest"`
		RepoDigests []string `json:"RepoDigests"`
		Config      struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.Unmarshal([]byte(result.Output), &records); err != nil {
		return ImageInfo{}, fmt.Errorf("parse %s image inspection: %w", c.kind, err)
	}
	if len(records) != 1 {
		return ImageInfo{}, fmt.Errorf("%s returned %d image records, want one", c.kind, len(records))
	}
	record := records[0]
	if record.ID != "" && !strings.HasPrefix(record.ID, "sha256:") {
		record.ID = "sha256:" + record.ID
	}
	if len(record.RepoDigests) == 0 && strings.HasPrefix(record.Digest, "sha256:") {
		record.RepoDigests = []string{record.Digest}
	}
	return ImageInfo{ID: record.ID, RepoDigests: record.RepoDigests, Labels: record.Config.Labels}, nil
}

func missingImageOutput(output string) bool {
	output = strings.ToLower(output)
	return strings.Contains(output, "no such image") || strings.Contains(output, "image not known") || strings.Contains(output, "image not found")
}

func (c *CLI) Start(ctx context.Context, spec ContainerSpec) (string, error) {
	args, err := startArgs(spec)
	if err != nil {
		return "", err
	}
	result, err := c.run.Run(ctx, c.binary, args, ProcessSpec{})
	if err != nil {
		return "", fmt.Errorf("start %s container: %w", c.kind, err)
	}
	containerID := strings.TrimSpace(result.Output)
	if containerID == "" || strings.ContainsAny(containerID, " \t\r\n") {
		return "", fmt.Errorf("%s returned an invalid container ID", c.kind)
	}
	return containerID, nil
}

func startArgs(spec ContainerSpec) ([]string, error) {
	if spec.Name == "" || spec.Image == "" {
		return nil, errors.New("container name and image are required")
	}
	if spec.Network != NetworkNone && spec.Network != NetworkBridge {
		return nil, fmt.Errorf("unsupported network mode %q", spec.Network)
	}
	if spec.MemoryBytes <= 0 || spec.CPUs == "" || spec.PidsLimit <= 0 {
		return nil, errors.New("memory, CPU, and process limits must be positive")
	}
	if len(spec.Command) == 0 {
		return nil, errors.New("container command is required")
	}

	args := []string{
		"run", "--detach", "--name", spec.Name,
		"--read-only", "--cap-drop", "ALL",
		"--network", string(spec.Network),
		"--memory", strconv.FormatInt(spec.MemoryBytes, 10),
		"--cpus", spec.CPUs,
		"--pids-limit", strconv.Itoa(spec.PidsLimit),
	}
	if spec.NestedCodexSandbox {
		args = append(args,
			"--init",
			"--cap-add", "SYS_ADMIN",
			"--cap-add", "SYS_CHROOT",
			"--cap-add", "SETUID",
			"--cap-add", "SETGID",
			"--cap-add", "SYS_PTRACE",
			"--cap-add", "NET_ADMIN",
			"--security-opt", "seccomp=unconfined",
			"--security-opt", "apparmor=unconfined",
		)
	} else {
		args = append(args, "--security-opt", "no-new-privileges")
	}
	if spec.WorkDir != "" {
		args = append(args, "--workdir", spec.WorkDir)
	}
	if spec.User != "" {
		args = append(args, "--user", spec.User)
	}
	if spec.Hostname != "" {
		args = append(args, "--hostname", spec.Hostname)
	}
	for _, mount := range spec.Tmpfs {
		if !filepath.IsAbs(mount.Target) || mount.SizeBytes <= 0 {
			return nil, fmt.Errorf("invalid tmpfs mount %q", mount.Target)
		}
		mode := mount.Mode
		if mode == 0 {
			mode = 0o700
		}
		value := fmt.Sprintf("%s:rw,nosuid,nodev,size=%d,mode=%04o", mount.Target, mount.SizeBytes, mode)
		if mount.UID > 0 {
			value += ",uid=" + strconv.Itoa(mount.UID)
		}
		if mount.GID > 0 {
			value += ",gid=" + strconv.Itoa(mount.GID)
		}
		args = append(args, "--tmpfs", value)
	}
	for _, mount := range spec.Mounts {
		if !filepath.IsAbs(mount.Source) || !filepath.IsAbs(mount.Target) {
			return nil, fmt.Errorf("bind mount paths must be absolute: %q -> %q", mount.Source, mount.Target)
		}
		value := "type=bind,src=" + mount.Source + ",dst=" + mount.Target
		if mount.ReadOnly {
			value += ",readonly"
		}
		args = append(args, "--mount", value)
	}
	for _, value := range spec.Environment {
		if !validEnvironment(value) {
			return nil, fmt.Errorf("invalid environment entry %q", value)
		}
		args = append(args, "--env", value)
	}
	args = append(args, spec.Image)
	args = append(args, spec.Command...)
	return args, nil
}

func validEnvironment(value string) bool {
	name, _, ok := strings.Cut(value, "=")
	if !ok || name == "" {
		return false
	}
	for index, r := range name {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || index > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func (c *CLI) Exec(ctx context.Context, containerID string, spec ProcessSpec) (ProcessResult, error) {
	if containerID == "" || len(spec.Argv) == 0 {
		return ProcessResult{}, errors.New("container ID and process argv are required")
	}
	args := []string{"exec"}
	if spec.WorkDir != "" {
		args = append(args, "--workdir", spec.WorkDir)
	}
	for _, value := range spec.Environment {
		if !validEnvironment(value) {
			return ProcessResult{}, fmt.Errorf("invalid environment entry %q", value)
		}
		args = append(args, "--env", value)
	}
	args = append(args, containerID)
	args = append(args, spec.Argv...)
	return c.run.Run(ctx, c.binary, args, spec)
}

func (c *CLI) Stop(ctx context.Context, containerID string, grace time.Duration) error {
	seconds := max(int(grace.Round(time.Second)/time.Second), 1)
	_, err := c.run.Run(ctx, c.binary, []string{"stop", "--time", strconv.Itoa(seconds), containerID}, ProcessSpec{})
	return err
}

func (c *CLI) Kill(ctx context.Context, containerID string) error {
	_, err := c.run.Run(ctx, c.binary, []string{"kill", containerID}, ProcessSpec{})
	return err
}

func (c *CLI) Remove(ctx context.Context, containerID string) error {
	_, err := c.run.Run(ctx, c.binary, []string{"rm", "--force", containerID}, ProcessSpec{})
	return err
}

type CommandError struct {
	Command  string
	ExitCode int
	Output   string
}

func (e *CommandError) Error() string {
	if e.Output == "" {
		return fmt.Sprintf("%s exited with status %d", e.Command, e.ExitCode)
	}
	return fmt.Sprintf("%s exited with status %d: %s", e.Command, e.ExitCode, e.Output)
}

func (execRunner) Run(ctx context.Context, binary string, args []string, spec ProcessSpec) (ProcessResult, error) {
	command := exec.CommandContext(ctx, binary, args...)
	command.Stdin = spec.Stdin
	output := &limitedBuffer{limit: maxRuntimeOutput}
	command.Stdout = output
	command.Stderr = output
	err := command.Run()
	result := ProcessResult{Output: output.String(), Truncated: output.truncated}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, &CommandError{Command: filepath.Base(binary), ExitCode: result.ExitCode, Output: strings.TrimSpace(result.Output)}
	}
	return result, err
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	original := len(value)
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || original > 0
		return original, nil
	}
	if len(value) > remaining {
		value = value[:remaining]
		b.truncated = true
	}
	_, err := b.buffer.Write(value)
	return original, err
}

func (b *limitedBuffer) String() string { return b.buffer.String() }
