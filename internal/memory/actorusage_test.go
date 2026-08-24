package memory

import (
	"context"
	"testing"
	"time"
)

// El censo de actores responde QUIÉN llama al cerebro. Los tests de abajo cubren un invariante
// cada uno, y cada uno se verificó FALLANDO bajo un sabotaje que ataca justo lo que declara.

// sondeoDePrueba imita la taxonomía real del riel (internal/mcp/livefeed.go) sin importarla:
// este paquete no conoce tools, y ese es el punto del parámetro.
var sondeoDePrueba = []string{"musubi_sync_pull", "musubi_doctor"}

func llamadas(t *testing.T, e *DbEngine, ctx context.Context, batch []ToolInvocation) {
	t.Helper()
	if err := e.RecordToolInvocations(ctx, batch); err != nil {
		t.Fatalf("registrar: %v", err)
	}
}

// A1 — las llamadas SIN principal no se mezclan con las de alguien.
//
// En stdio local `principal` queda vacío en todas las filas (medido: 230.682 de 230.682 en la
// base de este repo). Si entraran al agrupamiento, el censo tendría un actor anónimo más grande
// que todos los demás juntos, y el panel dibujaría una neurona gigante sin dueño.
func TestA1ElCensoNoInventaUnActorAnonimo(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	llamadas(t, e, ctx, []ToolInvocation{
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "gio"},
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond}, // stdio local
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond}, // stdio local
	})

	actores, _, err := e.ActorUsage(ctx, 30, sondeoDePrueba)
	if err != nil {
		t.Fatalf("censo: %v", err)
	}
	if len(actores) != 1 {
		t.Fatalf("esperaba UN actor (gio), obtuve %d: %+v", len(actores), actores)
	}
	if actores[0].Principal != "gio" || actores[0].Calls != 1 {
		t.Errorf("el único actor debía ser gio con 1 llamada, obtuve %+v", actores[0])
	}
}

// A2 — y aun así las anónimas se DECLARAN. Excluirlas del censo sin decir cuántas son convierte
// «no se sabe de quién es esto» en «esto no existe», que es la mentira más cara que puede decir
// un panel de observabilidad.
func TestA2LasAnonimasSeCuentanAparte(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	llamadas(t, e, ctx, []ToolInvocation{
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "gio"},
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond},
		{Tool: "musubi_doctor", Outcome: OutcomeOK, Duration: time.Millisecond},
	})

	_, sinPrincipal, err := e.ActorUsage(ctx, 30, sondeoDePrueba)
	if err != nil {
		t.Fatalf("censo: %v", err)
	}
	if sinPrincipal != 2 {
		t.Errorf("esperaba 2 llamadas sin principal declaradas, obtuve %d", sinPrincipal)
	}
}

// A3 — el sondeo y el trabajo se separan, y el DEFAULT es trabajo.
//
// Las dos mitades del invariante se sabotean por separado: que clasifique bien con lista, y que
// con lista VACÍA no clasifique nada como sondeo. La segunda mitad es la que importa de verdad:
// si el default fuera sondeo, una tool nueva nacería invisible en el panel.
func TestA3SondeoYTrabajoSeSeparanYElDefaultEsTrabajo(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	llamadas(t, e, ctx, []ToolInvocation{
		{Tool: "musubi_sync_pull", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "crm-cabina"},
		{Tool: "musubi_sync_pull", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "crm-cabina"},
		{Tool: "musubi_doctor", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "crm-cabina"},
		{Tool: "musubi_save_observation", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "crm-cabina"},
	})

	con, _, err := e.ActorUsage(ctx, 30, sondeoDePrueba)
	if err != nil {
		t.Fatalf("censo: %v", err)
	}
	if len(con) != 1 {
		t.Fatalf("esperaba 1 actor, obtuve %d", len(con))
	}
	if con[0].Sondeo != 3 || con[0].Trabajo != 1 {
		t.Errorf("esperaba sondeo=3 trabajo=1, obtuve sondeo=%d trabajo=%d", con[0].Sondeo, con[0].Trabajo)
	}

	// Sin taxonomía, NADA es sondeo. Una tool desconocida aparece de más, nunca de menos.
	sin, _, err := e.ActorUsage(ctx, 30, nil)
	if err != nil {
		t.Fatalf("censo sin taxonomía: %v", err)
	}
	if sin[0].Sondeo != 0 || sin[0].Trabajo != 4 {
		t.Errorf("con lista vacía todo debía ser trabajo, obtuve sondeo=%d trabajo=%d", sin[0].Sondeo, sin[0].Trabajo)
	}
}

// A4 — cuántas tools DISTINTAS tocó separa a un poller de un agente. Sin esta columna, un bot
// que llama tres cosas diez mil veces y una terminal que usa media caja se ven idénticos: el
// mismo `calls` significando dos cosas opuestas.
func TestA4LasToolsDistintasSeparanAlPollerDelAgente(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	batch := []ToolInvocation{}
	for i := 0; i < 20; i++ {
		batch = append(batch, ToolInvocation{Tool: "musubi_sync_pull", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "poller"})
	}
	for _, tool := range []string{"musubi_recall", "musubi_save_observation", "musubi_judge", "musubi_distill", "musubi_impact"} {
		batch = append(batch, ToolInvocation{Tool: tool, Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "agente"})
	}
	llamadas(t, e, ctx, batch)

	actores, _, err := e.ActorUsage(ctx, 30, sondeoDePrueba)
	if err != nil {
		t.Fatalf("censo: %v", err)
	}
	por := map[string]ActorUsageRow{}
	for _, a := range actores {
		por[a.Principal] = a
	}
	if por["poller"].Tools != 1 {
		t.Errorf("el poller debía tocar 1 tool distinta, obtuve %d", por["poller"].Tools)
	}
	if por["agente"].Tools != 5 {
		t.Errorf("el agente debía tocar 5 tools distintas, obtuve %d", por["agente"].Tools)
	}
	if por["poller"].Calls <= por["agente"].Calls {
		t.Fatalf("el fixture perdió sentido: el poller tiene que llamar MÁS veces que el agente")
	}
}

// A5 — TENANCY. El censo dice quién llama al cerebro, con qué frecuencia y cuándo: es
// exactamente el patrón de trabajo de un equipo. Un principal acotado a su proyecto no puede
// ver a los actores de otro. Es el mismo recorte que ya aplica /api/stream sobre el riel.
func TestA5UnPrincipalAcotadoNoVeActoresDeOtroProyecto(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	llamadas(t, e, ctx, []ToolInvocation{
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "gio", ProjectID: "lastchaos"},
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "davantis-altura", ProjectID: "altura"},
	})

	acotado := WithProjectScope(ctx, ProjectScope{ProjectID: "altura"})
	actores, _, err := e.ActorUsage(acotado, 30, sondeoDePrueba)
	if err != nil {
		t.Fatalf("censo acotado: %v", err)
	}
	for _, a := range actores {
		if a.Principal == "gio" {
			t.Fatalf("FUGA DE TENANCY: un principal acotado a «altura» vio al actor de «lastchaos»: %+v", actores)
		}
	}
	if len(actores) != 1 || actores[0].Principal != "davantis-altura" {
		t.Errorf("esperaba sólo davantis-altura, obtuve %+v", actores)
	}
}

// A6 — cuando un actor llamó desde VARIOS proyectos, el censo no elige uno. Devolver el primero
// que salga sería atribuirle un dueño a dedo, que es justo lo que el grafo de personas no puede
// hacer: `crm-cabina` ni siquiera declara proyecto y de quién es se DECLARA, no se deduce.
func TestA6ElProyectoQuedaVacioSiLlamoDesdeVarios(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	llamadas(t, e, ctx, []ToolInvocation{
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "puente", ProjectID: "altura"},
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "puente", ProjectID: "musubi"},
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond, Principal: "propio", ProjectID: "musubi"},
	})

	actores, _, err := e.ActorUsage(ctx, 30, sondeoDePrueba)
	if err != nil {
		t.Fatalf("censo: %v", err)
	}
	por := map[string]ActorUsageRow{}
	for _, a := range actores {
		por[a.Principal] = a
	}
	if por["puente"].Project != "" {
		t.Errorf("un actor de dos proyectos no puede declarar uno; obtuve %q", por["puente"].Project)
	}
	if por["propio"].Project != "musubi" {
		t.Errorf("un actor de un solo proyecto sí lo declara; obtuve %q", por["propio"].Project)
	}
}
