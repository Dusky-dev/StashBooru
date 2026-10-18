-- Remote review history is independent of native media and file lifetimes.
CREATE TABLE booru_review (
  instance TEXT NOT NULL,
  post_id TEXT NOT NULL,
  source_id TEXT NOT NULL,
  engine TEXT NOT NULL,
  source_url TEXT NOT NULL,
  version INTEGER NOT NULL CHECK(version > 0),
  ignored_at TEXT NOT NULL DEFAULT '',
  downloaded_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY(instance, post_id)
) WITHOUT ROWID;
