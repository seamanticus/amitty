package ui

import (
	"github.com/seamanticus/dmwire/discord"
	"github.com/seamanticus/dmwire/ningen"
)

// IsMe reports whether id is the logged in user.
func IsMe(state *ningen.State, id discord.UserID) bool {
	me, _ := state.Cabinet.Me()
	return me != nil && id == me.ID
}
