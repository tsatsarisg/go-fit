-- +goose Up
-- +goose StatementBegin
-- Widen tokens.expiry precision from TIMESTAMP(0) to TIMESTAMP(3) so
-- token expiry rounds to milliseconds instead of whole seconds, matching
-- the precision used by Go's time.Now() in the issuance path.
ALTER TABLE tokens
    ALTER COLUMN expiry TYPE TIMESTAMP(3) WITH TIME ZONE USING expiry;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tokens
    ALTER COLUMN expiry TYPE TIMESTAMP(0) WITH TIME ZONE USING expiry;
-- +goose StatementEnd
