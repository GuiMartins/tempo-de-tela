// Package alerts guarda as regras de cada pessoa (limites, hora de dormir, aula) e gera os
// alertas que viram notificação no celular.
package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/GuiMartins/tempo-de-tela/backend/internal/catalog"
	"github.com/GuiMartins/tempo-de-tela/backend/internal/store"
)

// Window é um intervalo do dia, "HH:MM"–"HH:MM"; se Start > End, atravessa a meia-noite.
type Window struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// School é o horário de aula nos dias da semana (1 = segunda … 7 = domingo).
type School struct {
	Days  []int  `json:"days"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// Rules são as regras de uma pessoa. Limits: minutos por dia por categoria, ou "total".
type Rules struct {
	Limits  map[string]int `json:"limits"`
	Bedtime *Window        `json:"bedtime"`
	School  *School        `json:"school"`
}

const TotalLimit = "total"

func (r *Rules) Validate(cat *catalog.Catalog) error {
	if r.Limits == nil {
		r.Limits = map[string]int{}
	}
	for c, m := range r.Limits {
		if c != TotalLimit {
			category := cat.Category(c)
			if category == nil || category.Ignore {
				return fmt.Errorf("categoria inválida no limite: %q", c)
			}
		}
		if m <= 0 {
			delete(r.Limits, c)
		} else if m > 24*60 {
			return fmt.Errorf("limite de %s maior que 24 h", c)
		}
	}
	if r.Bedtime != nil {
		if _, err := clockMinutes(r.Bedtime.Start); err != nil {
			return err
		}
		if _, err := clockMinutes(r.Bedtime.End); err != nil {
			return err
		}
		if r.Bedtime.Start == r.Bedtime.End {
			return fmt.Errorf("hora de dormir com início e fim iguais")
		}
	}
	if r.School != nil {
		s, err1 := clockMinutes(r.School.Start)
		e, err2 := clockMinutes(r.School.End)
		if err1 != nil || err2 != nil || s >= e {
			return fmt.Errorf("horário de aula inválido (use HH:MM, início antes do fim)")
		}
		for _, d := range r.School.Days {
			if d < 1 || d > 7 {
				return fmt.Errorf("dia da semana inválido: %d", d)
			}
		}
		if len(r.School.Days) == 0 {
			r.School = nil
		}
	}
	return nil
}

func clockMinutes(hhmm string) (int, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, fmt.Errorf("horário inválido: %q (use HH:MM)", hhmm)
	}
	return t.Hour()*60 + t.Minute(), nil
}

// Load lê as regras da pessoa (vazias se nunca foram definidas).
func Load(ctx context.Context, st *store.Store, personID int64) (Rules, error) {
	r := Rules{Limits: map[string]int{}}
	raw, ok, err := st.PersonRules(ctx, personID)
	if err != nil || !ok {
		return r, err
	}
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return Rules{Limits: map[string]int{}}, nil
	}
	if r.Limits == nil {
		r.Limits = map[string]int{}
	}
	return r, nil
}

func Save(ctx context.Context, st *store.Store, personID int64, r Rules) error {
	b, _ := json.Marshal(r)
	return st.SetPersonRules(ctx, personID, string(b), time.Now())
}

// windows devolve os trechos de [dia 00:00, dia+1 00:00) cobertos pela janela, com um rótulo.
func (w Window) windows(day time.Time) []span {
	s, _ := clockMinutes(w.Start)
	e, _ := clockMinutes(w.End)
	at := func(m int) int64 { return day.Add(time.Duration(m) * time.Minute).Unix() }
	if s < e {
		return []span{{at(s), at(e), "dia"}}
	}
	return []span{{at(0), at(e), "madrugada"}, {at(s), at(24 * 60), "noite"}}
}

type span struct {
	start, end int64
	part       string
}
