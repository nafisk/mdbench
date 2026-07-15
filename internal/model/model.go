package model

import "time"

const SchemaVersion = 1

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Path     string   `json:"path,omitempty"`
	Line     int      `json:"line,omitempty"`
	Hint     string   `json:"hint,omitempty"`
}

type Metrics struct {
	Bytes      int `json:"bytes"`
	Characters int `json:"characters"`
	Words      int `json:"words"`
	Lines      int `json:"lines"`
	Headings   int `json:"headings"`
	CodeBlocks int `json:"code_blocks"`
}

type ArtifactFile struct {
	Path       string `json:"path"`
	Mode       uint32 `json:"mode"`
	Size       int64  `json:"size"`
	ContentSHA string `json:"content_sha"`
	Content    []byte `json:"-"`
}

type Artifact struct {
	SchemaVersion     int            `json:"schema_version"`
	ID                string         `json:"id"`
	Label             string         `json:"label,omitempty"`
	Source            string         `json:"source"`
	BundleRoot        string         `json:"bundle_root,omitempty"`
	EntryPath         string         `json:"entry_path"`
	ContentSHA        string         `json:"content_sha"`
	BundleSHA         string         `json:"bundle_sha"`
	EffectiveSHA      string         `json:"effective_sha"`
	TransformVersion  string         `json:"transform_version,omitempty"`
	Frontmatter       map[string]any `json:"frontmatter,omitempty"`
	Files             []ArtifactFile `json:"files"`
	Findings          []Finding      `json:"findings"`
	Metrics           Metrics        `json:"metrics"`
	CreatedAt         time.Time      `json:"created_at"`
	Markdown          []byte         `json:"-"`
	EffectiveMarkdown []byte         `json:"-"`
}

func (a Artifact) HasBlockingFindings() bool {
	for _, finding := range a.Findings {
		if finding.Severity == SeverityError {
			return true
		}
	}
	return false
}
