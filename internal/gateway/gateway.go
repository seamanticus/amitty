package gateway

import (
	"github.com/seamanticus/amitty/internal/dmwire/api"
	discordgateway "github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
	"github.com/seamanticus/amitty/internal/http"
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
