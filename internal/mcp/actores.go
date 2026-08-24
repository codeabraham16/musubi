package mcp

// actores.go expone el CENSO DE ACTORES del cerebro: quién llama, cuánto y con qué.
//
// POR QUÉ ES UN ENDPOINT HTTP Y NO UNA TOOL MCP. El consumidor es un panel, no un agente: el
// grafo de personas necesita el censo entero de una para dimensionar las neuronas, y lo pide una
// vez por carga. Una tool MCP devuelve texto para que lo lea un modelo; acá hace falta JSON para
// que lo dibuje un navegador. `musubi_tool_usage` sigue siendo la vista por herramienta.
//
// LA MITAD QUE FALTABA. El ledger guarda `principal` desde el primer día, pero todo lo que se
// podía preguntar era «qué tools se usan». Quién las usa estaba en la base y no salía por ningún
// lado, así que los bots y los servicios —`b1-adjudicador`, `crm-cabina`, `davantis-crm`— no
// aparecían en ninguna vista: no escriben memoria, sólo llaman.

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"musubi/internal/memory"
)

// actorReader lo implementa el motor real. Interfaz por el mismo motivo que usageReader: el
// backend del server es memory.StorageBackend, y un fake de test que no sepa de ledger tiene que
// dar un censo vacío en vez de tirar el proceso.
type actorReader interface {
	ActorUsage(ctx context.Context, days int, sondeo []string) ([]memory.ActorUsageRow, int, error)
}

// censoActores es la respuesta. `SinPrincipal` va SIEMPRE, incluso en cero: es la diferencia
// entre «no hay llamadas anónimas» y «no me fijé», y el que mira el panel no puede distinguirlas
// si el campo desaparece cuando vale cero.
type censoActores struct {
	Days         int                    `json:"days"`
	Actores      []memory.ActorUsageRow `json:"actores"`
	SinPrincipal int                    `json:"sin_principal"`
	// Sondeo lista la taxonomía que se usó para partir las llamadas. Viaja con la respuesta
	// para que el panel no tenga que mantener su propia copia: dos listas que se separan
	// clasifican el mismo evento distinto y nadie se entera hasta que los números no cierran.
	Sondeo []string `json:"sondeo"`
}

// toolsDeSondeoLista devuelve la taxonomía como slice ordenado. Ordenado a propósito: el orden
// de un mapa de Go es aleatorio por diseño, y el SQL que sale de acá conviene que sea el mismo
// entre corridas para poder compararlo cuando algo no cierra.
func toolsDeSondeoLista() []string {
	out := make([]string, 0, len(toolsDeSondeo))
	for t := range toolsDeSondeo {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// handlerActores sirve GET /api/actores?days=N.
//
// AUTH Y TENANCY IGUAL QUE /api/stream, y por la misma razón exacta: el censo dice quién trabaja,
// cuándo y a qué ritmo. Es el patrón de trabajo de un equipo, o sea información de negocio. Un
// principal acotado ve lo suyo; el recorte se aplica en la consulta (ActorUsage lee el scope del
// contexto), no acá, para que no dependa de que este handler se acuerde.
func (s *McpServer) handlerActores(opt httpOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var principal *Principal
		if opt.registry != nil {
			p, ok := opt.registry.resolve(bearerToken(r.Header.Get("Authorization")))
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			principal = p
		} else if opt.token != "" && !validBearer(r.Header.Get("Authorization"), opt.token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// El tope de 90 días no es prudencia decorativa: la consulta hace COUNT(DISTINCT) sobre
		// una tabla que crece ~100.000 filas por día, y `created_at` es el único índice que la
		// puede podar. Sin techo, un `days=100000` es un escaneo completo servido a pedido.
		days := 7
		if s := r.URL.Query().Get("days"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				days = n
			}
		}
		if days > 90 {
			days = 90
		}

		lector, ok := s.engine.(actorReader)
		if !ok {
			// Censo vacío y 200, no un 500: que este motor no tenga ledger es un estado
			// normal del sistema, no una falla de la petición.
			escribirJSON(w, censoActores{Days: days, Actores: []memory.ActorUsageRow{}, Sondeo: toolsDeSondeoLista()})
			return
		}

		ps, fed := recallScopeFor(principal)
		ctx := memory.WithProjectScope(r.Context(), memory.ProjectScope{ProjectID: ps, Federate: fed})
		sondeo := toolsDeSondeoLista()
		filas, sinPrincipal, err := lector.ActorUsage(ctx, days, sondeo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if filas == nil {
			filas = []memory.ActorUsageRow{}
		}
		escribirJSON(w, censoActores{Days: days, Actores: filas, SinPrincipal: sinPrincipal, Sondeo: sondeo})
	}
}

func escribirJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
