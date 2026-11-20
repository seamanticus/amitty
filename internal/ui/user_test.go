package ui

import (
	"slices"
	"testing"

	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen"
)

func TestIsDeleted(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		user discord.User
		want bool
	}{
		{discord.User{Username: "Deleted User", Discriminator: "0000"}, true},
		{discord.User{Username: "Deleted User", Discriminator: "0"}, true},
		{discord.User{Username: "deleted_user_3f9a1b2c4d5e", Discriminator: "0"}, true},
		{discord.User{Username: "Deleted User", Discriminator: "1234"}, false},
		{discord.User{Username: "deleteduser", Discriminator: "0"}, false},
		{discord.User{}, false},
	} {
		if got := IsDeleted(test.user); got != test.want {
			t.Errorf("IsDeleted(%+v) = %v, want %v", test.user, got, test.want)
		}
	}
}

func TestVisiblePrivateChannels(t *testing.T) {
	state := ningen.FromState(discordtest.State(t))
	deleted := discord.User{ID: 2, Username: "deleted_user_ab12"}
	for _, channel := range []discord.Channel{
		{ID: 10, Type: discord.DirectMessage, DMRecipients: []discord.User{{ID: 1, Username: "ann"}}, LastMessageID: 30},
		{ID: 11, Type: discord.DirectMessage, DMRecipients: []discord.User{deleted}, LastMessageID: 31},
		// A group with a deleted member is still a group of the others.
		{ID: 12, Type: discord.GroupDM, DMRecipients: []discord.User{{ID: 1}, deleted}, LastMessageID: 32},
	} {
		if err := state.Cabinet.ChannelSet(&channel, false); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(hide bool) []discord.ChannelID {
		channels, err := VisiblePrivateChannels(state, hide)
		if err != nil {
			t.Fatal(err)
		}
		var out []discord.ChannelID
		for _, channel := range channels {
			out = append(out, channel.ID)
		}
		return out
	}
	if got, want := ids(true), []discord.ChannelID{12, 10}; !slices.Equal(got, want) {
		t.Errorf("hidden = %v, want %v, newest first", got, want)
	}
	if got, want := ids(false), []discord.ChannelID{12, 11, 10}; !slices.Equal(got, want) {
		t.Errorf("shown = %v, want %v", got, want)
	}
}
