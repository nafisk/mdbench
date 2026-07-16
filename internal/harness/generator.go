package harness

import (
	"context"

	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/nafiskhan/mdbench/internal/suite"
)

type GenerateRequest struct {
	Artifact   model.Artifact
	CaseCount  int
	FixtureIDs []string
}

type Generator interface {
	GenerateSuite(context.Context, GenerateRequest) (suite.Draft, error)
}
