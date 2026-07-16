package suite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/nafiskhan/mdbench/internal/fixture"
)

func Validate(draft Draft) error {
	if draft.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported suite schema %d", draft.SchemaVersion)
	}
	if strings.TrimSpace(draft.ID) == "" || strings.TrimSpace(draft.OriginArtifactSHA) == "" {
		return errors.New("suite ID and origin artifact hash are required")
	}
	if strings.TrimSpace(draft.Applicability) == "" || strings.TrimSpace(draft.Rubric) == "" {
		return errors.New("suite applicability and rubric are required")
	}
	dimensions, err := validateDimensions(draft.Dimensions)
	if err != nil {
		return err
	}
	assertions := map[string]bool{}
	cases := map[string]bool{}
	enabled := 0
	for index, testCase := range draft.Cases {
		if err := validateCase(index, testCase, dimensions, assertions); err != nil {
			return err
		}
		if cases[testCase.ID] {
			return fmt.Errorf("case ID %q is duplicated", testCase.ID)
		}
		cases[testCase.ID] = true
		if testCase.Enabled {
			enabled++
		}
	}
	if enabled == 0 {
		return errors.New("suite must have at least one enabled case")
	}
	for _, rule := range draft.HardFailureRules {
		if strings.TrimSpace(rule.ID) == "" || strings.TrimSpace(rule.Reason) == "" {
			return errors.New("hard-failure rules require an ID and reason")
		}
		if !assertions[rule.TriggerID] && !strings.HasPrefix(rule.TriggerID, "static.") {
			return fmt.Errorf("hard-failure rule %q has unknown trigger %q", rule.ID, rule.TriggerID)
		}
		if rule.ScoreCap < 0 || rule.ScoreCap > 10 {
			return fmt.Errorf("hard-failure rule %q score cap must be between 0 and 10", rule.ID)
		}
	}
	return nil
}

func CanonicalHash(draft Draft) (string, error) {
	if err := Validate(draft); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		return "", fmt.Errorf("encode suite: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validateDimensions(values []Dimension) (map[string]bool, error) {
	if len(values) == 0 {
		return nil, errors.New("suite requires score dimensions")
	}
	known := map[string]bool{}
	applicable := 0
	for _, dimension := range values {
		if strings.TrimSpace(dimension.ID) == "" || strings.TrimSpace(dimension.Name) == "" || strings.TrimSpace(dimension.Guidance) == "" {
			return nil, errors.New("dimensions require an ID, name, and guidance")
		}
		if known[dimension.ID] {
			return nil, fmt.Errorf("dimension ID %q is duplicated", dimension.ID)
		}
		if dimension.Weight <= 0 {
			return nil, fmt.Errorf("dimension %q weight must be positive", dimension.ID)
		}
		if strings.TrimSpace(dimension.Anchors.Zero) == "" || strings.TrimSpace(dimension.Anchors.Five) == "" || strings.TrimSpace(dimension.Anchors.Ten) == "" {
			return nil, fmt.Errorf("dimension %q requires 0, 5, and 10 anchors", dimension.ID)
		}
		known[dimension.ID] = dimension.Applicable
		if dimension.Applicable {
			applicable++
		}
	}
	if applicable == 0 {
		return nil, errors.New("suite requires at least one applicable dimension")
	}
	return known, nil
}

func validateCase(index int, value Case, dimensions map[string]bool, assertions map[string]bool) error {
	if strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.Title) == "" || strings.TrimSpace(value.Prompt) == "" {
		return fmt.Errorf("case %d requires an ID, title, and prompt", index+1)
	}
	if value.TimeoutSeconds <= 0 || value.Weight <= 0 {
		return fmt.Errorf("case %q timeout and weight must be positive", value.ID)
	}
	fixtureValue, err := fixture.Find(value.Fixture.ID)
	if err != nil {
		return fmt.Errorf("case %q: %w", value.ID, err)
	}
	if value.Fixture.ContentSHA != fixtureValue.ContentSHA {
		return fmt.Errorf("case %q fixture hash does not match %q", value.ID, value.Fixture.ID)
	}
	if len(value.Assertions) == 0 && len(value.EvidenceRequired) == 0 {
		return fmt.Errorf("case %q requires an assertion or evidence item", value.ID)
	}
	if len(value.JudgeCriteria) == 0 || len(value.Dimensions) == 0 {
		return fmt.Errorf("case %q requires judge criteria and dimensions", value.ID)
	}
	for _, dimension := range value.Dimensions {
		if applicable, exists := dimensions[dimension]; !exists || !applicable {
			return fmt.Errorf("case %q uses unavailable dimension %q", value.ID, dimension)
		}
	}
	for _, assertion := range value.Assertions {
		if err := validateAssertion(value.ID, assertion); err != nil {
			return err
		}
		if assertions[assertion.ID] {
			return fmt.Errorf("assertion ID %q is duplicated", assertion.ID)
		}
		assertions[assertion.ID] = true
	}
	return nil
}

func validateAssertion(caseID string, value AssertionSpec) error {
	if strings.TrimSpace(value.ID) == "" {
		return fmt.Errorf("case %q has an assertion without an ID", caseID)
	}
	known := map[AssertionType]bool{
		AssertionCommand: true, AssertionFileExists: true, AssertionContentMatch: true,
		AssertionDiffScope: true, AssertionDiffSize: true, AssertionDependency: true,
		AssertionForbidden: true, AssertionStatus: true,
	}
	if !known[value.Type] {
		return fmt.Errorf("assertion %q has unknown type %q", value.ID, value.Type)
	}
	if value.Type == AssertionCommand {
		if len(value.Argv) == 0 || strings.TrimSpace(value.Argv[0]) == "" || value.TimeoutSeconds <= 0 {
			return fmt.Errorf("command assertion %q requires argv and a positive timeout", value.ID)
		}
		if value.CWD != "" && !safeRelative(value.CWD) {
			return fmt.Errorf("assertion %q working directory must stay inside the fixture", value.ID)
		}
	}
	if value.Path != "" && !safeRelative(value.Path) {
		return fmt.Errorf("assertion %q path must stay inside the fixture", value.ID)
	}
	if (value.Type == AssertionFileExists || value.Type == AssertionContentMatch) && value.Path == "" {
		return fmt.Errorf("assertion %q requires a path", value.ID)
	}
	return nil
}

func safeRelative(value string) bool {
	cleaned := path.Clean(strings.ReplaceAll(value, "\\", "/"))
	return cleaned != ".." && !strings.HasPrefix(cleaned, "../") && !strings.HasPrefix(cleaned, "/")
}
