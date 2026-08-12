package memory

import (
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// El outbox se siembra HACIA UN DESTINO, y hasta el 2026-08-12 no lo verificaba.
//
// Medido en el cerebro central: el arranque purgó 1.401 filas 'shared' huérfanas —la limpieza
// correcta de un nodo TERMINAL, que no tiene upstream— y 39 segundos después estaban las 1.409
// de vuelta, con attempts=0 y sin error. No eran reintentos: eran filas nuevas. BackfillOutbox
// corre en CADA apertura de la base (no sólo al arrancar el servicio) y no miraba la config, así
// que la purga vivía segundos y `sync_status` reportaba miles de pendientes que no eran de nadie.
//
// La causa de fondo eran DOS condiciones que no coincidían: la purga preguntaba por el DESTINO
// (`!Enabled || CentralURL == ""`) y el gate del encolado preguntaba por la INTENCIÓN (`Enabled`).
// Ahora las dos pasan por config.SyncConfig.HasDestination.

// escribirConfig deja un .musubi/config.yaml en dir. Sin bloque `sync:` el nodo es TERMINAL.
func escribirConfig(t *testing.T, dir, yaml string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.DirName, config.ConfigFile), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

const configConDestino = "version: \"1.0\"\nsync:\n  enabled: true\n  central_url: http://127.0.0.1:7717\n  allow_insecure_token: true\n"
const configSinDestino = "version: \"1.0\"\nmemory:\n  team_mode: true\n"

func abrir(t *testing.T, dir string) *DbEngine {
	t.Helper()
	e, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine(%s): %v", dir, err)
	}
	return e
}

func guardarCompartida(t *testing.T, e *DbEngine, id string) {
	t.Helper()
	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", id, "t/a", "memoria "+id, 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
}

// HasDestination es el predicado único: hace falta la intención Y el destino. Es una tabla y no
// tres tests porque el punto es justamente que las cuatro combinaciones den una sola respuesta.
func TestHasDestinationExigeIntencionYDestino(t *testing.T) {
	casos := []struct {
		nombre   string
		sync     config.SyncConfig
		esperado bool
	}{
		{"habilitado con url", config.SyncConfig{Enabled: true, CentralURL: "http://x:7717"}, true},
		{"habilitado sin url — la grieta del central", config.SyncConfig{Enabled: true}, false},
		{"habilitado con url en blanco", config.SyncConfig{Enabled: true, CentralURL: "   "}, false},
		{"apagado con url", config.SyncConfig{CentralURL: "http://x:7717"}, false},
		{"ni una cosa ni la otra", config.SyncConfig{}, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.sync.HasDestination(); got != c.esperado {
				t.Errorf("HasDestination() = %v, esperaba %v", got, c.esperado)
			}
		})
	}
}

// EL TEST DE LA REGRESIÓN MEDIDA. Un nodo terminal con residuo de un binario viejo: se purga y,
// al reabrir la base, NO se vuelve a sembrar. Antes de la guarda, reabrir reponía las 1.401.
func TestNodoTerminalNoResiembraLoQuePurgo(t *testing.T) {
	dir := t.TempDir()
	escribirConfig(t, dir, configSinDestino)

	// Residuo: un binario viejo (o un comando CLI sin gate) dejó filas encoladas sin destino.
	e := abrir(t, dir)
	e.SetOutboxEnabled(true) // simula el binario de antes del gate
	for _, id := range []string{"obs-1", "obs-2", "obs-3"} {
		guardarCompartida(t, e, id)
	}
	if n := pendientesOutbox(t, e); n != 3 {
		t.Fatalf("preparación: esperaba 3 filas de residuo, hay %d", n)
	}
	if _, err := e.PurgeOutboxPending(); err != nil {
		t.Fatal(err)
	}
	if n := pendientesOutbox(t, e); n != 0 {
		t.Fatalf("la purga dejó %d filas, esperaba 0", n)
	}
	e.Close()

	// Reabrir es lo que pasaba cada 30 minutos en el server (el timer de captura abre la base).
	e2 := abrir(t, dir)
	defer e2.Close()
	if n := pendientesOutbox(t, e2); n != 0 {
		t.Errorf("al reabrir un nodo SIN destino se resembraron %d filas — la purga vuelve a durar segundos", n)
	}
	// Y el contenido sigue intacto: lo que se descarta es el envío, nunca la memoria.
	var c int
	if err := e2.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&c); err != nil {
		t.Fatal(err)
	}
	if c != 3 {
		t.Errorf("quedan %d observaciones de 3 — la guarda no puede tocar el contenido", c)
	}
}

// Un nodo con config y SIN destino tampoco encola lo nuevo. Cierra el agujero de los comandos
// CLI (`capture`, `ingest`, `turn`), que nunca fijaban el gate y dejaban una fila muerta por
// captura en el central: sólo `serve` y `daemon` lo hacían a mano después de abrir.
func TestNodoTerminalNoEncolaLoNuevo(t *testing.T) {
	dir := t.TempDir()
	escribirConfig(t, dir, configSinDestino)

	e := abrir(t, dir)
	defer e.Close()
	guardarCompartida(t, e, "obs-1")

	if n := pendientesOutbox(t, e); n != 0 {
		t.Errorf("un nodo sin destino encoló %d fila(s); esperaba 0", n)
	}
	var scope string
	if err := e.db.QueryRow(`SELECT scope FROM observations WHERE id='obs-1'`).Scan(&scope); err != nil {
		t.Fatalf("la observación NO se guardó: %v", err)
	}
	if scope != ScopeShared {
		t.Errorf("scope = %q, esperaba 'shared': no encolar no puede cambiar lo que se guarda", scope)
	}
}

// LA CONTRACARA, Y LA QUE JUSTIFICA LA GUARDA: no se pierde intención. Lo que se acumuló sin
// destino se siembra ENTERO en cuanto el nodo gana uno. Si este test se cae, la guarda dejó de
// ser una optimización y pasó a ser pérdida de datos.
func TestAlGanarDestinoSeSiembraTodoLoAcumulado(t *testing.T) {
	dir := t.TempDir()
	escribirConfig(t, dir, configSinDestino)

	e := abrir(t, dir)
	for _, id := range []string{"obs-1", "obs-2", "obs-3"} {
		guardarCompartida(t, e, id)
	}
	if n := pendientesOutbox(t, e); n != 0 {
		t.Fatalf("preparación: sin destino no debería haber encolado nada, hay %d", n)
	}
	e.Close()

	// El nodo gana un central: la próxima apertura recupera TODO lo de antes.
	escribirConfig(t, dir, configConDestino)
	e2 := abrir(t, dir)
	defer e2.Close()
	if n := pendientesOutbox(t, e2); n != 3 {
		t.Errorf("al ganar destino se sembraron %d de 3 — se perdió intención durable", n)
	}
}

// Un nodo CON destino encola y siembra como siempre: la guarda no puede romper a un cliente real.
func TestClienteConDestinoEncolaYSiembra(t *testing.T) {
	dir := t.TempDir()
	escribirConfig(t, dir, configConDestino)

	e := abrir(t, dir)
	defer e.Close()
	guardarCompartida(t, e, "obs-1")
	if n := pendientesOutbox(t, e); n != 1 {
		t.Fatalf("un cliente con destino DEBE encolar: esperaba 1 fila, hay %d", n)
	}
}

// Sin config.yaml no hay política declarada, así que se mantiene el default histórico de encolar.
// Es lo que deja intactos a los engines de test sobre un TempDir pelado (y sus 22 aserciones).
func TestSinConfigSeMantieneElDefaultDeEncolar(t *testing.T) {
	e := abrir(t, t.TempDir()) // TempDir pelado: no hay .musubi/config.yaml
	defer e.Close()
	guardarCompartida(t, e, "obs-1")
	if n := pendientesOutbox(t, e); n != 1 {
		t.Errorf("sin config el default histórico es encolar: esperaba 1 fila, hay %d", n)
	}
}
