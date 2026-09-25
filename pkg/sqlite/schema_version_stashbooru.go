package sqlite

// StashBooru-specific migrations currently extend through 94. Keep the app
// schema version aligned with the highest embedded migration so existing
// databases run Copyright-to-Tag migration 92 and the P04 Copyright taxonomy
// migration 94 before opening.
func init() {
	if appSchemaVersion < 94 {
		appSchemaVersion = 94
	}
}
