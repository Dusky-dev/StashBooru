CREATE TABLE following_checkpoints (
  follow_id TEXT NOT NULL REFERENCES booru_follows(id),
  revision INTEGER NOT NULL CHECK(revision > 0),
  version INTEGER NOT NULL CHECK(version > 0),
  cursor TEXT NOT NULL,
  has_more INTEGER NOT NULL CHECK(has_more IN (0,1)),
  checked_at TEXT NOT NULL,
  last_error TEXT NOT NULL DEFAULT '',
  error_at TEXT NOT NULL DEFAULT '',
  PRIMARY KEY(follow_id,revision)
) WITHOUT ROWID;

CREATE TABLE following_posts (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  instance TEXT NOT NULL,
  post_id TEXT NOT NULL,
  payload BLOB NOT NULL,
  retrieved_at TEXT NOT NULL,
  UNIQUE(instance,post_id)
);

CREATE TABLE following_matches (
  sequence INTEGER NOT NULL REFERENCES following_posts(sequence) ON DELETE CASCADE,
  follow_id TEXT NOT NULL REFERENCES booru_follows(id),
  revision INTEGER NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  target TEXT NOT NULL,
  PRIMARY KEY(sequence,follow_id,revision)
) WITHOUT ROWID;
CREATE INDEX following_matches_follow ON following_matches(follow_id,revision,sequence);

-- Explicit decisions outlive pruning of the bounded snapshot cache.
CREATE TABLE following_seen (
  instance TEXT NOT NULL,
  post_id TEXT NOT NULL,
  version INTEGER NOT NULL CHECK(version > 0),
  seen_at TEXT NOT NULL,
  PRIMARY KEY(instance,post_id)
) WITHOUT ROWID;
