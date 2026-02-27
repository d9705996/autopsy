-- 0002_users.up.sql
CREATE TABLE IF NOT EXISTS users (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID        NULL,          -- FR-000j: multi-tenancy schema hook (no FK, not enforced until MVP)
    email           TEXT        NOT NULL UNIQUE,
    name            TEXT        NOT NULL DEFAULT '',
    password_hash   TEXT        NOT NULL DEFAULT '',
    roles           TEXT[]      NOT NULL DEFAULT '{}',
    notification_channels JSONB NOT NULL DEFAULT '[]',
    oidc_sub        TEXT        NULL,
    deactivated_at  TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
