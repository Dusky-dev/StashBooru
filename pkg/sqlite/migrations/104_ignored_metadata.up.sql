-- Ignored Tags, Characters, Artists and Copyrights are hidden from lists and
-- never added again by downloads or tagging. Existing media links remain.
ALTER TABLE tags ADD COLUMN ignored BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE performers ADD COLUMN ignored BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE studios ADD COLUMN ignored BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE copyrights ADD COLUMN ignored BOOLEAN NOT NULL DEFAULT 0;
