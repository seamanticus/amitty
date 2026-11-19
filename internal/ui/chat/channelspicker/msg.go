package channelspicker

import "github.com/seamanticus/dmwire/discord"

type SelectedMsg struct {
	ChannelID discord.ChannelID
}

type CancelMsg struct{}
