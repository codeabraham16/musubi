package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"
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

// metaTokenLedger es la casilla del formato VIEJO (una sola sesión). Sólo se LEE, para migrar.
const metaTokenLedger = "token_ledger"

// metaTokenLedgerV2 es donde vive el formato por sesión. Es una clave NUEVA a propósito.
//
// Una revisión adversarial lo encontró antes del merge: los servidores MCP son procesos largos que
// siguen corriendo el binario VIEJO después de instalar el nuevo, hasta que se reinicia cada sesión,
// mientras los hooks ya corren el nuevo. Con las dos versiones escribiendo la MISMA clave, el viejo
// leía el formato nuevo como un TokenLedger vacío (json.Unmarshal no falla, ignora lo que no conoce),
// le sumaba lo suyo y guardaba su casilla encima: todas las sesiones en cero, en cada hidratación.
// Con claves separadas, el binario viejo sólo pisa su propia casilla, que el nuevo ya no escribe.
const metaTokenLedgerV2 = "token_ledger_v2"

// maxSesionesEnLedger acota cuántas sesiones se guardan: el ledger es telemetría, no un registro
// contable, y sin tope un solo valor de `meta` crecería sin freno. Es 64 y no 16 porque en este repo
// corren workflows de 40 y 72 sub-agentes con id propio: con 16, la terminal principal que los lanzó
// quedaba desalojada mientras esperaba. Cada sesión pesa unos cientos de bytes; 64 son decenas de KB.
const maxSesionesEnLedger = 64

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
// `Orden` es un número de escritura: dice quién escribió después de quién sin depender del reloj.
// `Ultima` es la hora (unix) de la última escritura de cada sesión, y sirve para una sola cosa:
// saber cuáles llevan HORAS sin escribir (ver podar). La base es de una sola máquina, así que no hay
// dos relojes que comparar.
type ledgerStore struct {
	Sesiones map[string]TokenLedger `json:"sesiones"`
	Orden    map[string]int         `json:"orden"`
	Ultima   map[string]int64       `json:"ultima,omitempty"`
	Seq      int                    `json:"seq"`
}

// relojLedger es la hora que se estampa en cada escritura. Variable para que las pruebas puedan
// armar sesiones «de ayer» sin esperar un día.
var relojLedger = time.Now

// sesionesRecientesProtegidas es cuántas de las últimas sesiones que escribieron no se desalojan
// nunca, además de la recién escrita. inactivaTras es cuánto tiempo sin escribir hace que una sesión
// se considere cerrada. Ver podar.
const (
	sesionesRecientesProtegidas = 8
	inactivaTras                = 2 * time.Hour
)

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

// LedgerReset pone en cero la cuenta de UNA sesión: la indicada por sessionID, o —si viene vacío— la
// escrita más recientemente, que es la misma a la que LedgerAdd le imputa un llamado sin id. Quien lo
// pide es musubi_tokens, que corre por MCP y no conoce su propio id: por eso puede recibirlo, y la
// lista de sesiones del reporte es de donde el agente lo saca.
//
// No borra TODAS, y lo encontró la revisión adversarial: el reset desde una terminal vaciaba el valor
// entero, o sea que reintroducía por otra puerta exactamente el defecto que este formato viene a
// cerrar —una sesión borrando la cuenta de las demás—.
func (e *DbEngine) LedgerReset(sessionID string) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al abrir la transacción del ledger: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	st, err := leerLedgerStoreTx(tx)
	if err != nil {
		return err
	}
	id := st.destinoDe(sessionID)
	if _, hay := st.Sesiones[id]; !hay {
		return nil // nada que poner en cero: no se crea una sesión para vaciarla
	}
	st.Sesiones[id] = TokenLedger{SessionID: id, Surfaces: map[string]int{}}
	if err := guardarLedgerStoreTx(tx, st); err != nil {
		return err
	}
	return tx.Commit()
}

// SesionLedger es el resumen de una sesión para listar todas: quién, cuánto, y en qué orden escribió.
type SesionLedger struct {
	SessionID string `json:"session_id"`
	Total     int    `json:"total"`
	Orden     int    `json:"orden"`
	// UltimaEscritura es la hora de su última escritura (RFC3339), vacía si no se sabe. Es lo que
	// deja a un agente reconocer su propia sesión en la lista: la que escribió recién es la suya.
	UltimaEscritura string `json:"ultima_escritura,omitempty"`
}

// LedgerSesiones lista todas las sesiones guardadas, de la escrita más recientemente a la más vieja.
// Existe para que musubi_tokens no muestre un número suelto sin decir de quién es: corre por MCP, no
// sabe cuál es su propia sesión, y con varias terminales «la última que escribió» puede ser otra.
func (e *DbEngine) LedgerSesiones() ([]SesionLedger, error) {
	st, err := e.loadLedgerStore()
	if err != nil {
		return nil, err
	}
	out := make([]SesionLedger, 0, len(st.Sesiones))
	for id, l := range st.Sesiones {
		sl := SesionLedger{SessionID: id, Total: l.Total, Orden: st.Orden[id]}
		if u := st.Ultima[id]; u > 0 {
			sl.UltimaEscritura = time.Unix(u, 0).UTC().Format(time.RFC3339)
		}
		out = append(out, sl)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Orden != out[j].Orden {
			return out[i].Orden > out[j].Orden
		}
		return out[i].SessionID < out[j].SessionID
	})
	return out, nil
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

// podar desaloja sesiones cuando se pasa del tope. Nunca la recién escrita ni las
// sesionesRecientesProtegidas que escribieron último; entre las demás, primero las INACTIVAS (sin
// escribir hace más de inactivaTras, la más vieja antes) y, si no hay, la de MENOR total.
//
// Dos revisiones adversariales, dos formas de equivocarse, y cada criterio tapa una:
//
//   - La primera versión desalojaba la menos recientemente escrita: la terminal principal que lanza
//     un workflow de sub-agentes queda quieta mientras ellos escriben con sus ids, así que era la más
//     vieja justo cuando más había gastado. Por eso, entre las activas, se desaloja por total: las
//     sesiones de sub-agentes son chicas y la de quien trabaja acumula.
//   - Pero SÓLO por total, las sesiones más gordas de toda la historia —terminales de días
//     anteriores, ya cerradas— se volvían inmortales, y toda sesión viva que todavía no las
//     alcanzaba era la candidata apenas escribía cualquier otra: dos terminales abiertas se
//     borraban la cuenta mutuamente en cada escritura (medido en la segunda ronda: A y B gastaron
//     2.500 cada una y el ledger decía 0 y 500). Por eso las inactivas salen primero, y las últimas
//     que escribieron no son candidatas: dos terminales que se alternan están siempre ahí.
//
// La recién escrita se excluye porque una sesión que acaba de nacer tiene total chico, y sin la
// excepción la desalojaría la misma escritura que la creó.
func (s *ledgerStore) podar(protegida string) {
	for len(s.Sesiones) > maxSesionesEnLedger {
		protegidas := s.recientes(sesionesRecientesProtegidas)
		protegidas[protegida] = true
		limite := relojLedger().Add(-inactivaTras).Unix()
		peorID, peorInactiva, hay := "", false, false
		for id, l := range s.Sesiones {
			if protegidas[id] {
				continue
			}
			inactiva := s.Ultima[id] <= limite
			switch {
			case !hay:
			case inactiva != peorInactiva:
				if !inactiva {
					continue // una inactiva ya ganó el lugar: una activa no le pasa adelante
				}
			case inactiva:
				if !masViejaAntes(s.Ultima[id], s.Orden[id], id, s.Ultima[peorID], s.Orden[peorID], peorID) {
					continue
				}
			default:
				if !desalojarAntes(l.Total, s.Orden[id], id, s.Sesiones[peorID].Total, s.Orden[peorID], peorID) {
					continue
				}
			}
			peorID, peorInactiva, hay = id, inactiva, true
		}
		if !hay {
			return
		}
		delete(s.Sesiones, peorID)
		delete(s.Orden, peorID)
		delete(s.Ultima, peorID)
	}
}

// recientes devuelve las n sesiones con el número de escritura más alto.
func (s ledgerStore) recientes(n int) map[string]bool {
	ids := make([]string, 0, len(s.Sesiones))
	for id := range s.Sesiones {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if s.Orden[ids[i]] != s.Orden[ids[j]] {
			return s.Orden[ids[i]] > s.Orden[ids[j]]
		}
		return ids[i] > ids[j]
	})
	out := make(map[string]bool, n+1)
	for i := 0; i < n && i < len(ids); i++ {
		out[ids[i]] = true
	}
	return out
}

// masViejaAntes ordena inactivas: la que escribió hace más tiempo primero; a igual hora, la de menor
// orden; a igual orden, por id (salida determinista).
func masViejaAntes(ultima int64, orden int, id string, ultimaB int64, ordenB int, idB string) bool {
	if ultima != ultimaB {
		return ultima < ultimaB
	}
	if orden != ordenB {
		return orden < ordenB
	}
	return id < idB
}

// desalojarAntes ordena candidatas a desalojo: menor total primero; a igual total, la menos
// recientemente escrita; a igual orden, por id, para que la salida sea determinista (el recorrido de
// un map en Go es aleatorio a propósito).
func desalojarAntes(total, orden int, id string, totalB, ordenB int, idB string) bool {
	if total != totalB {
		return total < totalB
	}
	if orden != ordenB {
		return orden < ordenB
	}
	return id < idB
}

// LedgerAdd suma tokens a una superficie de la sesión sessionID y devuelve su ledger. Cada sesión
// tiene su propia cuenta: sumar en una no toca a las demás. Un sessionID vacío se imputa a la sesión
// escrita más recientemente (ver destinoDe).
//
// Con tokens <= 0 es una LECTURA PURA: no abre transacción, no crea la sesión y no la sube en el
// orden. Hay llamadores que la usan para preguntar —el aviso una-vez-por-sesión de precheck lee su
// marca con 0 tokens—, y en la primera versión esa pregunta escribía: una sesión que sólo leía ocupaba
// un lugar y empujaba a otras al desalojo.
func (e *DbEngine) LedgerAdd(sessionID, surface string, tokens int) (TokenLedger, error) {
	// En sólo lectura el ledger no se toca: es telemetría del ahorro, no parte de ninguna
	// respuesta. Se devuelve lo que hay hoy —una lectura— para que el caller no tenga que
	// distinguir el modo. Sin esta guarda, `musubi_recall_code` intentaría escribir y `query_only`
	// lo rechazaría, dejando fuera de servicio a otra tool que es puramente de lectura. Y el destino
	// se resuelve IGUAL que en el camino escribible: en la primera versión, un llamado sin id
	// devolvía acá la clave literal "" (ceros) y allá la última sesión.
	if e.soloLectura || tokens <= 0 {
		st, err := e.loadLedgerStore()
		if err != nil {
			return TokenLedger{Surfaces: map[string]int{}}, err
		}
		return st.de(st.destinoDe(sessionID)), nil
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

	st, err := leerLedgerStoreDe(tx)
	if err != nil {
		return TokenLedger{}, err
	}
	destino := st.destinoDe(sessionID)
	l := st.de(destino)
	l.Total += tokens
	if surface != "" {
		l.Surfaces[surface] += tokens
	}
	st.Seq++
	st.Sesiones[destino] = l
	st.Orden[destino] = st.Seq
	st.Ultima[destino] = relojLedger().Unix()
	st.podar(destino)

	if err := guardarLedgerStoreTx(tx, st); err != nil {
		return TokenLedger{}, err
	}
	if err := tx.Commit(); err != nil {
		return TokenLedger{}, fmt.Errorf("error al confirmar el ledger: %w", err)
	}
	return l, nil
}

// destinoDe resuelve a qué sesión se imputa un llamado. Un caller SIN id —el camino MCP, que no ve el
// id del hook— no puede decir a qué sesión pertenece, así que se imputa a la última que escribió: es
// el contrato de siempre, y el único con sentido cuando hay una sola terminal. Con varias es una
// atribución imperfecta, y no se puede hacer mejor sin un id; pero ya no BORRA a nadie, que era el
// defecto. Si todavía no hay ninguna sesión, cae en la clave vacía: el acumulado del servidor
// always-on (ver el encabezado del archivo).
func (s ledgerStore) destinoDe(sessionID string) string {
	if sessionID != "" {
		return sessionID
	}
	id, _ := s.ultimaID()
	return id
}

func (e *DbEngine) loadLedgerStore() (ledgerStore, error) {
	return leerLedgerStoreDe(e.db)
}

// leerLedgerStoreTx lee el ledger dentro de una transacción ya abierta.
func leerLedgerStoreTx(tx *sql.Tx) (ledgerStore, error) {
	return leerLedgerStoreDe(tx)
}

// lectorLedger es lo que *sql.DB y *sql.Tx tienen en común para leer.
type lectorLedger interface {
	QueryRow(query string, args ...any) *sql.Row
}

// leerLedgerStoreDe lee el formato por sesión de SU clave (metaTokenLedgerV2) y, si todavía no existe,
// MIGRA la casilla vieja en vez de tirarla: sin esto, instalar el binario nuevo pondría en cero el
// contador de la sesión en curso sin decir nada — la misma pérdida silenciosa que este formato viene
// a eliminar. La migración sólo LEE la clave vieja; nunca la escribe, para que un binario viejo que
// siga corriendo pise únicamente su propia casilla (ver metaTokenLedgerV2).
//
// Un `json.Unmarshal` del formato viejo sobre ledgerStore NO falla —ignora los campos que no conoce—
// y deja `Sesiones` en nil: eso es lo que distingue un formato del otro.
func leerLedgerStoreDe(c lectorLedger) (ledgerStore, error) {
	vacio := ledgerStore{Sesiones: map[string]TokenLedger{}, Orden: map[string]int{}, Ultima: map[string]int64{}}
	if v, ok, err := leerValorMeta(c, metaTokenLedgerV2); err != nil {
		return vacio, err
	} else if ok && v != "" {
		var st ledgerStore
		if err := json.Unmarshal([]byte(v), &st); err != nil || st.Sesiones == nil {
			return vacio, nil // valor corrupto: arrancar de cero en vez de fallar
		}
		if st.Orden == nil {
			st.Orden = map[string]int{}
		}
		if st.Ultima == nil {
			st.Ultima = map[string]int64{}
		}
		return st, nil
	}
	v, ok, err := leerValorMeta(c, metaTokenLedger)
	if err != nil {
		return vacio, err
	}
	if !ok || v == "" {
		return vacio, nil
	}
	var viejo TokenLedger
	if err := json.Unmarshal([]byte(v), &viejo); err != nil {
		return vacio, nil
	}
	if viejo.Surfaces == nil {
		viejo.Surfaces = map[string]int{}
	}
	return ledgerStore{
		Sesiones: map[string]TokenLedger{viejo.SessionID: viejo},
		Orden:    map[string]int{viejo.SessionID: 1},
		Ultima:   map[string]int64{viejo.SessionID: relojLedger().Unix()},
		Seq:      1,
	}, nil
}

func leerValorMeta(c lectorLedger, key string) (string, bool, error) {
	var v string
	switch err := c.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v); {
	case err == sql.ErrNoRows:
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("error al leer el ledger: %w", err)
	}
	return v, true, nil
}

// guardarLedgerStoreTx escribe el formato por sesión en SU clave, nunca en la del formato viejo.
func guardarLedgerStoreTx(tx *sql.Tx, st ledgerStore) error {
	data, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("error al serializar ledger: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		metaTokenLedgerV2, string(data),
	); err != nil {
		return fmt.Errorf("error al guardar el ledger: %w", err)
	}
	return nil
}
