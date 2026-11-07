// Package oui traduz o prefixo do MAC no fabricante (base MA-L da IEEE, embutida).
// Regenerar: scripts/update-oui.sh
package oui

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"net"
	"strings"
	"sync"
)

//go:embed oui.tsv.gz
var data []byte

var (
	once   sync.Once
	vendor map[string]string
)

func load() {
	vendor = make(map[string]string, 41000)
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return
	}
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "\t")
		if ok {
			vendor[k] = v
		}
	}
}

// Vendor devolve o fabricante, ou "" se desconhecido ou se o MAC é aleatório.
func Vendor(mac net.HardwareAddr) string {
	if len(mac) < 3 || IsRandomized(mac) {
		return ""
	}
	once.Do(load)
	k := strings.ToUpper(strings.ReplaceAll(mac[:3].String(), ":", ""))
	return vendor[k]
}

// IsRandomized: bit "localmente administrado" ligado = MAC privado/aleatório (Android, iOS, Windows).
func IsRandomized(mac net.HardwareAddr) bool {
	return len(mac) > 0 && mac[0]&0x02 != 0
}
