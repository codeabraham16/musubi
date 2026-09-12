package recalleval

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// EL RECALL TIENE QUE SER DETERMINISTA CUANDO EL REFUERZO ESTÁ APAGADO, Y ESO SOSTIENE AL BANCO
// ENTERO.
//
// Todo lo que este paquete mide —el A/B de señales, el barrido de MMR, la comparación pareada—
// descansa en una premisa que nadie estaba verificando: que dos corridas de la misma consulta
// sobre el mismo corpus dan el mismo orden. Si no, cada diferencia entre dos brazos mezcla el
// efecto real con ruido de ordenamiento, y no hay forma de separarlos después.
//
// POR QUÉ NO ES OBVIO QUE SE CUMPLA, medido el 2026-09-11 antes de escribir esto:
//
//   - `Recall` ESCRIBE lo que lee. `bumpAccess` actualiza `access_count` sobre lo que acaba de
//     devolver, y `accessRate` —la señal de frecuencia— lo usa como numerador. Corriendo la misma
//     consulta 12 veces con el refuerzo ENCENDIDO, 25 de 25 consultas del corpus real dieron DOS
//     órdenes distintos. Apagándolo con `NoBump`: 25 de 25 estables.
//   - Y el desempate final es `sort.SliceStable` sobre el orden de llegada del slice, no sobre un
//     criterio explícito. O sea que cualquier cosa que cambie el orden en que los candidatos se
//     hidratan —un `IN (...)` sin `ORDER BY`, un pool que se arme distinto— reordena los empates
//     sin tocar ningún puntaje.
//
// Esta guarda es lo que hace que «el brazo A ganó» signifique algo. Sin ella, el día que alguien
// meta concurrencia en el ranking o itere un map en el camino, todos los números de este paquete
// empiezan a mentir Y LAS PRUEBAS SIGUEN VERDES, porque ninguna otra compara dos corridas entre sí.
//
// SABOTAJE QUE LA HACE FALLAR, corrido y verificado: meter un map en el camino del ranking — en
// `rankCandidates` (internal/memory/recall.go), volcar `out` a un `map[string]scoredCandidate` y
// reconstruirlo iterándolo, justo antes del `sort.SliceStable`. Con eso `migration-en` y
// `backup-dr-es` se intercambian entre corridas y la guarda lo dice.
// arnes: prueba="TestElRecallEsDeterministaSinRefuerzo"
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\tsort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })"
// arnes: a="\tporID := make(map[string]scoredCandidate, len(out))\n\tfor _, sc := range out {\n\t\tporID[sc.id] = sc\n\t}\n\tout = out[:0]\n\tfor _, sc := range porID {\n\t\tout = append(out, sc)\n\t}\n\tsort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })"
//
// LA RECETA QUE NO SIRVE, y la dejo escrita porque fue la primera que se me ocurrió: sacar
// `opts.NoBump = true` de `RunFixture`. NO toca esta prueba, porque acá el NoBump se pone en el
// test mismo. Una guarda sólo vale contra un sabotaje que ejercite el CAMINO que dice cubrir, y
// el camino de ésta es el ranking, no la opción.
func TestElRecallEsDeterministaSinRefuerzo(t *testing.T) {
	fx, err := LoadFixture(filepath.Join("testdata", "golden.json"))
	if err != nil {
		t.Fatalf("fixture golden: %v", err)
	}
	e, err := SeedEngine(t.TempDir(), fx, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	defer e.Close()

	opts := OptsDeProduccion()
	opts.NoBump = true // lo mismo que hace RunFixture, y la razón por la que el banco es medible

	const vueltas = 8
	revisadas := 0
	for _, q := range fx.Queries {
		var primera string
		for v := 0; v < vueltas; v++ {
			res, err := e.Recall(context.Background(), q.Text, opts)
			if err != nil {
				t.Fatalf("recall de %q: %v", q.ID, err)
			}
			ids := make([]string, 0, len(res.Items))
			for _, it := range res.Items {
				ids = append(ids, it.ID)
			}
			firma := strings.Join(ids, ",")
			if v == 0 {
				primera = firma
				continue
			}
			if firma != primera {
				t.Errorf("la consulta %q devolvió DOS órdenes distintos contra el mismo engine:\n  corrida 1: %s\n  corrida %d: %s\n"+
					"  Todo lo que mide este paquete compara dos corridas; si una consulta sola ya es inestable, ninguna diferencia entre brazos es atribuible.",
					q.ID, primera, v+1, firma)
				break
			}
		}
		// Una consulta que no devuelve NADA es estable por vacía y no prueba nada. Se cuenta sólo
		// lo que de verdad ejercitó el ranking, y abajo se exige un piso.
		if primera != "" {
			revisadas++
		}
	}
	if revisadas < 3 {
		t.Fatalf("sólo %d consulta(s) del golden devolvieron resultados: esta guarda no midió nada. ¿Cambió el fixture?", revisadas)
	}
	t.Logf("%d consultas del golden, %d corridas cada una, mismo orden siempre", revisadas, vueltas)
}
