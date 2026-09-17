-- The most recent agent log tail uploaded on demand (server "sendlog" command),
-- for investigating a machine without remote access.
ALTER TABLE machines ADD COLUMN IF NOT EXISTS last_log    TEXT;
ALTER TABLE machines ADD COLUMN IF NOT EXISTS last_log_at TIMESTAMPTZ;
