//go:build no_syntax_highlighting

package markdown

import "github.com/seamanticus/amitty/internal/tui"

func (r *Renderer) writeCodeBlock(builder *Builder, code string, language string, base tui.Style) {
	header := "code"
	if language != "" {
		header += ": " + language
	}
	builder.Write(codeBlockIndent+header, base.Dim(true))
	builder.NewLine()

	builder.Write(codeBlockIndent, base)
	writeCode(builder, code, base, base)
}
