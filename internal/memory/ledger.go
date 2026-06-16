package memory

import (
	"encoding/json"
	"fmt"
)

// ledger.go lleva un LEDGER de tokens por sesión: cuántos tokens inyectó Musubi
// en el contexto (priming de arranque + recall por turno + hidratación), para
// medir y acotar el gasto real. Es model-free y se persiste como un único valor
// JSON en la tabla meta. Se reinicia al cambiar de sesión.

const metaTokenLedger = "token_ledger"

// TokenLedger es el acumulado de tokens inyectados en la sesión activa.
type TokenLedger struct {
	SessionID string         `json:"session_id"`
	Total     int            `json:"total"`
	Surfaces  map[string]int `json:"surfaces"`
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
