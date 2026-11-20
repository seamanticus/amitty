package ui

import (
	"slices"
	"strings"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen"
)

// IsMe reports whether id is the logged in user.
func IsMe(state *ningen.State, id discord.UserID) bool {
	me, _ := state.Cabinet.Me()
	return me != nil && id == me.ID
}

// IsDeleted reports whether user is a deleted account, as Discord renames one: "Deleted User" with the tag 0000 before the username migration, "deleted_user_" and a hex code after it.
// A user cannot pick either name: the first has a space and capitals, which new usernames lack, but the second could be claimed, so a live account named so is hidden too.
func IsDeleted(user discord.User) bool {
	return user.Username == "Deleted User" && (user.Discriminator == "0000" || user.Discriminator == "0") ||
		strings.HasPrefix(user.Username, "deleted_user_")
}

// VisiblePrivateChannels returns the DMs and group DMs to show, most recent first: all of them but those Discord filtered as spam, and DMs with a deleted account when hideDeleted.
func VisiblePrivateChannels(state *ningen.State, hideDeleted bool) ([]discord.Channel, error) {
	channels, err := state.Cabinet.PrivateChannels()
	if err != nil {
		return nil, err
	}
	channels = slices.DeleteFunc(slices.Clone(channels), func(channel discord.Channel) bool {
		deleted := channel.Type == discord.DirectMessage && len(channel.DMRecipients) == 1 && IsDeleted(channel.DMRecipients[0])
		return channel.IsSpam || hideDeleted && deleted
	})
	SortPrivateChannels(channels)
	return channels, nil
}

// DirectMessage returns your DM with userID, if you have one.
func DirectMessage(state *ningen.State, userID discord.UserID) (discord.Channel, bool) {
	channels, err := state.Cabinet.PrivateChannels()
	if err != nil {
		return discord.Channel{}, false
	}
	index := slices.IndexFunc(channels, func(channel discord.Channel) bool {
		return channel.Type == discord.DirectMessage && len(channel.DMRecipients) == 1 && channel.DMRecipients[0].ID == userID
	})
	if index < 0 {
		return discord.Channel{}, false
	}
	return channels[index], true
}
