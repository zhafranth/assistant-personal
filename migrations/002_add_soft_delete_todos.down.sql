DROP INDEX IF EXISTS idx_todos_not_deleted;

ALTER TABLE todos DROP COLUMN deleted_at;
