package naming

import "testing"

func TestUseful(t *testing.T) {
	for name, want := range map[string]bool{
		"MacBook-de-Guilherme": true, "Samsung-Room-Airconditioner": true, "MiWiFi-R3600": true,
		"LG_Smart_Fridge2_open": true, "Sala-de-Estar": true,
		"50fdd5ed0e89": false, "c4:f7:c1:4b:72:5f": false, "bb38707b-6653-44c3-99e4-c165a1b2c3d4": false,
		"Android_I0FWBVKB": false, "ESP_1A2B3C": false, "espressif": false, "192.168.31.9": false, "": false,
		"deadbeefcafe": false,
	} {
		if got := Useful(name); got != want {
			t.Errorf("Useful(%q) = %v, esperado %v", name, got, want)
		}
	}
}

func TestCleanPTR(t *testing.T) {
	for in, want := range map[string]string{
		"macbook-de-guilherme.tail292b6d.ts.net.": "macbook-de-guilherme",
		"MiWiFi-R3600.lan":                        "MiWiFi-R3600",
		"Samsung-Room-Airconditioner":             "Samsung-Room-Airconditioner",
	} {
		if got := CleanPTR(in); got != want {
			t.Errorf("CleanPTR(%q) = %q, esperado %q", in, got, want)
		}
	}
}

func TestShortVendor(t *testing.T) {
	for in, want := range map[string]string{
		"Beijing Xiaomi Mobile Software Co., Ltd": "Xiaomi",
		"Sony Interactive Entertainment Inc.":     "PlayStation",
		"LG Innotek":                              "LG",
		"Nintendo Co.,Ltd":                        "Nintendo",
		"Apple, Inc.":                             "Apple",
		"SJI Industry Company":                    "SJI",
		"SAMJIN Co., Ltd.":                        "SAMJIN",
		"":                                        "",
	} {
		if got := ShortVendor(in); got != want {
			t.Errorf("ShortVendor(%q) = %q, esperado %q", in, got, want)
		}
	}
}

func TestLabel(t *testing.T) {
	cases := []struct {
		c    Clues
		want string
	}{
		{Clues{System: true, Hostname: "x"}, "Este servidor"},
		{Clues{Hostname: "Android_I0FWBVKB", PTR: "c60e05189664", Kind: "phone", OS: "Android"}, "Celular Android"},
		{Clues{PTR: "MiWiFi-R3600", Vendor: "Beijing Xiaomi Mobile Software Co., Ltd"}, "MiWiFi R3600"},
		{Clues{MDNS: "bb38707b-6653-44c3-99e4-c165a1b2c3d4", DNSLabel: "TV LG", Vendor: "LG Innotek"}, "TV LG"},
		{Clues{Vendor: "Espressif Inc.", DNSLabel: "Impressora Bambu Lab"}, "Impressora Bambu Lab"},
		{Clues{Vendor: "Nintendo Co.,Ltd", Kind: "console"}, "Nintendo"},
		{Clues{Vendor: "Beijing Xiaomi Mobile Software Co., Ltd"}, "Aparelho Xiaomi"},
		{Clues{Vendor: "LG Innotek", Kind: "tv"}, "TV LG"},
		{Clues{Remote: true}, "Acesso remoto (Tailscale)"},
		{Clues{Remote: true, PTR: "macbook-de-guilherme"}, "macbook de guilherme"},
		{Clues{}, ""},
	}
	for _, c := range cases {
		if got := Label(c.c); got != c.want {
			t.Errorf("Label(%+v) = %q, esperado %q", c.c, got, c.want)
		}
	}
}
