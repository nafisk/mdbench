package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

const cleanupTimeout = 15 * time.Second

type Runtime interface {
	Name() string
	Version(context.Context) (string, error)
	ImageExists(context.Context, string) (bool, error)
	InspectImage(context.Context, string) (ImageInfo, error)
	Start(context.Context, ContainerSpec) (string, error)
	Exec(context.Context, string, ProcessSpec) (ProcessResult, error)
	Stop(context.Context, string, time.Duration) error
	Kill(context.Context, string) error
	Remove(context.Context, string) error
}

type ImageInfo struct {
	ID          string
	RepoDigests []string
	Labels      map[string]string
}

func (i ImageInfo) ImmutableReference() (string, error) {
	for _, reference := range i.RepoDigests {
		if validDigestReference(reference) {
			return reference, nil
		}
	}
	if validImageID(i.ID) {
		return i.ID, nil
	}
	return "", errors.New("container image has no immutable SHA-256 reference")
}

type ContainerSpec struct {
	Name        string
	Image       string
	WorkDir     string
	User        string
	Hostname    string
	Network     NetworkMode
	MemoryBytes int64
	CPUs        string
	PidsLimit   int
	Tmpfs       []TmpfsMount
	Mounts      []BindMount
	Environment []string
	Command     []string
	StopTimeout time.Duration
}

type NetworkMode string

const (
	NetworkNone   NetworkMode = "none"
	NetworkBridge NetworkMode = "bridge"
)

type TmpfsMount struct {
	Target    string
	SizeBytes int64
	Mode      uint32
	UID       int
	GID       int
}

type BindMount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type ProcessSpec struct {
	Argv        []string
	WorkDir     string
	Environment []string
	Stdin       io.Reader
}

type ProcessResult struct {
	Output    string
	ExitCode  int
	Truncated bool
}

func WithContainer(ctx context.Context, runtime Runtime, spec ContainerSpec, work func(context.Context, string) error) (err error) {
	containerID, err := runtime.Start(ctx, spec)
	if err != nil {
		return err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		err = errors.Join(err, Cleanup(cleanupCtx, runtime, containerID, spec.StopTimeout))
	}()
	return work(ctx, containerID)
}

func Cleanup(ctx context.Context, runtime Runtime, containerID string, grace time.Duration) error {
	if containerID == "" {
		return errors.New("container ID is required")
	}
	if grace <= 0 {
		grace = 3 * time.Second
	}

	var cleanupErr error
	if err := runtime.Stop(ctx, containerID, grace); err != nil {
		cleanupErr = fmt.Errorf("stop container: %w", err)
		if killErr := runtime.Kill(ctx, containerID); killErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("kill container: %w", killErr))
		}
	}
	if err := runtime.Remove(ctx, containerID); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove container: %w", err))
	}
	return cleanupErr
}
