package discord

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// RoleSyncAdapter is the Discord side of internal/rolesync (it satisfies
// rolesync.Adapter without importing it, which would be an import cycle): a player's
// identity is their linked players.discord_id, groups are guild roles.
type RoleSyncAdapter struct {
	Bot *Bot
}

func (a RoleSyncAdapter) Platform() string { return "discord" }

func (a RoleSyncAdapter) Identities(ctx context.Context, playerID int64) ([]string, error) {
	var id *string
	if err := a.Bot.pool.QueryRow(ctx, `SELECT discord_id FROM players WHERE id = $1`, playerID).Scan(&id); err != nil {
		return nil, err
	}
	if id == nil || *id == "" {
		return nil, nil
	}
	return []string{*id}, nil
}

func (a RoleSyncAdapter) CurrentGroups(ctx context.Context, identity string) ([]string, bool, error) {
	m, err := a.Bot.session.GuildMember(a.Bot.guildID, identity, discordgo.WithContext(ctx))
	if err != nil {
		var rest *discordgo.RESTError
		if errors.As(err, &rest) && rest.Response != nil && rest.Response.StatusCode == http.StatusNotFound {
			return nil, false, nil // not in the server (yet, or any more)
		}
		return nil, false, err
	}
	return m.Roles, true, nil
}

func (a RoleSyncAdapter) AddGroup(ctx context.Context, identity, group string) error {
	return a.Bot.session.GuildMemberRoleAdd(a.Bot.guildID, identity, group,
		discordgo.WithContext(ctx), discordgo.WithAuditLogReason("TasDyn-ALife role sync"))
}

func (a RoleSyncAdapter) RemoveGroup(ctx context.Context, identity, group string) error {
	return a.Bot.session.GuildMemberRoleRemove(a.Bot.guildID, identity, group,
		discordgo.WithContext(ctx), discordgo.WithAuditLogReason("TasDyn-ALife role sync"))
}

// GuildRole is a role offered in the role-sync mapping UI.
type GuildRole struct {
	ID       string
	Name     string
	Position int
	// Assignable is false for @everyone, integration-managed roles (bots,
	// boosters) and roles at or above the bot's own highest role -- Discord
	// won't let the bot assign those, so the UI won't offer them.
	Assignable bool
	Why        string
}

// GuildRoles lists the guild's roles, highest first.
func (b *Bot) GuildRoles() ([]GuildRole, error) {
	if b.guildID == "" {
		return nil, errors.New("no DISCORD_GUILD_ID configured")
	}
	roles, err := b.session.GuildRoles(b.guildID)
	if err != nil {
		return nil, err
	}
	me, err := b.session.GuildMember(b.guildID, b.session.State.User.ID)
	if err != nil {
		return nil, err
	}
	top := 0
	for _, r := range roles {
		for _, id := range me.Roles {
			if r.ID == id && r.Position > top {
				top = r.Position
			}
		}
	}
	out := make([]GuildRole, 0, len(roles))
	for _, r := range roles {
		gr := GuildRole{ID: r.ID, Name: r.Name, Position: r.Position, Assignable: true}
		switch {
		case r.ID == b.guildID:
			continue // @everyone
		case r.Managed:
			gr.Assignable, gr.Why = false, "managed by an integration"
		case r.Position >= top:
			gr.Assignable, gr.Why = false, "above the bot's own role"
		}
		out = append(out, gr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position > out[j].Position })
	return out, nil
}

// CanManageRoles reports whether the bot has the Manage Roles permission.
func (b *Bot) CanManageRoles() bool {
	missing, err := b.missingPermissionsFor([]permission{{discordgo.PermissionManageRoles, "Manage Roles"}})
	// Administrator (reported as a warning line) implies Manage Roles.
	return err == nil && (len(missing) == 0 || strings.HasPrefix(missing[0], "(bot has Administrator"))
}
