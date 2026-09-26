package memory

import (
	"context"
	"fmt"
	"strings"
)

// tareas_del_sistema.go — el trabajo que MUSUBI MISMO le deja al agente en su tablero.
//
// Musubi no tiene modelo: sabe detectar que algo necesita criterio, pero no darlo. Hasta acá ese
// trabajo esperaba a que alguien lo pidiera, y nadie lo pedía. Medido el 2026-09-25 en la memoria de
// este repo: 125 propuestas en cuarentena (lo que el agente guarda con musubi_propose_observation,
// que es lo que las instrucciones le piden) y CERO corroboradas alguna vez. El recall filtra la
// cuarentena, así que todo eso era invisible: memoria escrita que ninguna sesión podía encontrar.
//
// El tablero del multi-agente (work.go) ya tenía lo que hace falta para repartir trabajo entre
// sesiones que corren a la vez —lease, fencing, reintentos, autonomía—. Lo que faltaba era que Musubi
// posteara ahí lo suyo. Lo postea el hook del turno (cmd/musubi/tareas.go) y lo hace un subagente que
// trae el plugin.

// PrefijoLoteDelSistema marca los lotes que postea Musubi, no un agente que orquesta. Un lote así lo
// toma sólo quien lo nombra: ni el reclamo «de cualquier lote» ni el recordatorio del lote activo lo
// ven (work.go).
const PrefijoLoteDelSistema = "musubi/"

// LoteCuarentena es el lote de las propuestas en cuarentena que esperan veredicto.
const LoteCuarentena = PrefijoLoteDelSistema + "cuarentena"

// EsLoteDelSistema dice si un lote lo postea Musubi.
func EsLoteDelSistema(batchID string) bool {
	return strings.HasPrefix(batchID, PrefijoLoteDelSistema)
}

// PropuestaEnCuarentena es una observación que escribió un modelo y espera veredicto.
type PropuestaEnCuarentena struct {
	ID        string
	TopicKey  string
	CreatedAt string
}

// PropuestasEnCuarentena lista las propuestas vivas en cuarentena, de la más vieja a la más nueva.
//
// Sin tope: la lee el hook del turno sobre la memoria LOCAL de un proyecto, donde son decenas o
// cientos, y quien la usa necesita el conjunto entero para saber qué no cubre todavía ninguna unidad.
func (e *DbEngine) PropuestasEnCuarentena() ([]PropuestaEnCuarentena, error) {
	rows, err := e.db.Query(`
		SELECT id, COALESCE(topic_key,''), COALESCE(created_at,'')
		  FROM observations
		 WHERE quarantined = 1 AND archived = 0
		 ORDER BY created_at, rowid`)
	if err != nil {
		return nil, fmt.Errorf("error al listar la cuarentena: %w", err)
	}
	defer rows.Close()
	var out []PropuestaEnCuarentena
	for rows.Next() {
		var p PropuestaEnCuarentena
		if err := rows.Scan(&p.ID, &p.TopicKey, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("error al leer la cuarentena: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DescartarPropuesta archiva una propuesta en cuarentena que no merece salir: la reemplaza otra más
// nueva del mismo tema (reemplazadaPor), o ya hay una nota visible que dice lo mismo.
//
// ANTES NO HABÍA CÓMO. La única salida de la cuarentena era corroborar, así que una propuesta
// equivocada o vieja quedaba para siempre, y un estado de trabajo superado por el siguiente no se
// podía sacar sin hacerlo visible. Descartar es el veredicto que faltaba.
//
// ARCHIVA, NO BORRA: la fila queda con su sello, invisible, y la purga de archivadas la borra recién
// después de su gracia. Sigue en cuarentena: descartarla no la verifica. Y sólo actúa sobre una
// propuesta viva en cuarentena —descartar una nota visible sería una puerta lateral para ocultar
// memoria autoritativa—.
func (e *DbEngine) DescartarPropuesta(id, reemplazadaPor string) error {
	id, reemplazadaPor = strings.TrimSpace(id), strings.TrimSpace(reemplazadaPor)
	var existe int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id = ?`, id).Scan(&existe); err != nil {
		return fmt.Errorf("error al leer la propuesta %s: %w", id, err)
	}
	if existe == 0 {
		return fmt.Errorf("%w: %s", ErrObservationNotFound, id)
	}
	if reemplazadaPor != "" {
		if reemplazadaPor == id {
			return fmt.Errorf("una propuesta no se reemplaza a sí misma (%s)", id)
		}
		var existe int
		if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id = ?`, reemplazadaPor).Scan(&existe); err != nil {
			return fmt.Errorf("error al leer la que la reemplaza (%s): %w", reemplazadaPor, err)
		}
		if existe == 0 {
			return fmt.Errorf("%w: la que la reemplaza (%s)", ErrObservationNotFound, reemplazadaPor)
		}
	}
	// LA GUARDA ES EL WHERE, en la misma sentencia que escribe: sólo una propuesta viva en
	// cuarentena. Una nota visible, una ya descartada, o una que alguien corroboró mientras tanto no
	// se toca, y eso se informa como «no está en cuarentena».
	res, err := e.db.Exec(`
		UPDATE observations
		   SET archived = 1, archived_at = CURRENT_TIMESTAMP, superseded_by = NULLIF(?, '')
		 WHERE id = ? AND quarantined = 1 AND archived = 0`, reemplazadaPor, id)
	if err != nil {
		return fmt.Errorf("error al descartar la propuesta %s: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("%w: %s", ErrNotQuarantined, id)
	}
	return nil
}

// DescartarPropuestaCtx es DescartarPropuesta acotada al proyecto de la credencial, con la misma
// guarda que CorroborateObservationCtx: conocer un id ajeno no alcanza para archivar memoria de otro
// tenant. La que la reemplaza también tiene que ser del proyecto.
func (e *DbEngine) DescartarPropuestaCtx(ctx context.Context, id, reemplazadaPor string) error {
	for _, cual := range []string{id, reemplazadaPor} {
		if strings.TrimSpace(cual) == "" {
			continue
		}
		ok, err := e.obsInScope(ctx, cual)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: no se puede descartar %s desde otro proyecto", ErrCrossTenant, cual)
		}
	}
	return e.DescartarPropuesta(id, reemplazadaPor)
}
