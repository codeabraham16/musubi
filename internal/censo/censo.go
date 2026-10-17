// Package censo es el CENSO DE TOOLS VIVAS (A3): el mapa commiteado que asigna a cada tool
// `musubi_*` del registro una CLASE DE INVOCACIÓN esperada. Es el complemento dinámico del
// trinquete de código muerto (A2): A2 atrapa el código estáticamente inalcanzable; el censo atrapa
// la tool que está cableada (pasa A2) pero que el catálogo sumó sin que nadie decidiera si su
// silencio en el ledger es esperado o DUELE.
//
// POR QUÉ UN PAQUETE HOJA, Y NO un archivo dentro de internal/mcp. La guarda que pinnea el censo
// vive en internal/mcp (necesita enumerar s.tools); el chequeo que lo confirma contra el ledger
// vive en internal/memory (el doctor). Pero internal/mcp importa internal/memory, así que memory
// NO puede importar mcp sin un ciclo. Como el censo tiene que ser UNA sola fuente para las dos
// puntas (si fueran dos listas, divergirían — que es justo la deriva que A3 frena), tiene que vivir
// en un paquete que las dos puedan importar: éste, sin dependencias internas.
//
// POR QUÉ DATO TIPADO, y no un .txt como el baseline de A2. A2 congela una lista PLANA de símbolos;
// un archivo de texto le alcanza. Acá cada entrada lleva una CLASE de un enum de cinco: el tipo
// `Clase` atrapa un typo en tiempo de compilación, no hay parser que equivocar, y cada constante
// lleva su godoc explicando qué significa su cero.
//
// LA REGLA PARA CLASIFICAR, Y SU DEFECTO SEGURO. La pregunta por cada tool es: «en una máquina sana
// y activa, un cero persistente en el ledger MCP, ¿es un problema que alguien debería mirar?». Si
// NO —porque pide una acción humana rara, porque la cubre un ciclo del daemon, porque nadie emite a
// ese sumidero, o porque se corre por CLI/hook (ciega al ledger)— va a una de las cuatro clases que
// EXCUSAN el cero. Si SÍ, va a DebeInvocarse. ANTE LA DUDA se clasifica DebeInvocarse: así el
// doctor la surface como HIPÓTESIS (honesto: «alguien debería mirar»), en vez de excusar en
// silencio un «se construye y no se enciende». Meter una tool en una clase que excusa su cero
// ESCONDE el defecto; es el error caro, y por eso no es el default.
//
// Grounded en la investigación medida del 2026-09-07 (investigacion/por-que-nada-se-invoca-solo):
// de las 18 tools muertas en los dos ledgers, el split de cuatro clases salió de ahí. La clase
// DebeInvocarse recoge además el «cero que SÍ duele» medido {debate, detect_changes, promote,
// save_fact, readiness, resolve_skills}: capacidades de juicio oportuno sin disparador.
package censo

import "sort"

// Clase es la clase de invocación esperada de una tool. Un cero en el ledger se lee según la clase.
type Clase int

const (
	// Administrativa: dispara con una decisión o aprobación humana, o es la superficie de operación
	// de la flota / las credenciales. Un cero es CORRECTO por diseño e indefinidamente: no se tocan
	// salvo que alguien esté administrando. Ej.: fleet_approve, token_revoke.
	Administrativa Clase = iota
	// CubiertaPorCiclo: el trabajo lo hace un ticker del daemon o un disparador por evento, no una
	// llamada directa. Su cero es la PRUEBA de que el ciclo funciona. Ej.: maintain (ciclo de 24 h),
	// codegraph_index (re-índice cada 6 h).
	CubiertaPorCiclo
	// SinProductor: sumidero de telemetría al que nadie emite TODAVÍA. Su cero no es un problema de
	// esta tool sino la ausencia de un productor. Ej.: log_error, resolve_telemetry.
	SinProductor
	// CaminoCLI: se invoca por CLI o por un hook del ciclo de vida, caminos CIEGOS al ledger MCP (el
	// ledger sólo registra el camino MCP). Su cero en el ledger NO significa que no se use. Ej.:
	// detect_stack (corre por `musubi detect --hook-mode`), ingest_url (`musubi ingest`).
	CaminoCLI
	// DebeInvocarse: capacidad que el agente DEBERÍA ejercer en el turno como parte de su lazo
	// (memoria, cognición, código, SDD, observabilidad). Un cero persistente acá es el olor «se
	// construye y no se enciende» que A3 existe para surfacear. Es el default ante la duda, y la
	// única clase sobre la que el doctor reporta.
	DebeInvocarse
)

func (c Clase) String() string {
	switch c {
	case Administrativa:
		return "administrativa"
	case CubiertaPorCiclo:
		return "cubierta-por-ciclo"
	case SinProductor:
		return "sin-productor"
	case CaminoCLI:
		return "camino-cli"
	case DebeInvocarse:
		return "debe-invocarse"
	default:
		return "desconocida"
	}
}

// censo es la fuente única: cada tool `musubi_*` del registro vivo, con su clase. La guarda
// internal/mcp/censo_test.go lo pinnea contra s.tools en ambas direcciones (una tool sin censar o
// una entrada huérfana ponen rojo el CI), así que este mapa no puede divergir del registro.
var censo = map[string]Clase{
	// --- Memoria, cognición, código, SDD, observabilidad: el lazo del agente en el turno. ---
	"musubi_ask":                 DebeInvocarse,
	"musubi_brain_graph":         DebeInvocarse,
	"musubi_code_context":        DebeInvocarse,
	"musubi_code_graph":          DebeInvocarse,
	"musubi_code_graph_viz":      DebeInvocarse,
	"musubi_cognition_stats":     DebeInvocarse,
	"musubi_conflicts":           DebeInvocarse,
	"musubi_corroborate":         DebeInvocarse,
	"musubi_debate":              DebeInvocarse, // cero-que-duele medido; revivió
	"musubi_design":              DebeInvocarse,
	"musubi_detect_changes":      DebeInvocarse, // cero-que-duele medido; revivió
	"musubi_discard_proposal":    DebeInvocarse,
	"musubi_doctor":              DebeInvocarse,
	"musubi_entity_context":      DebeInvocarse,
	"musubi_impact":              DebeInvocarse,
	"musubi_insights":            DebeInvocarse,
	"musubi_judge":               DebeInvocarse,
	"musubi_list_skills":         DebeInvocarse,
	"musubi_map":                 DebeInvocarse,
	"musubi_memory_expand":       DebeInvocarse,
	"musubi_phase":               DebeInvocarse,
	"musubi_promote":             DebeInvocarse, // cero-que-duele medido; revivió
	"musubi_promote_skill":       DebeInvocarse,
	"musubi_propose_facts":       DebeInvocarse,
	"musubi_propose_observation": DebeInvocarse,
	"musubi_readiness":           DebeInvocarse, // cero-que-duele medido; revivió
	"musubi_recall":              DebeInvocarse,
	"musubi_recall_code":         DebeInvocarse,
	"musubi_recall_facts":        DebeInvocarse,
	"musubi_resolve_skills":      DebeInvocarse, // cero-que-duele medido; SIGUE en cero (sin trigger)
	"musubi_save_code":           DebeInvocarse,
	"musubi_save_fact":           DebeInvocarse, // cero-que-duele medido; SIGUE en cero (sin trigger)
	"musubi_save_observation":    DebeInvocarse,
	"musubi_sdd":                 DebeInvocarse,
	"musubi_search_keyword":      DebeInvocarse,
	"musubi_search_semantic":     DebeInvocarse,
	"musubi_search_skills":       DebeInvocarse,
	"musubi_sharpen":             DebeInvocarse,
	"musubi_skill_usage":         DebeInvocarse,
	"musubi_sync_status":         DebeInvocarse,
	"musubi_tokens":              DebeInvocarse,
	"musubi_tool_usage":          DebeInvocarse,
	"musubi_whoami":              DebeInvocarse,
	"musubi_work":                DebeInvocarse,

	// --- Administrativa: superficie de operación de flota/credenciales; cero esperado. ---
	"musubi_fleet_approvals":        Administrativa,
	"musubi_fleet_approve":          Administrativa,
	"musubi_fleet_consent":          Administrativa,
	"musubi_fleet_contexto":         Administrativa,
	"musubi_fleet_cronologia":       Administrativa,
	"musubi_fleet_enroll":           Administrativa,
	"musubi_fleet_exec":             Administrativa,
	"musubi_fleet_list":             Administrativa,
	"musubi_fleet_log":              Administrativa,
	"musubi_fleet_maintenance":      Administrativa,
	"musubi_fleet_metrics":          Administrativa,
	"musubi_fleet_probe":            Administrativa,
	"musubi_fleet_rename":           Administrativa,
	"musubi_fleet_require_approval": Administrativa,
	"musubi_fleet_revoke":           Administrativa,
	"musubi_fleet_rotate":           Administrativa,
	"musubi_fleet_screen":           Administrativa,
	"musubi_fleet_service_declare":  Administrativa,
	"musubi_fleet_services":         Administrativa,
	"musubi_fleet_sessions":         Administrativa,
	"musubi_fleet_shell":            Administrativa,
	"musubi_fleet_shell_log":        Administrativa,
	"musubi_sync_requeue":           Administrativa, // recuperación manual del outbox
	"musubi_token_list":             Administrativa,
	"musubi_token_new":              Administrativa,
	"musubi_token_revoke":           Administrativa,

	// --- CubiertaPorCiclo: la dispara un ticker del daemon o un disparador por evento, no una llamada directa. ---
	"musubi_codegraph_index": CubiertaPorCiclo, // RunCodeGraphScheduler, cada 6 h
	"musubi_codegraph_push":  CubiertaPorCiclo, // lo empuja el daemon al federar el grafo al central (no un subcomando CLI)
	"musubi_distill":         CubiertaPorCiclo, // destilado del acervo (ticker; opt-in por default)
	"musubi_maintain":        CubiertaPorCiclo, // ciclo de mantenimiento de 24 h
	"musubi_sync_pull":       CubiertaPorCiclo, // drain/pull del scheduler de sync

	// --- SinProductor: telemetría que nadie emite todavía. ---
	"musubi_log_error":          SinProductor,
	"musubi_log_skill_decision": SinProductor,
	"musubi_resolve_telemetry":  SinProductor,

	// --- CaminoCLI: se corre por CLI/hook, ciego al ledger MCP. ---
	"musubi_author_skill":    CaminoCLI, // autoría de skills (opt-in; dormida)
	"musubi_detect_stack":    CaminoCLI, // `musubi detect --hook-mode`
	"musubi_discover_skills": CaminoCLI, // catálogo remoto opt-in
	"musubi_ingest_url":      CaminoCLI, // `musubi ingest`
	"musubi_install_skill":   CaminoCLI, // setup/instalación
	"musubi_save_skill":      CaminoCLI, // autoría/guardado de skill
	"musubi_workflow":        CaminoCLI, // orquestación opt-in (dormida)
}

// ClaseDe devuelve la clase de una tool y si está censada.
func ClaseDe(name string) (Clase, bool) {
	c, ok := censo[name]
	return c, ok
}

// Tools devuelve los nombres censados, ordenados (para iteración determinista).
func Tools() []string {
	out := make([]string, 0, len(censo))
	for name := range censo {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// DebenInvocarse devuelve las tools de clase DebeInvocarse, ordenadas. Es lo que el chequeo del
// doctor confirma contra el ledger: una de éstas en cero es una HIPÓTESIS a confirmar, no un rojo.
func DebenInvocarse() []string {
	out := []string{}
	for name, c := range censo {
		if c == DebeInvocarse {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
