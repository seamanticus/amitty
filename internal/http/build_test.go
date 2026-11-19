package http

import (
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

func TestBuildNumber(t *testing.T) {
	if got := (Build{}).Number(); got != clientBuildNumber {
		t.Fatalf("zero Build = %d, want the generated %d", got, clientBuildNumber)
	}
	if got := (Build{number: 700000}).Number(); got != 700000 {
		t.Fatalf("fetched Build = %d", got)
	}
}

// evidence: witnesses
func TestReadBuild(t *testing.T) {
	for _, tt := range []struct {
		name, page string
		want       int
	}{
		{"as the login page has it", `<script>window.GLOBAL_ENV = {"BUILD_NUMBER":"712345","VERSION_HASH":"x"}</script>`, 712345},
		{"with spaces", `"BUILD_NUMBER": "1"`, 1},
		{"first of two", `"BUILD_NUMBER":"5" "BUILD_NUMBER":"6"`, 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			build, err := readBuild(strings.NewReader(tt.page))
			if err != nil || build.Number() != tt.want {
				t.Fatalf("got %d, %v; want %d", build.Number(), err, tt.want)
			}
		})
	}

	for _, tt := range []struct{ name, page string }{
		{"empty page", ""},
		{"no build number", `<html>login</html>`},
		{"number not quoted", `"BUILD_NUMBER":712345`},
		{"zero", `"BUILD_NUMBER":"0"`},
		{"too large for int", `"BUILD_NUMBER":"99999999999999999999"`},
		{"beyond the read limit", strings.Repeat(" ", maxLoginPage) + `"BUILD_NUMBER":"1"`},
	} {
		t.Run(tt.name+" fails", func(t *testing.T) {
			if build, err := readBuild(strings.NewReader(tt.page)); err == nil {
				t.Fatalf("got %d, want an error", build.Number())
			}
		})
	}

	t.Run("a failing read fails", func(t *testing.T) {
		if _, err := readBuild(iotest.ErrReader(errors.New("reset"))); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestBuildIsSent(t *testing.T) {
	build := Build{number: 712345}

	t.Run("gateway identify", func(t *testing.T) {
		if got := IdentifyProperties(build)["client_build_number"]; got != 712345 {
			t.Fatalf("client_build_number = %v", got)
		}
	})

	t.Run("X-Super-Properties", func(t *testing.T) {
		raw, err := base64.StdEncoding.DecodeString(Headers(build).Get("X-Super-Properties"))
		if err != nil {
			t.Fatal(err)
		}
		var props struct {
			ClientBuildNumber int `json:"client_build_number"`
		}
		if err := json.Unmarshal(raw, &props); err != nil {
			t.Fatal(err)
		}
		if props.ClientBuildNumber != 712345 {
			t.Fatalf("client_build_number = %d", props.ClientBuildNumber)
		}
	})
}
