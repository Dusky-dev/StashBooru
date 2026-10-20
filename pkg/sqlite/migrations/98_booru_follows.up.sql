-- Library-wide follow intent. No provider credentials, native entity mappings,
-- feed checkpoints or implicit download authorization are stored here.
CREATE TABLE booru_follows (
  id TEXT PRIMARY KEY NOT NULL,
  instance TEXT NOT NULL,
  source_id TEXT NOT NULL,
  engine TEXT NOT NULL,
  source_url TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK(kind IN ('artist','character','copyright','query')),
  target TEXT NOT NULL,
  version INTEGER NOT NULL CHECK(version > 0),
  query_revision INTEGER NOT NULL CHECK(query_revision > 0),
  active INTEGER NOT NULL CHECK(active IN (0,1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(instance,kind,target)
);
