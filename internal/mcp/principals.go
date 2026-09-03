package mcp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"musubi/internal/fleet"

	"gopkg.in/yaml.v3"
)

// principals.go implementa la IDENTIDAD por-principal del cerebro central (Track 16 F1
// 16.1c): un registro de tokens en archivo mapea cada bearer a un principal con proyecto
// y rol. Reemplaza el "un solo token para todo el equipo" por credenciales por-miembro
// revocables (borrás la línea y recargás) con autorización por rol. El archivo guarda el
// SHA-256 del token, NO el token crudo: un leak del registro no entrega credenciales
// usables. Solo aplica en modo `serve` (HTTP); el daemon stdio local no tiene principal
// (agente local confiable → acceso pleno). Sin archivo de registro, el comportamiento es
// el histórico (un único bearer legacy).

// Roles de un principal, en orden de privilegio: reader < writer < admin.
const (
	RoleReader = "reader" // solo tools de lectura
	RoleWriter = "writer" // lectura + tools que mutan
	RoleAdmin  = "admin"  // todo (reservado para operaciones destructivas/mantenimiento)
)

// ALCANCE y AUTORIDAD son EJES INDEPENDIENTES, y el `role` los tenía colapsados en un enum.
//
// El enum sabe expresar "ve lo suyo y escribe lo suyo" (writer) y "ve todo y escribe en todos"
// (admin) — pero NO sabe expresar las dos identidades que un cerebro central de verdad necesita:
//
//   - SALA DE MANDO (el repo de Musubi): VE TODO —para diagnosticar los demás proyectos— pero
//     ESCRIBE SÓLO EN LO SUYO. Con el enum había que darle `admin`, que además lo deja escribir
//     dentro de la memoria de producción de cualquier otro proyecto.
//   - CABINA (el CRM, el gateway): VE TODO y NO ESCRIBE NADA. Con el enum, `reader` no ve más que
//     su propio tenant y `admin` puede escribir en todos: no había término medio.
//
// Separarlos también cierra una fuga real: un `admin` que escribe SIN declarar project_id deja la
// fila SIN ATRIBUIR, y una fila sin atribuir es visible desde TODOS los tenants (ver el filtro de
// recall). Medido en el cerebro real: 2 filas de test contaminando los 3 proyectos.
const (
	ReadOwn = "own" // ve sólo su propio proyecto
	ReadAll = "all" // ve TODOS los proyectos (federado): diagnóstico, cabina, sala de mando

	WriteNone = "none" // no muta nada (cabina de sólo lectura)
	WriteOwn  = "own"  // muta SÓLO su proyecto: la atribución la fija la credencial, no el cliente
	WriteAny  = "any"  // muta cualquier proyecto DECLARÁNDOLO (mantenimiento/reparación)
)

// Principal es una identidad autenticada: quién es, sobre qué proyecto opera, qué VE y qué ESCRIBE.
type Principal struct {
	Name      string
	ProjectID string
	Role      string // conservado para logs y compat; el comportamiento lo deciden Read/Write
	Read      string // ReadOwn | ReadAll
	Write     string // WriteNone | WriteOwn | WriteAny
	// Fleet es el TERCER EJE (track «Control de flota», S3): qué puede pedirle esta persona a
	// qué MÁQUINAS. Read/Write hablan de la memoria y no saben decir «mira las métricas de las
	// 40, ejecuta en tres, no abre la pantalla de ninguna».
	//
	// Es un mapa capacidad -> selectores de máquina (nombres, o el comodín "*"). NIL O AUSENTE
	// SIGNIFICA NINGUNA CAPACIDAD SOBRE NINGUNA MÁQUINA — nunca "todas". Esa asimetría es
	// deliberada y es la valla del track: un admin de la memoria no se convierte, de rebote, en
	// root de la flota. Ver fleet_authz.go.
	Fleet map[fleet.Cap][]string
	// ExecAllow ACOTA la capacidad `exec` a una lista de comandos, por máquina (S10, I7-I10).
	// Mapa: selector de máquina (nombre exacto, o "*") -> comandos permitidos (argv[0] exacto).
	//
	// NO OTORGA NADA. Se evalúa DESPUÉS de la compuerta de tres lados, jamás en su lugar: nadie
	// gana acceso por figurar acá. Y vive en la CREDENCIAL y no en el dispositivo a propósito —
	// un techo declarado por la máquina lo declara la máquina que se supone acotada, así que una
	// máquina comprometida se auto-otorgaría todo y el control valdría cero.
	//
	// NIL ⇒ SIN RESTRICCIÓN: `exec` sigue significando exactamente lo que significaba antes de
	// que esta función existiera, así que estrenarla no le rompe la configuración a nadie. Pero
	// NO-NIL ⇒ EXHAUSTIVA: una máquina sin entrada y sin "*" de respaldo no permite nada. La
	// sección entera es el opt-in; una vez adentro, no hay huecos silenciosos.
	ExecAllow map[string][]string
	// Expires es cuándo deja de valer esta credencial. CERO ⇒ no vence.
	//
	// Un token que vale para siempre es lo que un auditor mira primero (SOC2 CC6.1, ISO A.5.17):
	// una credencial filtrada hace tres meses sigue abriendo la puerta hoy, y la única forma de
	// cerrarla es que alguien se acuerde de borrar la línea a mano.
	Expires time.Time
	hash    string // hex del SHA-256 del token (nunca el token crudo)
}

// capsFromRole traduce el rol histórico al par (alcance, autoridad). Es la tabla de
// backward-compat: todo principals.yaml existente sigue significando exactamente lo mismo.
func capsFromRole(role string) (read, write string) {
	switch role {
	case RoleReader:
		return ReadOwn, WriteNone
	case RoleAdmin:
		return ReadAll, WriteAny
	default: // RoleWriter
		return ReadOwn, WriteOwn
	}
}

// caps devuelve las capacidades EFECTIVAS del principal: las declaradas, y si no, las del rol.
//
// El fallback NO es cosmético: el cero de un string es "", así que un Principal construido a mano
// —en un test, o en cualquier código que no pase por el registro— tendría Read/Write vacíos y
// caería en un comportamiento accidental (un reader podría mutar; un admin dejaría de ser
// federado). Con el fallback, un Principal sin capacidades declaradas se comporta EXACTAMENTE como
// dice su rol. Nunca leer p.Read / p.Write directo: leerlos por acá.
func (p *Principal) caps() (read, write string) {
	r, w := capsFromRole(p.Role)
	if p.Read != "" {
		r = p.Read
	}
	if p.Write != "" {
		w = p.Write
	}
	return r, w
}

// PrincipalRegistry es el conjunto de principals cargado del archivo de registro.
type PrincipalRegistry struct {
	principals []Principal
	legacyHash string // SHA-256 del MUSUBI_TOKEN legacy (si hay); actúa como admin federado
}

type principalEntry struct {
	Name        string `yaml:"name"`
	TokenSHA256 string `yaml:"token_sha256"`
	ProjectID   string `yaml:"project_id"`
	Role        string `yaml:"role"`
	// read/write son OPCIONALES: ausentes ⇒ se derivan del rol (capsFromRole). Presentes,
	// MANDAN sobre el rol — es la vía para expresar las identidades que el enum no sabía decir
	// (sala de mando: read=all + write=own; cabina: read=all + write=none).
	Read  string `yaml:"read,omitempty"`
	Write string `yaml:"write,omitempty"`
	// Fleet: capacidad -> lista de máquinas. Opcional; AUSENTE = ninguna capacidad de flota.
	// Los valores son nombres de dispositivo o el comodín "*". Se escribe como lista siempre
	// (`exec: ["*"]`), no como escalar: un tipo por campo evita el YAML que a veces es string y
	// a veces lista, que es de donde salen los parseos frágiles.
	Fleet map[string][]string `yaml:"fleet,omitempty"`
	// ExecAllow acota `exec` a ciertos comandos por máquina. Opcional; AUSENTE = sin restricción
	// (ver el campo homónimo de Principal para por qué la ausencia acá SÍ significa "todo",
	// al revés que en `fleet:`). La clave es un nombre de máquina o "*"; el valor, los argv[0]
	// permitidos, exactos.
	//
	//	fleet_exec_allow:
	//	  nas-casa: ["systemctl", "journalctl"]
	//	  "*":      ["uptime", "df"]
	ExecAllow map[string][]string `yaml:"fleet_exec_allow,omitempty"`
	// Expires es la fecha de vencimiento de la credencial, en RFC3339 (`2026-12-31T23:59:59Z`).
	// Opcional; AUSENTE ⇒ no vence, que es como se comportaba todo el registro hasta hoy y por
	// eso ningún principals.yaml existente cambia de significado al estrenar esto.
	//
	// Es texto y no time.Time acá a propósito: una fecha ilegible tiene que poder DISTINGUIRSE
	// de una ausente, y `yaml` con un time.Time convierte «basura» en «cero» sin decir nada —
	// o sea, convierte un error de configuración en una credencial eterna.
	Expires string `yaml:"expires,omitempty"`
}

type principalsFileYAML struct {
	Principals []principalEntry `yaml:"principals"`
}

// hashToken devuelve el SHA-256 hex de un token. Lo usan el registro y (a futuro) el CLI
// que genera credenciales, para guardar/comparar el hash en vez del token crudo.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// loadPrincipals lee el archivo de registro. Si el archivo NO existe devuelve (nil, nil):
// modo legacy (un único bearer), sin error. Un archivo presente pero malformado SÍ es error
// (fail-closed: no arrancar con un registro roto). legacyToken, si no es vacío, se admite
// además como principal admin (backward-compat con el MUSUBI_TOKEN de una sola clave).
func loadPrincipals(path, legacyToken string) (*PrincipalRegistry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if legacyToken == "" {
			return nil, nil
		}
		return &PrincipalRegistry{legacyHash: hashToken(legacyToken)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error al leer el registro de principals %q: %w", path, err)
	}
	var parsed principalsFileYAML
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("registro de principals %q malformado: %w", path, err)
	}
	reg := &PrincipalRegistry{}
	if legacyToken != "" {
		reg.legacyHash = hashToken(legacyToken)
	}
	seen := make(map[string]bool)
	seenNames := make(map[string]bool)
	for i, p := range parsed.Principals {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return nil, fmt.Errorf("principal #%d sin 'name' en %q", i+1, path)
		}
		// Unicidad de nombres (Track 18): el nombre es la CLAVE de la cuota por-principal y la
		// identidad en logs/atribución; dos principals homónimos compartirían bucket de cuota y
		// serían ambiguos. Case-insensitive, coherente con el rechazo de duplicados de AddPrincipal.
		lname := strings.ToLower(name)
		if seenNames[lname] {
			return nil, fmt.Errorf("principal %q: nombre duplicado en %q (los nombres deben ser únicos)", name, path)
		}
		seenNames[lname] = true
		h := strings.ToLower(strings.TrimSpace(p.TokenSHA256))
		if len(h) != 64 || !isHex(h) {
			return nil, fmt.Errorf("principal %q: token_sha256 debe ser 64 hex (el SHA-256 del token), no %q", name, p.TokenSHA256)
		}
		if seen[h] {
			return nil, fmt.Errorf("principal %q: token_sha256 duplicado", name)
		}
		seen[h] = true
		role := strings.ToLower(strings.TrimSpace(p.Role))
		switch role {
		case RoleReader, RoleWriter, RoleAdmin:
		case "":
			return nil, fmt.Errorf("principal %q sin 'role' (usá reader|writer|admin)", name)
		default:
			return nil, fmt.Errorf("principal %q: role inválido %q (usá reader|writer|admin)", name, role)
		}
		// Alcance y autoridad: por default se derivan del rol (compat total con los registros
		// existentes); si el YAML los declara, MANDAN sobre el rol.
		read, write := capsFromRole(role)
		if v := strings.ToLower(strings.TrimSpace(p.Read)); v != "" {
			switch v {
			case ReadOwn, ReadAll:
				read = v
			default:
				return nil, fmt.Errorf("principal %q: read inválido %q (usá own|all)", name, v)
			}
		}
		if v := strings.ToLower(strings.TrimSpace(p.Write)); v != "" {
			switch v {
			case WriteNone, WriteOwn, WriteAny:
				write = v
			default:
				return nil, fmt.Errorf("principal %q: write inválido %q (usá none|own|any)", name, v)
			}
		}

		// Tenancy fail-closed (Track 18, ahora expresado sobre los EJES y no sobre el rol): un
		// principal SIN project_id que escriba en "lo suyo" no tiene "lo suyo" ⇒ su escritura caería
		// SIN ATRIBUIR, y una fila sin atribuir la ven TODOS los tenants. Sólo puede no tener
		// proyecto quien no escribe (cabina) o quien declara el proyecto en cada escritura (any).
		projectID := strings.TrimSpace(p.ProjectID)
		if projectID == "" && write == WriteOwn {
			return nil, fmt.Errorf("principal %q: project_id es obligatorio cuando write=own (sin proyecto, su escritura caería sin atribuir y la verían todos los tenants)", name)
		}
		// Un principal ACOTADO a lo suyo (read=own) sin proyecto vería... todo. Fail-closed.
		if projectID == "" && read == ReadOwn {
			return nil, fmt.Errorf("principal %q: project_id es obligatorio cuando read=own (sin proyecto, el recall no tiene a qué acotarse y vería todos los proyectos)", name)
		}
		// Tercer eje (S3): capacidades de flota. Validación estricta acá, en el borde, para que
		// hacia adentro sólo circulen capacidades conocidas — un `fleet: {root: ["*"]}` mal
		// escrito tiene que ser un error de arranque, no un permiso que silenciosamente no
		// aplica (o peor, uno que alguien cree que aplica).
		grants, err := parsearFleet(name, p.Fleet)
		if err != nil {
			return nil, err
		}
		permitidos, err := parsearExecAllow(name, p.ExecAllow)
		if err != nil {
			return nil, err
		}
		// UNA FECHA ILEGIBLE ES UN ERROR DE ARRANQUE, no un principal sin vencimiento.
		//
		// Es la misma regla fail-closed que gobierna el resto de este archivo, y acá importa más
		// que en ningún otro campo: un `expires: 2026-13-45` que se ignorara en silencio
		// produciría exactamente lo contrario de lo que quiso quien lo escribió — una credencial
		// ETERNA donde alguien pidió una que venza. El modo de falla de un typo tiene que ser que
		// el cerebro no arranque, no que la puerta quede abierta para siempre.
		vence, err := parsearVencimiento(name, p.Expires)
		if err != nil {
			return nil, err
		}
		reg.principals = append(reg.principals, Principal{
			Name:      name,
			ProjectID: projectID,
			Role:      role,
			Read:      read,
			Write:     write,
			Fleet:     grants,
			ExecAllow: permitidos,
			Expires:   vence,
			hash:      h,
		})
	}
	return reg, nil
}

// parsearFleet valida la sección `fleet:` de un principal y la lleva al dominio.
//
// FAIL-CLOSED EN LOS DOS SENTIDOS, y conviene ver que son distintos:
//   - Sección ausente o vacía ⇒ mapa nil ⇒ NINGUNA capacidad. La ausencia nunca significa "todas".
//   - Capacidad desconocida ⇒ ERROR DE ARRANQUE, no se ignora. Un `root: ["*"]` mal escrito que
//     se descartara en silencio deja a alguien creyendo que otorgó algo. Preferible que el
//     servidor se niegue a arrancar con un registro que no significa lo que su autor cree.
//
// Una capacidad declarada con lista VACÍA también es error: `exec: []` se lee como una intención
// a medio escribir, y adivinar cuál era no es tarea del parser.
func parsearFleet(nombrePrincipal string, raw map[string][]string) (map[fleet.Cap][]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[fleet.Cap][]string, len(raw))
	for clave, maquinas := range raw {
		caps, err := fleet.NormalizarCaps([]string{clave})
		if err != nil || len(caps) != 1 {
			return nil, fmt.Errorf("principal %q: capacidad de flota inválida %q (usá metrics, exec o screen)", nombrePrincipal, clave)
		}
		limpias := make([]string, 0, len(maquinas))
		for _, m := range maquinas {
			if m = strings.TrimSpace(m); m != "" {
				limpias = append(limpias, m)
			}
		}
		if len(limpias) == 0 {
			return nil, fmt.Errorf("principal %q: la capacidad %q no nombra ninguna máquina (usá [\"*\"] para todas, o quitá la línea para no otorgarla)", nombrePrincipal, clave)
		}
		out[caps[0]] = limpias
	}
	return out, nil
}

// parsearExecAllow valida la sección `fleet_exec_allow:` y la lleva al dominio.
//
// LA ASIMETRÍA CON parsearFleet ES DELIBERADA Y CONVIENE VERLA DE FRENTE. Allá la ausencia
// significa NINGUNA capacidad; acá significa NINGUNA restricción. No es una inconsistencia: son
// dos cosas distintas. `fleet:` OTORGA —y lo que otorga tiene que escribirse—; esto RECORTA algo
// que ya fue otorgado. Un recorte que se aplicara por default rompería, el día que se estrena, la
// configuración de todo el que ya tenía `exec` andando.
//
// Lo que sí es igual de estricto es todo lo demás, porque ahí sí se pierden cosas en silencio:
//   - Lista VACÍA (`nas: []`) ⇒ CERO COMANDOS. Nunca "todos". Es el bug clásico de las
//     allowlists, el `len == 0 ⇒ pasa todo` que parece defensivo y es exactamente lo contrario.
//   - Una vez que la sección existe, es EXHAUSTIVA: una máquina sin entrada y sin "*" no permite
//     nada (lo aplica argvPermitido, no este parser).
func parsearExecAllow(nombrePrincipal string, raw map[string][]string) (map[string][]string, error) {
	if len(raw) == 0 {
		return nil, nil // sin sección ⇒ sin restricción
	}
	out := make(map[string][]string, len(raw))
	for maquina, comandos := range raw {
		m := strings.TrimSpace(maquina)
		if m == "" {
			return nil, fmt.Errorf("principal %q: fleet_exec_allow tiene una clave vacía (usá el nombre de una máquina, o \"*\")", nombrePrincipal)
		}
		limpios := make([]string, 0, len(comandos))
		for _, c := range comandos {
			if c = strings.TrimSpace(c); c != "" {
				limpios = append(limpios, c)
			}
		}
		// Una lista vacía es LEGAL y significa cero comandos: es cómo se apaga `exec` sobre una
		// máquina puntual sin sacarla de la concesión. Se acepta y NO se descarta la clave —
		// descartarla la volvería "sin entrada", que con "*" presente significaría otra cosa.
		out[m] = limpios
	}
	return out, nil
}

// avisosDeInterpretes devuelve las líneas de advertencia para allowlists que contienen un
// intérprete. NO es un error: `bash` en una allowlist puede ser justo lo que alguien quiso. Pero
// una allowlist se escribe una vez y se lee dentro de dos años, y para entonces `["sh"]` se lee
// como una restricción cuando en realidad no restringe nada (I10b).
//
// Devuelve strings en vez de logear para que sea una función pura y se pueda probar.
func avisosDeInterpretes(p Principal) []string {
	var avisos []string
	for maquina, comandos := range p.ExecAllow {
		for _, c := range comandos {
			if fleet.EsInterprete(c) {
				avisos = append(avisos, fmt.Sprintf(
					"principal %q, máquina %q: la allowlist permite %q, que puede LANZAR cualquier otro comando — esa entrada no restringe nada",
					p.Name, maquina, c))
			}
		}
	}
	sort.Strings(avisos) // el recorrido de un mapa es aleatorio; un log que cambia de orden en cada arranque no se puede diffear
	return avisos
}

// porNombre busca un principal por su nombre declarado. Lo usan las POLÍTICAS de flota (S10),
// que no presentan un token: nombran a alguien de principals.yaml y actúan con su autoridad.
//
// Devuelve una COPIA. Sin ella, quien la reciba tendría un puntero al snapshot vigente del
// registro, y el registro se recarga en caliente cada 10 s: una política podría quedarse
// evaluando contra una credencial que ya se revocó. Con copia, cada evaluación resuelve de nuevo.
// impactoDeNombre recorre el registro buscando quién NOMBRA esta máquina (A64).
//
// NO cuenta el comodín `*`: una concesión sobre todas las máquinas sobrevive a cualquier rename y
// listarla sería ruido que tapa lo que sí importa. Lo que importa es exactamente lo que se rompe:
// las que nombran a ESTA.
func (r *PrincipalRegistry) impactoDeNombre(device string) ImpactoDeNombre {
	var imp ImpactoDeNombre
	if r == nil || strings.TrimSpace(device) == "" {
		return imp
	}
	for i := range r.principals {
		p := &r.principals[i]
		for _, selectores := range p.Fleet {
			for _, sel := range selectores {
				if sel == device {
					imp.Concesiones = append(imp.Concesiones, p.Name)
					goto allow
				}
			}
		}
	allow:
		if _, hay := p.ExecAllow[device]; hay {
			imp.Allowlists = append(imp.Allowlists, p.Name)
		}
	}
	return imp
}

func (r *PrincipalRegistry) porNombre(nombre string) (*Principal, bool) {
	for i := range r.principals {
		if r.principals[i].Name == nombre {
			cp := r.principals[i]
			return &cp, true
		}
	}
	return nil, false
}

// resolve autentica un bearer contra el registro. Devuelve el principal y true si el token
// matchea una entrada; el token legacy matchea como principal admin ("legacy"). La
// comparación es en tiempo constante (no filtra por timing qué entrada matcheó).
// parsearVencimiento lleva el `expires:` del YAML al dominio. Vacío ⇒ cero ⇒ no vence.
func parsearVencimiento(nombre, valor string) (time.Time, error) {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, valor)
	if err != nil {
		return time.Time{}, fmt.Errorf("principal %q: `expires: %s` no es una fecha RFC3339 (ej: 2026-12-31T23:59:59Z): %w", nombre, valor, err)
	}
	return t, nil
}

// ahoraParaVencimiento es el reloj del registro, como variable para que las pruebas puedan fijarlo.
// Una prueba de vencimiento contra el reloj real o duerme, o depende de fechas escritas a mano que
// caducan — y una prueba que empieza a fallar sola en 2027 se borra en vez de arreglarse.
var ahoraParaVencimiento = time.Now

// Vencida dice si la credencial ya no vale. Cero ⇒ nunca vence.
func (p Principal) Vencida(ahora time.Time) bool {
	return !p.Expires.IsZero() && !ahora.Before(p.Expires)
}

func (r *PrincipalRegistry) resolve(token string) (*Principal, bool) {
	if token == "" {
		return nil, false
	}
	h := hashToken(token)
	var match *Principal
	for i := range r.principals {
		if subtle.ConstantTimeCompare([]byte(h), []byte(r.principals[i].hash)) == 1 {
			match = &r.principals[i]
		}
	}
	if match != nil {
		// EL VENCIMIENTO SE MIRA ACÁ Y NO EN LA COMPUERTA: acá es donde una credencial deja de
		// ser una identidad. Mirarlo más arriba dejaría a cada llamador con la obligación de
		// acordarse, y el que se olvide no falla — pasa.
		//
		// Devuelve el MISMO (nil, false) que un token desconocido, a propósito: distinguir
		// «vencido» de «no existe» le diría a quien prueba tokens que ése existió alguna vez.
		if match.Vencida(ahoraParaVencimiento()) {
			return nil, false
		}
		return match, true
	}
	if r.legacyHash != "" && subtle.ConstantTimeCompare([]byte(h), []byte(r.legacyHash)) == 1 {
		read, write := capsFromRole(RoleAdmin)
		return &Principal{Name: "legacy", Role: RoleAdmin, Read: read, Write: write}, true
	}
	return nil, false
}

// recallScopeFor deriva el ALCANCE del recall del principal: read=all ⇒ FEDERADO (ve todos los
// proyectos: sala de mando, cabina, diagnóstico); read=own ⇒ ACOTADO a su project_id. Sin
// principal (stdio local) ⇒ sin scope (federado, comportamiento histórico de confianza local).
// El project_id sale de la CREDENCIAL, no lo auto-declara el cliente.
func recallScopeFor(p *Principal) (projectScope string, federate bool) {
	if p == nil {
		return "", false
	}
	if read, _ := p.caps(); read == ReadAll {
		return "", true
	}
	return p.ProjectID, false
}

// canCall decide si el principal puede invocar una tool según su AUTORIDAD (no su alcance): quien
// no escribe (write=none) sólo puede tools de lectura, VEA LO QUE VEA. Eso es exactamente la
// cabina: el CRM y el gateway ven los 3 proyectos y no pueden mutar ninguno.
func (p *Principal) canCall(readOnly bool) bool {
	if p == nil {
		return true // stdio local (sin principal): confianza local, acceso pleno
	}
	if _, write := p.caps(); write == WriteNone {
		return readOnly
	}
	return true
}

// isAdmin indica si el principal puede correr operaciones DESTRUCTIVAS/GLOBALES (mantenimiento,
// reparación con apply): sólo el rol admin, que la doc reserva justo para eso. Sin principal (stdio
// local) ⇒ true (confianza local). Un write=own/any NO es admin: sin esta guarda cualquier writer
// podía disparar maintain/doctor-repair sobre TODA la base multi-tenant (auditoría 2026-07-26 #8).
func (p *Principal) isAdmin() bool {
	return p == nil || p.Role == RoleAdmin
}

// writeOriginFor decide a QUÉ PROYECTO se atribuye una escritura, dado lo que declaró el cliente.
// Es la guarda de atribución, y es fail-closed:
//
//   - write=own  ⇒ SIEMPRE su propio proyecto. Se IGNORA lo que declare el cliente: un principal
//     acotado no puede plantar memoria en el tenant de otro (ni dejarla sin atribuir).
//   - write=any  ⇒ el proyecto DECLARADO. Si no declara ninguno, cae al suyo; si tampoco tiene,
//     es un error: una escritura sin atribuir la ven TODOS los tenants, y eso ya contaminó el
//     cerebro real con 2 filas visibles desde los 3 proyectos. Antes esto pasaba en silencio.
//
// ok=false ⇒ rechazar la escritura (el caller responde -32001).
func writeOriginFor(p *Principal, declared string) (origin string, ok bool) {
	if p == nil {
		return declared, true // stdio local: confianza local, el engine estampa su project_id
	}
	if _, write := p.caps(); write != WriteAny {
		return p.ProjectID, true
	}
	if declared = strings.TrimSpace(declared); declared != "" {
		return declared, true
	}
	if p.ProjectID != "" {
		return p.ProjectID, true
	}
	return "", false
}

// isHex reporta si s es solo dígitos hexadecimales.
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return len(s) > 0
}

// --- principal en el contexto del request ---

type principalCtxKey struct{}

// withPrincipal adjunta el principal autenticado al contexto del request (lo setea el
// transporte HTTP tras autenticar; el dispatch lo lee para autorizar).
func withPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// principalFrom devuelve el principal del contexto, o nil si no hay (p.ej. stdio local).
func principalFrom(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalCtxKey{}).(*Principal)
	return p
}

// authorFrom deriva la atribución por PERSONA (C5.1) de la credencial: el nombre del principal,
// salvo nil (stdio/local sin auth) o el admin legacy (Name=="legacy", token único sin identidad de
// persona) ⇒ "". Un admin NOMBRADO conserva su nombre. Nunca se toma del cliente: es autoridad
// server-side, así que un payload entrante no puede falsificar el autor (sellado en el central).
func authorFrom(p *Principal) string {
	if p == nil || p.Name == "legacy" {
		return ""
	}
	return p.Name
}
