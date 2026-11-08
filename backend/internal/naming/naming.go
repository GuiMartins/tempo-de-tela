// Package naming escolhe o nome automático de um aparelho a partir das pistas da rede,
// descartando nomes que não dizem nada (MAC, UUID, IDs aleatórios).
package naming

import (
	"net/netip"
	"regexp"
	"strings"
	"unicode"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/inventory"
)

var (
	macLike    = regexp.MustCompile(`^([0-9a-f]{2}[-:_]?){5}[0-9a-f]{2}$`)
	uuidLike   = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}`)
	longHex    = regexp.MustCompile(`^[0-9a-f-]{16,}$`)
	randomID   = regexp.MustCompile(`^(android|esp|esp32|esp8266|espressif|wlan|wifi|host|device|client|dhcp)[-_]?[0-9a-z]{4,}$`)
	plainWords = map[string]bool{"localhost": true, "unknown": true, "espressif": true, "android": true, "none": true, "*": true}
)

// Useful diz se o nome identifica o aparelho (não é MAC, UUID, IP nem ID aleatório).
func Useful(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || plainWords[n] {
		return false
	}
	if _, err := netip.ParseAddr(n); err == nil {
		return false
	}
	if macLike.MatchString(n) || uuidLike.MatchString(n) || longHex.MatchString(n) || randomID.MatchString(n) {
		return false
	}
	hasLetter := false
	for _, r := range n {
		if unicode.IsLetter(r) && !strings.ContainsRune("abcdef", r) {
			hasLetter = true
			break
		}
	}
	return hasLetter
}

// CleanPTR tira o domínio de um nome reverso: "macbook.tail292b6d.ts.net." → "macbook".
func CleanPTR(name string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	lower := strings.ToLower(name)
	for _, suffix := range []string{".ts.net", ".lan", ".local", ".home", ".localdomain", ".router"} {
		if i := strings.Index(lower, suffix); i > 0 && strings.HasSuffix(lower, suffix) {
			name = name[:i]
			lower = lower[:i]
		}
	}
	// Tailscale: "macbook-de-guilherme.tail292b6d" → "macbook-de-guilherme".
	if i := strings.Index(lower, ".tail"); i > 0 {
		name = name[:i]
	}
	return name
}

// Pretty troca separadores por espaço: "MacBook-de-Guilherme" → "MacBook de Guilherme".
func Pretty(name string) string {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, " ") {
		return name
	}
	r := strings.NewReplacer("-", " ", "_", " ")
	return strings.Join(strings.Fields(r.Replace(name)), " ")
}

var knownVendors = []struct{ match, short string }{
	{"sony interactive", "PlayStation"},
	{"xiaomi", "Xiaomi"}, {"apple", "Apple"}, {"samsung", "Samsung"}, {"lg innotek", "LG"},
	{"lg electronics", "LG"}, {"nintendo", "Nintendo"}, {"espressif", "Espressif"}, {"amazon", "Amazon"},
	{"google", "Google"}, {"tp-link", "TP-Link"}, {"intel", "Intel"}, {"realtek", "Realtek"},
	{"huawei", "Huawei"}, {"motorola", "Motorola"}, {"microsoft", "Microsoft"}, {"sony", "Sony"},
	{"lenovo", "Lenovo"}, {"dell", "Dell"}, {"hewlett", "HP"}, {"asustek", "ASUS"}, {"oppo", "OPPO"},
	{"vivo mobile", "vivo"}, {"oneplus", "OnePlus"}, {"roku", "Roku"}, {"tuya", "Tuya"},
	{"raspberry", "Raspberry Pi"}, {"philips", "Philips"}, {"tcl", "TCL"}, {"hisense", "Hisense"},
	{"murata", "Murata"}, {"azurewave", "AzureWave"}, {"liteon", "Lite-On"}, {"positivo", "Positivo"},
}

var vendorNoise = []string{
	",", ".", " co ", " ltd", " inc", " corporation", " corp", " limited", " technologies", " technology",
	" electronics", " mobile software", " communications", " communication", " industry", " industrial",
	" international", " group", " gmbh", " s.a", " llc", " company",
}

// ShortVendor encurta o fabricante da base IEEE: "Beijing Xiaomi Mobile Software Co., Ltd" → "Xiaomi".
func ShortVendor(v string) string {
	lower := strings.ToLower(v)
	for _, k := range knownVendors {
		if strings.Contains(lower, k.match) {
			return k.short
		}
	}
	s := " " + lower + " "
	for _, noise := range vendorNoise {
		s = strings.ReplaceAll(s, noise, " ")
	}
	words := strings.Fields(s)
	for _, city := range []string{"beijing", "shenzhen", "guangdong", "hangzhou", "shanghai", "zhejiang", "dongguan", "qingdao"} {
		if len(words) > 1 && words[0] == city {
			words = words[1:]
		}
	}
	if len(words) == 0 {
		return ""
	}
	if len(words) > 2 {
		words = words[:2]
	}
	// Mantém a grafia original das palavras que sobraram.
	orig := strings.Fields(strings.NewReplacer(",", " ", ".", " ").Replace(v))
	out := make([]string, 0, len(words))
	for _, w := range words {
		for _, o := range orig {
			if strings.EqualFold(o, w) {
				w = o
				break
			}
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

var kindLabel = map[string]string{
	inventory.KindPhone: "Celular", inventory.KindTablet: "Tablet", inventory.KindComputer: "Computador",
	inventory.KindTV: "TV", inventory.KindSpeaker: "Caixa de som", inventory.KindIoT: "Aparelho inteligente",
	inventory.KindRouter: "Roteador", inventory.KindServer: "Servidor", inventory.KindPrinter: "Impressora",
}

// Clues reúne o que se sabe do aparelho para escolher o nome automático.
type Clues struct {
	Hostname, PTR, MDNS string // nomes que a rede informou
	DNSLabel            string // assinatura pelos domínios consultados ("Impressora Bambu Lab")
	Vendor              string // fabricante (base IEEE)
	Kind, OS            string
	System, Remote      bool // este servidor; cliente da Tailscale
}

// Label é o nome automático: hostname útil, nome reverso, mDNS, assinatura DNS, tipo + marca.
func Label(c Clues) string {
	if c.System {
		return "Este servidor"
	}
	for _, name := range []string{c.Hostname, c.PTR, c.MDNS} {
		if Useful(name) {
			return Pretty(name)
		}
	}
	if c.DNSLabel != "" {
		return c.DNSLabel
	}
	if c.Remote {
		return "Acesso remoto (Tailscale)"
	}
	brand := ShortVendor(c.Vendor)
	kind := kindLabel[c.Kind]
	switch {
	case c.Kind == inventory.KindConsole && brand != "":
		return brand
	case c.Kind == inventory.KindPhone && brand == "" && c.OS != "":
		return "Celular " + c.OS
	case kind != "" && brand != "":
		return kind + " " + brand
	case brand != "":
		return "Aparelho " + brand
	case kind != "":
		return kind
	}
	return ""
}
