-- Migration to create favorites table for user stories and tasks

CREATE TABLE IF NOT EXISTS favorites (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    item_type VARCHAR(50) NOT NULL,
    user_story_id UUID,
    task_id UUID,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_favorites_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_favorites_user_story FOREIGN KEY (user_story_id) REFERENCES user_stories(id),
    CONSTRAINT fk_favorites_task FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_favorites_user_user_story ON favorites(user_id, user_story_id) WHERE user_story_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_favorites_user_task ON favorites(user_id, task_id) WHERE task_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_favorites_user_id ON favorites(user_id);
CREATE INDEX IF NOT EXISTS idx_favorites_user_story_id ON favorites(user_story_id);
CREATE INDEX IF NOT EXISTS idx_favorites_task_id ON favorites(task_id);
