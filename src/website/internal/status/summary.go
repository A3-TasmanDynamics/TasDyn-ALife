package status

import (
	"context"
	"time"
)

// Summary is the short text form of the status page, for the Discord
// bot's /status: a headline plus one line per component.
func (m *Monitor) Summary(ctx context.Context) (string, []string, error) {
	snap, err := m.Snapshot(ctx, time.UTC)
	if err != nil {
		return "", nil, err
	}
	icons := map[string]string{"up": "🟢", "down": "🔴", "unknown": "⚪", "unmonitored": "⚫"}
	lines := make([]string, 0, len(snap.Components))
	for _, c := range snap.Components {
		line := icons[c.State] + " " + c.Name + " — " + c.StateText
		if c.Detail != "" {
			line += " (" + c.Detail + ")"
		}
		lines = append(lines, line)
	}
	return snap.Headline, lines, nil
}
