package inventory

import "strings"

// Tipos de aparelho (kind / kind_guess).
const (
	KindPhone    = "phone"
	KindTablet   = "tablet"
	KindComputer = "computer"
	KindTV       = "tv"
	KindConsole  = "console"
	KindSpeaker  = "speaker"
	KindIoT      = "iot"
	KindRouter   = "router"
	KindServer   = "server"
	KindOther    = "other"
)

type Clues struct {
	Hostname    string
	MDNSName    string
	Vendor      string
	DHCPVendor  string // opção 60
	Fingerprint string // opção 55, ex. "1,121,3,6,15,119,252"
	RandomMAC   bool
}

type rule struct {
	sub  string
	os   string
	kind string
}

// Pistas pelo nome (hostname/mDNS), em minúsculo.
var nameRules = []rule{
	{"iphone", "iOS", KindPhone},
	{"ipad", "iPadOS", KindTablet},
	{"macbook", "macOS", KindComputer},
	{"imac", "macOS", KindComputer},
	{"mac-mini", "macOS", KindComputer},
	{"apple-tv", "tvOS", KindTV},
	{"appletv", "tvOS", KindTV},
	{"watch", "watchOS", KindOther},
	{"galaxy-tab", "Android", KindTablet},
	{"galaxy", "Android", KindPhone},
	{"redmi", "Android", KindPhone},
	{"poco", "Android", KindPhone},
	{"moto", "Android", KindPhone},
	{"pixel", "Android", KindPhone},
	{"android", "Android", KindPhone},
	{"desktop-", "Windows", KindComputer},
	{"laptop-", "Windows", KindComputer},
	{"ps5", "PlayStation", KindConsole},
	{"ps4", "PlayStation", KindConsole},
	{"playstation", "PlayStation", KindConsole},
	{"xbox", "Xbox", KindConsole},
	{"nintendo", "Nintendo", KindConsole},
	{"switch", "Nintendo", KindConsole},
	{"chromecast", "Google TV", KindTV},
	{"googletv", "Google TV", KindTV},
	{"androidtv", "Android TV", KindTV},
	{"bravia", "Android TV", KindTV},
	{"lgwebos", "webOS", KindTV},
	{"webos", "webOS", KindTV},
	{"tizen", "Tizen", KindTV},
	{"[tv]", "", KindTV},
	{"roku", "Roku", KindTV},
	{"firetv", "Fire OS", KindTV},
	{"fire-tv", "Fire OS", KindTV},
	{"echo", "Alexa", KindSpeaker},
	{"alexa", "Alexa", KindSpeaker},
	{"nest", "", KindSpeaker},
	{"homepod", "", KindSpeaker},
	{"sonos", "", KindSpeaker},
	{"esp_", "", KindIoT},
	{"esp32", "", KindIoT},
	{"esp-", "", KindIoT},
	{"tuya", "", KindIoT},
	{"shelly", "", KindIoT},
	{"tasmota", "", KindIoT},
	{"camera", "", KindIoT},
	{"printer", "", KindIoT},
}

// Pistas pelo fabricante do MAC, em minúsculo.
var vendorRules = []rule{
	{"sony interactive", "PlayStation", KindConsole},
	{"nintendo", "Nintendo", KindConsole},
	{"espressif", "", KindIoT},
	{"tuya", "", KindIoT},
	{"amazon", "", KindSpeaker},
	{"sonos", "", KindSpeaker},
	{"roku", "Roku", KindTV},
	{"lg electronics", "", KindTV},
	{"tcl", "", KindTV},
	{"hisense", "", KindTV},
	{"vizio", "", KindTV},
	{"intel", "", KindComputer},
	{"liteon", "", KindComputer},
	{"azurewave", "", KindComputer},
	{"hon hai", "", KindComputer},
	{"raspberry", "Linux", KindComputer},
	{"xiaomi", "", ""},
	{"apple", "", ""},
	{"samsung", "", ""},
	{"google", "", ""},
}

// Guess estima sistema e tipo do aparelho a partir das pistas disponíveis.
func Guess(c Clues) (os, kind string) {
	names := strings.ToLower(c.MDNSName + " " + c.Hostname)
	for _, r := range nameRules {
		if strings.Contains(names, r.sub) {
			os, kind = r.os, r.kind
			break
		}
	}
	dv := strings.ToLower(c.DHCPVendor)
	if os == "" {
		switch {
		case strings.HasPrefix(dv, "android-dhcp"):
			os = "Android"
		case strings.HasPrefix(dv, "msft"):
			os = "Windows"
		case strings.HasPrefix(dv, "dhcpcd"):
			os = "Linux/Android"
		case strings.HasPrefix(dv, "udhcp"):
			os = "Linux embarcado"
		case strings.HasPrefix(c.Fingerprint, "1,121,3,6,15,108,114,119,252"),
			strings.HasPrefix(c.Fingerprint, "1,121,3,6,15,119,252"):
			os = "iOS/macOS"
		}
	}
	vend := strings.ToLower(c.Vendor)
	for _, r := range vendorRules {
		if vend != "" && strings.Contains(vend, r.sub) {
			if os == "" {
				os = r.os
			}
			if kind == "" {
				kind = r.kind
			}
			break
		}
	}
	if kind == "" {
		switch {
		case os == "Android" || os == "iOS" || (os == "iOS/macOS" && c.RandomMAC):
			// MAC aleatório é típico de celular/tablet.
			kind = KindPhone
		case os == "Windows":
			kind = KindComputer
		case os == "Linux embarcado":
			kind = KindIoT
		}
	}
	return os, kind
}
