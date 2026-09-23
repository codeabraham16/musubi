package memory

import (
	"database/sql"
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
// tabla meta, con una entrada POR SESIÓN: abrir una sesión nueva ya no borra la
// cuenta de las demás (ver ledgerStore). Cada suma corre dentro de una transacción,
// porque el valor lleva las cuentas de todas y una escritura pisada costaría la
// sesión entera de otra terminal, no un incremento.
//
// ⚠️ EL CASO ALWAYS-ON, que hasta acá no estaba escrito en ningún lado. Quien marca
// el corte de sesión es el sessionID, y ese id lo aportan SÓLO los hooks de sesión
// (cmd/musubi/turn.go, precheck.go, detect.go). El camino MCP llama con sessionID
// vacío, así que en un proceso que no reinicia y donde no corre ningún hook —el
// cerebro central bajo `musubi serve`— el ledger NO ROTA NUNCA y el total pasa a ser
// un acumulado de por vida. Se vio en vivo: 2.153.453 tokens contra un techo de 8000,
// sin moverse desde el deploy.
//
// No se arregla forzando un corte artificial: en un servidor always-on y multi-principal
// "sesión" no está definido, y un corte inventado mentiría igual. Como el techo es BLANDO
// (no recorta nada; ver config.SessionTokenBudget) el gasto real nunca estuvo en riesgo:
// lo único que fallaba era lo que se decía. Por eso quien consume el ledger tiene que
// mirar SessionID: vacío ⇒ es un acumulado, no una sesión, y no se compara contra el techo.

const metaTokenLedger = "token_ledger"

// maxSesionesEnLedger acota cuántas sesiones se guardan. El ledger es telemetría, no un registro
// contable: pasado ese número se desaloja la MENOS recientemente escrita. Sin tope, una máquina que
// abre y cierra terminales todo el día haría crecer un único valor de `meta` sin freno.
const maxSesionesEnLedger = 16

// ledgerStore es lo que se persiste: un ledger POR SESIÓN, no uno solo.
//
// EL DEFECTO QUE ESTO ARREGLA (medido 2026-09-23). La casilla era única y guardaba UNA sesión: en
// LedgerAdd, `if sessionID != l.SessionID` la reiniciaba entera. Con varias terminales sobre el
// mismo cuaderno —10 procesos `musubi` en esta máquina, más los sub-agentes, que traen su propio
// id— cada sesión que escribía BORRABA la cuenta de todas las demás. El número que mostraba
// `musubi_tokens` no era «lo que gastó esta sesión» sino «lo que sobrevivió desde el último
// cambio de sesión»: medido en vivo, 262 tokens de una sola superficie para una sesión que llevaba
// el día entero inyectando contexto de arranque, de turno y de PreToolUse.
//
// `Orden` es un número de escritura, no un reloj: desalojar por secuencia es determinista y no
// depende de que dos máquinas tengan la hora igual.
type ledgerStore struct {
	Sesiones map[string]TokenLedger `json:"sesiones"`
	Orden    map[string]int         `json:"orden"`
	Seq      int                    `json:"seq"`
}

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

// LedgerStatus devuelve el ledger de la sesión escrita más recientemente (ceros si no hay). Con una
// sola terminal abierta es exactamente lo de antes; con varias, deja de ser «la que pisó último» y
// pasa a ser una de verdad, con su total entero. Para una sesión concreta, LedgerStatusDe.
func (e *DbEngine) LedgerStatus() (TokenLedger, error) {
	st, err := e.loadLedgerStore()
	if err != nil {
		return TokenLedger{Surfaces: map[string]int{}}, err
	}
	return st.ultima(), nil
}

// LedgerStatusDe devuelve el ledger de UNA sesión (ceros si no hay). Es lo que permite preguntar
// «cuánto gastó esta sesión» sin que la respuesta dependa de quién escribió último.
func (e *DbEngine) LedgerStatusDe(sessionID string) (TokenLedger, error) {
	st, err := e.loadLedgerStore()
	if err != nil {
		return TokenLedger{Surfaces: map[string]int{}}, err
	}
	return st.de(sessionID), nil
}

// LedgerReset borra el ledger de TODAS las sesiones.
func (e *DbEngine) LedgerReset() error {
	return e.SetMeta(metaTokenLedger, "")
}

// de devuelve el ledger de una sesión, siempre con Surfaces no nil.
func (s ledgerStore) de(sessionID string) TokenLedger {
	l, ok := s.Sesiones[sessionID]
	if !ok {
		return TokenLedger{SessionID: sessionID, Surfaces: map[string]int{}}
	}
	if l.Surfaces == nil {
		l.Surfaces = map[string]int{}
	}
	return l
}

// ultima devuelve la sesión con el número de escritura más alto. El desempate por SessionID es para
// que la salida sea determinista: sin él, dos sesiones con el mismo orden saldrían según el recorrido
// del map, que en Go es aleatorio a propósito, y el reporte cambiaría entre corridas.
func (s ledgerStore) ultima() TokenLedger {
	id, hay := s.ultimaID()
	if !hay {
		return TokenLedger{Surfaces: map[string]int{}}
	}
	return s.de(id)
}

// ultimaID es ultima() pero devolviendo la clave, y hay=false cuando no hay ninguna sesión.
func (s ledgerStore) ultimaID() (string, bool) {
	mejorID, mejorSeq, hay := "", -1, false
	for id := range s.Sesiones {
		seq := s.Orden[id]
		if seq > mejorSeq || (seq == mejorSeq && id > mejorID) {
			mejorID, mejorSeq, hay = id, seq, true
		}
	}
	return mejorID, hay
}

// podar desaloja las sesiones más viejas cuando se pasa del tope.
func (s *ledgerStore) podar() {
	for len(s.Sesiones) > maxSesionesEnLedger {
		peorID, peorSeq, hay := "", 0, false
		for id := range s.Sesiones {
			seq := s.Orden[id]
			if !hay || seq < peorSeq || (seq == peorSeq && id < peorID) {
				peorID, peorSeq, hay = id, seq, true
			}
		}
		if !hay {
			return
		}
		delete(s.Sesiones, peorID)
		delete(s.Orden, peorID)
	}
}

// LedgerAdd suma tokens a una superficie de la sesión sessionID y devuelve el
// ledger actualizado. Si sessionID identifica una sesión distinta de la activa,
// reinicia el ledger antes de sumar. Si sessionID es vacío, acumula en la sesión
// activa sin reiniciar (caller sin id de hook) y el SessionID queda vacío, que es
// la señal de "esto es un acumulado, no una sesión" — ver el encabezado del archivo.
func (e *DbEngine) LedgerAdd(sessionID, surface string, tokens int) (TokenLedger, error) {
	// En sólo lectura el ledger no se toca: es telemetría del ahorro, no parte de ninguna
	// respuesta. Se devuelve lo que hay hoy —una lectura— para que el caller no tenga que
	// distinguir el modo. Sin esta guarda, `musubi_recall_code` intentaría escribir y `query_only`
	// lo rechazaría, dejando fuera de servicio a otra tool que es puramente de lectura.
	if e.soloLectura {
		return e.LedgerStatusDe(sessionID)
	}
	// LEER-MODIFICAR-ESCRIBIR DENTRO DE UNA TRANSACCIÓN. Con la casilla única esto ya era una
	// carrera, sólo que barata: dos procesos concurrentes perdían un incremento. Ahora el valor
	// lleva las cuentas de TODAS las sesiones, así que una escritura pisada no pierde un número:
	// pierde la sesión entera de otra terminal. El DSN lleva `_txlock=immediate` (database.go), así
	// que el lock de escritura se toma al nacer la transacción y nadie puede colarse en el medio.
	tx, err := e.db.Begin()
	if err != nil {
		return TokenLedger{}, fmt.Errorf("error al abrir la transacción del ledger: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	st, err := leerLedgerStore(tx.QueryRow(`SELECT value FROM meta WHERE key = ?`, metaTokenLedger))
	if err != nil {
		return TokenLedger{}, err
	}
	// Un caller SIN id —el camino MCP, que no ve el id del hook— no puede decir a qué sesión
	// pertenece, así que sigue acumulando en la última que escribió: es el contrato de siempre, y el
	// único con sentido cuando hay una sola terminal. Con varias es una atribución imperfecta, y no
	// se puede hacer mejor sin un id; pero ya no BORRA a nadie, que era el defecto. Si todavía no hay
	// ninguna sesión, cae en la clave vacía: el acumulado del servidor always-on (ver encabezado).
	destino := sessionID
	if destino == "" {
		destino, _ = st.ultimaID()
	}
	l := st.de(destino)
	if tokens > 0 {
		l.Total += tokens
		if surface != "" {
			l.Surfaces[surface] += tokens
		}
	}
	st.Seq++
	st.Sesiones[destino] = l
	st.Orden[destino] = st.Seq
	st.podar()

	data, err := json.Marshal(st)
	if err != nil {
		return TokenLedger{}, fmt.Errorf("error al serializar ledger: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		metaTokenLedger, string(data),
	); err != nil {
		return TokenLedger{}, fmt.Errorf("error al guardar el ledger: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TokenLedger{}, fmt.Errorf("error al confirmar el ledger: %w", err)
	}
	return l, nil
}

func (e *DbEngine) loadLedgerStore() (ledgerStore, error) {
	return leerLedgerStore(e.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, metaTokenLedger))
}

// leerLedgerStore decodifica el valor guardado, venga de la conexión o de una transacción.
//
// MIGRA EL FORMATO VIEJO en vez de tirarlo: el valor de una sola casilla
// (`{"session_id":…,"total":…}`) se lee como una sesión más. Sin esto, instalar el binario nuevo
// pondría en cero el contador de la sesión en curso sin decir nada — la misma clase de pérdida
// silenciosa que este arreglo viene a eliminar. Un `json.Unmarshal` del formato viejo sobre
// ledgerStore NO falla (ignora los campos que no conoce) y deja `Sesiones` en nil: eso es lo que
// distingue un formato del otro, no una versión escrita a mano que habría que acordarse de subir.
func leerLedgerStore(fila interface{ Scan(...any) error }) (ledgerStore, error) {
	vacio := ledgerStore{Sesiones: map[string]TokenLedger{}, Orden: map[string]int{}}
	var v string
	switch err := fila.Scan(&v); {
	case err == sql.ErrNoRows:
		return vacio, nil
	case err != nil:
		return vacio, fmt.Errorf("error al leer el ledger: %w", err)
	}
	if v == "" {
		return vacio, nil
	}
	var st ledgerStore
	if err := json.Unmarshal([]byte(v), &st); err == nil && st.Sesiones != nil {
		if st.Orden == nil {
			st.Orden = map[string]int{}
		}
		return st, nil
	}
	var viejo TokenLedger
	if err := json.Unmarshal([]byte(v), &viejo); err != nil {
		return vacio, nil // valor corrupto: arrancar de cero en vez de fallar
	}
	if viejo.Surfaces == nil {
		viejo.Surfaces = map[string]int{}
	}
	return ledgerStore{
		Sesiones: map[string]TokenLedger{viejo.SessionID: viejo},
		Orden:    map[string]int{viejo.SessionID: 1},
		Seq:      1,
	}, nil
}
