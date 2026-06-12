-- Run this against your Postgres database before starting the server.
-- v2: adds channels table, image_url, admin config

CREATE TABLE IF NOT EXISTS channels (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    color       TEXT        NOT NULL DEFAULT '#2f81f7',
    emoji       TEXT        NOT NULL DEFAULT '💬',
    logo_url    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id          BIGSERIAL PRIMARY KEY,
    channel_id  BIGINT      NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    content     TEXT        NOT NULL DEFAULT '',
    image_url   TEXT,
    view_count  BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reactions (
    message_id  BIGINT REFERENCES messages(id) ON DELETE CASCADE,
    emoji       TEXT   NOT NULL,
    count       BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (message_id, emoji)
);

-- seed a default channel
INSERT INTO channels (name, description, color, emoji)
VALUES ('Binahayat', 'The official Binahayat channel.', '#2f81f7', '⚡')
ON CONFLICT DO NOTHING;
