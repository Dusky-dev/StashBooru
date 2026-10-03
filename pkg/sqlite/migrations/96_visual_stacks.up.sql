CREATE TABLE visual_stacks (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE visual_stack_members (
  stack_id INTEGER NOT NULL REFERENCES visual_stacks(id) ON DELETE CASCADE,
  image_id INTEGER REFERENCES images(id) ON DELETE CASCADE,
  scene_id INTEGER REFERENCES scenes(id) ON DELETE CASCADE,
  position INTEGER NOT NULL CHECK(position >= 0),
  label TEXT NOT NULL DEFAULT '',
  representative INTEGER NOT NULL DEFAULT 0 CHECK(representative IN (0,1)),
  CHECK((image_id IS NOT NULL) <> (scene_id IS NOT NULL)),
  UNIQUE(image_id),
  UNIQUE(scene_id),
  UNIQUE(stack_id, position)
);
CREATE UNIQUE INDEX visual_stack_representative ON visual_stack_members(stack_id) WHERE representative = 1;
CREATE TRIGGER visual_stack_member_deleted AFTER DELETE ON visual_stack_members
BEGIN
  UPDATE visual_stack_members SET representative = 1
  WHERE stack_id = OLD.stack_id AND position = (SELECT MIN(position) FROM visual_stack_members WHERE stack_id = OLD.stack_id)
    AND NOT EXISTS(SELECT 1 FROM visual_stack_members WHERE stack_id = OLD.stack_id AND representative = 1);
  UPDATE visual_stacks SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = OLD.stack_id;
  DELETE FROM visual_stacks WHERE id = OLD.stack_id AND NOT EXISTS(SELECT 1 FROM visual_stack_members WHERE stack_id = OLD.stack_id);
END;
