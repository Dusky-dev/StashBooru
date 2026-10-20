package sqlite

import "testing"

func TestStashBooruSchemaIncludesCharacterVariantMigration(t *testing.T) {
	if appSchemaVersion < 95 {
		t.Fatalf("schema version %d does not include P05 Character migration 95", appSchemaVersion)
	}
}

func TestStashBooruSchemaIncludesFollowMigration(t *testing.T) {
	if appSchemaVersion < 98 {
		t.Fatalf("schema version %d does not include follow migration 98", appSchemaVersion)
	}
}

func TestStashBooruSchemaIncludesFollowingFeedMigration(t *testing.T) {
	if appSchemaVersion < 99 {
		t.Fatalf("schema version %d does not include Following migration 99", appSchemaVersion)
	}
}
