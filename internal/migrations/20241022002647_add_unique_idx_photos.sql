-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied.

CREATE UNIQUE INDEX IF NOT EXISTS idx_photos_user_id ON photos (user_id);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back.

DROP INDEX IF EXISTS idx_photos_user_id;
