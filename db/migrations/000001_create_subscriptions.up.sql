CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS subscriptions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) NOT NULL,
    repo          VARCHAR(255) NOT NULL,
    confirmed     BOOLEAN NOT NULL DEFAULT FALSE,
    confirm_token VARCHAR(255) UNIQUE NOT NULL,
    unsub_token   VARCHAR(255) UNIQUE NOT NULL,
    last_seen_tag VARCHAR(255),
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_email_repo UNIQUE (email, repo)
);
