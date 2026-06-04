-- +goose Up
CREATE TABLE chirps (
    id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    body TEXT NOT NULL,
    user_id UUID NOT NULL
);

-- +goose Down
DROP TABLE chirps;