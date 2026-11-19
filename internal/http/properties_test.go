package http

import (
	"strings"
	"testing"
	"uuid"

	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/tls"
)

func TestCommonProperties(t *testing.T) {
	properties := commonProperties(Build{})

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"properties browser", properties[gateway.IdentifyBrowser], tls.BrowserName()},
		{"properties version", properties["browser_version"], tls.BrowserVersion()},
		{"properties user agent", properties["browser_user_agent"], BrowserUserAgent()},
		{"properties os", properties[gateway.IdentifyOS], "Linux"},
		{"properties os version", properties["os_version"], ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got = %v, want = %v", test.got, test.want)
			}
		})
	}
}

func TestBrowserUserAgent(t *testing.T) {
	ua := BrowserUserAgent()
	if product := tls.BrowserName() + "/" + tls.BrowserVersion(); !strings.Contains(ua, product) {
		t.Fatalf("user agent does not contain %q", product)
	}
	if !strings.HasPrefix(ua, "Mozilla/5.0 (X11; Linux x86_64) ") {
		t.Fatalf("user agent %q is not Linux's", ua)
	}
}

func TestHeaders(t *testing.T) {
	headers := Headers(Build{})
	version := tls.BrowserMajorVersion()
	tests := []struct {
		name string
		want string
	}{
		{"Sec-Ch-Ua", brands("Chrome", version)},
		{"Sec-Ch-Ua-Mobile", "?0"},
		{"Sec-Ch-Ua-Platform", `"Linux"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := headers.Get(test.name); got != test.want {
				t.Fatalf("got = %q, want = %q", got, test.want)
			}
		})
	}
}

func TestXSuperProperties(t *testing.T) {
	properties := xSuperProperties(Build{})
	for _, key := range []gateway.IdentifyPropertyKey{"client_launch_id", "client_heartbeat_session_id", "launch_signature"} {
		t.Run(string(key), func(t *testing.T) {
			value, ok := properties[key].(string)
			if !ok {
				t.Fatalf("got %T, want string", properties[key])
			}
			id, err := uuid.Parse(value)
			if err != nil {
				t.Fatal(err)
			}
			if id[6]>>4 != 4 || id[8]>>6 != 2 {
				t.Errorf("got non-v4 UUID %q", id)
			}
		})
	}
}

func TestBrands(t *testing.T) {
	// As Chrome 150 and 156 send them.
	for version, want := range map[string]string{
		"150": `"Not;A=Brand";v="8", "Chromium";v="150", "Google Chrome";v="150"`,
		"156": `"Not:A-Brand";v="8", "Chromium";v="156", "Google Chrome";v="156"`,
	} {
		t.Run(version, func(t *testing.T) {
			if got := brands("Chrome", version); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
