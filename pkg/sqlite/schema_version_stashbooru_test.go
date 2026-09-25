package sqlite

import "testing"

func TestStashBooruSchemaIncludesTaxonomyMigrations(t *testing.T) {
	if appSchemaVersion < 94 {
		t.Fatalf("schema version %d does not include P04 taxonomy migration 94", appSchemaVersion)
	}
}
