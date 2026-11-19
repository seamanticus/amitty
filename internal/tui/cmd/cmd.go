package cmd

import (
	"image"

	"github.com/gdamore/tcell/v3"
)

type Msg any

// Cmd is work done outside Update, such as I/O, whose result comes back as a message.
type Cmd interface{ isCmd() }

// WriteRaw writes data, such as escape sequences, to the terminal as is.
type WriteRaw string

func (WriteRaw) isCmd() {}

// SetBase sets the style every frame starts from: a cell drawn without a foreground or background takes Style's. The zero Style leaves them to the terminal.
type SetBase struct{ Style tcell.Style }

func (SetBase) isCmd() {}

// SixelImage sets the image that the sixel placeholders of image ID show, already at the size it is drawn, with Cell pixels to a cell. A nil Image forgets it.
type SixelImage struct {
	ID    int
	Image *image.Paletted
	Cell  image.Point
}

func (SixelImage) isCmd() {}

type Quit struct{}

func (Quit) isCmd() {}

// Suspend runs a function while the terminal is suspended and sends the message it returns, if any.
type Suspend func() Msg

func (Suspend) isCmd() {}

// RequestTerminalInfo asks for the terminal's name and version, which arrive as a TerminalInfoMsg.
type RequestTerminalInfo struct{}

func (RequestTerminalInfo) isCmd() {}

type Notify struct{ Title, Body string }

func (Notify) isCmd() {}
