-- +goose Up
-- +goose StatementBegin
ALTER TABLE chat_members 
ADD COLUMN cleared_at TIMESTAMPTZ NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE chat_members 
DROP COLUMN IF EXISTS cleared_at;
-- +goose StatementEnd
