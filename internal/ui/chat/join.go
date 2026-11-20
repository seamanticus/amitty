package chat

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/chat/guildstree"
	"github.com/seamanticus/amitty/internal/ui/chat/joinprompt"
)

// inviteCode names the join a captcha answers for.
type inviteCode string

type (
	// inviteFoundMsg is the invite input looked up to.
	inviteFoundMsg struct {
		invite discord.Invite
		input  string
	}
	// joinGuildMsg asks to join with invite, once confirmed.
	joinGuildMsg struct{ invite discord.Invite }
	// joinFailedMsg ends a lookup or join that failed for status.
	joinFailedMsg struct{ status string }
	// guildJoinedMsg reports the guild a join made you a member of.
	guildJoinedMsg struct{ guild discord.Guild }
)

// joining is the invite asked for in the join prompt and the join it leads to.
type joining struct {
	invite  discord.Invite
	captcha ui.Captcha[inviteCode]
}

// errUnknownInvite is Discord's code for an invite that does not exist or expired.
const errUnknownInvite httputil.ErrorCode = 10006

// parseInvite returns the code of an invite typed as a discord.gg or discord.com/invite link, or as the code alone.
func parseInvite(input string) (string, bool) {
	s := strings.TrimSpace(input)
	s, _ = strings.CutPrefix(s, "https://")
	s, _ = strings.CutPrefix(s, "http://")
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, "/")
	if host, path, ok := strings.Cut(s, "/"); ok {
		host = strings.ToLower(host)
		for _, sub := range []string{"www.", "ptb.", "canary."} {
			host = strings.TrimPrefix(host, sub)
		}
		switch host {
		case "discord.gg":
			s = path
		case "discord.com", "discordapp.com":
			if s, ok = strings.CutPrefix(path, "invite/"); !ok {
				return "", false
			}
		default:
			return "", false
		}
	}
	valid := func(r rune) bool {
		return r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
	}
	if s == "" || strings.IndexFunc(s, func(r rune) bool { return !valid(r) }) >= 0 {
		return "", false
	}
	return s, true
}

// lookUpInvite looks up the invite typed as input.
func lookUpInvite(state *ningen.State, input string) tui.Cmd {
	code, ok := parseInvite(input)
	if !ok {
		return cmd.Task(func() tui.Msg { return joinFailedMsg{"Not an invite link or code"} })
	}
	return cmd.Task(func() tui.Msg {
		invite, err := state.Client.InviteToJoin(code, strings.TrimSpace(input))
		if err != nil {
			return joinFailedMsg{joinError(err)}
		}
		return inviteFoundMsg{*invite, input}
	})
}

// confirmJoin opens the server invite leads to if you are in it, or asks whether to join it, defaulting to No.
func (m *Model) confirmJoin(invite discord.Invite) tui.Cmd {
	if invite.Guild == nil || !invite.Guild.ID.IsValid() {
		m.joinPrompt.SetStatus("This invite is not to a server")
		return nil
	}
	if _, err := m.state.Cabinet.Guild(invite.Guild.ID); err == nil {
		return m.showJoined(*invite.Guild)
	}
	question := "Join " + invite.Guild.Name
	if n := invite.ApproximateMembers; n > 0 {
		question += " (" + strconv.FormatUint(uint64(n), 10) + " members)"
	}
	join := cmd.Task(func() tui.Msg { return joinGuildMsg{invite} })
	return ui.ShowDialog(question+"?", ui.DialogButton{Label: "No"}, ui.DialogButton{Label: "Yes", Cmd: join})
}

// startJoin joins with invite unless the outreach limit is reached at now.
func (m *Model) startJoin(invite discord.Invite, now time.Time) tui.Cmd {
	if !m.outreach.allow(now) {
		return outreachRefused()
	}
	m.joinPrompt.SetStatus("Joining...")
	var closing tui.Cmd
	m.join.invite = invite
	m.join.captcha, closing = m.join.captcha.Start(inviteCode(invite.Code))
	return cmd.Batch{closing, joinGuild(m.state, invite, captcha.Solution{})}
}

// joinGuild sends the join for invite once, with solution if set, as the gateway session the client runs.
func joinGuild(state *ningen.State, invite discord.Invite, solution captcha.Solution) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		sessionID := state.Ready().SessionID
		if sessionID == "" {
			return joinFailedMsg{"Not connected to Discord yet; try again in a moment"}
		}
		joined, err := state.Client.JoinInvite(invite, sessionID, solution.Option())
		if challenge, ok := captcha.FromError(err); ok {
			return ui.Challenged(inviteCode(invite.Code), challenge)
		}
		if err != nil {
			return joinFailedMsg{joinError(err)}
		}
		guild := joined.Guild
		if !guild.ID.IsValid() {
			guild = *invite.Guild
		}
		return guildJoinedMsg{guild}
	})
}

// joinError says why a lookup or join failed.
func joinError(err error) string {
	if httpErr, ok := errors.AsType[*httputil.HTTPError](err); ok && (httpErr.Code == errUnknownInvite || httpErr.Status == http.StatusNotFound) {
		return "Invite not found; it may have expired"
	}
	return "Failed: " + err.Error()
}

// updateJoin steps the captcha of the join in flight.
func (m *Model) updateJoin(msg tui.Msg) tui.Cmd {
	var command tui.Cmd
	switch msg := msg.(type) {
	case ui.CaptchaMsg[inviteCode]:
		m.join.captcha, command = m.join.captcha.Update(msg)
		if prompt := m.join.captcha.Prompt(); prompt != "" {
			m.joinPrompt.SetStatus(prompt)
		}
	case ui.CaptchaDoneMsg[inviteCode]:
		if msg.Key != inviteCode(m.join.invite.Code) {
			return nil
		}
		if msg.Err != nil {
			m.joinPrompt.SetStatus(msg.Err.Error())
			return m.endJoin()
		}
		m.joinPrompt.SetStatus("Joining...")
		command = joinGuild(m.state, m.join.invite, m.join.captcha.Solution())
	}
	return command
}

// showJoined closes the join prompt and highlights guild in the guilds tree. A guild that asks new members for rules or onboarding first says to finish that elsewhere, as amitty shows neither.
func (m *Model) showJoined(guild discord.Guild) tui.Cmd {
	stopped := m.closeJoin()
	m.guildsTreeVisible = true
	m.setFocus(paneGuildsTree)
	var command tui.Cmd
	m.guildsTree, command = m.guildsTree.Update(guildstree.ShowGuildMsg{Guild: guild})
	commands := cmd.Batch{stopped, command}
	if slices.Contains(guild.Features, discord.MemberVerificationGateEnabled) || slices.Contains(guild.Features, discord.GuildOnboarding) {
		commands = append(commands, ui.ShowDialog(guild.Name+" asks new members to accept its rules or answer questions first. Finish that in the web client or the mobile app.", ui.DialogButton{Label: "OK"}))
	}
	return commands
}

// openJoin opens an empty join prompt.
func (m *Model) openJoin() {
	m.closePicker()
	m.joinPrompt = joinprompt.New(m.cfg)
	m.overlay = overlayJoinPrompt
}

// closeJoin closes the join prompt and ends the join.
func (m *Model) closeJoin() tui.Cmd {
	if m.overlay == overlayJoinPrompt {
		m.overlay = overlayNone
	}
	return m.endJoin()
}

// endJoin forgets the join in flight, closing its captcha page, so the prompt takes another invite.
func (m *Model) endJoin() tui.Cmd {
	var closing tui.Cmd
	m.join.captcha, closing = m.join.captcha.Stop()
	m.join.invite = discord.Invite{}
	return closing
}
