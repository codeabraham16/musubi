package memory

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ledger.go lleva un LEDGER de tokens por sesión: cuántos tokens inyectó Musubi
// en el contexto, desglosado por SUPERFICIE, para medir y acotar el gasto real.
// La contabilidad es holística: cubre todas las superficies que inyectan contexto
// —arranque (priming, salud, cognitivo, generación de skills), por turno (fase,
// batch, recall, conflictos, captura), PreToolUse (código, telemetría) y las tools
// (hidratación, recall de código)— no solo el recall. Es model-free (estima el
// texto final con EstimateTokens) y se persiste como un único valor JSON en la
// tabla meta. Se reinicia al cambiar de sesión.

const metaTokenLedger = "token_ledger"

// TokenLedger es el acumulado de tokens inyectados en la sesión activa.
type TokenLedger struct {
	SessionID string         `json:"session_id"`
	Total     int            `json:"total"`
	Surfaces  map[string]int `json:"surfaces"`
}

// SurfaceStat es el gasto de una superficie del ledger y su porcentaje del total.
type SurfaceStat struct {
	Surface string `json:"surface"`
	Tokens  int    `json:"tokens"`
	Pct     int    `json:"pct"`
}

// BudgetStatus es el reporte del gobernador: el ledger contra el presupuesto BLANDO
// de sesión. Total y desglose por superficie (ordenado por gasto desc) más, si hay
// presupuesto, restante, % usado y estado. Es lo que devuelve musubi_tokens.
type BudgetStatus struct {
	SessionID string        `json:"session_id"`
	Total     int           `json:"total"`
	Budget    int           `json:"budget,omitempty"`
	Remaining int           `json:"remaining,omitempty"`
	PctUsed   int           `json:"pct_used,omitempty"`
	Status    string        `json:"status"` // unbudgeted | ok | watch | over
	Surfaces  []SurfaceStat `json:"surfaces"`
}

// Umbrales del gobernador (porcentaje del presupuesto de sesión).
const (
	budgetWatchPct = 75  // a partir de acá conviene mirar el gasto
	budgetOverPct  = 100 // presupuesto excedido
)

// Budget arma el reporte del ledger contra el presupuesto de sesión budget (0 = sin
// techo => estado "unbudgeted", solo desglose). Las superficies se ordenan por tokens
// desc (desempate por nombre, salida determinista) con su % del total. El estado es
// ok (<75%), watch (>=75%) u over (>=100%).
func (l TokenLedger) Budget(budget int) BudgetStatus {
	st := BudgetStatus{
		SessionID: l.SessionID,
		Total:     l.Total,
		Surfaces:  make([]SurfaceStat, 0, len(l.Surfaces)),
	}
	for surface, tokens := range l.Surfaces {
		pct := 0
		if l.Total > 0 {
			pct = int(float64(tokens)*100/float64(l.Total) + 0.5)
		}
		st.Surfaces = append(st.Surfaces, SurfaceStat{Surface: surface, Tokens: tokens, Pct: pct})
	}
	sort.Slice(st.Surfaces, func(i, j int) bool {
		if st.Surfaces[i].Tokens != st.Surfaces[j].Tokens {
			return st.Surfaces[i].Tokens > st.Surfaces[j].Tokens
		}
		return st.Surfaces[i].Surface < st.Surfaces[j].Surface
	})

	if budget <= 0 {
		st.Status = "unbudgeted"
		return st
	}
	st.Budget = budget
	st.Remaining = budget - l.Total
	st.PctUsed = int(float64(l.Total) * 100 / float64(budget))
	switch {
	case st.PctUsed >= budgetOverPct:
		st.Status = "over"
	case st.PctUsed >= budgetWatchPct:
		st.Status = "watch"
	default:
		st.Status = "ok"
	}
	return st
}

// LedgerStatus devuelve el ledger de la sesión activa (ceros si no hay).
func (e *DbEngine) LedgerStatus() (TokenLedger, error) {
	return e.loadLedger()
}

// LedgerReset borra el ledger.
func (e *DbEngine) LedgerReset() error {
	return e.SetMeta(metaTokenLedger, "")
}

// LedgerAdd suma tokens a una superficie de la sesión sessionID y devuelve el
// ledger actualizado. Si sessionID identifica una sesión distinta de la activa,
// reinicia el ledger antes de sumar. Si sessionID es vacío, acumula en la sesión
// activa sin reiniciar (caller sin id de hook).
func (e *DbEngine) LedgerAdd(sessionID, surface string, tokens int) (TokenLedger, error) {
	l, err := e.loadLedger()
	if err != nil {
		return TokenLedger{}, err
	}
	if sessionID != "" && sessionID != l.SessionID {
		l = TokenLedger{SessionID: sessionID, Surfaces: map[string]int{}}
	}
	if l.Surfaces == nil {
		l.Surfaces = map[string]int{}
	}
	if tokens > 0 {
		l.Total += tokens
		if surface != "" {
			l.Surfaces[surface] += tokens
		}
	}
	if err := e.saveLedger(l); err != nil {
		return TokenLedger{}, err
	}
	return l, nil
}

func (e *DbEngine) loadLedger() (TokenLedger, error) {
	v, ok, err := e.GetMeta(metaTokenLedger)
	if err != nil {
		return TokenLedger{}, err
	}
	l := TokenLedger{Surfaces: map[string]int{}}
	if !ok || v == "" {
		return l, nil
	}
	if err := json.Unmarshal([]byte(v), &l); err != nil {
		// Valor corrupto: arrancar de cero en vez de fallar.
		return TokenLedger{Surfaces: map[string]int{}}, nil
	}
	if l.Surfaces == nil {
		l.Surfaces = map[string]int{}
	}
	return l, nil
}

func (e *DbEngine) saveLedger(l TokenLedger) error {
	data, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("error al serializar ledger: %w", err)
	}
	return e.SetMeta(metaTokenLedger, string(data))
}
