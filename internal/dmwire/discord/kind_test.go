package discord

import "testing"

// proof: exhaustive (every uint16)
func TestChannelTypeKind(t *testing.T) {
	for x := range 1 << 16 {
		channelType := ChannelType(x)
		kind := channelType.Kind()
		if kind >= channelKindCount {
			t.Fatalf("ChannelType(%d).Kind() = %d, outside the kinds", x, kind)
		}
		if channelType.IsThread() != (kind == ThreadKind) {
			t.Fatalf("ChannelType(%d): IsThread disagrees with Kind", x)
		}
	}
}

// evidence: witnesses (one per kind, and a type Discord may add)
func TestChannelTypeKindWitnesses(t *testing.T) {
	for _, tt := range []struct {
		channelType ChannelType
		want        ChannelKind
	}{
		{DirectMessage, DirectMessageKind},
		{GroupDM, GroupDMKind},
		{GuildCategory, CategoryKind},
		{GuildForum, ForumKind},
		{GuildPublicThread, ThreadKind},
		{GuildText, GuildChannelKind},
		{GuildDirectory, GuildChannelKind},
		{ChannelType(16), UnknownChannelKind},
		{ChannelType(65535), UnknownChannelKind},
	} {
		if got := tt.channelType.Kind(); got != tt.want {
			t.Errorf("ChannelType(%d).Kind() = %d, want %d", tt.channelType, got, tt.want)
		}
	}
}

// proof: exhaustive (every uint8)
func TestMessageTypeKind(t *testing.T) {
	for x := range 1 << 8 {
		if kind := MessageType(x).Kind(); kind >= messageKindCount {
			t.Fatalf("MessageType(%d).Kind() = %d, outside the kinds", x, kind)
		}
	}
}

// evidence: witnesses (one per kind, a gap in Discord's numbering and a type Discord may add)
func TestMessageTypeKindWitnesses(t *testing.T) {
	for _, tt := range []struct {
		messageType MessageType
		want        MessageKind
	}{
		{DefaultMessage, PlainMessageKind},
		{InlinedReplyMessage, ReplyMessageKind},
		{GuildMemberJoinMessage, MemberJoinMessageKind},
		{ChannelPinnedMessage, PinMessageKind},
		{CallMessage, SystemMessageKind},
		{MessageType(13), UnknownMessageKind},
		{MessageType(255), UnknownMessageKind},
	} {
		if got := tt.messageType.Kind(); got != tt.want {
			t.Errorf("MessageType(%d).Kind() = %d, want %d", tt.messageType, got, tt.want)
		}
	}
}

// evidence: witnesses (every named status and one Discord may add; Visibility orders members, so the order is checked too)
func TestStatusVisibility(t *testing.T) {
	for _, tt := range []struct {
		status Status
		want   Visibility
	}{
		{OnlineStatus, ShownOnline},
		{IdleStatus, ShownIdle},
		{DoNotDisturbStatus, ShownDoNotDisturb},
		{OfflineStatus, ShownOffline},
		{InvisibleStatus, ShownOffline},
		{UnknownStatus, ShownOffline},
		{Status("streaming"), ShownOffline},
	} {
		if got := tt.status.Visibility(); got != tt.want {
			t.Errorf("%q.Visibility() = %d, want %d", tt.status, got, tt.want)
		}
	}
	if !(ShownOnline < ShownIdle && ShownIdle < ShownDoNotDisturb && ShownDoNotDisturb < ShownOffline && ShownOffline < visibilityCount) {
		t.Error("visibilities are not in Discord's member order")
	}
}
