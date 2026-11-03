package memory

import (
	"testing"
)

// shadow_cobertura_test.go custodia ShadowCoverage: que cada balde tenga su MOTIVO y que los cuatro
// números cierren entre sí.
//
// Por qué hay un custodio por COLUMNA de visibilidad y no uno solo del predicado: el expediente
// anterior (`fix/deteccion-no-pisa-el-veredicto`) midió que sabotear la condición COMPARTIDA
// enrojece cualquier test, mientras que recortar UNA sola columna sobrevivía a los once custodios y
// al paquete entero. Un predicado de tres columnas necesita tres sabotajes, no uno.

// sembrarObsCob crea una observación visible con el id dado.
func sembrarObsCob(t *testing.T, e *DbEngine, id string) {
	t.Helper()
	if err := e.SaveObservationTyped(id, "cobertura/sombra", "contenido de prueba para "+id, 1.0, "", "shared", nil); err != nil {
		t.Fatalf("SaveObservationTyped(%s): %v", id, err)
	}
}

// relacionCob siembra las dos puntas y el par, con el veredicto que se le pida.
func relacionCob(t *testing.T, e *DbEngine, src, tgt, relation, status, resolvedBy string) {
	t.Helper()
	sembrarObsCob(t, e, src)
	sembrarObsCob(t, e, tgt)
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: src, TargetID: tgt, Relation: relation,
		Status: status, ResolvedBy: resolvedBy, Confidence: 0.8,
	}); err != nil {
		t.Fatalf("UpsertObsRelation(%s->%s): %v", src, tgt, err)
	}
}

// supersedesAplicado siembra un `supersedes` Y le aplica el efecto, que es lo que deja la huella.
//
// Hay que aplicarlo a mano porque UpsertObsRelation NO dispara markSuperseded: lo hacen
// DetectRelations cuando el detector se auto-resuelve (internal/memory/conflicts.go:377) y ResolveObsRelation
// cuando alguien arbitra (relations_obs.go:253). Llamar al mismo markSuperseded que usan esos dos
// caminos deja el test fijado al EFECTO y no a la forma del UPDATE que lo produce.
func supersedesAplicado(t *testing.T, e *DbEngine, src, tgt, resolvedBy string) {
	t.Helper()
	relacionCob(t, e, src, tgt, RelSupersedes, RelStatusResolved, resolvedBy)
	if err := e.markSuperseded(tgt, src); err != nil {
		t.Fatalf("markSuperseded(%s, %s): %v", tgt, src, err)
	}
}

// TestShadowCoberturaSeparaLosBaldesPorMotivo cubre R4 y R5: los dos caminos que hacen medible a un
// par, los dos motivos de recorte, y el subconjunto que explica por qué la firma propia tiende a
// cero sola.
// Sabotaje que la hace fallar: cambiar la firma del balde medible por una AJENA, que es la que no se puede medir.
// También la pisa el custodio por celdas, y son dos guardas distintas: medido el 2026-11-02,
// con el `a` de allá puesto este test queda VERDE y sólo cae el de celdas. Si fueran la misma
// guarda contada dos veces, caerían las dos.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="COALESCE(r.resolved_by,\x27\x27) = '` + RelResolvedByHeuristic + `')"
// arnes: a="COALESCE(r.resolved_by,\x27\x27) = 'agent')"
// arnes: colision_ok="TestShadowCoberturaClasificaCadaFirmaPorSeparado"
// Sabotaje que la hace fallar: anular la rama `pending` del predicado medible y dejar sólo la firma.
// Pisa el `de` del sabotaje siguiente y es otra guarda, no la misma contada dos veces: éste mide que
// la rama EXISTA —anularla desbalancea los baldes— y el otro que mida la constante CORRECTA, que es
// un defecto sin desbalance. Medido: con la rama simétrica, éste caía rojo y el otro pasaba verde.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="medible := `(r.status = '` + RelStatusPending + `' OR "
// arnes: a="medible := `(1 = 0 OR "
// arnes: colision_ok="TestShadowCoberturaSeparaLosBaldesPorMotivo TestShadowCoberturaClasificaCadaFirmaPorSeparado"
// Sabotaje que la hace fallar: confundir el status medible con otro de la MISMA familia. Anular la
// rama (el sabotaje de arriba) desbalancea y por eso enrojece; confundir la constante PERMUTA —saca
// la fila `pending` del balde y mete una `resolved` que estaba en firma ajena— y con la rama
// simétrica pasaba VERDE en los dos paquetes. Es el caso que R4 nombra: «si el filtro de
// procedencia cambia, la cobertura DEBE cambiar con él».
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="`(r.status = '` + RelStatusPending + `' OR COALESCE"
// arnes: a="`(r.status = '` + RelStatusResolved + `' OR COALESCE"
// arnes: colision_ok="TestShadowCoberturaSeparaLosBaldesPorMotivo TestShadowCoberturaClasificaCadaFirmaPorSeparado"
// Sabotaje que la hace fallar: contar las auto-aplicadas por el EFECTO solo, sin exigir la firma del detector.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="WHERE COALESCE(r2.resolved_by,\x27\x27) = '`+RelResolvedByHeuristic+`'\n\t\t\t    AND b2.superseded_by = r2.source_id)"
// arnes: a="WHERE b2.superseded_by = r2.source_id)"
// arnes: colision_ok="TestShadowCoberturaSeparaLosBaldesPorMotivo TestShadowCoberturaAutoAplicadaExigeLaFirmaYElEfectoPropio"
// Sabotaje que la hace fallar: contar las auto-aplicadas por la FIRMA sola, sin exigir que el supersedes se haya aplicado.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="\n\t\t\t    AND b2.superseded_by = r2.source_id)"
// arnes: a=")"
// arnes: colision_ok="TestShadowCoberturaSeparaLosBaldesPorMotivo TestShadowCoberturaAutoAplicadaExigeLaFirmaYElEfectoPropio"
func TestShadowCoberturaSeparaLosBaldesPorMotivo(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	// (1) medible por PROPUESTA del detector: status pending.
	relacionCob(t, e, "p-src", "p-tgt", RelPending, RelStatusPending, "")
	// (1b) OTRA propuesta pendiente, por el MISMO motivo que (2b) y medido igual de caro: con una
	//      sola `pending` contra una firma ajena, confundir el status del balde por otro de la misma
	//      familia INTERCAMBIA las dos filas —sale la `pending`, entra la `resolved` ajena— y los
	//      seis contadores quedan idénticos, así que la permutación pasaba VERDE en los dos
	//      paquetes enteros. Con dos contra una, cualquier status al que se mute desbalancea.
	relacionCob(t, e, "p2-src", "p2-tgt", RelPending, RelStatusPending, "")
	// (2) medible por FIRMA PROPIA sin efecto aplicado: un auto-related no oculta nada, así que el
	//     par sigue siendo re-detectable. Es el caso que demuestra que el cero de la base viva no
	//     es una propiedad de toda firma propia, sólo del auto-supersedes.
	relacionCob(t, e, "h-src", "h-tgt", RelRelated, RelStatusResolved, RelResolvedByHeuristic)
	// (2b) OTRA firma propia sin efecto. No es redundante con (2), y sacarla deja un agujero
	//      medido: con un medible-por-firma y una firma ajena, mutar la constante del balde a la
	//      firma ajena INTERCAMBIA las dos filas y los totales no se mueven, así que el sabotaje
	//      de R4 pasaba VERDE. Con dos contra una, cualquier firma a la que se mute desbalancea.
	relacionCob(t, e, "h2-src", "h2-tgt", RelRelated, RelStatusResolved, RelResolvedByHeuristic)
	// (3) FIRMA AJENA: lo arbitró un agente, la guarda del upsert lo protege y medirlo anotaría esa
	//     decisión como acierto del heurístico.
	relacionCob(t, e, "a-src", "a-tgt", RelRelated, RelStatusResolved, "agent")
	// (4) AUTO-RESOLUCIÓN APLICADA: al aplicarse, el `supersedes` oculta su propia target, así que
	//     el par sale de lo re-detectable POR SU PROPIO EFECTO. Está firmado por el DETECTOR, que
	//     es lo que lo vuelve indistinguible de un medible si el balde se decidiera por la firma.
	supersedesAplicado(t, e, "s-src", "s-tgt", RelResolvedByHeuristic)
	// (5) SUPERSEDES AJENO YA APLICADO: mismo efecto que (4) —la target quedó invisible— pero lo
	//     decidió un agente, así que NO es una auto-resolución del detector. Es la mayoría del
	//     corpus real (medido el 2026-11-02: de los 65 pares `supersedes` del repo, 64 tienen el
	//     target supersedido por su source y sólo 2 están firmados por el detector), y es el caso
	//     que distingue contar por la FIRMA de contar por el efecto solo.
	supersedesAplicado(t, e, "sa-src", "sa-tgt", "agent")

	cob, err := e.ShadowCoverage()
	if err != nil {
		t.Fatalf("ShadowCoverage: %v", err)
	}
	for _, c := range []struct {
		que      string
		got, exp int
	}{
		{"Relaciones", cob.Relaciones, 7},
		{"Redetectables", cob.Redetectables, 5},
		{"Medibles", cob.Medibles, 4},
		{"FirmaAjena", cob.FirmaAjena, 1},
		{"PuntaNoVisible", cob.PuntaNoVisible, 2},
		// Dos pares perdieron la target y sólo UNO es auto-resolución del detector: el otro lo
		// arbitró un agente. Si el balde se contara por el efecto solo, acá habría 2.
		{"AutoAplicadas", cob.AutoAplicadas, 1},
	} {
		if c.got != c.exp {
			t.Errorf("%s = %d, esperaba %d (cobertura completa: %+v)", c.que, c.got, c.exp, cob)
		}
	}
}

// TestShadowCoberturaNoConfundeAutoAplicadaConFirmaAjena custodia la mitad de R5 que un solo
// contador no puede distinguir: el auto-supersedes está firmado POR EL DETECTOR, así que si el
// balde se decidiera por la firma caería en FirmaAjena y el reporte diría que la guarda lo protegió
// —que es falso—. Lo que lo saca del conjunto es que su propio efecto ocultó la target.
func TestShadowCoberturaNoConfundeAutoAplicadaConFirmaAjena(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	supersedesAplicado(t, e, "solo-src", "solo-tgt", RelResolvedByHeuristic)

	cob, err := e.ShadowCoverage()
	if err != nil {
		t.Fatalf("ShadowCoverage: %v", err)
	}
	if cob.FirmaAjena != 0 {
		t.Errorf("FirmaAjena = %d, esperaba 0: el auto-supersedes es firma PROPIA del detector", cob.FirmaAjena)
	}
	if cob.AutoAplicadas != 1 {
		t.Errorf("AutoAplicadas = %d, esperaba 1 (cobertura: %+v)", cob.AutoAplicadas, cob)
	}
	if cob.PuntaNoVisible != 1 {
		t.Errorf("PuntaNoVisible = %d, esperaba 1: AutoAplicadas es su SUBCONJUNTO", cob.PuntaNoVisible)
	}
	if cob.AutoAplicadas > cob.PuntaNoVisible {
		t.Errorf("AutoAplicadas (%d) no puede superar PuntaNoVisible (%d): es un subconjunto",
			cob.AutoAplicadas, cob.PuntaNoVisible)
	}
}

// TestShadowCoberturaExigeVisibilidadColumnaPorColumna custodia R3, y lo hace por COLUMNA y por
// PUNTA: tres columnas × dos puntas = seis sabotajes posibles, y una copia del predicado a la que
// le falte una sola columna tiene que enrojecer en su celda. Es el caso que `scope.go` documenta:
// el único sitio del repo que escribió el predicado a mano se olvidó `quarantined = 0`.
// Sabotaje que la hace fallar: sacarle `archived` al predicado de visibilidad de las dos puntas.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="`+visibleObsPredicateDe(\"a\")+` AND `+visibleObsPredicateDe(\"b\")"
// arnes: a="`+\"a.superseded_by IS NULL AND a.quarantined = 0\"+` AND `+\"b.superseded_by IS NULL AND b.quarantined = 0\""
// arnes: colision_ok="TestShadowCoberturaExigeVisibilidadColumnaPorColumna"
// Sabotaje que la hace fallar: sacarle `superseded_by` al predicado de visibilidad de las dos puntas.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="`+visibleObsPredicateDe(\"a\")+` AND `+visibleObsPredicateDe(\"b\")"
// arnes: a="`+\"a.archived = 0 AND a.quarantined = 0\"+` AND `+\"b.archived = 0 AND b.quarantined = 0\""
// arnes: colision_ok="TestShadowCoberturaExigeVisibilidadColumnaPorColumna"
// Sabotaje que la hace fallar: sacarle `quarantined` al predicado de visibilidad de las dos puntas.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="`+visibleObsPredicateDe(\"a\")+` AND `+visibleObsPredicateDe(\"b\")"
// arnes: a="`+\"a.archived = 0 AND a.superseded_by IS NULL\"+` AND `+\"b.archived = 0 AND b.superseded_by IS NULL\""
// arnes: colision_ok="TestShadowCoberturaExigeVisibilidadColumnaPorColumna"
func TestShadowCoberturaExigeVisibilidadColumnaPorColumna(t *testing.T) {
	casos := []struct {
		nombre string
		sql    string // lo que vuelve invisible a la punta
		punta  string // "src" o "tgt"
	}{
		{"source archivada", `UPDATE observations SET archived = 1 WHERE id = ?`, "src"},
		{"source supersedida", `UPDATE observations SET superseded_by = 'otra' WHERE id = ?`, "src"},
		{"source en cuarentena", `UPDATE observations SET quarantined = 1 WHERE id = ?`, "src"},
		{"target archivada", `UPDATE observations SET archived = 1 WHERE id = ?`, "tgt"},
		{"target supersedida", `UPDATE observations SET superseded_by = 'otra' WHERE id = ?`, "tgt"},
		{"target en cuarentena", `UPDATE observations SET quarantined = 1 WHERE id = ?`, "tgt"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			e, err := NewDbEngine(t.TempDir())
			if err != nil {
				t.Fatalf("NewDbEngine: %v", err)
			}
			defer e.Close()

			// Un par que SIN el sabotaje sería medible: pending, las dos puntas visibles.
			relacionCob(t, e, "v-src", "v-tgt", RelPending, RelStatusPending, "")
			objetivo := "v-src"
			if c.punta == "tgt" {
				objetivo = "v-tgt"
			}
			if _, err := e.db.Exec(c.sql, objetivo); err != nil {
				t.Fatalf("volver invisible a %s: %v", objetivo, err)
			}

			cob, err := e.ShadowCoverage()
			if err != nil {
				t.Fatalf("ShadowCoverage: %v", err)
			}
			if cob.Redetectables != 0 {
				t.Errorf("Redetectables = %d, esperaba 0: con la %s invisible la detección no vuelve a emitir el par",
					cob.Redetectables, c.punta)
			}
			if cob.Medibles != 0 {
				t.Errorf("Medibles = %d, esperaba 0: un par que no se re-detecta no se puede medir", cob.Medibles)
			}
			if cob.PuntaNoVisible != 1 {
				t.Errorf("PuntaNoVisible = %d, esperaba 1 (cobertura: %+v)", cob.PuntaNoVisible, cob)
			}
		})
	}
}

// TestShadowCoberturaLaAritmeticaCierra custodia que los baldes SUMEN, que es un chequeo de
// CONSISTENCIA y no de correctitud. La diferencia se midió y conviene no confundirla: contando
// fuentes distintas en vez de relaciones el titular cae de 901 a 394 —un error del 56 %— y las dos
// identidades CIERRAN IGUAL, porque `PuntaNoVisible` se deriva por resta y los dos `CASE` son
// complementos exactos. Lo que hace que los números estén bien NO es la aritmética: es que
// `observations.id` es PRIMARY KEY, así que los dos JOIN no pueden multiplicar filas.
// Sabotaje que la hace fallar: clavar `PuntaNoVisible` en cero en vez de derivarlo de la resta.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="r.PuntaNoVisible = r.Relaciones - r.Redetectables"
// arnes: a="r.PuntaNoVisible = 0"
func TestShadowCoberturaLaAritmeticaCierra(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	relacionCob(t, e, "m1-src", "m1-tgt", RelPending, RelStatusPending, "")
	relacionCob(t, e, "m2-src", "m2-tgt", RelRelated, RelStatusResolved, RelResolvedByHeuristic)
	relacionCob(t, e, "aj-src", "aj-tgt", RelConflictsWith, RelStatusResolved, "user")
	supersedesAplicado(t, e, "ap-src", "ap-tgt", RelResolvedByHeuristic)
	relacionCob(t, e, "q-src", "q-tgt", RelPending, RelStatusPending, "")
	if _, err := e.db.Exec(`UPDATE observations SET quarantined = 1 WHERE id = 'q-tgt'`); err != nil {
		t.Fatalf("cuarentena: %v", err)
	}

	cob, err := e.ShadowCoverage()
	if err != nil {
		t.Fatalf("ShadowCoverage: %v", err)
	}
	if cob.Medibles+cob.FirmaAjena != cob.Redetectables {
		t.Errorf("Medibles(%d) + FirmaAjena(%d) = %d, tiene que dar Redetectables(%d)",
			cob.Medibles, cob.FirmaAjena, cob.Medibles+cob.FirmaAjena, cob.Redetectables)
	}
	if cob.Redetectables+cob.PuntaNoVisible != cob.Relaciones {
		t.Errorf("Redetectables(%d) + PuntaNoVisible(%d) = %d, tiene que dar Relaciones(%d)",
			cob.Redetectables, cob.PuntaNoVisible, cob.Redetectables+cob.PuntaNoVisible, cob.Relaciones)
	}
	if cob.AutoAplicadas > cob.PuntaNoVisible {
		t.Errorf("AutoAplicadas(%d) > PuntaNoVisible(%d): tiene que ser un subconjunto",
			cob.AutoAplicadas, cob.PuntaNoVisible)
	}
}

// TestShadowCoberturaCorpusVacioNoFalla custodia el COALESCE: con cero filas re-detectables los dos
// SUM devuelven NULL, no 0, y el Scan a un int fallaría. Un corpus recién nacido tiene que informar
// la verdad —no hay nada medible— en vez de devolver un error que se lee como base corrupta.
// Sabotaje que la hace fallar: sacarle el COALESCE a los dos SUM, que con cero filas devuelven NULL y no 0.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="COALESCE(SUM(CASE WHEN `+medible+` THEN 1 ELSE 0 END), 0),\n\t\t\tCOALESCE(SUM(CASE WHEN `+medible+` THEN 0 ELSE 1 END), 0),"
// arnes: a="SUM(CASE WHEN `+medible+` THEN 1 ELSE 0 END),\n\t\t\tSUM(CASE WHEN `+medible+` THEN 0 ELSE 1 END),"
func TestShadowCoberturaCorpusVacioNoFalla(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	cob, err := e.ShadowCoverage()
	if err != nil {
		t.Fatalf("ShadowCoverage sobre corpus vacío: %v", err)
	}
	if cob != (ShadowCoverageReport{}) {
		t.Errorf("cobertura de un corpus vacío = %+v, esperaba todo en cero", cob)
	}
}

// TestSaveShadowVerdictComputaAgreeAdentro custodia R8, que hasta hoy no tenía ninguno: los ocho
// tests de sombra viven en `internal/mcp` y prueban el worker y el filtro de procedencia, no el
// cómputo de la columna de la que sale la TASA que el comando imprime.
//
// Qué enrojece si alguien mueve el cómputo al borde: el struct necesitaría un campo `Agree` que
// este test no setea, así que el caso coincidente pasaría a devolver 0 y la fila diría que el motor
// discrepó cuando dijo exactamente lo mismo. Ésa es la falla que importa — una tasa que miente por
// debajo— y es silenciosa, porque la tabla sigue sumando bien.
// Sabotaje que la hace fallar: clavar `agree` en 1 en vez de derivarlo de comparar las dos relaciones.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="agree := 0\n\tif v.HeurRelation == v.JudgeRelation {\n\t\tagree = 1\n\t}"
// arnes: a="agree := 1"
func TestSaveShadowVerdictComputaAgreeAdentro(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	casos := []struct {
		relID, heur, judge string
		esperaAgree        int
	}{
		{"rel-coincide", RelSupersedes, RelSupersedes, 1},
		{"rel-discrepa", RelSupersedes, RelRelated, 0},
		// El motor contestó algo que la normalización dejó vacío: no coincide, y no por eso deja de
		// contarse. Si `agree` se computara sobre JudgeRaw en vez de JudgeRelation, este caso
		// seguiría dando 0 y no informaría; está acá para fijar que la comparación es sobre el
		// vocabulario normalizado, que es lo que la tabla agrupa.
		{"rel-sin-lectura", RelRelated, "", 0},
	}
	for _, c := range casos {
		if err := e.SaveShadowVerdict(ShadowVerdict{
			RelationID: c.relID, SourceID: "x", TargetID: "y",
			HeurRelation: c.heur, HeurStatus: RelStatusResolved,
			JudgeRelation: c.judge, JudgeRaw: "lo que contestó el motor", JudgeModel: "modelo-de-prueba",
		}); err != nil {
			t.Fatalf("SaveShadowVerdict(%s): %v", c.relID, err)
		}
	}
	for _, c := range casos {
		var agree int
		if err := e.db.QueryRow(`SELECT agree FROM shadow_verdicts WHERE relation_id = ?`, c.relID).Scan(&agree); err != nil {
			t.Fatalf("leer agree de %s: %v", c.relID, err)
		}
		if agree != c.esperaAgree {
			t.Errorf("%s: agree = %d, esperaba %d (heur=%q judge=%q)", c.relID, agree, c.esperaAgree, c.heur, c.judge)
		}
	}
}

// TestShadowCoberturaClasificaCadaFirmaPorSeparado cubre R4 donde el fixture AGREGADO no puede: con
// UNA fila por celda y un motor FRESCO en cada una, no hay forma de que una mutación compense un
// balde con otro.
//
// 🔑 POR QUÉ UNA CELDA POR FIRMA Y NO UNA FILA MÁS EN EL FIXTURE AGREGADO. El agregado de
// TestShadowCoberturaSeparaLosBaldesPorMotivo tiene P=2 pendientes, H=2 firmadas por el detector y
// A=1 ajena, y declara Medibles=4 / FirmaAjena=1. Medido en la verify: mutar la constante del balde
// a `<> 'agent'` deja esos dos números IDÉNTICOS, porque el fixture sólo contiene UNA firma ajena y
// justo la que la mutación nombra, así que `<> 'agent'` y `= 'heuristic'` son el MISMO predicado
// sobre ese corpus. Y agregarle una fila por cada firma que falta EMPEORA el problema en vez de
// arreglarlo: con P=2, H=2, A=1, U=1 aparece una unión de igual tamaño —H+A+U = 4 = P+H—, así que un
// predicado que midiera «tiene cualquier firma» devolvería los mismos 4 medibles y 2 ajenos. El
// agregado no puede distinguir conjuntos por su tamaño cuando dos subconjuntos distintos suman lo
// mismo; con una sola fila no hay suma que empate.
//
// Las firmas de la tabla no son inventadas: relations_obs.go:49-51 las enumera —«"agent", "user",
// "afilador", "destilador", el que le pase el caller a RestoreDuplicate»— y el repo las escribe en
// methods_dedup.go:86 y methods_distill.go:52. R4 (spec.md:25-28) pide que el criterio se derive de
// las MISMAS constantes que usa encolarSombra: «si el filtro de procedencia cambia, la cobertura
// DEBE cambiar con él». Una cobertura que mide «no es de un agente» en vez de «es del detector»
// cuenta como medible todo lo que firmó una persona, el afilador o el destilador.
// Sabotaje que la hace fallar: medir por lo que NO es —«no lo firmó un agente»— en vez de por la firma del detector.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="COALESCE(r.resolved_by,\x27\x27) = '` + RelResolvedByHeuristic + `')"
// arnes: a="COALESCE(r.resolved_by,\x27\x27) <> 'agent')"
// arnes: colision_ok="TestShadowCoberturaSeparaLosBaldesPorMotivo"
// Sabotaje que la hace fallar: decidir la rama de la propuesta por la AUSENCIA de firma en vez de por el status.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="medible := `(r.status = '` + RelStatusPending + `' OR "
// arnes: a="medible := `(COALESCE(r.resolved_by,\x27\x27) = \x27\x27 OR "
// arnes: colision_ok="TestShadowCoberturaSeparaLosBaldesPorMotivo"
func TestShadowCoberturaClasificaCadaFirmaPorSeparado(t *testing.T) {
	celdas := []struct {
		nombre     string
		status     string
		resolvedBy string
		medible    bool
	}{
		// Los dos caminos que SÍ hacen medible a un par.
		{"propuesta pendiente del detector", RelStatusPending, "", true},
		{"firma del detector", RelStatusResolved, RelResolvedByHeuristic, true},
		// Las firmas ajenas, una por una. `agent` es la única que el fixture agregado contiene, y
		// por eso es la única contra la que una mutación «no es de un agente» no se delata.
		{"arbitrada por un agente", RelStatusResolved, "agent", false},
		{"arbitrada por una persona", RelStatusResolved, "user", false},
		{"arbitrada por el afilador", RelStatusResolved, "afilador", false},
		{"arbitrada por el destilador", RelStatusResolved, "destilador", false},
		// Resuelta y SIN firma: el esquema lo admite y hoy nadie lo produce, pero es la única celda
		// que distingue «el status es pending» de «no tiene firma». Sin ella, cambiar la primera
		// rama del predicado por `resolved_by = ''` no mueve un solo contador, porque en todas las
		// demás celdas las dos condiciones coinciden. Es un caso de CONTRATO, igual que el corpus
		// vacío de más abajo: fija cuál de las dos columnas decide.
		{"resuelta sin firma", RelStatusResolved, "", false},
	}
	for _, c := range celdas {
		t.Run(c.nombre, func(t *testing.T) {
			// Motor fresco por celda: es lo que impide que un balde compense a otro.
			e, err := NewDbEngine(t.TempDir())
			if err != nil {
				t.Fatalf("NewDbEngine: %v", err)
			}
			defer e.Close()

			relacionCob(t, e, "u-src", "u-tgt", RelRelated, c.status, c.resolvedBy)

			cob, err := e.ShadowCoverage()
			if err != nil {
				t.Fatalf("ShadowCoverage: %v", err)
			}
			// Las dos puntas están visibles en todas las celdas: lo único que cambia es el balde.
			if cob.Redetectables != 1 {
				t.Fatalf("Redetectables = %d, esperaba 1: la celda tiene sus dos puntas visibles (cobertura: %+v)",
					cob.Redetectables, cob)
			}
			medibles, ajena := 0, 1
			if c.medible {
				medibles, ajena = 1, 0
			}
			if cob.Medibles != medibles {
				t.Errorf("Medibles = %d, esperaba %d para status=%q resolved_by=%q (cobertura: %+v)",
					cob.Medibles, medibles, c.status, c.resolvedBy, cob)
			}
			if cob.FirmaAjena != ajena {
				t.Errorf("FirmaAjena = %d, esperaba %d para status=%q resolved_by=%q (cobertura: %+v)",
					cob.FirmaAjena, ajena, c.status, c.resolvedBy, cob)
			}
		})
	}
}

// TestShadowCoberturaAutoAplicadaExigeLaFirmaYElEfectoPropio cubre R5c —«el target está supersedido
// por su PROPIO source» (spec.md:35)— celda por celda, por el mismo motivo que el test de arriba:
// el fixture agregado tiene UNA auto-aplicada del detector y UNA de un agente, y con ese corpus
// medir «lo firmó el detector» y medir «no lo firmó un agente» da el mismo 1.
//
// Las dos mitades del predicado se miden por separado porque fallan distinto: aflojar la FIRMA
// cuenta como auto-resolución lo que arbitró una persona, y aflojar el EFECTO cuenta como
// auto-resolución un supersedes que quedó apuntando a una tercera observación. Y ese segundo caso no
// es hipotético: medido el 2026-11-02 en la base viva de este repo, de los 66 pares `supersedes` con
// el target ya supersedido había 64 por su propio source y 2 POR OTRA observación.
// Sabotaje que la hace fallar: contar la auto-resolución por lo que NO es —«no lo firmó un agente»— en vez de por la firma del detector.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="WHERE COALESCE(r2.resolved_by,\x27\x27) = '`+RelResolvedByHeuristic+`'"
// arnes: a="WHERE COALESCE(r2.resolved_by,\x27\x27) <> 'agent'"
// arnes: colision_ok="TestShadowCoberturaSeparaLosBaldesPorMotivo"
// Sabotaje que la hace fallar: aceptar cualquier supersedido en vez de exigir que lo haya supersedido su PROPIO source.
// arnes: archivo="internal/memory/shadow.go"
// arnes: de="AND b2.superseded_by = r2.source_id)"
// arnes: a="AND b2.superseded_by IS NOT NULL)"
// arnes: colision_ok="TestShadowCoberturaSeparaLosBaldesPorMotivo"
func TestShadowCoberturaAutoAplicadaExigeLaFirmaYElEfectoPropio(t *testing.T) {
	celdas := []struct {
		nombre     string
		resolvedBy string
		porOtra    bool // el target quedó supersedido por una TERCERA observación, no por su source
		cuenta     bool
	}{
		{"auto-resolución del detector", RelResolvedByHeuristic, false, true},
		{"supersedes de un agente", "agent", false, false},
		// La celda que el fixture agregado no tiene: una firma ajena que NO es `agent`.
		{"supersedes de una persona", "user", false, false},
		{"supersedes del afilador", "afilador", false, false},
		// Firma correcta, efecto AJENO: el detector decidió, pero lo que dejó invisible al target
		// fue otra observación. No es una auto-resolución que surtió efecto.
		{"del detector pero supersedido por otra", RelResolvedByHeuristic, true, false},
	}
	for _, c := range celdas {
		t.Run(c.nombre, func(t *testing.T) {
			e, err := NewDbEngine(t.TempDir())
			if err != nil {
				t.Fatalf("NewDbEngine: %v", err)
			}
			defer e.Close()

			relacionCob(t, e, "ap-src", "ap-tgt", RelSupersedes, RelStatusResolved, c.resolvedBy)
			quien := "ap-src"
			if c.porOtra {
				// La tercera observación existe de verdad: el predicado compara ids, y un id
				// inventado dejaría el caso en pie por un motivo que no es el que se mide.
				sembrarObsCob(t, e, "tercera-obs")
				quien = "tercera-obs"
			}
			if err := e.markSuperseded("ap-tgt", quien); err != nil {
				t.Fatalf("markSuperseded(ap-tgt, %s): %v", quien, err)
			}

			cob, err := e.ShadowCoverage()
			if err != nil {
				t.Fatalf("ShadowCoverage: %v", err)
			}
			// En todas las celdas el target quedó invisible, así que el par sale de lo
			// re-detectable. Lo que se mide es si cae además en el SUBCONJUNTO de auto-aplicadas.
			if cob.PuntaNoVisible != 1 {
				t.Fatalf("PuntaNoVisible = %d, esperaba 1: el target quedó supersedido (cobertura: %+v)",
					cob.PuntaNoVisible, cob)
			}
			esperado := 0
			if c.cuenta {
				esperado = 1
			}
			if cob.AutoAplicadas != esperado {
				t.Errorf("AutoAplicadas = %d, esperaba %d para resolved_by=%q porOtra=%v (cobertura: %+v)",
					cob.AutoAplicadas, esperado, c.resolvedBy, c.porOtra, cob)
			}
			if cob.AutoAplicadas > cob.PuntaNoVisible {
				t.Errorf("AutoAplicadas(%d) > PuntaNoVisible(%d): tiene que ser un subconjunto",
					cob.AutoAplicadas, cob.PuntaNoVisible)
			}
		})
	}
}
