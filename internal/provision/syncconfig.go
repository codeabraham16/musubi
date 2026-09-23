package provision

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"musubi/internal/config"
)

// syncBlockRe captura el bloque `sync:` de nivel superior COMPLETO (la línea `sync:` más sus líneas
// indentadas) para poder REEMPLAZARLO en su lugar. No basta con detectar `^sync:` y anexar: el config
// que deja `ensureWorkspace` (config.Default().Marshal()) YA trae un bloque `sync:` con enabled:false,
// y anexar otro crearía una clave YAML duplicada (parseo fallido). Ver auditoría 2026-07-26 #2.
var syncBlockRe = regexp.MustCompile(`(?m)^sync:.*(?:\n[ \t]+.*)*\n?`)

// memoryBlockRe captura el bloque `memory:` de nivel superior COMPLETO, y teamModeRe la línea
// `team_mode:` de adentro. Son dos regex y no una porque el bloque `memory:` NO se puede reemplazar
// entero como el de `sync:`: ahí viven otras claves del proyecto (presupuestos de recall, modo de
// brevedad) y pisarlas sería borrar configuración que nadie pidió tocar. Acá se cambia UNA línea.
//
// Los dos capturan la SANGRÍA en un grupo, y eso no es cosmético: `config.Default().Marshal()` sale
// de `yaml.Marshal`, que indenta con CUATRO espacios, mientras que un config escrito a mano usa dos.
// Insertar la clave con una sangría fija produce un YAML que no parsea —para el parser es un dedent
// a mitad del mapa— y el config del proyecto queda ilegible. La sangría se deduce del bloque.
var (
	memoryBlockRe   = regexp.MustCompile(`(?m)^memory:.*(?:\n[ \t]+.*)*\n?`)
	teamModeRe      = regexp.MustCompile(`(?m)^([ \t]+)team_mode:.*$`)
	primerHijoRe    = regexp.MustCompile(`(?m)^memory:[^\n]*\n([ \t]+)\S`)
	sangriaPorMedio = []byte("  ")
)

// asegurarTeamMode deja `memory.team_mode: true` en el config, que es la llave de la BAJADA.
//
// POR QUÉ ESTO VA EN EL ALTA. `provision` habilita el sync saliente y nada más, así que una máquina
// recién dada de alta SUBE Y NO BAJA: RunInboundScheduler se apaga solo sin team_mode, y el pull
// nunca arranca. La máquina queda con un cuaderno vacío que además empieza a mandar. Medido el
// 2026-09-23: es el motivo por el que «un empleado nuevo hereda el cerebro» no funcionaba, y no era
// una función faltante sino una clave que el único camino automático de alta jamás escribía.
//
// Se FUERZA a true en vez de respetar un false previo, por la misma razón por la que el bloque
// `sync:` pisa el `enabled: false` del default: el default escribe false sin que nadie lo haya
// elegido, y `provision` ES el acto explícito de sumar esta máquina a un cerebro compartido. Dejar
// el false del default ganaría sobre la intención de quien corre el comando.
func asegurarTeamMode(content []byte) []byte {
	bloque := memoryBlockRe.Find(content)
	if bloque == nil {
		// Sin bloque `memory:`: se agrega uno. Anexar es seguro justamente porque no hay ninguno —
		// es el caso que el comentario de syncBlockRe advierte que NO hay que asumir a ciegas.
		if len(bytes.TrimSpace(content)) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
			content = append(content, '\n')
		}
		return append(content, []byte("\n# Bajada del cerebro híbrido: lo compartido del central desciende a esta máquina.\nmemory:\n"+
			string(sangriaPorMedio)+"team_mode: true\n")...)
	}
	// La clave ya está: se reescribe su valor CONSERVANDO su propia sangría.
	if teamModeRe.Match(bloque) {
		nuevo := teamModeRe.ReplaceAll(bloque, []byte("${1}team_mode: true"))
		return bytes.Replace(content, bloque, nuevo, 1)
	}
	// El bloque existe pero sin la clave: se INSERTA como primer hijo, con la sangría que ya usan
	// sus hermanos, sin tocar el resto del bloque.
	sangria := sangriaPorMedio
	if m := primerHijoRe.FindSubmatch(bloque); m != nil {
		sangria = m[1]
	}
	corte := bytes.IndexByte(bloque, '\n') + 1 // fin de la línea `memory:`
	nuevo := make([]byte, 0, len(bloque)+len(sangria)+16)
	nuevo = append(nuevo, bloque[:corte]...)
	nuevo = append(nuevo, sangria...)
	nuevo = append(nuevo, []byte("team_mode: true\n")...)
	nuevo = append(nuevo, bloque[corte:]...)
	return bytes.Replace(content, bloque, nuevo, 1)
}

// ensureSyncConfig deja el bloque `sync:` en el .musubi/config.yaml del proyecto para que el
// daemon LOCAL suba solo la memoria `shared` al cerebro central (outbox de F2). Sin esto, el
// `.mcp.json` conecta pero el auto-sync local→central queda apagado (era el hueco que hacía
// falta un paso manual). Idempotente: si el bloque ya está, no lo pisa. Incluye
// `allow_insecure_token` SÓLO cuando la base quedó en http:// —el central sobre el tailnet, donde
// WireGuard ya cifra el transporte y el cliente de sync es fail-closed sin ese opt-in—. Con una
// base https no hace falta, y escribirlo igual dejaría puesto un permiso para volver a texto
// plano sin que nada lo note.
//
// EL ESQUEMA SALE DEL NORMALIZADOR, y hasta el 2026-09-20 estaba escrito a máquina acá. Éste era
// el CUARTO sitio: los otros tres se migraron y éste se escapó porque su literal no es
// `"http://" + brain` sino `http://%s` adentro de un formato más grande, así que no lo
// encontraba el grep que buscaba la forma. Con `--brain https://host:10000` la MISMA corrida
// dejaba un .mcp.json correcto y un config.yaml con `central_url: http://https://host:10000` —
// que `url.Parse` acepta, que `NewSyncClient` acepta por empezar con http://, y que recién
// revienta en el primer drain como error de RED, o sea transitorio: la fila vuelve a pending
// con backoff para siempre, sin dead-letter, mientras el paso se reporta `done`.
func ensureSyncConfig(projectDir, brain, tokenEnv string, dryRun bool) StepResult {
	cfgPath := filepath.Join(projectDir, ".musubi", "config.yaml")

	existing, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo leer " + cfgPath + ": " + err.Error()}
	}

	// "Ya configurado" NO es "existe un bloque sync:": el default trae uno con enabled:false. Se
	// consulta el estado REAL (parseado, con defaults aplicados): sólo si el sync ya está habilitado y
	// con central_url no vacío se lo deja en paz. Así provision funciona sobre un proyecto ya inicializado.
	if cfg, lerr := config.Load(projectDir); lerr == nil && cfg.Sync.Enabled && cfg.Sync.CentralURL != "" {
		// El sync SALIENTE ya está, pero la bajada puede seguir apagada: ése es el estado «subo y no
		// bajo», y salir acá sin mirarlo lo dejaba puesto para siempre en toda máquina que ya tuviera
		// sync. Se repara la mitad que falta y nada más.
		if cfg.Memory.TeamMode {
			return StepResult{Name: "sync-config", Status: StatusOK, Detail: "sync ya habilitado hacia " + cfg.Sync.CentralURL}
		}
		if dryRun {
			return StepResult{Name: "sync-config", Status: StatusTodo,
				Detail: "el sync sube pero NO baja: encendería memory.team_mode en " + cfgPath}
		}
		if err := os.WriteFile(cfgPath, asegurarTeamMode(existing), 0o644); err != nil {
			return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo escribir " + cfgPath + ": " + err.Error()}
		}
		return StepResult{Name: "sync-config", Status: StatusDone,
			Detail: "el sync ya subía; se encendió la BAJADA (memory.team_mode) en " + cfgPath}
	}

	base, _, ok := direccionDelCerebro(brain)
	if !ok {
		return StepResult{Name: "sync-config", Status: StatusError,
			Detail: "la dirección del cerebro no es usable y no se escribe nada: " + brain + " (se espera host:port, http://host:port o https://host:port)"}
	}
	lineaInsegura := ""
	if strings.HasPrefix(base, "http://") {
		lineaInsegura = "  allow_insecure_token: true  # http sobre el tailnet (WireGuard ya cifra el transporte)\n"
	}
	block := fmt.Sprintf("# Sync saliente del cerebro híbrido: sube solo la memoria 'shared' al cerebro central.\n"+
		"sync:\n"+
		"  enabled: true\n"+
		"  central_url: %s\n"+
		"  auth_token_env: %s\n"+
		"  drain_interval_seconds: 30\n"+
		"%s",
		base, tokenEnv, lineaInsegura)

	if dryRun {
		return StepResult{Name: "sync-config", Status: StatusTodo,
			Detail: "habilitaría el sync en los DOS sentidos (sube 'shared' al cerebro y baja lo del central) en " + cfgPath}
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo crear .musubi/: " + err.Error()}
	}

	// Si ya hay un bloque `sync:` (el deshabilitado del default), se REEMPLAZA en su lugar para no
	// duplicar la clave. Si no hay ninguno, se anexa al final.
	content := existing
	if syncBlockRe.Match(content) {
		content = syncBlockRe.ReplaceAll(content, []byte(block))
	} else {
		if len(bytes.TrimSpace(content)) == 0 {
			content = []byte("# Configuración de Musubi (bootstrap por `musubi provision`).\n")
		} else if !bytes.HasSuffix(content, []byte("\n")) {
			content = append(content, '\n')
		}
		content = append(content, '\n')
		content = append(content, []byte(block)...)
	}
	// La otra mitad del enlace: sin esto la máquina sube y no baja.
	content = asegurarTeamMode(content)
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo escribir " + cfgPath + ": " + err.Error()}
	}
	return StepResult{Name: "sync-config", Status: StatusDone,
		Detail: "sync habilitado en los DOS sentidos (sube 'shared' y baja lo del central) en " + cfgPath}
}
