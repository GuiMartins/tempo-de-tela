package catalog

import "testing"

func TestMatchLongestSuffix(t *testing.T) {
	c, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for domain, want := range map[string]string{
		"rr3---sn-abc.googlevideo.com":                 "youtube",
		"youtubei.googleapis.com":                      "youtube",
		"music.youtube.com":                            "youtube-music",
		"www.googleapis.com":                           "system-google",
		"nrdp25.logs.netflix.com":                      "netflix",
		"account-public-service-prod.ol.epicgames.com": "fortnite",
		"fortnite-vod.akamaized.net":                   "fortnite",
		"e6858.dsce9.akamaiedge.net":                   "system-cdn",
		"graph.facebook.com":                           "system-meta-sdk",
		"www.facebook.com":                             "facebook",
		"mmx-ds.cdn.whatsapp.net":                      "whatsapp",
		"chatgpt.com":                                  "chatgpt",
		"gemini.google.com":                            "gemini",
		"0.cn.pool.ntp.org":                            "system-network",
		"exemplo-desconhecido.com.br":                  "",
	} {
		got := ""
		if a := c.Match(domain); a != nil {
			got = a.ID
		}
		if got != want {
			t.Errorf("Match(%q) = %q, esperado %q", domain, got, want)
		}
	}
	if !c.Category(c.App("system-cdn").Category).Ignore || c.Category("video").GapMin != 10 {
		t.Error("categorias carregadas errado")
	}
}

func TestUserRulesAndApps(t *testing.T) {
	c, err := Build(
		[]App{{ID: "custom-escola", Name: "Portal da escola", Category: "study"}},
		[]Rule{{Domain: "escola.exemplo.com.br", AppID: "custom-escola"}, {Domain: "globo.com", AppID: "youtube"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if a := c.Match("aluno.escola.exemplo.com.br"); a == nil || a.ID != "custom-escola" || !a.Custom {
		t.Fatalf("regra do usuário: %+v", a)
	}
	if a := c.Match("horizon.globo.com"); a == nil || a.ID != "youtube" {
		t.Fatalf("regra do usuário deveria vencer o embutido: %+v", a)
	}
	if _, err := Build([]App{{ID: "x", Name: "X", Category: "inexistente"}}, nil); err == nil {
		t.Fatal("categoria desconhecida deveria dar erro")
	}
}
