-- Persist user-confirmed not-duplicate Character pairs by native ID.
-- ON DELETE CASCADE makes decisions disappear when either profile is removed.
CREATE TABLE character_dedup_exclusions (
  left_id INTEGER NOT NULL REFERENCES performers(id) ON DELETE CASCADE,
  right_id INTEGER NOT NULL REFERENCES performers(id) ON DELETE CASCADE,
  CHECK(left_id > 0 AND left_id < right_id),
  PRIMARY KEY(left_id, right_id)
) WITHOUT ROWID;
