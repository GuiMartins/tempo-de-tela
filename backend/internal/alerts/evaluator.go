package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

const (
	sinceKey      = "alerts_since"
	appsBootstrap = "alerts_apps_bootstrapped"
	// Um app só entra como "primeira vez" com pelo menos isso de uso no dia.
	newAppMinSec = 60
)

// Run avalia os alertas a cada 5 min até o contexto acabar.
func Run(ctx context.Context, st *store.Store, catalogs *usage.Catalogs) {
	tick := time.NewTimer(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := Evaluate(ctx, st, catalogs, time.Now()); err != nil && ctx.Err() == nil {
				slog.Warn("falha ao avaliar alertas", "erro", err)
			}
			tick.Reset(5 * time.Minute)
		}
	}
}

// Evaluate gera os alertas que valem para o momento now. Pode rodar quantas vezes quiser:
// cada alerta tem uma chave e só é gravado uma vez.
func Evaluate(ctx context.Context, st *store.Store, catalogs *usage.Catalogs, now time.Time) error {
	since, err := alertsSince(ctx, st, now)
	if err != nil {
		return err
	}
	if err := newDevices(ctx, st, since, now); err != nil {
		return err
	}
	persons, err := st.Persons(ctx)
	if err != nil {
		return err
	}
	if err := bootstrapApps(ctx, st, catalogs, persons, now); err != nil {
		return err
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	date := day.Format("2006-01-02")
	cat := catalogs.Get()
	for _, p := range persons {
		if !p.Monitoring {
			continue
		}
		hits, err := st.PersonHits(ctx, p.ID, day, day.AddDate(0, 0, 1))
		if err != nil {
			return err
		}
		sum := usage.Summarize(usage.Sessionize(hits, cat), cat, day, 1)
		rules, err := Load(ctx, st, p.ID)
		if err != nil {
			return err
		}
		pid := p.ID
		emit := func(kind, key, title, message string) error {
			_, err := st.AddEvent(ctx, store.Event{TS: now.Unix(), Type: kind, PersonID: &pid, Key: key, Title: title, Message: message})
			return err
		}

		for _, a := range sum.Apps {
			if a.Sec < newAppMinSec {
				continue
			}
			first, err := st.AddKnownApp(ctx, p.ID, a.ID, now)
			if err != nil {
				return err
			}
			if first {
				if err := emit("new_app", fmt.Sprintf("app:%d:%s", p.ID, a.ID),
					p.Name+" usou "+a.Name+" pela primeira vez",
					fmt.Sprintf("%s hoje, desde %s.", formatDuration(a.Sec), clock(firstSessionStart(sum, a.ID), now.Location()))); err != nil {
					return err
				}
			}
		}

		for category, minutes := range rules.Limits {
			used, label := sum.TotalSec, "tempo de tela"
			if category != TotalLimit {
				used, label = 0, category
				for _, c := range sum.Categories {
					if c.ID == category {
						used, label = c.Sec, strings.ToLower(c.Label)
					}
				}
				if c := cat.Category(category); c != nil {
					label = strings.ToLower(c.Label)
				}
			}
			if used >= int64(minutes)*60 {
				if err := emit("limit", fmt.Sprintf("limit:%d:%s:%s", p.ID, date, category),
					p.Name+" passou do limite de "+label,
					fmt.Sprintf("%s hoje, o limite é %s.", formatDuration(used), formatDuration(int64(minutes)*60))); err != nil {
					return err
				}
			}
		}

		if rules.Bedtime != nil {
			for _, w := range rules.Bedtime.windows(day) {
				if s, ok := firstOverlap(sum.Sessions, w, nil); ok {
					if err := emit("bedtime", fmt.Sprintf("bedtime:%d:%s:%s", p.ID, date, w.part),
						p.Name+" usou "+s.AppName+" na hora de dormir",
						fmt.Sprintf("Às %s (horário de dormir: %s às %s).", clock(max(s.Start, w.start), now.Location()), rules.Bedtime.Start, rules.Bedtime.End)); err != nil {
						return err
					}
				}
			}
		}

		if rules.School != nil && containsDay(rules.School.Days, isoWeekday(day)) {
			w := Window{Start: rules.School.Start, End: rules.School.End}.windows(day)[0]
			notStudy := func(s usage.SessionView) bool { return s.Category != "study" }
			if s, ok := firstOverlap(sum.Sessions, w, notStudy); ok {
				if err := emit("school", fmt.Sprintf("school:%d:%s", p.ID, date),
					p.Name+" usou "+s.AppName+" no horário de aula",
					fmt.Sprintf("Às %s (aula: %s às %s).", clock(max(s.Start, w.start), now.Location()), rules.School.Start, rules.School.End)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// alertsSince é quando os alertas começaram a valer: aparelhos de antes disso não são "novos".
func alertsSince(ctx context.Context, st *store.Store, now time.Time) (time.Time, error) {
	raw, ok, err := st.GetSetting(ctx, sinceKey)
	if err != nil {
		return now, err
	}
	if !ok {
		return now, st.SetSetting(ctx, sinceKey, fmt.Sprint(now.Unix()))
	}
	var ts int64
	fmt.Sscan(raw, &ts)
	return time.Unix(ts, 0), nil
}

// bootstrapApps marca como conhecidos, sem alertar, os apps dos últimos 30 dias de cada pessoa.
func bootstrapApps(ctx context.Context, st *store.Store, catalogs *usage.Catalogs, persons []store.Person, now time.Time) error {
	if _, done, err := st.GetSetting(ctx, appsBootstrap); err != nil || done {
		return err
	}
	cat := catalogs.Get()
	from := now.AddDate(0, 0, -30)
	for _, p := range persons {
		hits, err := st.PersonHits(ctx, p.ID, from, now)
		if err != nil {
			return err
		}
		for _, s := range usage.Sessionize(hits, cat) {
			if _, err := st.AddKnownApp(ctx, p.ID, s.AppID, time.Unix(s.Start, 0)); err != nil {
				return err
			}
		}
	}
	return st.SetSetting(ctx, appsBootstrap, fmt.Sprint(now.Unix()))
}

func newDevices(ctx context.Context, st *store.Store, since, now time.Time) error {
	devices, err := st.NewDevicesSince(ctx, since)
	if err != nil {
		return err
	}
	for _, d := range devices {
		if d.Hidden {
			continue
		}
		name := firstNonEmpty(d.Name, d.Label, d.IPv4, d.MAC)
		detail := strings.Join(nonEmpty(d.IPv4, d.Brand, map[bool]string{true: "MAC aleatório"}[d.RandomMAC]), " · ")
		id := d.ID
		if _, err := st.AddEvent(ctx, store.Event{
			TS: now.Unix(), Type: "new_device", DeviceID: &id, Key: fmt.Sprintf("device:%d", d.ID),
			Title: "Aparelho novo na rede: " + name, Message: detail,
		}); err != nil {
			return err
		}
	}
	return nil
}

func firstOverlap(sessions []usage.SessionView, w span, keep func(usage.SessionView) bool) (usage.SessionView, bool) {
	for _, s := range sessions {
		if s.End > w.start && s.Start < w.end && (keep == nil || keep(s)) {
			return s, true
		}
	}
	return usage.SessionView{}, false
}

func firstSessionStart(sum usage.Summary, appID string) int64 {
	for _, s := range sum.Sessions {
		if s.AppID == appID {
			return s.Start
		}
	}
	return sum.FirstActivity
}

func isoWeekday(t time.Time) int { return (int(t.Weekday())+6)%7 + 1 }

func containsDay(days []int, d int) bool {
	for _, v := range days {
		if v == d {
			return true
		}
	}
	return false
}

func clock(ts int64, loc *time.Location) string { return time.Unix(ts, 0).In(loc).Format("15:04") }

func formatDuration(sec int64) string {
	min := (sec + 30) / 60
	switch {
	case min < 60:
		return fmt.Sprintf("%d min", min)
	case min%60 == 0:
		return fmt.Sprintf("%d h", min/60)
	}
	return fmt.Sprintf("%d h %d min", min/60, min%60)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
