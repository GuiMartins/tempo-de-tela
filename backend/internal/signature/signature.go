// Package signature reconhece o aparelho pelos domínios que ele consulta: a impressora conversa
// com bambulab.com, a geladeira com lgthinq.com, o videogame com playstation.net.
package signature

import (
	"strings"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/inventory"
)

type rule struct {
	suffix string
	label  string // nome do aparelho; vazio quando o domínio só indica o sistema
	kind   string
	os     string
}

// Domínios específicos de um tipo de aparelho. Serviços que qualquer aparelho usa (Netflix,
// Google, YouTube) não entram: não dizem o que o aparelho é.
var rules = []rule{
	{"miwifi.com", "Roteador Xiaomi", inventory.KindRouter, ""},
	{"io.mi.com", "Aparelho Xiaomi Home", inventory.KindIoT, ""},
	{"fds.api.xiaomi.com", "Aparelho Xiaomi Home", inventory.KindIoT, ""},
	{"bambulab.com", "Impressora Bambu Lab", inventory.KindPrinter, ""},
	{"lgthinq.com", "Eletrodoméstico LG ThinQ", inventory.KindIoT, ""},
	{"wiselg.com", "TV LG", inventory.KindTV, "webOS"},
	{"lgtvsdp.com", "TV LG", inventory.KindTV, "webOS"},
	{"lgappstv.com", "TV LG", inventory.KindTV, "webOS"},
	{"samsunghrm.com", "Eletrodoméstico Samsung", inventory.KindIoT, ""},
	{"samsungiotcloud.com", "Aparelho Samsung SmartThings", inventory.KindIoT, ""},
	{"samsungcloudsolution.com", "TV Samsung", inventory.KindTV, "Tizen"},
	{"samsungotn.net", "TV Samsung", inventory.KindTV, "Tizen"},
	{"playstation.net", "PlayStation", inventory.KindConsole, "PlayStation"},
	{"playstation.com", "PlayStation", inventory.KindConsole, "PlayStation"},
	{"nintendo.net", "Nintendo Switch", inventory.KindConsole, "Nintendo"},
	{"xboxlive.com", "Xbox", inventory.KindConsole, "Xbox"},
	{"roku.com", "Roku", inventory.KindTV, "Roku"},
	{"tuyaus.com", "Aparelho Tuya", inventory.KindIoT, ""},
	{"tuyaeu.com", "Aparelho Tuya", inventory.KindIoT, ""},
	{"tuyacn.com", "Aparelho Tuya", inventory.KindIoT, ""},
	{"tuya.com", "Aparelho Tuya", inventory.KindIoT, ""},
	{"coolkit.cc", "Aparelho Sonoff", inventory.KindIoT, ""},
	{"ewelink.cc", "Aparelho Sonoff", inventory.KindIoT, ""},
	{"meethue.com", "Philips Hue", inventory.KindIoT, ""},
	{"sonos.com", "Sonos", inventory.KindSpeaker, ""},
	{"ring.com", "Ring", inventory.KindIoT, ""},
	{"ezvizlife.com", "Câmera EZVIZ", inventory.KindIoT, ""},
	{"hik-connect.com", "Câmera Hikvision", inventory.KindIoT, ""},
	{"imoulife.com", "Câmera Imou", inventory.KindIoT, ""},
	{"wyzecam.com", "Câmera Wyze", inventory.KindIoT, ""},
	{"intelbras.com.br", "Aparelho Intelbras", inventory.KindIoT, ""},
	{"avs-alexa-na.amazon.com", "Echo (Alexa)", inventory.KindSpeaker, ""},
	{"device-metrics-us.amazon.com", "Aparelho Amazon", "", ""},
	{"chromecast.com", "Chromecast", inventory.KindTV, "Google TV"},
	// Só o sistema.
	{"connectivitycheck.gstatic.com", "", "", "Android"},
	{"android.clients.google.com", "", "", "Android"},
	{"captive.apple.com", "", "", "Apple"},
	{"msftconnecttest.com", "", "", "Windows"},
	{"msftncsi.com", "", "", "Windows"},
}

// Result é o que os domínios dizem do aparelho.
type Result struct {
	Label, Kind, OS string
}

// Classify recebe as consultas de um aparelho (domínio → quantidade) e devolve a assinatura
// com mais consultas. Pede pelo menos 2 consultas pra não confundir com um acesso eventual.
func Classify(counts map[string]int) Result {
	labelHits := map[rule]int{}
	osHits := map[string]int{}
	for domain, n := range counts {
		d := strings.ToLower(strings.TrimSuffix(domain, "."))
		for _, r := range rules {
			if d == r.suffix || strings.HasSuffix(d, "."+r.suffix) {
				if r.label != "" {
					labelHits[rule{label: r.label, kind: r.kind, os: r.os}] += n
				}
				if r.os != "" {
					osHits[r.os] += n
				}
				break
			}
		}
	}
	var res Result
	best := 1
	for r, n := range labelHits {
		if n > best || (n == best && res.Label != "" && r.label < res.Label) {
			best, res = n, Result{r.label, r.kind, r.os}
		}
	}
	if res.OS == "" {
		bestOS := 1
		for os, n := range osHits {
			if n > bestOS {
				bestOS, res.OS = n, os
			}
		}
	}
	return res
}
