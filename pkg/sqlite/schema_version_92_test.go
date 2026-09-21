package sqlite

import "testing"

func TestCopyrightTagsMigrationIsRequired(t *testing.T) {
	if appSchemaVersion < 92 {
		t.Fatalf("app schema version %d does not require migration 92 for copyrights_tags", appSchemaVersion)
	}
}
