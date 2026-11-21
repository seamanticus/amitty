package login

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/spacer"
)

// logoMargin is the blank rows above the logo, and logoSpacing those between it and the boxes.
const logoMargin, logoSpacing = 1, 1

// page lays out the logo above the method and form boxes, leaving the logo out when the area cannot fit it and the form.
type page struct {
	logo          tui.Widget
	methods, form box.Widget
}

var _ tui.Widget = page{}

func (page) Size() (width, height layout.Length) { return layout.Fill, layout.Fill }

func (page) Layout(limits layout.Limits) layout.Size {
	return layout.Atomic(limits, layout.Fill, layout.Fill)
}

func (p page) Draw(screen tui.Screen, area tui.Rectangle) { p.column(area).Draw(screen, area) }

func (p page) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	return p.column(area).Handle(msg, area)
}

func (p page) column(area tui.Rectangle) tui.Widget {
	if !p.logoFits(area) {
		return column.New(p.methods, p.form)
	}
	logo := column.New(p.logo).Height(layout.Fixed(logoHeight + logoSpacing)).AlignX(tui.HorizontalAlignmentCenter)
	return column.New(spacer.New().Height(layout.Fixed(logoMargin)), logo, p.methods, p.form)
}

// logoFits reports whether area is wide enough for the logo and tall enough for it above both boxes at their natural height.
func (p page) logoFits(area tui.Rectangle) bool {
	if area.Width < logoWidth {
		return false
	}
	natural := layout.Limits{Max: layout.Size{Width: area.Width, Height: area.Height}, Compression: layout.Axes{Width: true, Height: true}}
	boxes := p.methods.Layout(natural).Height + p.form.Layout(natural).Height
	return logoMargin+logoHeight+logoSpacing+boxes <= area.Height
}
