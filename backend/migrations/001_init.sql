-- Run this against your Postgres database before starting the server.

CREATE TABLE IF NOT EXISTS messages (
    id         BIGSERIAL PRIMARY KEY,
    content    TEXT        NOT NULL,
    view_count BIGINT      NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reactions (
    message_id BIGINT REFERENCES messages(id) ON DELETE CASCADE,
    emoji      TEXT   NOT NULL,
    count      BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (message_id, emoji)
);
