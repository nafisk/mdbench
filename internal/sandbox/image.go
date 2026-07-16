package sandbox

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultImageTag           = "mdbench-eval:0.1.0"
	ExpectedCodexVersion      = "0.144.3"
	ImageLabelVersion         = "0.1.0"
	CodexRuntimePolicyVersion = "codex-nested-v1"
)

//go:embed image.json
var imageMetadataJSON []byte

type ImageMetadata struct {
	SchemaVersion int    `json:"schema_version"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	LocalTag      string `json:"local_tag"`
	CodexVersion  string `json:"codex_version"`
	NodeVersion   string `json:"node_version"`
	PythonVersion string `json:"python_version"`
	GoVersion     string `json:"go_version"`
	NodeBase      string `json:"node_base"`
	GoBase        string `json:"go_base"`
	User          string `json:"user"`
}

func EvaluationImageMetadata() (ImageMetadata, error) {
	var metadata ImageMetadata
	if err := json.Unmarshal(imageMetadataJSON, &metadata); err != nil {
		return ImageMetadata{}, fmt.Errorf("parse embedded image metadata: %w", err)
	}
	if metadata.SchemaVersion != 1 || metadata.LocalTag != DefaultImageTag || metadata.CodexVersion != ExpectedCodexVersion {
		return ImageMetadata{}, errors.New("embedded evaluation image metadata is inconsistent")
	}
	if !validDigestReference(metadata.NodeBase) || !validDigestReference(metadata.GoBase) {
		return ImageMetadata{}, errors.New("evaluation image bases must be pinned by SHA-256 digest")
	}
	return metadata, nil
}

func DefaultContainerSpec(image string) (ContainerSpec, error) {
	if !validImageID(image) && !validDigestReference(image) {
		return ContainerSpec{}, errors.New("evaluation image must use an immutable SHA-256 ID or repository digest")
	}
	return ContainerSpec{
		Name:        "mdbench-trial",
		Image:       image,
		WorkDir:     "/work",
		User:        "10001:10001",
		Hostname:    "mdbench",
		Network:     NetworkBridge,
		MemoryBytes: 2 << 30,
		CPUs:        "1",
		PidsLimit:   128,
		Tmpfs: []TmpfsMount{
			{Target: "/work", SizeBytes: 256 << 20, UID: 10001, GID: 10001},
			{Target: "/out", SizeBytes: 32 << 20, UID: 10001, GID: 10001},
			{Target: "/tmp", SizeBytes: 64 << 20, Mode: 0o1777},
			{Target: "/codex-home", SizeBytes: 16 << 20, UID: 10001, GID: 10001},
		},
		Environment: []string{
			"CODEX_HOME=/codex-home",
			"HOME=/home/mdbench",
			"PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin",
			"TMPDIR=/tmp",
		},
		Command:     []string{"sleep", "infinity"},
		StopTimeout: 3 * time.Second,
	}, nil
}

func CodexContainerSpec(image string) (ContainerSpec, error) {
	spec, err := DefaultContainerSpec(image)
	if err != nil {
		return ContainerSpec{}, err
	}
	spec.NestedCodexSandbox = true
	return spec, nil
}

func validDigestReference(reference string) bool {
	separator := strings.LastIndex(reference, "@sha256:")
	return separator > 0 && validHexDigest(reference[separator+8:])
}

func validImageID(value string) bool {
	return strings.HasPrefix(value, "sha256:") && validHexDigest(strings.TrimPrefix(value, "sha256:"))
}

func validHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}
