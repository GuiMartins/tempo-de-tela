package signature

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		counts map[string]int
		want   Result
	}{
		{map[string]int{"api.bambulab.com": 19, "us.mqtt.bambulab.com": 19, "www.microsoft.com": 18}, Result{"Impressora Bambu Lab", "printer", ""}},
		{map[string]int{"api.miwifi.com": 7854, "0.cn.pool.ntp.org": 7256}, Result{"Roteador Xiaomi", "router", ""}},
		{map[string]int{"aic.tv.wiselg.com": 241, "nrdp25.logs.netflix.com": 425}, Result{"TV LG", "tv", "webOS"}},
		{map[string]int{"info.cspserver.net": 372, "ocfconnect-shard-na03-useast2.samsungiotcloud.com": 31, "log.samsunghrm.com": 8}, Result{"Aparelho Samsung SmartThings", "iot", ""}},
		{map[string]int{"connectivitycheck.gstatic.com": 22, "play.googleapis.com": 20}, Result{"", "", "Android"}},
		{map[string]int{"log.samsunghrm.com": 1}, Result{}},
		{map[string]int{"www.netflix.com": 500}, Result{}},
	}
	for _, c := range cases {
		if got := Classify(c.counts); got != c.want {
			t.Errorf("Classify(%v) = %+v, esperado %+v", c.counts, got, c.want)
		}
	}
}
