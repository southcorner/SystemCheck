-- SystemCheck initial schema.
-- Requires the TimescaleDB extension (provided by the timescale/timescaledb image).

CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- Machines & agents
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS machines (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname      TEXT NOT NULL,
    os            TEXT NOT NULL DEFAULT 'windows',
    os_version    TEXT,
    assigned_user TEXT,                       -- person who uses this machine
    "group"       TEXT,                       -- e.g. 'designers', 'accountants'
    enrolled_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen     TIMESTAMPTZ,
    agent_version TEXT,
    cert_fingerprint TEXT UNIQUE,             -- SHA-256 of the agent client cert
    active        BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS enrollment_tokens (
    token_hash  TEXT PRIMARY KEY,             -- hash of a one-time token
    "group"     TEXT,
    assigned_user TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);

-- ---------------------------------------------------------------------------
-- Policies
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS policies (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    version    INT NOT NULL DEFAULT 1,
    doc        JSONB NOT NULL,                -- full policy document (see proto/agent.md)
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS policy_assignments (
    machine_id UUID REFERENCES machines(id) ON DELETE CASCADE,
    "group"    TEXT,
    policy_id  UUID NOT NULL REFERENCES policies(id) ON DELETE RESTRICT,
    CHECK (machine_id IS NOT NULL OR "group" IS NOT NULL)
);

-- ---------------------------------------------------------------------------
-- Consent (gates agent activation) -- see docs/PRIVACY_AND_CONSENT.md
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS consent_records (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_user   TEXT NOT NULL,
    machine_id     UUID REFERENCES machines(id) ON DELETE SET NULL,
    policy_version INT NOT NULL,
    method         TEXT NOT NULL,            -- 'click-through', 'signed-form', ...
    acknowledged_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at     TIMESTAMPTZ
);

-- ---------------------------------------------------------------------------
-- Events (high volume -> Timescale hypertable)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS events (
    ts         TIMESTAMPTZ NOT NULL,
    machine_id UUID NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,                -- foreground|screenshot|dns|netflow|...
    data       JSONB NOT NULL DEFAULT '{}'
);
SELECT create_hypertable('events', 'ts', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS events_machine_kind_ts ON events (machine_id, kind, ts DESC);

-- Screenshot metadata (blob lives in MinIO under object_key)
CREATE TABLE IF NOT EXISTS screenshots (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id UUID NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    ts         TIMESTAMPTZ NOT NULL,
    object_key TEXT NOT NULL,
    width      INT,
    height     INT,
    bytes      BIGINT,
    format     TEXT
);
CREATE INDEX IF NOT EXISTS screenshots_machine_ts ON screenshots (machine_id, ts DESC);

-- Rolled-up app usage
CREATE TABLE IF NOT EXISTS app_usage (
    machine_id UUID NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    day        DATE NOT NULL,
    process    TEXT NOT NULL,
    active_sec BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (machine_id, day, process)
);

-- ---------------------------------------------------------------------------
-- Admin accounts, sessions, audit
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS admin_users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,             -- argon2id
    role          TEXT NOT NULL DEFAULT 'viewer',  -- admin|auditor|viewer
    totp_secret   TEXT,                      -- base32; NULL until MFA enrolled
    mfa_enabled   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    disabled      BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    mfa_passed BOOLEAN NOT NULL DEFAULT FALSE
);

-- Append-only: no UPDATE/DELETE granted to the app role in production.
CREATE TABLE IF NOT EXISTS audit_log (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ts         TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor      TEXT,                          -- admin email or 'system'
    action     TEXT NOT NULL,                 -- login|view_screenshots|export|policy_update|...
    target     TEXT,                          -- machine/user/resource affected
    detail     JSONB NOT NULL DEFAULT '{}'
);

-- ---------------------------------------------------------------------------
-- Alerts
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS alert_rules (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name     TEXT NOT NULL,
    kind     TEXT NOT NULL,                   -- domain_blocklist|large_upload|usb|...
    params   JSONB NOT NULL DEFAULT '{}',
    enabled  BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS alerts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id    UUID REFERENCES alert_rules(id) ON DELETE SET NULL,
    machine_id UUID REFERENCES machines(id) ON DELETE CASCADE,
    ts         TIMESTAMPTZ NOT NULL DEFAULT now(),
    severity   TEXT NOT NULL DEFAULT 'info',
    message    TEXT NOT NULL,
    data       JSONB NOT NULL DEFAULT '{}',
    acknowledged BOOLEAN NOT NULL DEFAULT FALSE
);
