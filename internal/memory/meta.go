package memory

import (
	"database/sql"
	"fmt"
	"time"

	"musubi/internal/logx"
)

// meta.go guarda metadatos clave/valor y la lógica de throttle del
// auto-mantenimiento (cada cuánto correr consolidación + olvido).

const metaLastMaintenance = "last_maintenance"

// MetaLastHealth es la clave de meta donde AutoHeal persiste el último DiagnoseReport
// (post-repair), para que el hook de arranque pueda surfacear problemas no auto-reparables.
const MetaLastHealth = "last_health"

// MetaStackFingerprint es la clave de meta donde se guarda la huella del stack
// para el cual ya se generaron skills. La comparten el hook SessionStart (que
// detecta drift del stack) y musubi_save_skill (que la actualiza al guardar).
const MetaStackFingerprint = "skills_stack"

// MetaCodegraphHead es la clave de meta donde el indexador del grafo de código deja el commit
// (git rev-parse --short HEAD) del árbol que indexó. Existe porque el grafo indexa lo que está
// CHECKOUTEADO y es ciego a las otras ramas: medido el 2026-09-03 en este repo, 95 commits y 334
// archivos de `feat/control-de-flota` no estaban en el grafo de `main`, y un `found:false` desnudo
// se leía como «el grafo no sabe» cuando la verdad era «estás en otra rama». Con el commit
// guardado, el miss puede decir de qué árbol es el índice. Vacío si no hay git o no es un repo.
const MetaCodegraphHead = "codegraph_head"

// MetaCodegraphDeriver guarda la versión del derivador que escribió el grafo. Cuando no coincide
// con la del binario que corre, el índice incremental re-deriva TODO una vez: las filas viejas las
// escribió un derivador que no sabía emitir lo que el nuevo emite, y el fingerprint por contenido
// no lo puede notar porque el archivo no cambió. Ver codeintel.GraphDeriverVersion.
const MetaCodegraphDeriver = "codegraph_deriver"

// GetMeta devuelve el valor de una clave de metadatos (ok=false si no existe).
func (e *DbEngine) GetMeta(key string) (string, bool, error) {
	var v string
	err := e.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("error al leer meta %q: %w", key, err)
	}
	return v, true, nil
}

// SetMeta inserta o actualiza una clave de metadatos.
func (e *DbEngine) SetMeta(key, value string) error {
	_, err := e.db.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("error al guardar meta %q: %w", key, err)
	}
	return nil
}

// MetaDue indica si corresponde correr una tarea throttled identificada por key:
// true si no hay marca previa, si no se puede parsear, o si pasaron >= intervalHours.
func (e *DbEngine) MetaDue(key string, intervalHours float64) (bool, error) {
	v, ok, err := e.GetMeta(key)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	last, perr := time.Parse(time.RFC3339, v)
	if perr != nil {
		return true, nil // marca corrupta: mejor correr
	}
	return time.Since(last).Hours() >= intervalHours, nil
}

// MetaEdadSegundos devuelve hace cuántos segundos se marcó una tarea throttled, o -1 si nunca se
// marcó (o si la marca quedó ilegible).
//
// EL -1 NO ES UN DETALLE: distingue «nunca corrió» de «corrió hace 0 segundos», que con un entero
// pelado serían el mismo número — y son las dos respuestas más distintas posibles. Es la misma
// convención que ya usan musubi_backup_local_age_seconds y su par off-host.
//
// Una marca ilegible cuenta como «nunca» y no como un error: para quien mira la métrica, una marca
// que no se puede leer y una que no está significan lo mismo —no hay evidencia de que la tarea
// haya corrido— y darle un tercer valor sólo agregaría un caso que nadie va a alertar.
func (e *DbEngine) MetaEdadSegundos(key string) int64 {
	v, ok, err := e.GetMeta(key)
	if err != nil || !ok {
		return -1
	}
	t, perr := time.Parse(time.RFC3339, v)
	if perr != nil {
		return -1
	}
	return int64(time.Since(t).Seconds())
}

// MantenimientoEdadSegundos es MetaEdadSegundos para el ciclo de memoria. Existe como método
// propio porque `metaLastMaintenance` no se exporta: sin esto, el caller de afuera tendría que
// escribir el nombre de la clave a mano, que es como se desincronizan dos lados.
func (e *DbEngine) MantenimientoEdadSegundos() int64 {
	return e.MetaEdadSegundos(metaLastMaintenance)
}

// MarkMetaNow registra que una tarea throttled acaba de correr.
func (e *DbEngine) MarkMetaNow(key string) error {
	return e.SetMeta(key, time.Now().UTC().Format(time.RFC3339))
}

// MetaEmbedModel es la clave donde se registra la identidad del modelo de embedding
// activo (Provider.Name()) del último arranque, para detectar un cambio de modelo.
const MetaEmbedModel = "embed_model_id"

// WarnOnEmbedModelSwitch detecta si el modelo de embedding activo cambió respecto del
// último arranque HABIENDO vectores ya almacenados: en ese caso los vectores viejos son
// de OTRO modelo y no son comparables por coseno con los nuevos. Desde F2.2 el contrato
// de procedencia (columna embeddings.model_id + regla de homogeneidad en la búsqueda) ya
// los EXCLUYE automáticamente del recall, así que NO corrompen el ranking; este aviso es
// informativo (recordar re-embeber para recuperarlos) y complementa al dim-guard, que
// cubre el cambio de dimensión. No migra ni borra nada. Registra el modelo actual para el
// próximo arranque. modelID vacío (sin embedder / NoopProvider) es no-op.
func (e *DbEngine) WarnOnEmbedModelSwitch(modelID string) {
	if modelID == "" {
		return
	}
	prev, ok, err := e.GetMeta(MetaEmbedModel)
	if err != nil {
		logx.Warn("no se pudo leer el modelo de embedding previo", "error", err)
		return
	}
	if ok && prev != "" && prev != modelID {
		if n, cerr := e.countActiveEmbeddings(); cerr == nil && n > 0 {
			logx.Warn("el modelo de embedding cambió: hay vectores de otro modelo en la base",
				"anterior", prev, "actual", modelID, "vectores_previos", n,
				"accion", "el contrato de procedencia (F2.2) ya los EXCLUYE del recall automáticamente (la búsqueda sólo compara igual model_id), así que no corrompen el ranking; re-embebé con el modelo actual para volver a hacerlos recuperables")
		}
	}
	if err := e.SetMeta(MetaEmbedModel, modelID); err != nil {
		logx.Warn("no se pudo registrar el modelo de embedding activo", "error", err)
	}
}

// MaintenanceDue indica si corresponde correr el auto-mantenimiento.
func (e *DbEngine) MaintenanceDue(intervalHours float64) (bool, error) {
	return e.MetaDue(metaLastMaintenance, intervalHours)
}

// MarkMaintenanceNow registra que el mantenimiento acaba de correr.
func (e *DbEngine) MarkMaintenanceNow() error {
	return e.MarkMetaNow(metaLastMaintenance)
}
