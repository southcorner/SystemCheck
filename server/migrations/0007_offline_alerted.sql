-- Tracks whether we've already raised an "agent offline" alert for a machine's
-- current outage, so the periodic sweep alerts once per outage (cleared when the
-- machine heartbeats again).
ALTER TABLE machines ADD COLUMN IF NOT EXISTS offline_alerted BOOLEAN NOT NULL DEFAULT false;
