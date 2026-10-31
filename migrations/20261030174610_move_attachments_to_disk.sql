-- +goose Up
-- +goose StatementBegin
-- Chat attachments
ALTER TABLE chat_attachments ADD COLUMN file_path TEXT;
-- (Optional) We drop the BYTEA column to save space immediately. 
-- If you want to keep old files, you'd need a script to write them to disk first.
ALTER TABLE chat_attachments DROP COLUMN encrypted_data;

-- Order deliverables
ALTER TABLE order_deliverables ADD COLUMN file_path TEXT;
ALTER TABLE order_deliverables DROP COLUMN encrypted_data;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE chat_attachments DROP COLUMN file_path;
ALTER TABLE chat_attachments ADD COLUMN encrypted_data BYTEA;

ALTER TABLE order_deliverables DROP COLUMN file_path;
ALTER TABLE order_deliverables ADD COLUMN encrypted_data BYTEA;
-- +goose StatementEnd
