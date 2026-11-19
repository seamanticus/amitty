package ui

import (
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen"
)

// IsMe reports whether id is the logged in user.
func IsMe(state *ningen.State, id discord.UserID) bool {
	me, _ := state.Cabinet.Me()
	return me != nil && id == me.ID
}
