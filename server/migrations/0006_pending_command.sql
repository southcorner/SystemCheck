-- A one-shot command the server hands the agent on its next heartbeat
-- (e.g. "restart" its collectors, "sendlog"). Cleared once delivered.
ALTER TABLE machines ADD COLUMN IF NOT EXISTS pending_command TEXT;
