-- Remove vectors and source records for Images deleted before this migration.
-- A vec0 table identifies Image rows by rowid (not an image_id column).
DELETE FROM image_embedding_vectors
WHERE rowid NOT IN (SELECT id FROM images);

DELETE FROM visual_embedding_sources
WHERE entity_type = 'image'
  AND entity_id NOT IN (SELECT id FROM images);

-- Keep the index in sync for every Image deletion path, including library
-- scans and direct repository deletes. The trigger runs in the same transaction.
CREATE TRIGGER image_visual_embedding_cleanup
AFTER DELETE ON images
BEGIN
    DELETE FROM image_embedding_vectors WHERE rowid = OLD.id;
    DELETE FROM visual_embedding_sources
    WHERE entity_type = 'image' AND entity_id = OLD.id;
END;
