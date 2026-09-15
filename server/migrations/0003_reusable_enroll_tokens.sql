-- Reusable (group) enrollment tokens: one token can enroll many machines within
-- its TTL, so a single installer can be deployed across a fleet. Single-use
-- tokens remain the default (reusable = false) and keep their existing behavior.
ALTER TABLE enrollment_tokens ADD COLUMN IF NOT EXISTS reusable  BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE enrollment_tokens ADD COLUMN IF NOT EXISTS max_uses  INT;              -- NULL = unlimited (only meaningful when reusable)
ALTER TABLE enrollment_tokens ADD COLUMN IF NOT EXISTS use_count INT NOT NULL DEFAULT 0;
