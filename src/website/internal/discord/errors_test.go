package discord

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func restErr(status, code int) error {
	return fmt.Errorf("wrapped: %w", &discordgo.RESTError{
		Response: &http.Response{StatusCode: status},
		Message:  &discordgo.APIErrorMessage{Code: code},
	})
}

// After moving to a new Discord app, the live status and welcome messages
// were posted by the old bot and can't be edited (403, code 50005): those
// must be reposted, like deleted ones, not retried forever.
func TestCantEditStored(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"deleted", restErr(http.StatusNotFound, discordgo.ErrCodeUnknownMessage), true},
		{"another bot's message", restErr(http.StatusForbidden, discordgo.ErrCodeCannotEditFromAnotherUser), true},
		{"missing permission", restErr(http.StatusForbidden, discordgo.ErrCodeMissingPermissions), false},
		{"rate limited / outage", restErr(http.StatusInternalServerError, 0), false},
		{"network", errors.New("connection reset"), false},
	}
	for _, c := range cases {
		if got := cantEditStored(c.err); got != c.want {
			t.Errorf("%s: cantEditStored = %v, want %v", c.name, got, c.want)
		}
	}
}
