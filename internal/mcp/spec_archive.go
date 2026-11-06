// spec_archive.go cablea el DESTINO de la fase `archive` del SDD (spec-graph F4) en la CAPA DE TOOLS:
// proyecta el delta spec de PROCESO de un cambio —el specs/<cambio>/spec.md que emite el flujo SDD— a
// la forma CANÓNICA antes de fusionarlo con specgraph.MergeSpecInto en la SPEC.md del módulo.
//
// Por qué la proyección vive ACÁ y no en specgraph (D8: el core fiel, la política en las tools):
// specgraph.MergeSpecInto es el merge canónica↔canónica y DEBE quedar agnóstico del SDD. El delta de
// proceso NO es una spec canónica —no tiene id/type y arrastra el encabezado de proceso del SDD
// (artifact/schema_version/change)— así que fusionarlo CRUDO:
//   - CONTAMINA la canónica con esas claves de proceso (frontmatterMergeEdit Set-ea cada escalar del
//     delta sobre la canónica), y
//   - PISA el status canónico con el status de proceso del delta (p. ej. draft), degradando en silencio
//     una spec `active` a `draft`.
//
// La proyección es la política SDD que repara las dos cosas ANTES del merge, dejando MergeSpecInto puro.

package mcp

import (
	"fmt"
	"slices"

	"musubi/internal/specgraph"
)

// Claves del encabezado de PROCESO del SDD. No son vocabulario de spec-graph (que es id/type/status/…):
// las escribe la plantilla del flujo SDD (.musubi/templates/sdd/*.md) en cada specs/<cambio>/spec.md, y
// specgraph las trata como «diario SDD, no spec» justamente porque le FALTAN id+type (ver
// internal/specgraph/recall.go: «frontmatter artifact+schema_version, sin id+type»). Se enumeran acá
// —en la capa de política— para PODARLAS al proyectar; viven en inglés porque son el contrato de datos
// en disco, igual que los Field* de specgraph.
const (
	sddFieldArtifact      = "artifact"
	sddFieldSchemaVersion = "schema_version"
	sddFieldChange        = "change"
)

// sddProcessKeys son las claves de proceso que la proyección SIEMPRE poda del delta. `status` NO está
// acá a propósito: existe también en la canónica y la proyección lo trata aparte (autoritativo de la
// canónica), no lo poda como ruido de proceso.
var sddProcessKeys = []string{sddFieldArtifact, sddFieldSchemaVersion, sddFieldChange}

// projectSDDProcessDelta transforma el delta spec de PROCESO del SDD a la forma canónica que
// specgraph.MergeSpecInto consume. Es PURA y determinista (sin I/O: el caller lee el delta y la canónica
// del disco). Dos modos, según exista ya la SPEC.md canónica del módulo:
//
//   - PROMOCIÓN (hasCanonical=false): la canónica NACE del delta, así que el resultado DEBE ser una spec
//     válida (id+type, R13). El delta de proceso no los trae: se EXIGEN e INYECTAN moduleID+moduleType,
//     y se repone el status con promoteStatus (una canónica nueva nace con un status explícito del ciclo
//     de vida). MergeSpecInto devuelve ese delta proyectado VERBATIM como la canónica (E8).
//
//   - MERGE (hasCanonical=true): la identidad y el status los pone la canónica EXISTENTE. NO se inyecta
//     id/type (los pone la canónica) ni se repone status (la canónica conserva el suyo: ÉSA es la
//     corrección del downgrade). El delta proyectado sólo aporta cuerpo y los campos-lista spec que
//     llegara a declarar.
//
// En AMBOS modos se podan las claves de proceso (artifact/schema_version/change) y se quita `status` del
// delta. Reserializar el frontmatter del delta con specgraph.SerializeFrontmatter es correcto y no viola
// el lossless (R16): lo lossless es la EDICIÓN de la CANÓNICA (MergeSpecInto la edita con
// UpdateFrontmatterText); el delta de proceso es material que se TRANSFORMA, no se conserva byte a byte.
func projectSDDProcessDelta(delta []byte, hasCanonical bool, moduleID, moduleType, promoteStatus string) ([]byte, error) {
	parsed := specgraph.ParseFile(string(delta))

	// Copia PODADA del frontmatter del delta: nunca se muta el mapa parseado, y se arranca de una copia
	// para no depender del orden original (SerializeFrontmatter reordena por FieldOrder igual).
	clean := specgraph.Frontmatter{}
	for k, v := range parsed.Frontmatter {
		clean[k] = v
	}
	for _, k := range sddProcessKeys {
		delete(clean, k)
	}
	// `status` del delta se quita SIEMPRE: el status canónico es autoritativo (no se degrada desde el
	// delta de proceso). En promoción se repone abajo con promoteStatus; en merge no se repone.
	delete(clean, specgraph.FieldStatus)

	if hasCanonical {
		// MERGE: la canónica pone identidad y status. No se inyecta nada; el delta sólo aporta cuerpo.
		return []byte(specgraph.SerializeFrontmatter(clean) + parsed.Body), nil
	}

	// PROMOCIÓN: la canónica nace del delta y debe ser una spec válida (R13). El delta de proceso no
	// trae id/type, así que son obligatorios: promover sin ellos dejaría una canónica que el grafo no
	// puede navegar (y MergeSpecInto la rechazaría igual) —fallar acá da un error que nombra la causa.
	if moduleID == "" || moduleType == "" {
		return nil, fmt.Errorf("promover el delta a una SPEC.md canónica nueva exige id y type del módulo (R13): el spec.md de proceso del SDD no los trae")
	}
	// El status de una canónica nueva debe pertenecer al ciclo de vida del dialecto: promoverla con un
	// status basura la dejaría inválida de nacimiento. Se valida en vez de coaccionar en silencio.
	if promoteStatus != "" && !slices.Contains(specgraph.SpecStatuses, promoteStatus) {
		return nil, fmt.Errorf("status de promoción %q fuera del vocabulario de ciclo de vida %v (R13)", promoteStatus, specgraph.SpecStatuses)
	}
	clean[specgraph.FieldID] = moduleID
	clean[specgraph.FieldType] = moduleType
	if promoteStatus != "" {
		clean[specgraph.FieldStatus] = promoteStatus
	}
	return []byte(specgraph.SerializeFrontmatter(clean) + parsed.Body), nil
}
