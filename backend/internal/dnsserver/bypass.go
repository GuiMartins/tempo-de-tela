package dnsserver

import "strings"

// Domínios-sinal: responder "não existe" desliga o recurso no aparelho.
var bypassExact = map[string]bool{
	"use-application-dns.net": true, // Firefox desliga o DoH automático
	"mask.icloud.com":         true, // iCloud Private Relay
	"mask-h2.icloud.com":      true,
}

// Provedores de DNS-over-HTTPS/TLS conhecidos (inclui subdomínios). Sem resolver
// esses nomes, o Android ("DNS privado automático"), o Chrome e outros voltam pro DNS da rede.
var bypassSuffixes = []string{
	"dns.google",
	"dns.google.com",
	"cloudflare-dns.com",
	"one.one.one.one",
	"dns.quad9.net",
	"dns9.quad9.net",
	"dns10.quad9.net",
	"dns11.quad9.net",
	"doh.opendns.com",
	"doh.familyshield.opendns.com",
	"dns.nextdns.io",
	"doh.cleanbrowsing.org",
	"dns.adguard-dns.com",
	"dns.adguard.com",
	"doh.dns.sb",
	"doh.mullvad.net",
	"dns.mullvad.net",
	"dns.controld.com",
	"freedns.controld.com",
	"doh.libredns.gr",
	"doh.xfinity.com",
	"dns.alidns.com",
	"doh.pub",
	"dns.twnic.tw",
	"dns0.eu",
}

// IsBypassDomain diz se o nome (minúsculo, sem ponto final) serve pra contornar o DNS da rede.
func IsBypassDomain(name string) bool {
	if bypassExact[name] {
		return true
	}
	for _, s := range bypassSuffixes {
		if name == s || strings.HasSuffix(name, "."+s) {
			return true
		}
	}
	return false
}
