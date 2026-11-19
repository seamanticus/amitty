package member

import (
	"slices"
	"testing"

	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/state"
)

type mockNingen struct {
	*state.State
	MemberState *State
}

func ningenFromState(s *state.State) *mockNingen {
	return &mockNingen{s, NewState(s, s.Handler)}
}

func TestComputeListID(t *testing.T) {
	perms := []discord.Overwrite{
		{Type: discord.OverwriteRole, ID: 361910177961738242, Deny: 1024, Allow: 0},
		{Type: discord.OverwriteRole, ID: 361919857836425217, Deny: 0, Allow: 117760},
		{Type: discord.OverwriteRole, ID: 532359766694035457, Deny: 0, Allow: 10240},
		{Type: discord.OverwriteRole, ID: 564702909519101952, Deny: 93184, Allow: 0},
		{Type: discord.OverwriteRole, ID: 578035907232530432, Deny: 2112, Allow: 0},
		{Type: discord.OverwriteRole, ID: 697931217521082455, Deny: 0, Allow: 1024},
	}

	if id := ComputeListID(perms); id != "3720633681" {
		t.Fatal("Unexpected ID:", id, "expected", "3720633681")
	}

	// The ID does not depend on the order of the overwrites.
	slices.Reverse(perms)
	if id := ComputeListID(perms); id != "3720633681" {
		t.Fatal("Unexpected ID for reversed overwrites:", id, "expected", "3720633681")
	}

	// IDs of different lengths are sorted as text, as discord.py-self does.
	perms = []discord.Overwrite{
		{Type: discord.OverwriteRole, ID: 99999999999999999, Allow: discord.PermissionViewChannel},
		{Type: discord.OverwriteRole, ID: 1000000000000000000, Allow: discord.PermissionViewChannel},
	}
	if id := ComputeListID(perms); id != "1908181601" {
		t.Fatal("Unexpected ID for IDs of different lengths:", id, "expected", "1908181601")
	}
}

func TestRequestMemberList(t *testing.T) {
	s := discordtest.State(t)
	s.Cabinet.ChannelSet(&discord.Channel{ID: 1, GuildID: 10}, false)
	s.Cabinet.ChannelSet(&discord.Channel{ID: 2, GuildID: 10}, false)
	m := NewState(s, s.Handler)
	// The gateway is closed, so the subscriptions fail to send.
	m.OnError = func(error) {}

	for _, tt := range []struct {
		channelID discord.ChannelID
		want      bool
	}{
		{1, true},
		{1, false},
		{2, true},
		// Returning to a channel subscribes to it again.
		{1, true},
	} {
		if got := m.RequestMemberList(10, tt.channelID, 0) != nil; got != tt.want {
			t.Fatalf("requested channel %d = %v, want %v", tt.channelID, got, tt.want)
		}
	}
}

func TestStateListID(t *testing.T) {
	const guildID = 10
	s := discordtest.State(t)
	s.Cabinet.RoleSet(guildID, &discord.Role{ID: guildID, Permissions: discord.PermissionViewChannel}, false)
	// In the other guild, the @everyone role cannot view channels.
	s.Cabinet.RoleSet(guildID+1, &discord.Role{ID: guildID + 1}, false)
	m := NewState(s, s.Handler)

	for _, tt := range []struct {
		name       string
		guildID    discord.GuildID
		overwrites []discord.Overwrite
		want       string
	}{
		{"no overwrites", guildID, nil, "everyone"},
		{"everyone allowed", guildID, []discord.Overwrite{{ID: guildID, Type: discord.OverwriteRole, Allow: discord.PermissionViewChannel}}, "everyone"},
		{"everyone denied", guildID, []discord.Overwrite{{ID: guildID, Type: discord.OverwriteRole, Deny: discord.PermissionViewChannel}}, ComputeListID([]discord.Overwrite{{ID: guildID, Deny: discord.PermissionViewChannel}})},
		{"role allowed", guildID, []discord.Overwrite{{ID: 2, Type: discord.OverwriteRole, Allow: discord.PermissionViewChannel}}, "everyone"},
		{"role denied", guildID, []discord.Overwrite{{ID: 2, Type: discord.OverwriteRole, Deny: discord.PermissionViewChannel}}, ComputeListID([]discord.Overwrite{{ID: 2, Deny: discord.PermissionViewChannel}})},
		// Like Discord, an @everyone overwrite that allows viewing does not make the list "everyone".
		{"everyone cannot view", guildID + 1, nil, "0"},
		{"everyone cannot view but is allowed", guildID + 1, []discord.Overwrite{{ID: guildID + 1, Type: discord.OverwriteRole, Allow: discord.PermissionViewChannel}}, ComputeListID([]discord.Overwrite{{ID: guildID + 1, Allow: discord.PermissionViewChannel}})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.ListID(&discord.Channel{GuildID: tt.guildID, Overwrites: tt.overwrites}); got != tt.want {
				t.Fatalf("ListID() = %q, want %q", got, tt.want)
			}
		})
	}
}
