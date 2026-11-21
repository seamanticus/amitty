package gateway

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/json"
	"github.com/seamanticus/amitty/internal/dmwire/utils/json/option"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
)

func TestRequestGuildMembersCommand(t *testing.T) {
	assert := func(cmd Event, data map[string]any) {
		cmdBytes, err := json.Marshal(cmd)
		if err != nil {
			t.Fatal("failed to marshal command:", err)
		}

		var cmdMap map[string]any
		if err := json.Unmarshal(cmdBytes, &cmdMap); err != nil {
			t.Fatal("failed to unmarshal command:", err)
		}

		if !reflect.DeepEqual(cmdMap, data) {
			t.Fatalf("mismatched command, got %#v", cmdMap)
		}
	}

	t.Run("userIDs", func(t *testing.T) {
		cmd := RequestGuildMembersCommand{
			GuildIDs: []discord.GuildID{123},
			UserIDs:  []discord.UserID{456},
		}

		assert(&cmd, map[string]any{
			"guild_id":  []any{"123"},
			"user_ids":  []any{"456"},
			"presences": false,
		})
	})

	t.Run("query_empty", func(t *testing.T) {
		cmd := RequestGuildMembersCommand{
			GuildIDs: []discord.GuildID{123},
			Query:    option.Some(""),
		}

		assert(&cmd, map[string]any{
			"guild_id":  []any{"123"},
			"query":     "",
			"limit":     float64(0),
			"presences": false,
		})
	})

	t.Run("query_nonempty", func(t *testing.T) {
		cmd := RequestGuildMembersCommand{
			GuildIDs: []discord.GuildID{123},
			Query:    option.Some("abc"),
		}

		assert(&cmd, map[string]any{
			"guild_id":  []any{"123"},
			"query":     "abc",
			"limit":     float64(0),
			"presences": false,
		})
	})

	t.Run("both", func(t *testing.T) {
		cmd := RequestGuildMembersCommand{
			GuildIDs: []discord.GuildID{123},
			UserIDs:  []discord.UserID{456},
			Query:    option.Some("abc"),
		}

		// Gateway should never be touched when Marshal fails, so we can just
		// create a zero-value.
		var gateway ws.Gateway

		err := gateway.Send(context.Background(), &cmd)
		if err == nil || !strings.Contains(err.Error(), "neither UserIDs nor Query can be filled") {
			t.Fatal("unexpected error:", err)
		}
	})
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
