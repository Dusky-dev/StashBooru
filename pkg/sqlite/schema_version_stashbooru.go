package sqlite

// Keep the fork schema version aligned with its highest embedded migration.
func init() {
	if appSchemaVersion < 104 {
		appSchemaVersion = 104
	}
}
