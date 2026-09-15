package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/camietagger"
	"github.com/stashapp/stash/pkg/models"
)

type camieEnrichmentCopyrightRepository struct {
	models.CopyrightReaderWriter
	findErr error
}

func (r *camieEnrichmentCopyrightRepository) FindByName(context.Context, string, bool) (*models.Copyright, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return nil, nil
}

func (r *camieEnrichmentCopyrightRepository) FindByAlias(context.Context, string, bool) (*models.Copyright, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return nil, nil
}

func TestEnrichNativeCamiePredictionTargetsInTxnPropagatesLookupError(t *testing.T) {
	expected := errors.New("copyright lookup failed")
	repository := models.Repository{
		Copyright: &camieEnrichmentCopyrightRepository{findErr: expected},
	}
	predictions := []camietagger.Tag{{
		Name:     "pokemon",
		RawName:  "pokemon",
		Category: "copyright",
		Score:    1,
	}}

	_, err := enrichNativeCamiePredictionTargetsInTxn(context.Background(), repository, predictions)
	if !errors.Is(err, expected) {
		t.Fatalf("expected lookup error to propagate, got %v", err)
	}
}

func TestEnrichNativeCamiePredictionTargetsInTxnPreservesNoMatch(t *testing.T) {
	repository := models.Repository{
		Copyright: &camieEnrichmentCopyrightRepository{},
	}
	predictions := []camietagger.Tag{{
		Name:     "pokemon",
		RawName:  "pokemon",
		Category: "copyright",
		Score:    1,
	}}

	result, err := enrichNativeCamiePredictionTargetsInTxn(context.Background(), repository, predictions)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected one enriched prediction, got %#v", result)
	}
	if result[0].TargetExists {
		t.Fatalf("ordinary no-match must remain non-existent, got %#v", result[0])
	}
	if !strings.HasPrefix(result[0].TargetPath, "/copyrights?q=") {
		t.Fatalf("expected Copyright search target for ordinary no-match, got %q", result[0].TargetPath)
	}
}
