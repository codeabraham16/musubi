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

// AddPrincipal genera un token, agrega el principal al registro (guardando su SHA-256) y
// devuelve el token CRUDO una sola vez (para entregárselo al miembro). Rechaza nombres
// duplicados. Crea el archivo si no existe.
func AddPrincipal(path, name, projectID, role string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("el nombre del principal es obligatorio")
	}
	r, err := validRole(role)
	if err != nil {
		return "", err
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
	f.Principals = append(f.Principals, principalEntry{
		Name: name, TokenSHA256: hashToken(token), ProjectID: strings.TrimSpace(projectID), Role: r,
	})
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
		out = append(out, PrincipalInfo{Name: p.Name, ProjectID: p.ProjectID, Role: p.Role})
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
