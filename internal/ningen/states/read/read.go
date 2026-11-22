// Package read implements a read state with an event handler API.
package read

import (
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/state"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
)

type UpdateEvent struct {
	gateway.ReadState
	GuildID discord.GuildID
	Unread  bool
}

var _ gateway.Event = (*UpdateEvent)(nil)

func (ev UpdateEvent) Op() ws.OpCode           { return -1 }
func (ev UpdateEvent) EventType() ws.EventType { return "__read.UpdateEvent" }

type State struct {
	mutex  sync.Mutex
	state  *state.State
	states map[discord.ChannelID]*gateway.ReadState

	selfID discord.UserID
}

func NewState(state *state.State, r *handler.Handler) *State {
	readstate := &State{
		state:  state,
		states: make(map[discord.ChannelID]*gateway.ReadState),
	}

	r.Subscribe(func(ev ws.Event) {
		switch ev := ev.(type) {
		case *gateway.ReadyEvent:
			readstate.onReady(ev)
		case *gateway.MessageAckEvent:
			// do not send a duplicate ack
			readstate.markRead(ev.ChannelID, ev.MessageID, false)
		case *gateway.MessageCreateEvent:
			readstate.onMessageCreate(ev)
		case *gateway.ThreadCreateEvent:
			readstate.onThreadCreate(ev)
		// A channel or thread that is deleted leaves no read state behind.
		case *gateway.ChannelDeleteEvent:
			readstate.remove(ev.ID)
		case *gateway.ThreadDeleteEvent:
			readstate.remove(ev.ID)
		case *gateway.ChannelUnreadUpdateEvent:
			readstate.onUnreadUpdate(ev)
		}
	})

	return readstate
}

func (r *State) onReady(ready *gateway.ReadyEvent) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.selfID = ready.User.ID
	clear(r.states)

	for i, rs := range ready.ReadStates.Entries {
		r.states[rs.ChannelID] = &ready.ReadStates.Entries[i]
	}
}

func (r *State) onMessageCreate(c *gateway.MessageCreateEvent) {
	ch, _ := r.state.Cabinet.Channel(c.ChannelID)
	if ch != nil {
		cpy := *ch
		cpy.LastMessageID = c.ID
		r.state.ChannelSet(&cpy, true)
	}

	selfID := r.SelfID()
	if c.Author.ID == selfID {
		r.markRead(c.ChannelID, c.ID, false)
		return
	}

	var mentions int
	for _, u := range c.Mentions {
		if u.ID == selfID {
			mentions++
		}
	}
	// Discord counts each message of someone else in a DM or group DM as a mention, which is the badge the web client shows on a DM, though the message lists none.
	if ch != nil && (ch.Type == discord.DirectMessage || ch.Type == discord.GroupDM) {
		mentions = 1
	}

	r.MarkUnread(c.ChannelID, c.ID, mentions)
}

// onThreadCreate reads the forum we created a thread in, which Discord does without sending an ack for it.
func (r *State) onThreadCreate(c *gateway.ThreadCreateEvent) {
	if !c.NewlyCreated || c.OwnerID != r.SelfID() {
		return
	}

	if parent, _ := r.state.Cabinet.Channel(c.ParentID); parent != nil && parent.Type == discord.GuildForum {
		r.markRead(c.ParentID, discord.MessageID(c.ID), false)
	}
}

func (r *State) onUnreadUpdate(u *gateway.ChannelUnreadUpdateEvent) {
	for _, update := range u.ChannelUnreadUpdates {
		ch, _ := r.state.Cabinet.Channel(update.ID)
		if ch == nil {
			continue
		}

		cpy := *ch
		cpy.LastMessageID = update.LastMessageID
		r.state.ChannelSet(&cpy, true)
	}
}

func (r *State) remove(channelID discord.ChannelID) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.states, channelID)
}

func (r *State) SelfID() discord.UserID {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.selfID
}

// ReadState gets the read state for a channel: one read up to a message, or one with mentions, as a channel never read has once a message mentions the user.
func (r *State) ReadState(channelID discord.ChannelID) *gateway.ReadState {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if s, ok := r.states[channelID]; ok && (s.LastMessageID.IsValid() || s.MentionCount > 0) {
		return s
	}
	return nil
}

func (r *State) MarkUnread(chID discord.ChannelID, msgID discord.MessageID, mentions int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	rs, ok := r.states[chID]
	if !ok {
		rs = &gateway.ReadState{
			ChannelID: chID,
		}
		r.states[chID] = rs
	}

	rs.MentionCount += mentions

	ch, _ := r.state.Cabinet.Channel(chID)
	if ch == nil {
		return
	}

	if ch.LastMessageID < msgID {
		ch.LastMessageID = msgID
		r.state.ChannelSet(ch, false)
	}

	if msg, _ := r.state.Cabinet.Message(chID, msgID); msg != nil {
		if msg.Author.ID == r.selfID {
			// If the message is ours, we should marrk it as already read, since
			// it is registered like that on the Discord servers.
			rs.LastMessageID = msgID
			// Reset the mentions as well.
			rs.MentionCount = 0
		}
	}

	// The message is not read if the last read message's ID is less than the
	// latest message's ID. We don't check for inequality since the latest
	// message may have been deleted, leading to us trying to mark a deleted
	// message.
	unread := rs.LastMessageID < msgID
	rscp := *rs

	// Force callbacks to run in a goroutine. This is because MarkRead and
	// MarkUnread may be called by the user in their main thread, which means
	// these callbacks may occupy the main loop. It may also run in any other
	// goroutine, making it impossible to properly synchronize these callbacks.
	// Doing this helps making a consistent synchronizing behavior.
	go func() {
		// Announce that there is a change.
		r.state.Call(&UpdateEvent{
			ReadState: rscp,
			GuildID:   ch.GuildID,
			Unread:    unread,
		})
	}()
}

func (r *State) MarkRead(chID discord.ChannelID, msgID discord.MessageID) {
	// send ack
	r.markRead(chID, msgID, true)
}

// MarkReadBulk marks each channel read up to its message, in one bulk ack for those not read already, as the web client's Mark As Read does.
func (r *State) MarkReadBulk(reads map[discord.ChannelID]discord.MessageID) error {
	var acks []api.BulkAck
	for _, chID := range slices.Sorted(maps.Keys(reads)) {
		if msgID := reads[chID]; msgID.IsValid() && r.markRead(chID, msgID, false) {
			acks = append(acks, api.BulkAck{ChannelID: chID, MessageID: msgID})
		}
	}
	if len(acks) == 0 {
		return nil
	}
	return r.state.AckBulk(acks)
}

// markRead records chID as read up to msgID, sending its ack if sendack, and reports whether that changed its read state.
func (r *State) markRead(chID discord.ChannelID, msgID discord.MessageID, sendack bool) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	rs, ok := r.states[chID]
	if !ok {
		rs = &gateway.ReadState{
			ChannelID: chID,
		}
		r.states[chID] = rs
	}

	// If we've already marked the message as read.
	if rs.LastMessageID == msgID && rs.MentionCount == 0 {
		// log.Println("ningen: NOT acking", chID, "for message", msgID)
		return false
	}

	// Update.
	// prevMessageID := rs.LastMessageID
	rs.LastMessageID = msgID
	rs.MentionCount = 0

	// The channel cursor is a valid ack target even if the referenced message
	// is not in the message cache.
	if sendack && msgID.IsValid() {
		go r.ack(chID, msgID)
	}

	// copy
	rscp := *rs

	go func() {
		ch, _ := r.state.Cabinet.Channel(chID)
		if ch == nil {
			return
		}

		// Announce that there is a change.
		r.state.Call(&UpdateEvent{
			ReadState: rscp,
			GuildID:   ch.GuildID,
			Unread:    false,
		})
	}()
	return true
}

func (r *State) ack(chID discord.ChannelID, msgID discord.MessageID) {
	if err := r.state.Ack(chID, msgID, r.ackData(chID, time.Now())); err != nil {
		slog.Error(
			"cannot send message ack to Discord",
			"module", "ningen",
			"channel_id", chID,
			"message_id", msgID,
			"err", err)
	}
}

// TotalMentionCount returns the total mention count of all channels for the
// current user.
func (r *State) TotalMentionCount() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var total int
	for _, rs := range r.states {
		total += rs.MentionCount
	}
	return total
}

// ackData returns the ack the web client sends for chID viewed at now: no token, the channel's flags and the day.
func (r *State) ackData(chID discord.ChannelID, now time.Time) *api.Ack {
	var flags api.ReadStateFlags
	if ch, err := r.state.Cabinet.Channel(chID); err == nil {
		if ch.GuildID.IsValid() {
			flags |= api.ReadStateGuildChannel
		}
		if ch.Type.IsThread() {
			flags |= api.ReadStateThread
		}
	}
	return &api.Ack{Flags: &flags, LastViewed: new(api.AckDay(now))}
}
