package audit

import (
	"strings"
	"testing"
)

func TestDescribeRankChange(t *testing.T) {
	got := Describe("rank_change:staff_rank_id",
		[]byte(`{"value":"2","label":"Moderator"}`), []byte(`{"value":"3","label":"Admin"}`))
	if got != "changed staff rank: Moderator → Admin" {
		t.Fatalf("got %q", got)
	}
}

func TestDescribeMissingValues(t *testing.T) {
	got := Describe("rank_change:cop_level", nil, []byte(`{"value":"3","label":"3"}`))
	if got != "changed police level: none → 3" {
		t.Fatalf("got %q", got)
	}
}

func TestDescribeOtherAction(t *testing.T) {
	if got := Describe("devlog_post", nil, nil); got != "devlog_post" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatEscapesAndAttributes(t *testing.T) {
	msg := Format("**Evil**", "Sam_`x`", "rank_change:staff_rank_id",
		"@everyone see this", []byte(`{"label":"Moderator"}`), []byte(`{"label":"Admin"}`), SourceWebsite)
	for _, want := range []string{`**\*\*Evil\*\***`, "Sam\\_\\`x\\`", "Moderator → Admin", "_via website_"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
	// Mentions are disabled at the webhook layer; the text itself stays readable.
	if !strings.Contains(msg, "@everyone") {
		t.Errorf("reason text should be kept: %q", msg)
	}
}

func TestFormatSourceWithoutStaff(t *testing.T) {
	if msg := Format("", "Sam", "rank_change:cop_level", "", nil, nil, SourceGame); !strings.HasPrefix(msg, "**In-game**") {
		t.Errorf("game change: %q", msg)
	}
	if msg := Format("", "Sam", "rank_change:cop_level", "", nil, nil, SourceManual); !strings.Contains(msg, "Manual database change") {
		t.Errorf("manual change: %q", msg)
	}
}
