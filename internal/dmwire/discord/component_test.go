package discord

import (
	"fmt"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/utils/json"
)

func TestButtonStyleUnmarshal(t *testing.T) {
	// evidence: witnesses (both edges of the range, the zero value and the link style)
	for style := range basicButtonStyleCount + 1 {
		var b ButtonComponent
		err := json.Unmarshal(fmt.Appendf(nil, `{"style":%d,"url":"https://x"}`, style), &b)
		if valid := style >= primaryButtonStyle && style < basicButtonStyleCount; (err == nil) != valid {
			t.Errorf("style %d: err = %v, want valid = %v", style, err, valid)
		}
	}
}
