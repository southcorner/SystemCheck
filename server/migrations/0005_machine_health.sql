-- Per-machine agent health: the latest collector-status report from the agent,
-- so the dashboard can show what's running and the server can alert when a
-- collector that should be running isn't.
ALTER TABLE machines ADD COLUMN IF NOT EXISTS collectors JSONB;
ALTER TABLE machines ADD COLUMN IF NOT EXISTS health_at  TIMESTAMPTZ;
