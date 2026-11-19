package gateway_test

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/seamanticus/amitty/internal/dmwire/gateway"
)

func Example() {
	token := os.Getenv("BOT_TOKEN")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	g, err := gateway.NewWithIntents(ctx, token, gateway.IntentGuilds)
	if err != nil {
		log.Panicln("failed to initialize gateway:", err)
	}

	for op := range g.Connect(ctx) {
		switch data := op.Data.(type) {
		case *gateway.ReadyEvent:
			log.Println("logged in as", data.User.Username)
		case *gateway.MessageCreateEvent:
			log.Println("got message", data.Content)
		}
	}
}
