package sqlite

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stashapp/stash/pkg/models"
)

func TestTagThumbnailVisibilityRoundTrip(t *testing.T) {
	tag := models.NewTag()
	tag.Name = "Example"
	tag.HideThumbnail = true

	var row tagRow
	row.fromTag(tag)
	if !row.HideThumbnail {
		t.Fatal("tag thumbnail visibility not written to sqlite row")
	}
	got := row.resolve()
	if !got.HideThumbnail {
		t.Fatal("tag thumbnail visibility not read from sqlite row")
	}

	partial := models.NewTagPartial()
	partial.HideThumbnail = models.NewOptionalBool(false)
	record := tagRowRecord{updateRecord: updateRecord{Record: make(exp.Record)}}
	record.fromPartial(partial)
	value, ok := record.Record["hide_thumbnail"]
	if !ok || value != false {
		t.Fatalf("tag thumbnail visibility was not updated: %v (%t)", value, ok)
	}
}
