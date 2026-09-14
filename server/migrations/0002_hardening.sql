-- Phase 4 hardening: dual-approval to view an individual's screenshots.

CREATE TABLE IF NOT EXISTS view_approvals (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id   UUID NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    requested_by TEXT NOT NULL,           -- admin email
    reason       TEXT,
    approved_by  TEXT,                     -- admin email; NULL until approved
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    approved_at  TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS view_approvals_lookup
    ON view_approvals (machine_id, requested_by, expires_at);
