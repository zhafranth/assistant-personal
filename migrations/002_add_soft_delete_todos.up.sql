ALTER TABLE todos ADD COLUMN deleted_at TIMESTAMPTZ;

CREATE INDEX idx_todos_not_deleted ON todos (user_id) WHERE deleted_at IS NULL;
