-- Optional human-friendly label for a machine (e.g. the employee's name), set
-- from the dashboard, so admins can identify devices beyond the hostname.
ALTER TABLE machines ADD COLUMN IF NOT EXISTS nickname TEXT;
