package analyze

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

const transformVersion = "plain-markdown-wrapper-v1"

var (
	skillNamePattern   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	placeholderPattern = regexp.MustCompile(`(?i)\b(TODO|TBD|PLACEHOLDER|FIXME)\b`)
	secretPatterns     = []struct {
		name string
		re   *regexp.Regexp
	}{
		{"private key", regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
		{"OpenAI-style key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)},
		{"GitHub token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`)},
		{"AWS access key", regexp.MustCompile(`\bAKIA[A-Z0-9]{16}\b`)},
	}
)

type Analyzer struct {
	MaxArtifactBytes int64
	MaxBundleBytes   int64
	MaxBundleFiles   int
}

type markdownInfo struct {
	frontmatter    map[string]any
	findings       []model.Finding
	metrics        model.Metrics
	references     []string
	hasFrontmatter bool
	frontmatterEnd int
}

func (a Analyzer) InspectFile(path, label string) (model.Artifact, error) {
	if strings.TrimSpace(path) == "" {
		return model.Artifact{}, errors.New("file path is empty")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return model.Artifact{}, fmt.Errorf("resolve file path: %w", err)
	}
	info, err := os.Lstat(absPath)
	if err != nil {
		return model.Artifact{}, fmt.Errorf("read input: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return model.Artifact{}, errors.New("input must not be a symlink")
	}
	if info.IsDir() {
		absPath, err = skillFileIn(absPath)
		if err != nil {
			return model.Artifact{}, err
		}
		info, err = os.Lstat(absPath)
		if err != nil {
			return model.Artifact{}, fmt.Errorf("read input: %w", err)
		}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return model.Artifact{}, errors.New("input must be a regular Markdown file, not a symlink")
	}
	if info.Size() > a.MaxArtifactBytes {
		return model.Artifact{}, fmt.Errorf("input is %d bytes; limit is %d", info.Size(), a.MaxArtifactBytes)
	}
	markdown, err := os.ReadFile(absPath)
	if err != nil {
		return model.Artifact{}, fmt.Errorf("read input: %w", err)
	}
	if !utf8.Valid(markdown) {
		return model.Artifact{}, errors.New("input is not valid UTF-8")
	}

	md := inspectMarkdown(markdown, filepath.Base(absPath))
	root := filepath.Dir(absPath)
	files, bundleFindings := a.bundleFiles(root, filepath.Base(absPath), markdown, md.references)
	md.findings = append(md.findings, bundleFindings...)
	return makeArtifact(absPath, root, filepath.Base(absPath), label, markdown, files, md)
}

func skillFileIn(directory string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("read skill folder: %w", err)
	}
	for _, exact := range []bool{true, false} {
		for _, entry := range entries {
			name := entry.Name()
			matches := name == "SKILL.md"
			if !exact {
				matches = strings.EqualFold(name, "SKILL.md")
			}
			if matches && !entry.IsDir() {
				return filepath.Join(directory, name), nil
			}
		}
	}
	return "", errors.New("folder does not contain SKILL.md")
}

func (a Analyzer) InspectPaste(markdown []byte, label string) (model.Artifact, error) {
	if len(bytes.TrimSpace(markdown)) == 0 {
		return model.Artifact{}, errors.New("pasted Markdown is empty")
	}
	if int64(len(markdown)) > a.MaxArtifactBytes {
		return model.Artifact{}, fmt.Errorf("pasted Markdown is %d bytes; limit is %d", len(markdown), a.MaxArtifactBytes)
	}
	if !utf8.Valid(markdown) {
		return model.Artifact{}, errors.New("pasted Markdown is not valid UTF-8")
	}
	md := inspectMarkdown(markdown, "SKILL.md")
	file := artifactFile("SKILL.md", 0o600, markdown)
	for _, secret := range secretFindings("SKILL.md", markdown) {
		md.findings = append(md.findings, secret)
	}
	return makeArtifact("stdin", "", "SKILL.md", label, markdown, []model.ArtifactFile{file}, md)
}

func makeArtifact(source, root, entry, label string, markdown []byte, files []model.ArtifactFile, md markdownInfo) (model.Artifact, error) {
	id, err := newID()
	if err != nil {
		return model.Artifact{}, err
	}
	effective, version := effectiveMarkdown(markdown, md)
	contentHash := sha256.Sum256(markdown)
	effectiveHash := sha256.Sum256(effective)
	return model.Artifact{
		SchemaVersion:     model.SchemaVersion,
		ID:                id,
		Label:             strings.TrimSpace(label),
		Source:            source,
		BundleRoot:        root,
		EntryPath:         filepath.ToSlash(entry),
		ContentSHA:        hex.EncodeToString(contentHash[:]),
		BundleSHA:         bundleHash(files),
		EffectiveSHA:      hex.EncodeToString(effectiveHash[:]),
		TransformVersion:  version,
		Frontmatter:       md.frontmatter,
		Files:             files,
		Findings:          md.findings,
		Metrics:           md.metrics,
		CreatedAt:         time.Now().UTC(),
		Markdown:          append([]byte(nil), markdown...),
		EffectiveMarkdown: effective,
	}, nil
}

func inspectMarkdown(source []byte, path string) markdownInfo {
	info := markdownInfo{frontmatter: map[string]any{}}
	frontmatter, hasFrontmatter, end, err := parseFrontmatter(source)
	info.hasFrontmatter, info.frontmatterEnd = hasFrontmatter, end
	if err != nil {
		info.findings = append(info.findings, finding("frontmatter.invalid", model.SeverityError, "YAML frontmatter is invalid", path, 1, "Fix the YAML before evaluation."))
	} else if hasFrontmatter {
		info.frontmatter = frontmatter
		info.findings = append(info.findings, validateFrontmatter(frontmatter, path)...)
		info.references = append(info.references, frontmatterReferences(frontmatter)...)
	}

	markdownSource := source
	baseOffset := 0
	if hasFrontmatter && end > 0 && end <= len(source) {
		markdownSource = source[end:]
		baseOffset = end
	}
	doc := goldmark.DefaultParser().Parse(text.NewReader(markdownSource))
	headings := map[string]int{}
	var internalLinks []struct {
		anchor string
		line   int
	}
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := node.(type) {
		case *ast.Heading:
			info.metrics.Headings++
			slug := headingSlug(string(typed.Text(markdownSource)))
			line := lineAt(source, baseOffset+typed.Pos())
			if first, exists := headings[slug]; exists {
				info.findings = append(info.findings, finding("heading.duplicate", model.SeverityWarning, "Duplicate heading anchor \""+slug+"\"", path, line, fmt.Sprintf("Rename this heading or the heading on line %d.", first)))
			} else {
				headings[slug] = line
			}
		case *ast.Link:
			destination := string(typed.Destination)
			if strings.HasPrefix(destination, "#") {
				internalLinks = append(internalLinks, struct {
					anchor string
					line   int
				}{strings.TrimPrefix(destination, "#"), lineAt(source, baseOffset+typed.Pos())})
			} else if ref := localReference(destination); ref != "" {
				info.references = append(info.references, ref)
			}
		case *ast.Image:
			if ref := localReference(string(typed.Destination)); ref != "" {
				info.references = append(info.references, ref)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, link := range internalLinks {
		if _, ok := headings[headingSlug(link.anchor)]; !ok {
			info.findings = append(info.findings, finding("link.internal_missing", model.SeverityWarning, "Internal heading link does not resolve", path, link.line, "Update the anchor or add the missing heading."))
		}
	}

	info.metrics.Bytes = len(source)
	info.metrics.Characters = utf8.RuneCount(source)
	info.metrics.Words = len(strings.Fields(string(source)))
	info.metrics.Lines = bytes.Count(source, []byte("\n")) + 1
	blocks, balanced := fencedCodeBlocks(markdownSource)
	info.metrics.CodeBlocks = blocks
	if !balanced {
		info.findings = append(info.findings, finding("markdown.unbalanced_fence", model.SeverityError, "Code fence is not closed", path, 0, "Close the final fenced code block."))
	}
	if location := placeholderPattern.FindIndex(source); location != nil {
		info.findings = append(info.findings, finding("content.placeholder", model.SeverityWarning, "Placeholder marker remains in the artifact", path, lineAt(source, location[0]), "Replace or remove the placeholder."))
	}
	return info
}

func (a Analyzer) bundleFiles(root, entry string, markdown []byte, references []string) ([]model.ArtifactFile, []model.Finding) {
	files := map[string]model.ArtifactFile{filepath.ToSlash(entry): artifactFile(filepath.ToSlash(entry), 0o600, markdown)}
	var findings []model.Finding
	for _, reference := range uniqueStrings(references) {
		resolved, rel, err := resolveWithin(root, reference)
		if err != nil {
			findings = append(findings, finding("reference.invalid", model.SeverityError, "Referenced path is outside the artifact root", entry, 0, "Use a relative path inside the skill directory."))
			continue
		}
		info, err := lstatNoSymlink(root, rel)
		if err != nil {
			if strings.Contains(err.Error(), "symlink") {
				findings = append(findings, finding("reference.symlink", model.SeverityError, "Referenced symlinks are not allowed: "+filepath.ToSlash(reference), entry, 0, "Reference a regular file or directory."))
				continue
			}
			findings = append(findings, finding("reference.missing", model.SeverityError, "Referenced path does not exist: "+filepath.ToSlash(reference), entry, 0, "Fix the path or add the missing file."))
			continue
		}
		if info.IsDir() {
			walkErr := filepath.WalkDir(resolved, func(path string, dirEntry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if dirEntry.Name() == ".git" && dirEntry.IsDir() {
					return filepath.SkipDir
				}
				entryInfo, statErr := dirEntry.Info()
				if statErr != nil {
					return statErr
				}
				if entryInfo.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("symlink %s", path)
				}
				if !entryInfo.Mode().IsRegular() {
					return nil
				}
				childRel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				return addBundleFile(files, path, filepath.ToSlash(childRel), entryInfo.Mode(), a)
			})
			if walkErr != nil {
				findings = append(findings, finding("reference.unreadable", model.SeverityError, "Referenced directory cannot be safely bundled: "+filepath.ToSlash(rel), entry, 0, "Remove symlinks and unreadable files from the referenced directory."))
			}
			continue
		}
		if !info.Mode().IsRegular() {
			findings = append(findings, finding("reference.unsupported", model.SeverityError, "Referenced path is not a regular file: "+filepath.ToSlash(reference), entry, 0, "Use a regular file or directory."))
			continue
		}
		if err := addBundleFile(files, resolved, rel, info.Mode(), a); err != nil {
			findings = append(findings, finding("reference.unreadable", model.SeverityError, "Referenced file cannot be safely bundled: "+filepath.ToSlash(reference), entry, 0, err.Error()))
		}
	}

	result := make([]model.ArtifactFile, 0, len(files))
	var total int64
	for _, file := range files {
		total += file.Size
		result = append(result, file)
		findings = append(findings, secretFindings(file.Path, file.Content)...)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	if len(result) > a.MaxBundleFiles {
		findings = append(findings, finding("bundle.file_limit", model.SeverityError, fmt.Sprintf("Bundle has %d files; limit is %d", len(result), a.MaxBundleFiles), entry, 0, "Reference fewer files or raise the configured limit."))
	}
	if total > a.MaxBundleBytes {
		findings = append(findings, finding("bundle.size_limit", model.SeverityError, fmt.Sprintf("Bundle is %d bytes; limit is %d", total, a.MaxBundleBytes), entry, 0, "Reference fewer files or raise the configured limit."))
	}
	return result, findings
}

func addBundleFile(files map[string]model.ArtifactFile, absolute, relative string, mode fs.FileMode, analyzer Analyzer) error {
	if len(files) >= analyzer.MaxBundleFiles+1 {
		return errors.New("bundle file limit reached")
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return err
	}
	var currentBytes int64
	for _, file := range files {
		currentBytes += file.Size
	}
	if info.Size() > analyzer.MaxBundleBytes-currentBytes {
		return errors.New("bundle byte limit reached")
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return err
	}
	if int64(len(content)) > analyzer.MaxBundleBytes-currentBytes {
		return errors.New("bundle byte limit reached")
	}
	files[filepath.ToSlash(relative)] = artifactFile(filepath.ToSlash(relative), uint32(mode.Perm()), content)
	return nil
}

func artifactFile(path string, mode uint32, content []byte) model.ArtifactFile {
	hash := sha256.Sum256(content)
	return model.ArtifactFile{
		Path: path, Mode: mode, Size: int64(len(content)), ContentSHA: hex.EncodeToString(hash[:]), Content: append([]byte(nil), content...),
	}
}

func bundleHash(files []model.ArtifactFile) string {
	copyFiles := append([]model.ArtifactFile(nil), files...)
	sort.Slice(copyFiles, func(i, j int) bool { return copyFiles[i].Path < copyFiles[j].Path })
	hash := sha256.New()
	for _, file := range copyFiles {
		fmt.Fprintf(hash, "%s\x00%d\x00%d\x00", file.Path, file.Mode, file.Size)
		hash.Write(file.Content)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func parseFrontmatter(source []byte) (map[string]any, bool, int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	offset := 0
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return map[string]any{}, false, 0, nil
	}
	offset += len(scanner.Bytes()) + 1
	start := offset
	for scanner.Scan() {
		line := scanner.Text()
		offset += len(scanner.Bytes()) + 1
		if strings.TrimSpace(line) == "---" {
			var values map[string]any
			frontmatter := source[start : offset-len(scanner.Bytes())-1]
			if err := yaml.Unmarshal(frontmatter, &values); err != nil {
				normalized, ok := normalizePlainDescription(frontmatter)
				values = nil
				if !ok || yaml.Unmarshal(normalized, &values) != nil {
					return nil, true, offset, err
				}
			}
			if values == nil {
				values = map[string]any{}
			}
			return values, true, offset, nil
		}
	}
	return nil, true, len(source), errors.New("frontmatter closing delimiter is missing")
}

func normalizePlainDescription(frontmatter []byte) ([]byte, bool) {
	lines := bytes.Split(frontmatter, []byte("\n"))
	for index, line := range lines {
		const prefix = "description:"
		if !bytes.HasPrefix(line, []byte(prefix)) {
			continue
		}
		value := strings.TrimSpace(string(line[len(prefix):]))
		if value == "" || strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">") || strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
			return nil, false
		}
		lines[index] = []byte(prefix + " " + strconv.Quote(value))
		return bytes.Join(lines, []byte("\n")), true
	}
	return nil, false
}

func validateFrontmatter(values map[string]any, path string) []model.Finding {
	var findings []model.Finding
	for _, key := range []string{"name", "description"} {
		if value, exists := values[key]; exists {
			if _, ok := value.(string); !ok {
				findings = append(findings, finding("frontmatter.type", model.SeverityError, key+" must be a string", path, 1, "Use a YAML string value."))
			}
		}
	}
	if value, ok := values["name"].(string); ok && !skillNamePattern.MatchString(value) {
		findings = append(findings, finding("frontmatter.name", model.SeverityError, "Skill name must use lowercase letters, numbers, and hyphens", path, 1, "Use a name such as mdbench-candidate."))
	}
	for _, key := range []string{"scripts", "assets", "references"} {
		if value, exists := values[key]; exists && !stringList(value, nil) {
			findings = append(findings, finding("frontmatter.type", model.SeverityError, key+" must be a path or list of paths", path, 1, "Use a string or YAML list of strings."))
		}
	}
	return findings
}

func frontmatterReferences(values map[string]any) []string {
	var references []string
	for _, key := range []string{"scripts", "assets", "references"} {
		stringList(values[key], &references)
	}
	return references
}

func stringList(value any, target *[]string) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		if target != nil && strings.TrimSpace(typed) != "" {
			*target = append(*target, typed)
		}
		return true
	case []any:
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return false
			}
			if target != nil && strings.TrimSpace(text) != "" {
				*target = append(*target, text)
			}
		}
		return true
	default:
		return false
	}
}

func effectiveMarkdown(source []byte, info markdownInfo) ([]byte, string) {
	if name, ok := info.frontmatter["name"].(string); ok && skillNamePattern.MatchString(name) {
		return append([]byte(nil), source...), ""
	}
	if info.hasFrontmatter && info.frontmatterEnd > 0 && info.frontmatterEnd <= len(source) {
		firstLine := bytes.IndexByte(source, '\n')
		if firstLine >= 0 {
			result := append([]byte{}, source[:firstLine+1]...)
			result = append(result, []byte("name: mdbench-candidate\n")...)
			result = append(result, source[firstLine+1:]...)
			return result, transformVersion
		}
	}
	header := []byte("---\nname: mdbench-candidate\ndescription: Candidate skill under evaluation.\n---\n")
	return append(header, source...), transformVersion
}

func secretFindings(path string, content []byte) []model.Finding {
	if !utf8.Valid(content) {
		return nil
	}
	var findings []model.Finding
	for _, pattern := range secretPatterns {
		if location := pattern.re.FindIndex(content); location != nil {
			findings = append(findings, finding("secret.detected", model.SeverityError, "Possible "+pattern.name+" detected; value redacted", path, lineAt(content, location[0]), "Remove the value before saving or evaluation."))
		}
	}
	return findings
}

func localReference(destination string) string {
	parsed, err := url.Parse(destination)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Path == "" || strings.HasPrefix(destination, "#") {
		return ""
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return ""
	}
	return path
}

func resolveWithin(root, reference string) (string, string, error) {
	if filepath.IsAbs(reference) {
		return "", "", errors.New("absolute path")
	}
	clean := filepath.Clean(filepath.FromSlash(reference))
	resolved := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", errors.New("path escape")
	}
	return resolved, filepath.ToSlash(rel), nil
}

func lstatNoSymlink(root, relative string) (fs.FileInfo, error) {
	current := root
	parts := strings.Split(filepath.FromSlash(relative), string(filepath.Separator))
	for index, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink component %q", strings.Join(parts[:index+1], "/"))
		}
		if index == len(parts)-1 {
			return info, nil
		}
	}
	return os.Lstat(current)
}

func headingSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastHyphen = false
		case unicode.IsSpace(r) || r == '-':
			if builder.Len() > 0 && !lastHyphen {
				builder.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.TrimSuffix(builder.String(), "-")
}

func fencedCodeBlocks(source []byte) (int, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	var marker byte
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
			continue
		}
		run := 1
		for run < len(line) && line[run] == line[0] {
			run++
		}
		if run < 3 {
			continue
		}
		if marker == 0 {
			marker = line[0]
			count++
		} else if marker == line[0] {
			marker = 0
		}
	}
	return count, marker == 0
}

func lineAt(source []byte, offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	return bytes.Count(source[:offset], []byte("\n")) + 1
}

func finding(id string, severity model.Severity, message, path string, line int, hint string) model.Finding {
	return model.Finding{ID: id, Severity: severity, Message: message, Path: filepath.ToSlash(path), Line: line, Hint: hint}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func newID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate artifact ID: %w", err)
	}
	return time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(bytes), nil
}
