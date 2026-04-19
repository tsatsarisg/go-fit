-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_workouts_user_id
    ON workouts (user_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_workout_entries_workout_id_order_index
    ON workout_entries (workout_id, order_index);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_tokens_user_id
    ON tokens (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tokens_user_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_workout_entries_workout_id_order_index;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_workouts_user_id;
-- +goose StatementEnd
