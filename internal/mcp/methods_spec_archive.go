// methods_spec_archive.go cablea el DESTINO de la fase `archive` del SDD (spec-graph F4b): la 10ª tool
// del grafo de specs, musubi_spec_archive. Lee del FS el delta spec de PROCESO de un cambio
// (specs/<change>/spec.md) y la SPEC.md canónica de su módulo, PROYECTA el delta a forma canónica
// (projectSDDProcessDelta, que repara H1/H7 de la review e2f650be: poda las claves de proceso y no deja
// que el status del delta degrade la canónica) y lo FUSIONA con specgraph.MergeSpecInto.
//
// write, NO read-only (R21): en el camino limpio ESCRIBE el merged en la canónica —archivar ES persistir
// el pliegue; una tool que sólo devolviera el texto dejaría a `archive` otra vez sin destino real—. Muta
// el FS igual que create/update/delete (candado exclusivo del despacho); marcar read-only algo que muta
// es un lost-update que -race no ve. PERO surface-not-auto (R15): si el merge devuelve `conflicts` (prosa
// divergente del MISMO heading) NO escribe —devuelve el merged para inspección y los conflicts para que
// una persona los reconcilie primero—. El merged conserva la prosa canónica intacta en ambos casos.
//
// Sólo toca el FS (os.Root, confinado a la raíz): no lee tablas scopeadas por tenant como coverage, así
// que su handler es noCtx, como create/update/delete.

package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/specgraph"
)

// specArchiveOut es la respuesta de musubi_spec_archive (tags json camelCase explícitos, musubi-rules #6).
type specArchiveOut struct {
	Mode      string                      `json:"mode"`             // "promote" (canónica nueva) | "merge"
	Change    string                      `json:"change"`           // el cambio SDD cuyo delta se archivó
	Path      string                      `json:"path"`             // la SPEC.md canónica (root-relative)
	Written   bool                        `json:"written"`          // true ⇒ se escribió la canónica (sin conflictos)
	Conflicts []specgraph.SectionConflict `json:"conflicts"`        // secciones surfaceadas sin resolver (R15); lista, nunca null
	Merged    string                      `json:"merged,omitempty"` // el texto fusionado; SÓLO cuando NO se escribió (acota el payload en el camino limpio)
	Message   string                      `json:"message"`
}

// toolSpecArchive fusiona el delta spec de proceso de un cambio en la SPEC.md canónica de su módulo
// (destino de la fase `archive` del SDD, F4b). `change` localiza el delta (specs/<change>/spec.md) y
// `path` la canónica. Sin canónica PROMUEVE (nace del delta; exige id+type del módulo, R13); con canónica
// FUSIONA (la canónica es autoritativa sobre identidad y status, R14). id/type/status se usan SÓLO al
// promover; en merge se ignoran.
func (s *McpServer) toolSpecArchive(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Change string `json:"change"`
		Path   string `json:"path"`
		ID     string `json:"id"`
		Type   string `json:"type"`
		Status string `json:"status"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	change := strings.TrimSpace(args.Change)
	if change == "" {
		return nil, rpcErrorf(codeInvalidParams, "change es obligatorio (el cambio SDD cuyo specs/<change>/spec.md archivar)")
	}
	// `change` es un SEGMENTO de ruta, no una ruta: un separador o un '..' lo volvería traversal. Se
	// valida acá además del confinamiento de os.Root para dar un error que nombra la causa, no un fallo
	// opaco de resolución.
	if strings.ContainsAny(change, `/\`) || strings.Contains(change, "..") {
		return nil, rpcErrorf(codeInvalidParams, "change %q inválido: es un nombre de cambio, sin separadores ni '..'", change)
	}
	if strings.TrimSpace(args.Path) == "" {
		return nil, rpcErrorf(codeInvalidParams, "path es obligatorio (la SPEC.md canónica del módulo donde fusionar)")
	}

	// Ambas rutas se resuelven y confinan a la raíz (mismo os.Root/TOCTOU que create/update, R11/R15):
	// el resolve valida `.md` dentro del repo, y os.Root cierra la carrera del junction en use-time.
	canonRel, _, err := specgraph.ResolveSpecPath(s.projectPath, args.Path)
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "path inválido: %v", err)
	}
	deltaRel, _, err := specgraph.ResolveSpecPath(s.projectPath, "specs/"+change+"/spec.md")
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo resolver el delta del cambio %q: %v", change, err)
	}

	root, oErr := os.OpenRoot(s.projectPath)
	if oErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo abrir la raíz del proyecto: %v", oErr)
	}
	defer root.Close()

	delta, rErr := root.ReadFile(deltaRel)
	if rErr != nil {
		if os.IsNotExist(rErr) {
			return nil, rpcErrorf(codeInvalidParams, "no existe el delta %s del cambio %q (¿corriste el flujo SDD?)", deltaRel, change)
		}
		return nil, rpcErrorf(codeInternalError, "no se pudo leer el delta %s: %v", deltaRel, rErr)
	}

	// ¿Existe ya la canónica? Determina promoción vs merge. Un error que NO sea "no existe" (permisos,
	// etc.) NO se trata como promoción —eso crearía una canónica pisando un error de lectura—: se reporta.
	var canonical []byte
	hasCanonical := false
	if existing, cErr := root.ReadFile(canonRel); cErr == nil {
		canonical = existing
		hasCanonical = true
	} else if !os.IsNotExist(cErr) {
		return nil, rpcErrorf(codeInternalError, "no se pudo leer la canónica %s: %v", canonRel, cErr)
	}

	// Al PROMOVER, el status de la canónica nueva lo pone el caller o, por defecto, "active": un módulo
	// recién archivado tiene una canónica viva. En MERGE id/type/status se ignoran (projectSDDProcessDelta
	// no los usa con hasCanonical=true; la canónica es autoritativa).
	promoteStatus := strings.TrimSpace(args.Status)
	if !hasCanonical && promoteStatus == "" {
		promoteStatus = "active"
	}
	projected, pErr := projectSDDProcessDelta(delta, hasCanonical,
		strings.TrimSpace(args.ID), strings.TrimSpace(args.Type), promoteStatus)
	if pErr != nil {
		// Promover sin id/type del módulo, o con un status fuera del vocabulario (R13/E15): el mensaje
		// del core nombra la causa y viaja verbatim.
		return nil, rpcErrorf(codeInvalidParams, "%v", pErr)
	}

	merged, conflicts, mErr := specgraph.MergeSpecInto(canonical, projected)
	if mErr != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", mErr)
	}
	// Contrato de lista no-nil (go-hygiene #6): un slice nil serializaría a null; `conflicts` siempre es [].
	if conflicts == nil {
		conflicts = make([]specgraph.SectionConflict, 0)
	}

	mode := "merge"
	if !hasCanonical {
		mode = "promote"
	}
	out := specArchiveOut{Mode: mode, Change: change, Path: canonRel, Conflicts: conflicts}

	// surface-not-auto (R15/R21): sin conflictos se ESCRIBE la canónica (archivar = persistir el pliegue);
	// con conflictos NO se escribe —el merged viaja para inspección y los conflicts para reconciliar—.
	if len(conflicts) == 0 {
		if mkErr := root.MkdirAll(filepath.Dir(canonRel), 0o755); mkErr != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo crear el directorio de %s: %v", canonRel, mkErr)
		}
		if wErr := root.WriteFile(canonRel, merged, 0o644); wErr != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo escribir %s: %v", canonRel, wErr)
		}
		out.Written = true
		if mode == "promote" {
			out.Message = fmt.Sprintf("Promovida la canónica %s desde el delta del cambio %q.", canonRel, change)
		} else {
			out.Message = fmt.Sprintf("Fusionado el delta del cambio %q en %s.", change, canonRel)
		}
	} else {
		out.Merged = string(merged)
		out.Message = fmt.Sprintf(
			"No se escribió: el delta del cambio %q choca en %d sección(es) con %s. Reconciliá la prosa surfaceada (conflicts) y volvé a archivar.",
			change, len(conflicts), canonRel)
	}
	return jsonResult(out)
}
