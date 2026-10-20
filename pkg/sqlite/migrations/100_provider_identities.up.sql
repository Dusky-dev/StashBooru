-- Provider assertions and user decisions are separate from native aliases.
CREATE TABLE provider_identities (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  instance TEXT NOT NULL,
  source_id TEXT NOT NULL,
  source_name TEXT NOT NULL,
  engine TEXT NOT NULL,
  source_url TEXT NOT NULL,
  kind TEXT NOT NULL CHECK(kind IN ('artist','character','copyright')),
  form TEXT NOT NULL CHECK(form IN ('tag','account')),
  target TEXT NOT NULL,
  label TEXT NOT NULL,
  label_rank INTEGER NOT NULL DEFAULT 0,
  context TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL DEFAULT 1 CHECK(version > 0),
  studio_id INTEGER REFERENCES studios(id) ON DELETE SET NULL,
  performer_id INTEGER REFERENCES performers(id) ON DELETE SET NULL,
  copyright_id INTEGER REFERENCES copyrights(id) ON DELETE SET NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(instance,kind,form,target),
  CHECK((studio_id IS NULL OR kind='artist')
    AND (performer_id IS NULL OR kind='character')
    AND (copyright_id IS NULL OR kind='copyright')),
  CHECK(form!='account' OR kind='artist')
);
CREATE INDEX provider_identities_studio ON provider_identities(studio_id);
CREATE INDEX provider_identities_performer ON provider_identities(performer_id);
CREATE INDEX provider_identities_copyright ON provider_identities(copyright_id);

CREATE TABLE provider_identity_rejections (
  identity_id INTEGER NOT NULL REFERENCES provider_identities(id) ON DELETE CASCADE,
  entity_id INTEGER NOT NULL,
  PRIMARY KEY(identity_id,entity_id)
) WITHOUT ROWID;

-- Native deletion invalidates an outstanding edit even though no user mutation
-- ran through the identity API. Explicit unlink already increments its version.
CREATE TRIGGER provider_identity_deleted_target AFTER UPDATE OF studio_id,performer_id,copyright_id ON provider_identities
WHEN NEW.version=OLD.version AND
 (COALESCE(OLD.studio_id,OLD.performer_id,OLD.copyright_id,0) !=
  COALESCE(NEW.studio_id,NEW.performer_id,NEW.copyright_id,0))
BEGIN
  UPDATE provider_identities SET version=version+1 WHERE id=NEW.id;
END;
