package suite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Frozen struct {
	Draft
	Revision   int       `json:"revision"`
	ContentSHA string    `json:"content_sha"`
	FrozenAt   time.Time `json:"frozen_at"`
}

func Freeze(draft Draft, revision int, frozenAt time.Time) (Frozen, error) {
	if revision < 1 {
		return Frozen{}, errors.New("suite revision must be positive")
	}
	hash, err := RevisionHash(draft, revision)
	if err != nil {
		return Frozen{}, err
	}
	return Frozen{Draft: clone(draft), Revision: revision, ContentSHA: hash, FrozenAt: frozenAt.UTC()}, nil
}

func RevisionHash(draft Draft, revision int) (string, error) {
	if err := Validate(draft); err != nil {
		return "", err
	}
	if revision < 1 {
		return "", errors.New("suite revision must be positive")
	}
	encoded, err := json.Marshal(struct {
		Draft    Draft `json:"suite"`
		Revision int   `json:"revision"`
	}{Draft: draft, Revision: revision})
	if err != nil {
		return "", fmt.Errorf("encode suite revision: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (f Frozen) EditableDraft() Draft { return clone(f.Draft) }

func clone(value Draft) Draft {
	result := value
	result.Dimensions = append([]Dimension(nil), value.Dimensions...)
	result.HardFailureRules = append([]HardFailureRule(nil), value.HardFailureRules...)
	result.Cases = make([]Case, len(value.Cases))
	for index, testCase := range value.Cases {
		result.Cases[index] = testCase
		result.Cases[index].JudgeCriteria = append([]string(nil), testCase.JudgeCriteria...)
		result.Cases[index].EvidenceRequired = append([]string(nil), testCase.EvidenceRequired...)
		result.Cases[index].Dimensions = append([]string(nil), testCase.Dimensions...)
		result.Cases[index].Assertions = make([]AssertionSpec, len(testCase.Assertions))
		for assertionIndex, assertion := range testCase.Assertions {
			result.Cases[index].Assertions[assertionIndex] = assertion
			result.Cases[index].Assertions[assertionIndex].Argv = append([]string(nil), assertion.Argv...)
		}
	}
	return result
}
