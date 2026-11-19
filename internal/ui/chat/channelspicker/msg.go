package channelspicker

import "github.com/seamanticus/amitty/internal/dmwire/discord"

type SelectedMsg struct {
	ChannelID discord.ChannelID
}

type CancelMsg struct{}
