package sqlite

// StashBooru-specific migrations currently extend through 95. Keep the app
// schema version aligned with the highest embedded migration so existing
// databases run the P05 Character variant/context migration before opening.
func init() {
	if appSchemaVersion < 95 {
		appSchemaVersion = 95
	}
}
