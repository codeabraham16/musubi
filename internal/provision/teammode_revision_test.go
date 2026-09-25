package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// cargarDe parsea el texto de un config con el mismo parser que Load.
func cargarDe(t *testing.T, contenido string) config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(contenido))
	if err != nil {
		t.Fatalf("el config que quedó no parsea: %v\n%s", err, contenido)
	}
	return cfg
}

// teammode_revision_test.go custodia lo que encontró la revisión adversarial del alta, antes del
// merge: todos son YAML válidos que el editor por regex dejaba ILEGIBLES mientras el paso reportaba
// «hecho», o que daba vuelta sin decirlo.

// provisionar corre el alta sobre un config y devuelve el estado y cómo quedó el archivo.
func provisionar(t *testing.T, contenido string) (StepResult, string) {
	t.Helper()
	dir := escribirCfg(t, contenido)
	r := ensureSyncConfig(dir, "https://cerebro:10000", "MUSUBI_TOKEN", false)
	got, _ := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	return r, string(got)
}

// HALLAZGO: una línea en blanco o un comentario en columna 0 ENTRE dos claves de `memory:` cortaba el
// bloque de la regex: el editor no veía el team_mode de abajo, insertaba otro, y la clave quedaba
// duplicada (yaml.v3 lo rechaza) con el paso en «hecho». Ahora el bloque llega hasta su último hijo.
func TestProvisionLineaEnBlancoOComentarioDentroDeMemory(t *testing.T) {
	for nombre, contenido := range map[string]string{
		"línea en blanco":      "version: \"1.0\"\nmemory:\n  recall_token_budget: 400\n\n  team_mode: false\n",
		"comentario columna 0": "version: \"1.0\"\nmemory:\n  recall_token_budget: 400\n# apagado a mano\n  team_mode: false\n",
	} {
		t.Run(nombre, func(t *testing.T) {
			r, got := provisionar(t, contenido)
			if r.Status != StatusDone {
				t.Fatalf("estado %q: %s", r.Status, r.Detail)
			}
			if strings.Count(got, "team_mode:") != 1 {
				t.Fatalf("team_mode quedó %d veces (tiene que ser una):\n%s", strings.Count(got, "team_mode:"), got)
			}
			cfg := cargarDe(t, got)
			if !cfg.Memory.TeamMode || cfg.Memory.RecallTokenBudget != 400 {
				t.Fatalf("team_mode=%v budget=%d; quería true y 400", cfg.Memory.TeamMode, cfg.Memory.RecallTokenBudget)
			}
		})
	}
}

// HALLAZGO: el molde de sangría se tomaba de la primera línea del bloque, aunque fuera un comentario
// con otra sangría: la clave quedaba más honda que sus hermanas y el YAML dejaba de parsear.
func TestProvisionComentarioComoPrimerHijoNoEsElMolde(t *testing.T) {
	r, got := provisionar(t, "version: \"1.0\"\nmemory:\n    # notas del equipo\n  recall_token_budget: 400\n")
	if r.Status != StatusDone {
		t.Fatalf("estado %q: %s", r.Status, r.Detail)
	}
	if cfg := cargarDe(t, got); !cfg.Memory.TeamMode || cfg.Memory.RecallTokenBudget != 400 {
		t.Fatalf("team_mode=%v budget=%d:\n%s", cfg.Memory.TeamMode, cfg.Memory.RecallTokenBudget, got)
	}
}

// HALLAZGO: un archivo que termina en `memory:` sin salto de línea hacía que la clave se insertara
// ANTES de la cabecera. Se prueba sobre la función sola Y de punta a punta con `sync:` ANTES de
// `memory:`: la primera versión de esta prueba usaba un config sin `sync:`, el alta le AGREGA ese
// bloque al final, y entonces `memory:` dejaba de ser la última línea — la rama que se custodia no
// se ejecutaba nunca y la prueba pasaba con el arreglo saboteado.
func TestProvisionMemorySinSaltoDeLineaAlFinal(t *testing.T) {
	got, err := asegurarTeamMode([]byte("version: \"1.0\"\nmemory:"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg := cargarDe(t, string(got)); !cfg.Memory.TeamMode {
		t.Fatalf("team_mode tenía que quedar en true:\n%s", got)
	}
	r, final := provisionar(t, "version: \"1.0\"\nsync:\n  enabled: false\nmemory:")
	if r.Status != StatusDone {
		t.Fatalf("de punta a punta, estado %q: %s", r.Status, r.Detail)
	}
	if cfg := cargarDe(t, final); !cfg.Memory.TeamMode || !cfg.Sync.Enabled {
		t.Fatalf("de punta a punta tenían que quedar el sync y team_mode:\n%s", final)
	}
}

// HALLAZGO: `memory: {}` o `memory: null` —YAML válido— terminaban con una clave colgada de un mapa
// en línea o de un escalar: ilegible, y el paso en «hecho». Eso no se edita a ciegas: error, el
// archivo intacto, y la instrucción para hacerlo a mano.
func TestProvisionMemoryEnLineaNoSeEditaACiegas(t *testing.T) {
	for _, contenido := range []string{
		"version: \"1.0\"\nmemory: {}\n",
		"version: \"1.0\"\nmemory: {team_mode: false}\n",
		"version: \"1.0\"\nmemory: null\n",
	} {
		r, got := provisionar(t, contenido)
		if r.Status != StatusError {
			t.Errorf("%q: tenía que negarse; estado %q: %s", contenido, r.Status, r.Detail)
		}
		if got != contenido {
			t.Errorf("%q: al negarse NO se escribe nada, y quedó:\n%s", contenido, got)
		}
		if !strings.Contains(r.Detail, "team_mode: true") {
			t.Errorf("el error tiene que decir cómo hacerlo a mano: %s", r.Detail)
		}
	}
}

// La misma trampa de la línea en blanco estaba en el bloque `sync:` desde antes de este cambio: al
// reemplazarlo, los hijos de después de la línea en blanco quedaban sueltos y duplicados.
func TestProvisionLineaEnBlancoDentroDeSync(t *testing.T) {
	r, got := provisionar(t, "version: \"1.0\"\nsync:\n  enabled: false\n\n  drain_interval_seconds: 15\nmemory:\n  recall_token_budget: 400\n")
	if r.Status != StatusDone {
		t.Fatalf("estado %q: %s", r.Status, r.Detail)
	}
	if n := strings.Count(got, "drain_interval_seconds:"); n != 1 {
		t.Fatalf("drain_interval_seconds quedó %d veces:\n%s", n, got)
	}
	if cfg := cargarDe(t, got); !cfg.Sync.Enabled {
		t.Fatalf("el sync tenía que quedar habilitado:\n%s", got)
	}
}

// Los comentarios que siguen al bloque `sync:` son de la sección siguiente: reemplazar el bloque no
// se los puede llevar puestos.
func TestProvisionComentarioDespuesDeSyncSobrevive(t *testing.T) {
	_, got := provisionar(t, "version: \"1.0\"\nsync:\n  enabled: false\n\n# lo que sigue es de memoria\nmemory:\n  recall_token_budget: 400\n")
	if !strings.Contains(got, "# lo que sigue es de memoria") {
		t.Fatalf("reemplazar sync: se llevó el comentario de la sección siguiente:\n%s", got)
	}
}

// La red de seguridad: si una edición deja el config sin parsear, NO se escribe. Se prueba con un
// original que ya no parsea —el caso donde ningún editor puede tener razón—: el archivo queda intacto.
func TestProvisionNoEscribeSiElResultadoNoValida(t *testing.T) {
	roto := "version: \"1.0\"\nmemory:\n  recall_token_budget: [esto no cierra\n"
	r, got := provisionar(t, roto)
	if r.Status != StatusError {
		t.Fatalf("un config que no parsea no se toca; estado %q: %s", r.Status, r.Detail)
	}
	if got != roto {
		t.Fatalf("se escribió encima de un config roto:\n%s", got)
	}
}

// El Detail del alta dice lo que cambia de verdad: además de bajar, lo que se guarde sin scope sube
// al central, y las sesiones abiertas no lo toman hasta reabrirse.
func TestProvisionElDetalleDiceLoQueCambia(t *testing.T) {
	r, _ := provisionar(t, "version: \"1.0\"\n")
	for _, frase := range []string{"sube al central", "reabrilas"} {
		if !strings.Contains(r.Detail, frase) {
			t.Errorf("el Detail tiene que decir %q: %s", frase, r.Detail)
		}
	}
}

// Cada guarda, por separado. La validación final es la red que ataja cualquier edición rota, y justo
// por eso tapa a las guardas de adelante: sabotear una sola seguiría en verde porque la validación la
// rechaza igual. Estas dos pruebas miran cada guarda sin la otra.

// La guarda de valor en línea, sola: asegurarTeamMode se niega antes de editar.
func TestAsegurarTeamModeSeNiegaConValorEnLinea(t *testing.T) {
	for _, contenido := range []string{"memory: {}\n", "memory: null\n", "memory: {team_mode: false} # nota\n"} {
		if _, err := asegurarTeamMode([]byte(contenido)); err == nil {
			t.Errorf("%q: tenía que negarse a editar un memory: con valor en la misma línea", contenido)
		}
	}
	if _, err := asegurarTeamMode([]byte("memory: # sólo un comentario\n  recall_token_budget: 400\n")); err != nil {
		t.Errorf("un comentario en la cabecera no es un valor: %v", err)
	}
}

// La comparación de la validación, sola: si la edición toca algo más que el sync y team_mode, no pasa.
func TestValidarEdicionRechazaCambiosDeMas(t *testing.T) {
	original := []byte("version: \"1.0\"\nmemory:\n  recall_token_budget: 400\n")
	bien := []byte("version: \"1.0\"\nsync:\n  enabled: true\n  central_url: https://cerebro:10000\nmemory:\n  team_mode: true\n  recall_token_budget: 400\n")
	demas := []byte("version: \"1.0\"\nsync:\n  enabled: true\n  central_url: https://cerebro:10000\nmemory:\n  team_mode: true\n  recall_token_budget: 777\n")
	if err := validarEdicion(original, bien, "https://cerebro:10000"); err != nil {
		t.Fatalf("la edición correcta tenía que pasar: %v", err)
	}
	if err := validarEdicion(original, demas, "https://cerebro:10000"); err == nil {
		t.Fatal("una edición que además cambia recall_token_budget no puede pasar")
	}
}
