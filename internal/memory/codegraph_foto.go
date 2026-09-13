package memory

import (
	"context"
	"database/sql"
	"fmt"
)

// consultor es lo que tienen en común *sql.DB y *sql.Tx para leer. Existe para que los volcados del
// grafo y de los gists se escriban UNA vez y corran igual contra la base suelta o dentro de una
// transacción de lectura, sin una segunda copia de cada consulta que se separe de la primera.
type consultor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// FotoDelGrafo es el grafo de código de un proyecto tal como viaja al central: nodos, aristas y
// gists leídos del MISMO instante.
type FotoDelGrafo struct {
	Nodes []GraphNode
	Edges []GraphEdge
	Gists []CodeMemory
	// Generacion es la generación del grafo (ver codegraph_generacion.go) en la MISMA instantánea
	// que las tres listas. Es lo que se marca como empujado si el push sale bien: leerla después,
	// fuera de la transacción, podría declarar empujado un cambio que la foto no trae.
	Generacion int64
}

// FotoDelGrafoCtx lee la foto completa del grafo (scopeada por la credencial) dentro de UNA
// transacción de lectura.
//
// POR QUÉ UNA TRANSACCIÓN Y NO TRES LECTURAS SUELTAS. El push al central es de REEMPLAZO: el
// central borra lo que no venga. Si entre leer los nodos y leer las aristas otro escritor re-deriva
// un paquete, viajan aristas que apuntan a nodos que no viajaron (o al revés), y el central se queda
// con esa mezcla hasta el próximo push. En WAL, una transacción diferida fija su instantánea en la
// primera lectura y la sostiene hasta el final: las tres lecturas ven la misma base.
//
// POR QUÉ ESTO Y NO LEER BAJO dispatchMu. El candado del despacho sólo serializa dentro de ESTE
// proceso, y sobre la base de Musubi corren dos daemons (y tres sobre altura-erp) que re-derivan el
// grafo sin ese candado: leer bajo dispatchMu no protegía contra el otro proceso, y encima congelaba
// las tools del daemon mientras se volcaban 11.415 nodos y 27.350 aristas. La transacción cubre a
// los dos escritores y no bloquea a nadie: en WAL un lector no frena a un escritor.
//
// ReadOnly es la mitad que importa: el DSN lleva `_txlock=immediate`, que haría que un Begin común
// tome el lock de ESCRITURA al nacer y serialice esta lectura contra todos los escritores. Con
// ReadOnly el driver emite un `BEGIN` diferido (ver el comentario del DSN en database.go).
func (e *DbEngine) FotoDelGrafoCtx(ctx context.Context) (FotoDelGrafo, error) {
	return e.fotoDelGrafo(ctx, nil)
}

// fotoDelGrafo es FotoDelGrafoCtx con una costura para pruebas: `entreLecturas`, si no es nil,
// corre después de leer los nodos y antes de leer las aristas. Es el único modo de meter un
// escritor justo en la ventana que la transacción cierra, sin depender de la suerte del scheduler.
func (e *DbEngine) fotoDelGrafo(ctx context.Context, entreLecturas func()) (FotoDelGrafo, error) {
	var foto FotoDelGrafo
	tx, err := e.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return foto, fmt.Errorf("error al abrir la lectura de la foto del grafo: %w", err)
	}
	// Sólo lectura: Rollback no puede dejar nada a medias, y cierra la instantánea al salir.
	defer func() { _ = tx.Rollback() }()

	// La generación va PRIMERO: en WAL la primera lectura fija la instantánea, y así la generación
	// es exactamente la de las listas que siguen.
	if foto.Generacion, err = leerGeneracionDelGrafo(ctx, tx); err != nil {
		return foto, err
	}
	if foto.Nodes, err = e.listAllGraphNodes(ctx, tx); err != nil {
		return foto, fmt.Errorf("error al leer los nodos de la foto del grafo: %w", err)
	}
	if entreLecturas != nil {
		entreLecturas()
	}
	if foto.Edges, err = e.listAllGraphEdges(ctx, tx); err != nil {
		return foto, fmt.Errorf("error al leer las aristas de la foto del grafo: %w", err)
	}
	if foto.Gists, err = e.allCodeMemory(ctx, tx); err != nil {
		return foto, fmt.Errorf("error al leer los gists de la foto del grafo: %w", err)
	}
	return foto, nil
}
