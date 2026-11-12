-- Admin panel speed: indexes for the queries the panel runs on every
-- dashboard / Online Now load (and on each live refresh).
-- Players online over the last day: only sessions that ended recently.
CREATE INDEX IF NOT EXISTS idx_player_sessions_disconnected ON player_sessions(disconnected_at);
-- Event Log (newest first, any filter by time): read the first page from
-- the index instead of sorting the whole window.
CREATE INDEX IF NOT EXISTS idx_player_log_time ON player_log(created_at DESC, id DESC);
