// Package images holds the messages that load images and pass them to the models that draw them.
package images

import (
	"image"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	tuiimage "github.com/seamanticus/amitty/internal/tui/image"
)

// RequestedMsg asks for images: Squares by rows tall, Previews by size in pixels.
type RequestedMsg struct {
	Squares  map[discord.URL]uint
	Previews map[discord.URL]image.Point
}

// ChangedMsg is every image by URL, blank while it loads, replacing the images before.
type ChangedMsg map[discord.URL]tuiimage.Widget

// LoadedMsg is an image that finished loading, or an empty one if it failed.
type LoadedMsg struct {
	URL   discord.URL
	Image tuiimage.Widget
	// Cell is the size of a cell in image pixels that the image was sized for.
	Cell image.Point
}
