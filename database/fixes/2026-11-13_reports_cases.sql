-- Player reports -> cases: a report ticket (Player Report, Report a Member)
-- can be turned into a moderation case; the two stay linked, and closing
-- the case tells the reporter on their ticket.
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS case_id BIGINT REFERENCES staff_cases(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_support_tickets_case ON support_tickets(case_id) WHERE case_id IS NOT NULL;
