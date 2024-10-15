-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied.

-- Изменение типа данных user_id на BIGINT
ALTER TABLE photos ALTER COLUMN user_id TYPE BIGINT;

-- В случае если ограничение на поле photo_url мало
-- ALTER TABLE photos ALTER COLUMN photo_url TYPE TEXT;

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back.

-- Откат обратно на тип INT, если необходимо
ALTER TABLE photos ALTER COLUMN user_id TYPE INTEGER;

-- ALTER TABLE photos ALTER COLUMN photo_url TYPE VARCHAR(255);
