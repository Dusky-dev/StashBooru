package sqlite

// Keep the fork schema version aligned with its highest embedded migration.
func init() {
	if appSchemaVersion < 97 {
		appSchemaVersion = 97
	}
}
