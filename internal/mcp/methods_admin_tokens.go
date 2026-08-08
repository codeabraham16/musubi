package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"musubi/internal/config"
)

// methods_admin_tokens.go expone la ADMINISTRACIÓN de identidades del cerebro central POR LA RED
// (mint / list / revoke de principals): la contracara de `musubi token` (que hoy es sólo CLI +
// editar principals.yaml a mano en el disco del server). Existe para que la administración del
// equipo se haga desde una interfaz (el cuerpo, gateada a role=admin) en vez de SSH.
//
// Las tres tools son ADMIN-ONLY, y la guarda real es SERVER-SIDE (isAdmin()), no que la UI del
// cliente muestre o esconda el panel: mintear un token es la joya de la corona. Sólo un principal
// admin —o el stdio local de confianza— puede. Mismo patrón que musubi_maintain / doctor repair.
//
// Escriben el MISMO principals.yaml que el server usa para autenticar; el watcher de recarga en
// caliente (principals_reload.go) las hace efectivas en ≤10s sin reiniciar el daemon.

// principalsFilePath devuelve la ruta del registro que el server administra. La fija
// ListenAndServeHTTP (serve/HTTP); si está vacía (stdio local o test) cae al default del
// workspace, coherente con principalsPath.
func (s *McpServer) principalsFilePath() string {
	if p := strings.TrimSpace(s.principalsFile); p != "" {
		return p
	}
	return filepath.Join(s.projectPath, config.DirName, "principals.yaml")
}

// toolTokenNew da de alta un principal y devuelve su token CRUDO una SOLA vez (el registro sólo
// guarda su SHA-256). Admin-only. role vacío ⇒ writer, igual que el CLI. read/write vacíos se
// derivan del rol; declarados, mandan (así se expresan la sala de mando y la cabina).
func (s *McpServer) toolTokenNew(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	if !principalFrom(ctx).isAdmin() {
		return nil, rpcErrorf(codeUnauthorized, "musubi_token_new administra las identidades del equipo: requiere un principal admin")
	}
	var args struct {
		Name    string `json:"name"`
		Project string `json:"project"`
		Role    string `json:"role"`
		Read    string `json:"read"`
		Write   string `json:"write"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	role := strings.TrimSpace(args.Role)
	if role == "" {
		role = RoleWriter
	}
	token, err := AddPrincipalWithCaps(s.principalsFilePath(), args.Name, args.Project, role, args.Read, args.Write)
	if err != nil {
		// Toda la validación (nombre vacío, rol/eje inválido, tenancy fail-closed, duplicado) vive en
		// AddPrincipalWithCaps: acá se propaga como argumento inválido, no como error interno.
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	efRead, efWrite := EffectiveCaps(role, args.Read, args.Write)
	return jsonResult(map[string]interface{}{
		"token":      token, // se muestra UNA vez; entregáselo al miembro por un canal seguro
		"name":       strings.TrimSpace(args.Name),
		"project_id": strings.TrimSpace(args.Project),
		"role":       role,
		"read":       efRead,
		"write":      efWrite,
	})
}

// toolTokenList lista los principals del registro SIN los hashes. Admin-only: saber QUIÉN tiene
// acceso y con qué poder es información sensible del equipo.
func (s *McpServer) toolTokenList(ctx context.Context, _ json.RawMessage) (interface{}, *RpcError) {
	if !principalFrom(ctx).isAdmin() {
		return nil, rpcErrorf(codeUnauthorized, "musubi_token_list lista las identidades del equipo: requiere un principal admin")
	}
	infos, err := ListPrincipalsInfo(s.principalsFilePath())
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo leer el registro de principals: %v", err)
	}
	out := make([]map[string]interface{}, 0, len(infos))
	for _, p := range infos {
		out = append(out, map[string]interface{}{
			"name":       p.Name,
			"project_id": p.ProjectID,
			"role":       p.Role,
			"read":       p.Read,  // capacidad EFECTIVA (own|all), ya resuelta del rol
			"write":      p.Write, // capacidad EFECTIVA (none|own|any)
		})
	}
	return jsonResult(map[string]interface{}{"principals": out})
}

// toolTokenRevoke da de baja un principal por nombre. Admin-only. found=false si no existía (sin
// error). No deja que un admin se revoque a SÍ MISMO: si es el único admin quedaría sin acceso y
// nadie podría re-mintear (lockout). La revocación surte efecto en ≤10s (recarga en caliente).
func (s *McpServer) toolTokenRevoke(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	caller := principalFrom(ctx)
	if !caller.isAdmin() {
		return nil, rpcErrorf(codeUnauthorized, "musubi_token_revoke da de baja identidades del equipo: requiere un principal admin")
	}
	var args struct {
		Name string `json:"name"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	name := strings.TrimSpace(args.Name)
	if name == "" {
		return nil, rpcErrorf(codeInvalidParams, "name es obligatorio")
	}
	if caller != nil && caller.Name != "" && strings.EqualFold(caller.Name, name) {
		return nil, rpcErrorf(codeInvalidParams, "no podés revocar tu propia credencial admin (te dejaría sin acceso); pedile a otro admin que lo haga)")
	}
	found, err := RemovePrincipal(s.principalsFilePath(), name)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo revocar: %v", err)
	}
	return jsonResult(map[string]interface{}{
		"found": found,
		"name":  name,
	})
}
