package discord

import (
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/utils/json"
)

func TestUserShownTag(t *testing.T) {
	// evidence: witnesses (shapes seen in the gateway's member lists)
	for _, test := range []struct {
		name, body, want string
	}{
		{"no field", `{"id":"1"}`, ""},
		{"null", `{"id":"1","primary_guild":null}`, ""},
		{"nulls inside", `{"id":"1","primary_guild":{"identity_guild_id":null,"identity_enabled":false,"tag":null,"badge":null}}`, ""},
		{"hidden", `{"id":"1","primary_guild":{"identity_guild_id":"2","identity_enabled":false,"tag":"ABC","badge":"ff"}}`, ""},
		{"shown", `{"id":"1","primary_guild":{"identity_guild_id":"2","identity_enabled":true,"tag":"ABC","badge":"ff"}}`, "ABC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var user User
			if err := json.Unmarshal([]byte(test.body), &user); err != nil {
				t.Fatal(err)
			}
			if got := user.ShownTag(); got != test.want {
				t.Fatalf("ShownTag = %q, want %q", got, test.want)
			}
		})
	}
}
