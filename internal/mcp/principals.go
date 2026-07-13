package mcp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

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
	hash      string // hex del SHA-256 del token (nunca el token crudo)
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
		reg.principals = append(reg.principals, Principal{
			Name:      name,
			ProjectID: projectID,
			Role:      role,
			Read:      read,
			Write:     write,
			hash:      h,
		})
	}
	return reg, nil
}

// resolve autentica un bearer contra el registro. Devuelve el principal y true si el token
// matchea una entrada; el token legacy matchea como principal admin ("legacy"). La
// comparación es en tiempo constante (no filtra por timing qué entrada matcheó).
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
