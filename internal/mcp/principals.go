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

// Principal es una identidad autenticada: quién es, sobre qué proyecto opera y con qué rol.
type Principal struct {
	Name      string
	ProjectID string
	Role      string
	hash      string // hex del SHA-256 del token (nunca el token crudo)
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
	for i, p := range parsed.Principals {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return nil, fmt.Errorf("principal #%d sin 'name' en %q", i+1, path)
		}
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
		// Tenancy fail-closed (Track 18): un reader/writer SIN project_id resolvería a scope vacío
		// ⇒ recall federado + escritura sin atribuir (fuga silenciosa entre tenants). Se exige
		// project_id no-vacío para esos roles; solo 'admin' puede ser federado (por diseño). Aplica
		// también a un YAML editado a mano (defensa en profundidad, espeja la guarda de AddPrincipal).
		projectID := strings.TrimSpace(p.ProjectID)
		if projectID == "" && role != RoleAdmin {
			return nil, fmt.Errorf("principal %q (rol %s): project_id es obligatorio para reader/writer (aislamiento multi-tenant); solo 'admin' puede ser federado", name, role)
		}
		reg.principals = append(reg.principals, Principal{
			Name:      name,
			ProjectID: projectID,
			Role:      role,
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
		return &Principal{Name: "legacy", Role: RoleAdmin}, true
	}
	return nil, false
}

// recallScopeFor deriva el scope del recall del principal autenticado (16.1c-3): un admin
// ve FEDERADO (todos los proyectos); un reader/writer con project_id queda ACOTADO a él; sin
// principal (stdio local) o sin project_id ⇒ sin scope (federado, comportamiento histórico).
// El project_id sale de la credencial, no lo auto-declara el cliente.
func recallScopeFor(p *Principal) (projectScope string, federate bool) {
	if p == nil {
		return "", false
	}
	if p.Role == RoleAdmin {
		return "", true
	}
	return p.ProjectID, false
}

// canCall decide si el principal puede invocar una tool según su rol. reader solo puede
// las de solo-lectura; writer y admin pueden todas (admin queda reservado para gatear
// operaciones destructivas en un paso posterior).
func (p *Principal) canCall(readOnly bool) bool {
	if p == nil {
		return true // stdio local (sin principal): confianza local, acceso pleno
	}
	if p.Role == RoleReader {
		return readOnly
	}
	return true
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
