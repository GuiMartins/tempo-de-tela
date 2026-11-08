package alerts

import (
	"context"
	"fmt"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/catalog"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/usage"
)

const (
	// Sessões separadas por até isso contam como o mesmo uso (trocou de app, pausa curta).
	usageJoinGap = 5 * 60
	// Uso que terminou há mais que isso não avisa mais (servidor reiniciou, aviso atrasado).
	usageStale = 15 * 60
)

// EvaluateUsage gera os avisos de uso cadastrados: um alerta cada vez que começa um bloco de uso
// do app (ou categoria, ou qualquer coisa) no aparelho ou pela pessoa.
func EvaluateUsage(ctx context.Context, st *store.Store, catalogs *usage.Catalogs, now time.Time) error {
	list, err := st.UsageAlerts(ctx)
	if err != nil || len(list) == 0 {
		return err
	}
	persons, err := st.Persons(ctx)
	if err != nil {
		return err
	}
	devices, err := st.DeviceCoverage(ctx, store.AllDevices)
	if err != nil {
		return err
	}
	personByID := map[int64]store.Person{}
	for _, p := range persons {
		personByID[p.ID] = p
	}
	deviceByID := map[int64]store.DeviceCoverage{}
	for _, d := range devices {
		deviceByID[d.ID] = d
	}
	cat := catalogs.Get()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, a := range list {
		var hits []store.Hit
		var person *store.Person
		switch {
		case a.DeviceID != nil:
			d, ok := deviceByID[*a.DeviceID]
			if !ok {
				continue
			}
			if d.PersonID != nil {
				if p, ok := personByID[*d.PersonID]; ok {
					person = &p
				}
			}
			if person != nil && !person.Monitoring {
				continue
			}
			hits, err = st.DeviceHits(ctx, d.ID, day, day.AddDate(0, 0, 1))
		case a.PersonID != nil:
			p, ok := personByID[*a.PersonID]
			if !ok || !p.Monitoring {
				continue
			}
			person = &p
			hits, err = st.PersonHits(ctx, p.ID, day, day.AddDate(0, 0, 1))
		default:
			continue
		}
		if err != nil {
			return err
		}
		b, ok := lastBlock(usage.Sessionize(hits, cat), a, cat)
		if !ok || b.end < a.CreatedAt || now.Unix()-b.end > usageStale {
			continue
		}
		prefix := fmt.Sprintf("usage:%d:", a.ID)
		last, err := st.LastEventTS(ctx, prefix)
		if err != nil {
			return err
		}
		if last >= b.start {
			continue // já avisou deste bloco
		}
		appName := b.first.AppID
		if app := cat.App(b.first.AppID); app != nil {
			appName = app.Name
		}
		deviceName := ""
		if d, ok := deviceByID[b.first.DeviceID]; ok {
			deviceName = firstNonEmpty(d.Name, d.Label, d.IPv4, d.MAC)
		}
		e := store.Event{TS: now.Unix(), Type: "usage", Key: fmt.Sprintf("%s%d", prefix, b.start)}
		since := "Desde " + clock(b.start, now.Location())
		if a.DeviceID != nil {
			e.Title = deviceName + ": " + appName + " em uso"
			e.Message = since + "."
		} else {
			e.Title = person.Name + " está usando " + appName
			e.Message = since + " · " + deviceName + "."
		}
		deviceID := b.first.DeviceID
		e.DeviceID = &deviceID
		if person != nil {
			pid := person.ID
			e.PersonID = &pid
		}
		if _, err := st.AddEvent(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

type usageBlock struct {
	start, end int64
	first      usage.Session // a sessão que abriu o bloco
}

// lastBlock junta as sessões que batem com o aviso em blocos de uso e devolve o mais recente.
func lastBlock(sessions []usage.Session, a store.UsageAlert, cat *catalog.Catalog) (usageBlock, bool) {
	var b usageBlock
	found := false
	for _, s := range sessions { // em ordem de início
		if !matchesAlert(s, a, cat) {
			continue
		}
		if found && s.Start <= b.end+usageJoinGap {
			b.end = max(b.end, s.End)
			continue
		}
		b, found = usageBlock{s.Start, s.End, s}, true
	}
	return b, found
}

func matchesAlert(s usage.Session, a store.UsageAlert, cat *catalog.Catalog) bool {
	switch {
	case a.AppID != "":
		return s.AppID == a.AppID
	case a.Category != "":
		app := cat.App(s.AppID)
		return app != nil && app.Category == a.Category
	}
	return true
}
