package sqlite

// Migration 92 adds the native Copyright-to-Tag relationship used by Image
// Tagging profile inheritance. Keep the StashBooru schema version aligned with
// the highest embedded migration so existing version-91 databases are prompted
// to run migration 92 instead of opening without copyrights_tags.
func init() {
	if appSchemaVersion < 92 {
		appSchemaVersion = 92
	}
}
