package gateway

import (
	"reflect"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/json"
)

// evidence: witnesses
func TestRequestGuildMembersCommand(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  RequestGuildMembersCommand
		want map[string]any
	}{
		{"by id", MembersByID([]discord.GuildID{123}, []discord.UserID{456}), map[string]any{
			"guild_id": []any{"123"}, "user_ids": []any{"456"}, "limit": float64(0), "presences": false,
		}},
		{"by name", MembersByName([]discord.GuildID{123}, "abc", 10), map[string]any{
			"guild_id": []any{"123"}, "query": "abc", "limit": float64(10), "presences": false,
		}},
		// An empty query is still sent: it asks for every member, up to limit.
		{"by empty name", MembersByName([]discord.GuildID{123}, "", 0), map[string]any{
			"guild_id": []any{"123"}, "query": "", "limit": float64(0), "presences": false,
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(&tt.cmd)
			if err != nil {
				t.Fatal("failed to marshal command:", err)
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal("failed to unmarshal command:", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("command = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// evidence: witnesses
func TestReadyEventDecodes(t *testing.T) {
	ready := NewOpUnmarshalers().Lookup(0, "READY")().(*ReadyEvent)
	payload := `{
		"user": {"id": "1"},
		"auth_token": "new",
		"read_state": {"entries": [{"id": "2"}], "partial": false, "version": 4},
		"user_guild_settings": {"entries": [{"guild_id": "3"}], "partial": true, "version": 5}
	}`
	if err := json.Unmarshal([]byte(payload), ready); err != nil {
		t.Fatal("failed to unmarshal READY:", err)
	}
	if ready.AuthToken != "new" {
		t.Errorf("auth token = %q, want new", ready.AuthToken)
	}
	if rs := ready.ReadStates; len(rs.Entries) != 1 || rs.Entries[0].ChannelID != 2 || rs.Version != 4 {
		t.Errorf("read states = %#v", rs)
	}
	if gs := ready.UserGuildSettings; len(gs.Entries) != 1 || gs.Entries[0].GuildID != 3 || !gs.Partial {
		t.Errorf("user guild settings = %#v", gs)
	}
}

// evidence: witnesses
func TestReadyEventHydrateUsers(t *testing.T) {
	ready := NewOpUnmarshalers().Lookup(0, "READY")().(*ReadyEvent)
	payload := []byte(`{
		"user": {"id": "1"},
		"users": [{"id": "3", "username": "friend"}],
		"private_channels": [{"id": "2", "recipient_ids": ["3"]}, {"id": "4", "recipient_ids": ["5"]}],
		"relationships": [{"id": "3", "type": 1}]
	}`)
	if err := json.Unmarshal(payload, ready); err != nil {
		t.Fatal("failed to unmarshal READY:", err)
	}
	if len(ready.PrivateChannels[0].DMRecipients) != 0 {
		t.Fatal("decoding filled recipients; only HydrateUsers should")
	}
	ready.HydrateUsers()

	if got := ready.PrivateChannels[0].DMRecipients; len(got) != 1 || got[0].ID != 3 || got[0].Username != "friend" {
		t.Errorf("known recipient = %#v", got)
	}
	// An ID missing from users keeps its ID, so the DM still has its recipient.
	if got := ready.PrivateChannels[1].DMRecipients; len(got) != 1 || got[0].ID != 5 {
		t.Errorf("unknown recipient = %#v", got)
	}
	if got := ready.Relationships[0].User; got.Username != "friend" {
		t.Errorf("relationship user = %#v", got)
	}
}
