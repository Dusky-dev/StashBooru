package sqlite

import sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"

const visualEmbeddingSchemaVersion uint = 87

func init() {
	// sqlite-vec uses SQLite's auto-extension mechanism, so registering it here
	// makes vec0 and vec_* functions available on every Stash SQLite connection,
	// including connections opened through the custom sqlite3ex driver.
	sqlite_vec.Auto()

	// Keep this feature branch self-contained. When upstream advances the schema,
	// never lower its version; only claim v87 while it is the newest migration.
	if appSchemaVersion < visualEmbeddingSchemaVersion {
		appSchemaVersion = visualEmbeddingSchemaVersion
	}
}
