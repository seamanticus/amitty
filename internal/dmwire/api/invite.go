package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/dmwire/utils/json"
	"github.com/seamanticus/amitty/internal/dmwire/utils/json/option"
)

var EndpointInvites = Endpoint + "invites/"

// Invite returns an invite object for the given code.
//
// ApproxMembers will not get filled.
func (c *Client) Invite(code string) (*discord.Invite, error) {
	return c.RequestJSON[*discord.Invite](
		"GET",
		EndpointInvites+code,
	)
}

// InviteWithCounts returns an invite object for the given code and fills
// ApproxMembers.
func (c *Client) InviteWithCounts(code string) (*discord.Invite, error) {
	var params struct {
		WithCounts bool `schema:"with_counts,omitempty"`
	}

	params.WithCounts = true

	return c.RequestJSON[*discord.Invite](
		"GET",
		EndpointInvites+code,
		httputil.WithSchema(c, params),
	)
}

// ChannelInvites returns a list of invite objects (with invite metadata) for
// the channel. Only usable for guild channels.
//
// Requires the MANAGE_CHANNELS permission.
func (c *Client) ChannelInvites(channelID discord.ChannelID) ([]discord.Invite, error) {
	return c.RequestJSON[[]discord.Invite]("GET",
		EndpointChannels+channelID.String()+"/invites")
}

// GuildInvites returns a list of invite objects (with invite metadata) for the
// guild.
//
// Requires the MANAGE_GUILD permission.
func (c *Client) GuildInvites(guildID discord.GuildID) ([]discord.Invite, error) {
	return c.RequestJSON[[]discord.Invite]("GET",
		EndpointGuilds+guildID.String()+"/invites")
}

// https://discord.com/developers/docs/resources/channel#create-channel-invite-json-params
type CreateInviteData struct {
	// MaxAge is the duration of invite in seconds before expiry, or 0 for
	// never.
	//
	// Default:	86400 (24 hours)
	MaxAge option.Option[uint] `json:"max_age,omitempty"`
	// MaxUses is the max number of uses or 0 for unlimited.
	//
	// Default:	0
	MaxUses uint `json:"max_uses,omitempty"`
	// Temporary specifies whether this invite only grants temporary membership.
	//
	// Default:	false
	Temporary bool `json:"temporary,omitempty"`
	// Unique has the following behavior: if true, don't try to reuse a similar
	// invite (useful for creating many unique one time use invites).
	//
	// Default:	false
	Unique bool `json:"unique,omitempty"`

	AuditLogReason `json:"-"`
}

// CreateInvite creates a new invite object for the channel. Only usable for
// guild channels.
//
// Requires the CREATE_INSTANT_INVITE permission.
func (c *Client) CreateInvite(
	channelID discord.ChannelID, data CreateInviteData) (*discord.Invite, error) {
	return c.RequestJSON[*discord.Invite](
		"POST",
		EndpointChannels+channelID.String()+"/invites",
		httputil.WithJSONBody(data), httputil.WithHeaders(data.Header()),
	)
}

// JoinedInvite is the invite JoinInvite accepted.
type JoinedInvite struct {
	Code      string          `json:"code"`
	NewMember bool            `json:"new_member"`
	Guild     discord.Guild   `json:"guild"`
	Channel   discord.Channel `json:"channel"` // id, name, type only
}

// InviteToJoin looks up the invite code, typed as input, as the web client's Join a Server box does, with the member counts.
func (c *Client) InviteToJoin(code, input string) (*discord.Invite, error) {
	query := url.Values{
		"inputValue":       {input},
		"with_counts":      {"true"},
		"with_expiration":  {"true"},
		"with_permissions": {"true"},
	}
	return c.RequestJSON[*discord.Invite]("GET", EndpointInvites+url.PathEscape(code), httputil.WithSchema(c, query))
}

// JoinInvite accepts invite, looked up by InviteToJoin, for the gateway session sessionID, as the web client's Join a Server box does. A guild join fires Guild Create and Guild Member Add Gateway events.
func (c *Client) JoinInvite(invite discord.Invite, sessionID string, opts ...httputil.RequestOption) (*JoinedInvite, error) {
	var guildID discord.GuildID
	if invite.Guild != nil {
		guildID = invite.Guild.ID
	}
	context, err := json.Marshal(struct {
		Location    string              `json:"location"`
		GuildID     discord.GuildID     `json:"location_guild_id"`
		ChannelID   discord.ChannelID   `json:"location_channel_id"`
		ChannelType discord.ChannelType `json:"location_channel_type"`
	}{"Join Guild", guildID, invite.Channel.ID, invite.Channel.Type})
	if err != nil {
		return nil, fmt.Errorf("encode the join context: %w", err)
	}
	body := struct {
		SessionID string `json:"session_id"`
	}{sessionID}
	opts = append([]httputil.RequestOption{
		httputil.WithJSONBody(body),
		httputil.WithHeaders(http.Header{"X-Context-Properties": {base64.StdEncoding.EncodeToString(context)}}),
	}, opts...)
	return c.RequestJSON[*JoinedInvite]("POST", EndpointInvites+url.PathEscape(invite.Code), opts...)
}

// DeleteInvite deletes an invite.
//
// Requires the MANAGE_CHANNELS permission on the channel this invite belongs
// to, or MANAGE_GUILD to remove any invite across the guild.
//
// Fires an Invite Delete Gateway event.
func (c *Client) DeleteInvite(code string, reason AuditLogReason) (*discord.Invite, error) {
	return c.RequestJSON[*discord.Invite](
		"DELETE", EndpointInvites+code,
		httputil.WithHeaders(reason.Header()),
	)
}
