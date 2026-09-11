package sqlite

// Copyrights are introduced after the visual-embedding migration. Keep this in
// a small companion file so the feature stays self-contained while it is being
// developed; this value can be folded back into database.go when the feature is
// merged upstream.
func init() {
	if appSchemaVersion < 88 {
		appSchemaVersion = 88
	}
}
