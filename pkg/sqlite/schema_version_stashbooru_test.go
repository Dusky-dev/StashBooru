package sqlite

import "testing"

func TestStashBooruSchemaIncludesCopyrightTagsMigration(t *testing.T) {
	if appSchemaVersion < 92 {
		t.Fatalf("schema version %d does not include migration 92 for copyrights_tags", appSchemaVersion)
	}
}
