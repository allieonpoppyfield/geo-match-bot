-- +goose Up
-- +goose StatementBegin
alter table users add column title_name varchar(255);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table users drop column title_name;
-- +goose StatementEnd
