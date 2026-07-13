package mcp

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// principals_admin.go administra el archivo de registro de principals (alta/baja/listado):
// lo consume el CLI `musubi token`. Genera el token, guarda su SHA-256 (nunca el crudo) y
// mantiene el YAML. Separado de principals.go (que solo LEE el registro para autenticar).

// PrincipalInfo es la vista pública de un principal (sin el hash del token), para listar.
type PrincipalInfo struct {
	Name      string
	ProjectID string
	Role      string
	// Read/Write son las capacidades EFECTIVAS (ya resueltas del rol si el registro no las
	// declara): sin esto, un `token list` no distingue una cabina de un reader normal.
	Read  string
	Write string
}

// GenerateToken produce un token opaco aleatorio (256 bits) con prefijo "msb_". Es el
// secreto que se le entrega al miembro; el registro solo guarda su SHA-256.
func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("no se pudo generar el token: %w", err)
	}
	return "msb_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

// validRole normaliza y valida un rol.
func validRole(role string) (string, error) {
	r := strings.ToLower(strings.TrimSpace(role))
	switch r {
	case RoleReader, RoleWriter, RoleAdmin:
		return r, nil
	default:
		return "", fmt.Errorf("role inválido %q (usá reader|writer|admin)", role)
	}
}

// readPrincipalsFile lee el YAML crudo del registro (o vacío si no existe).
func readPrincipalsFile(path string) (*principalsFileYAML, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &principalsFileYAML{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error al leer %q: %w", path, err)
	}
	var parsed principalsFileYAML
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("registro %q malformado: %w", path, err)
	}
	return &parsed, nil
}

// writePrincipalsFile escribe el registro (0600, crea el directorio padre si falta).
func writePrincipalsFile(path string, f *principalsFileYAML) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0600)
}

// AddPrincipal genera un token con las capacidades por DEFAULT del rol (compat: la firma de
// siempre). Para expresar alcance y autoridad por separado, usar AddPrincipalWithCaps.
func AddPrincipal(path, name, projectID, role string) (string, error) {
	return AddPrincipalWithCaps(path, name, projectID, role, "", "")
}

// EffectiveCaps resuelve el par (alcance, autoridad) final: el del rol, salvo que se declaren
// explícitamente. La usan el CLI (para informar qué quedó) y AddPrincipalWithCaps.
func EffectiveCaps(role, read, write string) (string, string) {
	r, w := capsFromRole(strings.ToLower(strings.TrimSpace(role)))
	if v := strings.ToLower(strings.TrimSpace(read)); v != "" {
		r = v
	}
	if v := strings.ToLower(strings.TrimSpace(write)); v != "" {
		w = v
	}
	return r, w
}

// AddPrincipalWithCaps genera un token, agrega el principal al registro (guardando su SHA-256) y
// devuelve el token CRUDO una sola vez. read/write vacíos ⇒ se derivan del rol.
//
// ALCANCE (qué VE) y AUTORIDAD (qué ESCRIBE) son ejes independientes. El rol solo, que los
// colapsaba, no sabía expresar las dos identidades que un cerebro central necesita:
//
//	sala de mando (el repo de Musubi): read=all + write=own  ⇒ diagnostica todo, muta sólo lo suyo
//	cabina (el CRM, el gateway):       read=all + write=none ⇒ ve todo, no muta nada
func AddPrincipalWithCaps(path, name, projectID, role, read, write string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("el nombre del principal es obligatorio")
	}
	r, err := validRole(role)
	if err != nil {
		return "", err
	}
	efRead, efWrite := EffectiveCaps(r, read, write)
	switch efRead {
	case ReadOwn, ReadAll:
	default:
		return "", fmt.Errorf("--read inválido %q (usá own|all)", efRead)
	}
	switch efWrite {
	case WriteNone, WriteOwn, WriteAny:
	default:
		return "", fmt.Errorf("--write inválido %q (usá none|own|any)", efWrite)
	}

	// Tenancy fail-closed, ahora sobre los EJES y no sobre el rol: sin project_id, un principal que
	// escribe "lo suyo" no tiene "lo suyo" (su fila caería SIN ATRIBUIR, visible desde TODOS los
	// tenants), y uno que lee "lo suyo" no tiene a qué acotarse (vería todos los proyectos). Sólo
	// puede no tener proyecto quien NO escribe (cabina) o quien lo DECLARA en cada escritura (any).
	projectID = strings.TrimSpace(projectID)
	if projectID == "" && efWrite == WriteOwn {
		return "", fmt.Errorf("write=own requiere --project: sin proyecto propio, su escritura caería sin atribuir y la verían todos los tenants")
	}
	if projectID == "" && efRead == ReadOwn {
		return "", fmt.Errorf("read=own requiere --project: sin proyecto propio, el recall no tiene a qué acotarse y vería todos los proyectos")
	}

	f, err := readPrincipalsFile(path)
	if err != nil {
		return "", err
	}
	for _, p := range f.Principals {
		if strings.EqualFold(strings.TrimSpace(p.Name), name) {
			return "", fmt.Errorf("ya existe un principal %q (revocalo primero con 'musubi token revoke')", name)
		}
	}
	token, err := GenerateToken()
	if err != nil {
		return "", err
	}
	e := principalEntry{Name: name, TokenSHA256: hashToken(token), ProjectID: projectID, Role: r}
	// Sólo se persisten read/write cuando DIFIEREN del default del rol: un registro que usa los
	// pares clásicos queda idéntico a como estaba (diff limpio, compat total).
	if dr, dw := capsFromRole(r); efRead != dr || efWrite != dw {
		e.Read, e.Write = efRead, efWrite
	}
	f.Principals = append(f.Principals, e)
	if err := writePrincipalsFile(path, f); err != nil {
		return "", err
	}
	return token, nil
}

// ListPrincipalsInfo devuelve los principals del registro SIN los hashes (name/project/role).
func ListPrincipalsInfo(path string) ([]PrincipalInfo, error) {
	f, err := readPrincipalsFile(path)
	if err != nil {
		return nil, err
	}
	out := make([]PrincipalInfo, 0, len(f.Principals))
	for _, p := range f.Principals {
		r, w := EffectiveCaps(p.Role, p.Read, p.Write)
		out = append(out, PrincipalInfo{Name: p.Name, ProjectID: p.ProjectID, Role: p.Role, Read: r, Write: w})
	}
	return out, nil
}

// RemovePrincipal borra el principal de nombre `name` del registro. Devuelve found=false si
// no existía (sin error). Revocación = el token deja de autenticar en el próximo arranque.
func RemovePrincipal(path, name string) (bool, error) {
	name = strings.TrimSpace(name)
	f, err := readPrincipalsFile(path)
	if err != nil {
		return false, err
	}
	kept := f.Principals[:0]
	found := false
	for _, p := range f.Principals {
		if strings.EqualFold(strings.TrimSpace(p.Name), name) {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		return false, nil
	}
	f.Principals = kept
	if err := writePrincipalsFile(path, f); err != nil {
		return false, err
	}
	return true, nil
}
