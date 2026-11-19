package gateway

import (
	"github.com/seamanticus/discordo/internal/http"
	"github.com/seamanticus/dmwire/api"
	discordgateway "github.com/seamanticus/dmwire/gateway"
	"github.com/seamanticus/dmwire/utils/ws"
)

const gatewayURL = "wss://gateway.discord.gg"

func New(id discordgateway.Identifier) *discordgateway.Gateway {
	codec := ws.NewCodec(discordgateway.NewOpUnmarshalers(id.Capabilities))
	codec.Headers.Set("Origin", api.BaseEndpoint)
	codec.Headers.Set("User-Agent", http.BrowserUserAgent())

	conn := ws.NewConnWithDialer(codec, NewDialer())
	socket := ws.NewCustomWebsocket(conn, discordgateway.AddGatewayParams(gatewayURL))
	return discordgateway.FromWebsocketGateway(ws.NewGateway(socket, &discordgateway.DefaultGatewayOpts), discordgateway.State{Identifier: id})
}
