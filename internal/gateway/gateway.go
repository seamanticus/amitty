package gateway

import (
	"context"
	"log/slog"

	discordgateway "github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/logger"
)

const gatewayURL = "wss://gateway.discord.gg"

// New returns the gateway of id that connects through network.
func New(network http.Network, id discordgateway.Identifier) *discordgateway.Gateway {
	codec := ws.NewCodec(discordgateway.NewOpUnmarshalers())
	codec.Headers = HandshakeHeaders()
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		codec.Log = logFrame
	}

	conn := ws.NewConnWithDialer(codec, NewDialer(network))
	socket := ws.NewCustomWebsocket(conn, discordgateway.AddGatewayParams(gatewayURL))
	return discordgateway.FromWebsocketGateway(ws.NewGateway(socket, &discordgateway.DefaultGatewayOpts), discordgateway.State{Identifier: id})
}

// logFrame logs a gateway frame's op, type and sequence at debug level, and at LevelRaw its body too, with credentials blanked.
func logFrame(frame ws.Frame) {
	msg := "gateway recv"
	if frame.Sent {
		msg = "gateway send"
	}
	attrs := []slog.Attr{slog.Int("op", int(frame.Code))}
	if frame.Type != "" {
		attrs = append(attrs, slog.String("t", string(frame.Type)))
	}
	if frame.Sequence != 0 {
		attrs = append(attrs, slog.Int64("s", frame.Sequence))
	}
	ctx := context.Background()
	level := slog.LevelDebug
	if slog.Default().Enabled(ctx, logger.LevelRaw) {
		level = logger.LevelRaw
		attrs = append(attrs, slog.Any("body", logger.Body(frame.Data)))
	}
	slog.LogAttrs(ctx, level, msg, attrs...)
}
