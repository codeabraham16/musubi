package memory

import (
	"context"
	"testing"
)

// EL CURSOR DE BAJADA SALTA LO QUE EL FILTRO DE PROYECTO DESCARTA, Y NO VUELVE.
//
// ✅ EL DEFECTO ESTÁ CERRADO, PERO NO ACÁ, y por eso estos tests siguen afirmando el salto. La
// consulta de abajo sigue salteando: eso es lo que se le pide y no cambió. Lo que se agregó es la
// REPARACIÓN, y vive en el cliente — el central declara su recorte en cada pull (`alcance`) y el
// cliente reinicia el cursor cuando cambia, con ReiniciarBajadaPorAlcance (bajada_lease.go). La prueba
// de punta a punta está en internal/mcp/inbound_alcance_test.go.
//
// Estos dos se conservan porque son la mitad que explica POR QUÉ hace falta ese reinicio. Si algún día
// se arregla el salto en el SQL —haciendo que el `next_cursor` salga de lo ESCANEADO en vez de lo
// devuelto—, se van a poner rojos, y ahí la reparación del cliente pasa a ser redundante.
//
// EL MECANISMO. scopeSQL (el acotamiento por tenant) entra al MISMO WHERE que `sync_seq > ?`, y el
// LIMIT se aplica DESPUÉS. El filtro no acorta la página: la corre hacia arriba por encima de las
// filas ajenas. Después, toolSyncPull (internal/mcp/methods.go) calcula el next_cursor con el máximo
// de las filas que DEVOLVIÓ —ya filtradas—, así que lo descartado queda DEBAJO del cursor sin haberse
// entregado. El cliente lo adopta y AvanzarCursorBajada (bajada_lease.go) no retrocede nunca.
//
// CUÁNDO SE COBRA. Mientras el token es read:own la fila ajena no corresponde y no falta. Cuando pasa
// a read:all el filtro desaparece, pero el cursor ya está arriba: esa historia no vuelve JAMÁS.
//
// MEDIDO EL 2026-09-24 en la base real del central cruzada contra la de davantis-1: de 3.073 filas
// pullable, 64 ausentes, TODAS con sync_seq por debajo del cursor (10.990) y ninguna por encima. El
// corte cae exacto en el borde de proyecto: `altura` 61 de 61 ausentes bajo sync_seq 855, y 0 de 643
// por encima. La prueba de que es el cursor y no un borrado es el entrelazado — seq 709 presente,
// 711-717 ausentes, 718 PRESENTE, 719 y 721 ausentes, 722 presente.
//
// ⚠️ Y CÓMO NO SE VERIFICA: comparar el cursor contra max(sync_seq) del central y verlo «al día» NO
// prueba nada. El cursor llegando arriba es el síntoma, no la salud. La prueba honesta es cruzar los
// ids del universo pullable contra los de la base local.

// sembrarAjenas deja dos filas de `propio` con una de `ajeno` INTERCALADA entre ellas, que es la forma
// que tienen los datos reales. Devuelve el sync_seq de la fila ajena y el de la propia que va después.
func sembrarAjenas(t *testing.T, e *DbEngine) (seqAjena, seqPosterior int64) {
	t.Helper()
	orden := []struct {
		id, proyecto string
	}{
		{"propia-1", "propio"},
		{"ajena-1", "ajeno"},
		{"propia-2", "propio"},
	}
	for _, o := range orden {
		if _, err := e.IngestShared(SharedObs{
			ID: o.id, TopicKey: "t/x", Content: "contenido de " + o.id,
			Importance: 1, MemType: "semantic", Author: "otro", ProjectID: o.proyecto,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.db.QueryRow(`SELECT sync_seq FROM observations WHERE id='ajena-1'`).Scan(&seqAjena); err != nil {
		t.Fatal(err)
	}
	if err := e.db.QueryRow(`SELECT sync_seq FROM observations WHERE id='propia-2'`).Scan(&seqPosterior); err != nil {
		t.Fatal(err)
	}
	if seqAjena >= seqPosterior {
		t.Fatalf("la siembra no quedó entrelazada: ajena=%d posterior=%d", seqAjena, seqPosterior)
	}
	return seqAjena, seqPosterior
}

// TestPullAcotadoSaltaLaFilaAjena afirma que un pull acotado a un proyecto devuelve la fila POSTERIOR
// a una ajena sin devolver la ajena — o sea que el cursor que el cliente va a adoptar queda por encima
// de una fila que nunca vio.
//
// CUANDO EL DEFECTO SE CIERRE, este test tiene que cambiar de forma: el pull acotado puede seguir sin
// entregar la ajena (es correcto: no le corresponde), pero el cursor que se le comunica al cliente NO
// puede quedar por encima de ella. O sea: el next_cursor deja de salir de las filas devueltas.
func TestPullAcotadoSaltaLaFilaAjena(t *testing.T) {
	e := newTestEngine(t)
	seqAjena, seqPosterior := sembrarAjenas(t, e)

	acotado := WithProjectScope(context.Background(), ProjectScope{ProjectID: "propio"})
	items, err := e.ListSharedForPull(acotado, 0, 200)
	if err != nil {
		t.Fatalf("ListSharedForPull: %v", err)
	}

	var maxDevuelto int64
	vioAjena := false
	for _, it := range items {
		if it.RowID > maxDevuelto {
			maxDevuelto = it.RowID
		}
		if it.ID == "ajena-1" {
			vioAjena = true
		}
	}
	if vioAjena {
		t.Fatal("precondición: el pull acotado no debería entregar la fila de otro proyecto")
	}
	if maxDevuelto != seqPosterior {
		t.Fatalf("esperaba que el máximo devuelto fuera el de la fila posterior (%d), fue %d", seqPosterior, maxDevuelto)
	}
	// Ésta es la afirmación que documenta el defecto: el cursor que el cliente va a guardar
	// (el máximo devuelto) queda POR ENCIMA de una fila que no se entregó.
	if maxDevuelto <= seqAjena {
		t.Errorf("el cursor devuelto (%d) NO pasó por encima de la fila ajena (%d): el next_cursor dejó de salir de las filas devueltas, así que la reparación del cliente pasó a ser redundante — revisala", maxDevuelto, seqAjena)
	}
}

// TestElSaltoNoSeCuraSoloConElCursorArriba fija que el SALTO sigue estando en el SQL: con el cursor ya
// adelantado, ensanchar el alcance no hace que `ListSharedForPull` devuelva la fila saltada. Eso no
// cambió y no tiene por qué cambiar — la consulta hace lo que se le pide.
//
// ⚠️ ESTE TEST SE LLAMABA «EsIrreversible» Y ESO YA NO ES CIERTO A NIVEL SISTEMA. La reparación existe
// y vive en el CLIENTE, no acá: el central declara su recorte en cada pull (`alcance`) y el cliente
// reinicia el cursor cuando cambia, con memory.ReiniciarBajadaPorAlcance. La prueba de punta a punta
// está en internal/mcp/inbound_alcance_test.go (TestEnsancharElAlcanceRecuperaLoSalteado).
//
// Se conserva porque es la mitad que explica POR QUÉ hace falta el reinicio: si alguien algún día
// arregla el salto en el SQL —haciendo que el `next_cursor` salga de lo escaneado en vez de lo
// devuelto—, este test se va a poner rojo, y ahí la reparación del cliente pasa a ser redundante.
func TestElSaltoNoSeCuraSoloConElCursorArriba(t *testing.T) {
	e := newTestEngine(t)
	seqAjena, seqPosterior := sembrarAjenas(t, e)

	// La credencial se ensancha: ahora es federada y el filtro de proyecto desaparece.
	// Pero el cursor quedó donde lo dejó el pull acotado.
	federado := context.Background()
	items, err := e.ListSharedForPull(federado, seqPosterior, 200)
	if err != nil {
		t.Fatalf("ListSharedForPull: %v", err)
	}
	for _, it := range items {
		if it.ID == "ajena-1" {
			t.Errorf("la fila ajena volvió con el cursor en %d: el salto se arregló en el SQL, así que la reparación del cliente (ReiniciarBajadaPorAlcance) pasó a ser redundante — revisala", seqPosterior)
		}
	}

	// Y la contraprueba de que el contenido SÍ está en el central y sería entregable con el cursor
	// en el lugar correcto: pedirlo desde debajo de la fila ajena la trae.
	desdeAbajo, err := e.ListSharedForPull(federado, seqAjena-1, 200)
	if err != nil {
		t.Fatalf("ListSharedForPull: %v", err)
	}
	encontrada := false
	for _, it := range desdeAbajo {
		if it.ID == "ajena-1" {
			encontrada = true
		}
	}
	if !encontrada {
		t.Fatal("la fila ajena no es entregable ni con el cursor debajo: la siembra o el filtro no son lo que este test cree")
	}
}
