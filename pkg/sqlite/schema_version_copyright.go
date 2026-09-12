package sqlite

func init() {
	if appSchemaVersion < 90 {
		appSchemaVersion = 90
	}
}
