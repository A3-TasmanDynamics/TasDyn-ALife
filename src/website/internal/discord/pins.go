package discord

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// The bot pins the messages it keeps up to date -- the #welcome message and
// the live #server-status message -- so they stay at hand for members.
// Pinning needs Discord's "Pin Messages" permission, which discordgo
// v0.29 doesn't name yet.
const permissionPinMessages int64 = 1 << 51

// pinMessage pins a message, if it isn't already. A failure never undoes the
// post: callers report or log it and carry on.
func (b *Bot) pinMessage(ctx context.Context, channelID, messageID string) error {
	err := b.session.ChannelMessagePin(channelID, messageID, discordgo.WithContext(ctx))
	if err == nil {
		return nil
	}
	var rest *discordgo.RESTError
	if errors.As(err, &rest) && rest.Message != nil {
		switch rest.Message.Code {
		case discordgo.ErrCodeMissingPermissions, discordgo.ErrCodeMissingAccess:
			return fmt.Errorf("the bot needs Pin Messages in that channel: %w", err)
		case discordgo.ErrCodeMaximumPinsReached:
			return fmt.Errorf("that channel already has the maximum 50 pins: %w", err)
		}
	}
	return err
}
