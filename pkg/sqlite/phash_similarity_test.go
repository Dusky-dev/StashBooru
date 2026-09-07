package sqlite

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestParsePHashSimilaritySort(t *testing.T) {
	base, opts, err := parsePHashSimilaritySort("perceptual_similarity:5:42")
	if err != nil {
		t.Fatal(err)
	}
	if base != "perceptual_similarity" || opts == nil || opts.Distance != 5 || opts.ReferenceID == nil || *opts.ReferenceID != 42 {
		t.Fatalf("unexpected parse result: base=%q opts=%+v", base, opts)
	}
	if _, _, err := parsePHashSimilaritySort("perceptual_similarity:9"); err == nil {
		t.Fatal("expected distance validation error")
	}
}

func TestOrderPHashSimilarityCandidates(t *testing.T) {
	candidates := []pHashSimilarityCandidate{
		{ID: 1, PHash: sql.NullInt64{Int64: 0, Valid: true}},
		{ID: 2, PHash: sql.NullInt64{Int64: 1, Valid: true}},
		{ID: 3, PHash: sql.NullInt64{Int64: -1, Valid: true}},
		{ID: 4, PHash: sql.NullInt64{}},
	}
	got := orderPHashSimilarityCandidates(candidates, 1)
	want := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order=%v want=%v", got, want)
	}
}
