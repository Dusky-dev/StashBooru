package sqlite

// Migration 92 added the native Copyright↔Tag join table. The migration was
// merged without advancing the application's required schema version, so
// existing schema-91 databases never created copyrights_tags. Keep the version
// bump isolated here so upgrades immediately require and run migration 92.
func init() {
	appSchemaVersion = 92
}
