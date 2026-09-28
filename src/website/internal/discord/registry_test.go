package discord

import (
	"regexp"
	"testing"

	"github.com/bwmarrin/discordgo"

	"website/internal/auth"
	"website/internal/status"
)

var slashName = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// Discord rejects the whole bulk registration if any one command is
// invalid, which would leave the bot with no commands at all -- so the
// rules it enforces are checked here instead of discovered at startup.
func TestRegistryIsValid(t *testing.T) {
	b := &Bot{}
	cmds, comps := b.registry()
	if len(cmds) > 100 {
		t.Fatalf("%d commands, Discord allows 100", len(cmds))
	}
	for name, c := range cmds {
		if c.Def.Name != name {
			t.Errorf("%s: registered under the wrong name", name)
		}
		if c.Permission != "" && !auth.Known(c.Permission) {
			t.Errorf("%s: permission %q isn't in the catalogue", name, c.Permission)
		}
		if c.Def.Type == discordgo.UserApplicationCommand {
			if c.Def.Description != "" || len(c.Def.Options) > 0 {
				t.Errorf("%s: context menu commands can't have a description or options", name)
			}
			continue
		}
		if !slashName.MatchString(name) {
			t.Errorf("%s: slash command names must be lowercase, 1-32 chars", name)
		}
		if l := len(c.Def.Description); l == 0 || l > 100 {
			t.Errorf("%s: description must be 1-100 chars, is %d", name, l)
		}
		checkOptions(t, name, c.Def.Options, c.Autocomplete != nil)
	}
	for prefix, c := range comps {
		if c.Prefix != prefix || prefix == "" {
			t.Errorf("component %q registered under the wrong prefix", prefix)
		}
		if c.Permission != "" && !auth.Known(c.Permission) {
			t.Errorf("component %s: permission %q isn't in the catalogue", prefix, c.Permission)
		}
	}
}

func checkOptions(t *testing.T, cmd string, opts []*discordgo.ApplicationCommandOption, hasAutocomplete bool) {
	t.Helper()
	optional := false
	for _, o := range opts {
		if !slashName.MatchString(o.Name) {
			t.Errorf("%s: option name %q invalid", cmd, o.Name)
		}
		if l := len(o.Description); l == 0 || l > 100 {
			t.Errorf("%s %s: description must be 1-100 chars, is %d", cmd, o.Name, l)
		}
		if o.Autocomplete && !hasAutocomplete {
			t.Errorf("%s %s: autocomplete option but the command has no Autocomplete handler", cmd, o.Name)
		}
		if o.Type == discordgo.ApplicationCommandOptionSubCommand {
			checkOptions(t, cmd+" "+o.Name, o.Options, hasAutocomplete)
			continue
		}
		if o.Required && optional {
			t.Errorf("%s: required option %q after an optional one", cmd, o.Name)
		}
		if !o.Required {
			optional = true
		}
	}
}

func TestPresenceText(t *testing.T) {
	snap := status.Snapshot{Components: []status.ComponentView{{Key: "game", State: "up", Detail: "12 / 64 players"}}}
	if got := presenceText(snap, nil, nil); got != "12/64 on Altis" {
		t.Errorf("up: %q", got)
	}
	snap.Components[0].State = "down"
	if got := presenceText(snap, nil, nil); got != "Server offline" {
		t.Errorf("down: %q", got)
	}
}
