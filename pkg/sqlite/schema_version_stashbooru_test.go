package sqlite

import "testing"

func TestStashBooruSchemaIncludesCharacterVariantMigration(t *testing.T) {
	if appSchemaVersion < 95 {
		t.Fatalf("schema version %d does not include P05 Character migration 95", appSchemaVersion)
	}
}
