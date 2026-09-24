package provision

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"musubi/internal/config"
)

// bloqueDeNivelSuperior ubica el bloque `clave:` de nivel superior del YAML: ini es el comienzo de la
// línea de cabecera, finCabecera el comienzo de la línea siguiente y fin el final del bloque (justo
// después de su último hijo). ok=false si no hay tal bloque.
//
// POR QUÉ UN RECORRIDO DE LÍNEAS Y NO UNA REGEX, que es como estaba. La regex terminaba el bloque en
// la primera línea que no arrancara con sangría, así que una LÍNEA EN BLANCO o un comentario en
// columna 0 entre dos claves del bloque lo cortaban. Una revisión adversarial lo reprodujo antes del
// merge: con `memory:\n  recall_token_budget: 400\n\n  team_mode: false`, el editor no veía el
// team_mode de abajo, insertaba otro arriba, y el config quedaba con la clave DUPLICADA —yaml.v3 lo
// rechaza— mientras el paso reportaba «hecho». Ahora las vacías y los comentarios que quedan ENTRE
// dos hijos son del bloque, y los del FINAL no: suelen ser la cabecera de la sección siguiente, y
// reemplazar el bloque `sync:` se los llevaría puestos.
func bloqueDeNivelSuperior(content []byte, clave string) (ini, finCabecera, fin int, ok bool) {
	cab := []byte(clave + ":")
	// Un BOM UTF-8 al principio —PowerShell 5.1 lo escribe con -Encoding utf8, y yaml.v3 lo acepta—
	// escondía la cabecera de la PRIMERA línea: el editor no veía ese bloque, agregaba otro al final y
	// la clave quedaba duplicada. El recorrido arranca después del BOM, así que un reemplazo lo conserva.
	inicio := 0
	if bytes.HasPrefix(content, []byte("\xEF\xBB\xBF")) {
		inicio = 3
	}
	for off := inicio; off < len(content); off += largoDeLinea(content[off:]) {
		linea := content[off : off+largoDeLinea(content[off:])]
		if !bytes.HasPrefix(linea, cab) {
			continue
		}
		// La clave tiene que terminar en los dos puntos: `sync:` no es `sync_extra:`.
		if resto := linea[len(cab):]; len(resto) > 0 && !bytes.ContainsAny(resto[:1], " \t\r\n#") {
			continue
		}
		ini, finCabecera = off, off+len(linea)
		fin = finCabecera
		for p := finCabecera; p < len(content); p += largoDeLinea(content[p:]) {
			l := content[p : p+largoDeLinea(content[p:])]
			t := bytes.TrimSpace(l)
			switch {
			case len(t) == 0 || t[0] == '#':
				// Vacía o comentario: es del bloque sólo si después aparece otro hijo.
			case l[0] == ' ' || l[0] == '\t':
				fin = p + len(l)
			default:
				return ini, finCabecera, fin, true
			}
		}
		return ini, finCabecera, fin, true
	}
	return 0, 0, 0, false
}

// largoDeLinea devuelve el largo de la primera línea de b, incluido su salto de línea.
func largoDeLinea(b []byte) int {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return i + 1
	}
	return len(b)
}

// finDeLinea devuelve el salto de línea con que termina la línea ("\r\n", "\n" o "" si es la última
// y no tiene): lo que se inserta respeta el estilo del archivo.
func finDeLinea(linea []byte) string {
	switch {
	case bytes.HasSuffix(linea, []byte("\r\n")):
		return "\r\n"
	case bytes.HasSuffix(linea, []byte("\n")):
		return "\n"
	}
	return ""
}

// asegurarTeamMode deja `memory.team_mode: true` en el config, que es la llave de la BAJADA.
//
// POR QUÉ ESTO VA EN EL ALTA. `provision` habilitaba el sync saliente y nada más, así que una
// máquina recién dada de alta SUBÍA Y NO BAJABA: RunInboundScheduler se apaga solo sin team_mode, y
// el pull nunca arrancaba. Medido el 2026-09-23: es el motivo por el que «un empleado nuevo hereda el
// cerebro» no funcionaba, y no era una función faltante sino una clave que el alta jamás escribía.
//
// El bloque `memory:` NO se reemplaza entero como el de `sync:`: ahí viven otras claves del proyecto
// y pisarlas sería borrar configuración que nadie pidió tocar. Se cambia UNA línea, con la sangría de
// sus hermanos —yaml.Marshal indenta con cuatro espacios y un config a mano con dos, y una sangría fija
// dejaba el YAML sin parsear—, salteando los comentarios al elegir el molde: un comentario con otra
// sangría como primera línea hacía que la clave quedara más honda que sus hermanas.
//
// Si `memory:` trae su valor en la misma línea (`memory: {}`, `memory: null`) no se edita a ciegas:
// se devuelve error y el que llama no escribe nada.
func asegurarTeamMode(content []byte) ([]byte, error) {
	ini, finCab, fin, ok := bloqueDeNivelSuperior(content, "memory")
	if !ok {
		out := append([]byte{}, content...)
		if len(bytes.TrimSpace(out)) > 0 && !bytes.HasSuffix(out, []byte("\n")) {
			out = append(out, '\n')
		}
		return append(out, []byte("\n# Bajada del cerebro híbrido: lo compartido del central desciende a esta máquina.\nmemory:\n  team_mode: true\n")...), nil
	}
	cabecera := content[ini:finCab]
	valor := strings.TrimSpace(strings.TrimPrefix(string(bytes.TrimSpace(cabecera)), "memory:"))
	if i := strings.Index(valor, "#"); i >= 0 {
		valor = strings.TrimSpace(valor[:i])
	}
	if valor != "" {
		return nil, fmt.Errorf("el bloque memory: trae su valor en la misma línea (%q), y eso no se edita a ciegas", valor)
	}

	// El molde de sangría es el del primer hijo REAL (no un comentario ni una vacía).
	var sangria []byte
	for p := finCab; p < fin; p += largoDeLinea(content[p:]) {
		l := content[p : p+largoDeLinea(content[p:])]
		t := bytes.TrimSpace(l)
		if len(t) == 0 || t[0] == '#' {
			continue
		}
		ind := l[:len(l)-len(bytes.TrimLeft(l, " \t"))]
		if sangria == nil {
			sangria = ind
		}
		// La clave ya está entre los hijos directos: se reescribe su valor, con su sangría y su fin de línea.
		if bytes.Equal(ind, sangria) && bytes.HasPrefix(t, []byte("team_mode:")) {
			nueva := string(ind) + "team_mode: true" + finDeLinea(l)
			return append(append(append([]byte{}, content[:p]...), nueva...), content[p+len(l):]...), nil
		}
	}
	if sangria == nil {
		sangria = []byte("  ")
	}
	eol := finDeLinea(cabecera)
	out := append([]byte{}, content[:finCab]...)
	if eol == "" { // `memory:` era la última línea y sin salto: primero el salto, después la clave
		out = append(out, '\n')
		eol = "\n"
	}
	out = append(out, sangria...)
	out = append(out, "team_mode: true"+eol...)
	return append(out, content[finCab:]...), nil
}

// validarEdicion comprueba el config editado con el MISMO parser que lo va a leer (config.Parse)
// antes de escribirlo: tiene que parsear, quedar con el sync hacia centralURL y con team_mode en
// true, y todo lo demás tiene que quedar IGUAL que antes. Un editor de texto puede equivocarse con
// un YAML que no previó; lo que no puede es escribir un config ilegible y reportar «hecho», que es
// lo que hacía antes (ver bloqueDeNivelSuperior).
//
// Un original que no parsea no se toca: no hay contra qué comparar, y pisarlo borraría la pista de
// qué estaba mal.
func validarEdicion(original, editado []byte, centralURL string) error {
	antes, err := config.Parse(original)
	if err != nil {
		return fmt.Errorf("el config actual no parsea y no lo toco: %w", err)
	}
	despues, err := config.Parse(editado)
	if err != nil {
		return fmt.Errorf("la edición dejaba el config sin parsear: %w", err)
	}
	if !despues.Sync.Enabled || despues.Sync.CentralURL != centralURL {
		return fmt.Errorf("la edición no dejaba el sync habilitado hacia %s", centralURL)
	}
	if !despues.Memory.TeamMode {
		return fmt.Errorf("la edición no dejaba memory.team_mode en true")
	}
	// Se eximen SÓLO los campos que el alta escribe, no el bloque entero. La primera versión copiaba
	// todo Sync y por eso no veía el borrado de lo que el usuario había puesto ahí: la segunda revisión
	// lo reprodujo con un `sync:` deshabilitado que traía `tls_server_name` (sin él, con la IP en la
	// URL, el handshake falla y la subida queda pending para siempre) y `flota_vivo: false` (sin él,
	// la telemetría se ENCIENDE contra un opt-out explícito), y el paso reportaba «hecho».
	antes.Sync.Enabled = despues.Sync.Enabled
	antes.Sync.CentralURL = despues.Sync.CentralURL
	antes.Sync.AuthTokenEnv = despues.Sync.AuthTokenEnv
	antes.Sync.DrainIntervalSeconds = despues.Sync.DrainIntervalSeconds
	antes.Sync.AllowInsecureToken = despues.Sync.AllowInsecureToken
	antes.Memory.TeamMode = despues.Memory.TeamMode
	if !reflect.DeepEqual(antes, despues) {
		return fmt.Errorf("la edición cambiaba algo más que lo que el alta escribe (enabled, central_url, auth_token_env, drain_interval_seconds, allow_insecure_token y memory.team_mode)")
	}
	return nil
}

// clavesQueEscribeElAlta son los hijos de `sync:` que el alta pone; los demás son del usuario.
var clavesQueEscribeElAlta = []string{"enabled", "central_url", "auth_token_env", "drain_interval_seconds", "allow_insecure_token"}

// bloqueSyncConservando arma el bloque `sync:` nuevo a partir del existente: las líneas que el alta
// escribe, más TODOS los hijos del bloque viejo que no son esas claves —`tls_server_name`,
// `flota_vivo`, `batch_size`, comentarios—, con la sangría y el fin de línea del archivo.
//
// POR QUÉ NO SE REEMPLAZA ENTERO. Un `sync:` deshabilitado no es siempre el default del init: puede
// traer ajustes del usuario, y reemplazarlo los borraba. `tls_server_name` es el que deja discar la
// IP del tailnet —sin él el handshake falla y la subida queda pending para siempre—, y
// `flota_vivo: false` es un opt-out de telemetría que el reemplazo daba vuelta en silencio.
func bloqueSyncConservando(cabecera, hijos []byte, lineas []string) []byte {
	eol := finDeLinea(cabecera)
	if eol == "" {
		eol = "\n"
	}
	var sangria []byte
	for p := 0; p < len(hijos); p += largoDeLinea(hijos[p:]) {
		l := hijos[p : p+largoDeLinea(hijos[p:])]
		if t := bytes.TrimSpace(l); len(t) > 0 && t[0] != '#' {
			sangria = l[:len(l)-len(bytes.TrimLeft(l, " \t"))]
			break
		}
	}
	if sangria == nil {
		sangria = []byte("  ")
	}
	out := []byte("# Sync saliente del cerebro híbrido: sube solo la memoria 'shared' al cerebro central." + eol + "sync:" + eol)
	for _, l := range lineas {
		out = append(append(append(out, sangria...), l...), eol...)
	}
	for p := 0; p < len(hijos); p += largoDeLinea(hijos[p:]) {
		l := hijos[p : p+largoDeLinea(hijos[p:])]
		ind := l[:len(l)-len(bytes.TrimLeft(l, " \t"))]
		t := bytes.TrimSpace(l)
		if bytes.Equal(ind, sangria) && esClaveDelAlta(t) {
			continue
		}
		if !bytes.HasSuffix(l, []byte("\n")) {
			l = append(append([]byte{}, l...), eol...)
		}
		out = append(out, l...)
	}
	return out
}

// esClaveDelAlta dice si una línea (sin sangría) es uno de los hijos que el alta escribe.
func esClaveDelAlta(t []byte) bool {
	for _, c := range clavesQueEscribeElAlta {
		if bytes.HasPrefix(t, []byte(c+":")) {
			return true
		}
	}
	return false
}

// ensureSyncConfig deja el sync del .musubi/config.yaml del proyecto en los DOS sentidos: el bloque
// `sync:` para que el daemon LOCAL suba la memoria `shared` al cerebro central (outbox de F2), y
// `memory.team_mode` para que baje lo del central (ver asegurarTeamMode). Idempotente. Incluye
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
	// consulta el estado REAL (parseado, con defaults aplicados): si el sync ya está habilitado y con
	// central_url, el archivo NO SE TOCA.
	//
	// Y si sube pero no baja, SE DICE en vez de repararlo. La primera versión lo reparaba solo, y la
	// revisión adversarial mostró dos costos: en máquinas que ya existían, este camino —que antes jamás
	// escribía el archivo— pasaba a reescribirlo; y un `team_mode: false` en una máquina con el sync ya
	// armado puede ser una DECISIÓN —una consola de alcance acotado, como la de otra persona, que sube
	// sólo lo que promueve a mano—, y darlo vuelta cambia el scope por defecto de todo lo que se guarde
	// a `shared`, o sea al central, sin que nadie lo haya pedido.
	if cfg, lerr := config.Load(projectDir); lerr == nil && cfg.Sync.Enabled && cfg.Sync.CentralURL != "" {
		if cfg.Memory.TeamMode {
			return StepResult{Name: "sync-config", Status: StatusOK, Detail: "sync ya habilitado en los dos sentidos hacia " + cfg.Sync.CentralURL}
		}
		return StepResult{Name: "sync-config", Status: StatusTodo,
			Detail: "el sync sube hacia " + cfg.Sync.CentralURL + " pero NO BAJA: memory.team_mode está en false en " + cfgPath +
				". Si esta máquina tiene que heredar el cerebro, ponelo en true y reabrí las sesiones abiertas. Ojo: eso también hace que lo que se guarde sin scope nazca 'shared' y suba al central. No lo cambio solo porque en una máquina ya configurada puede ser una decisión."}
	}

	base, _, ok := direccionDelCerebro(brain)
	if !ok {
		return StepResult{Name: "sync-config", Status: StatusError,
			Detail: "la dirección del cerebro no es usable y no se escribe nada: " + brain + " (se espera host:port, http://host:port o https://host:port)"}
	}
	lineas := []string{"enabled: true", "central_url: " + base, "auth_token_env: " + tokenEnv, "drain_interval_seconds: 30"}
	if strings.HasPrefix(base, "http://") {
		lineas = append(lineas, "allow_insecure_token: true  # http sobre el tailnet (WireGuard ya cifra el transporte)")
	}
	block := "# Sync saliente del cerebro híbrido: sube solo la memoria 'shared' al cerebro central.\nsync:\n"
	for _, l := range lineas {
		block += "  " + l + "\n"
	}

	// Si ya hay un bloque `sync:` (el deshabilitado del default), se REEMPLAZA en su lugar para no
	// duplicar la clave, CONSERVANDO los hijos que el alta no escribe (ver bloqueSyncConservando). Si
	// no hay ninguno, se anexa al final.
	content := append([]byte{}, existing...)
	if ini, finCab, fin, ok := bloqueDeNivelSuperior(content, "sync"); ok {
		nuevo := bloqueSyncConservando(content[ini:finCab], content[finCab:fin], lineas)
		content = append(append(append([]byte{}, content[:ini]...), nuevo...), content[fin:]...)
	} else {
		if len(bytes.TrimSpace(content)) == 0 {
			content = []byte("# Configuración de Musubi (bootstrap por `musubi provision`).\n")
		} else if !bytes.HasSuffix(content, []byte("\n")) {
			content = append(content, '\n')
		}
		content = append(content, '\n')
		content = append(content, []byte(block)...)
	}
	manual := " No se escribió nada. Para hacerlo a mano en " + cfgPath + ": en `sync:` poné `enabled: true` y `central_url: " + base +
		"`, y en `memory:` poné `team_mode: true`."
	content, err = asegurarTeamMode(content)
	if err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: err.Error() + "." + manual}
	}
	if err := validarEdicion(existing, content, base); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: err.Error() + "." + manual}
	}

	// El dry-run va DESPUÉS de armar y validar la edición, que es todo en memoria: antes prometía
	// «habilitaría el sync» para configs que la corrida real rechazaba (lo encontró la revisión).
	if dryRun {
		return StepResult{Name: "sync-config", Status: StatusTodo,
			Detail: "habilitaría el sync en los DOS sentidos (sube 'shared' al cerebro y baja lo del central) en " + cfgPath}
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo crear .musubi/: " + err.Error()}
	}
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo escribir " + cfgPath + ": " + err.Error()}
	}
	return StepResult{Name: "sync-config", Status: StatusDone,
		Detail: "sync habilitado en los DOS sentidos (sube 'shared' y baja lo del central) en " + cfgPath +
			". Desde ahora lo que se guarde sin scope nace 'shared' y sube al central. Si hay sesiones abiertas, reabrilas para que la bajada arranque."}
}
