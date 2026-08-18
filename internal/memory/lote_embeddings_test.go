package memory

import (
	"fmt"
	"strings"
	"testing"
)

// lote_embeddings_test.go defiende el invariante que hace SEGURO embeber en lote: los vectores se
// aparean con las observaciones POR ÍNDICE, así que un lote que devuelve una cantidad distinta a
// la pedida no es un error visible — CORRE los vectores una posición y le escribe a cada
// observación el embedding de OTRA. La memoria queda semánticamente barajada, el recall empieza a
// traer cosas ajenas, y no hay una sola línea de log que lo explique.

func sembrar(t *testing.T, e *DbEngine, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		id := string(rune('a'+i)) + "-obs"
		if err := e.SaveObservation(id, "t/x", "contenido de la observacion "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	e.SetVectorModelID("static:tabla@aaaa")
}

// ⚠️ B1 — UN LOTE CORTO ABORTA ANTES DE ESCRIBIR NADA. Es el test central del archivo.
func TestB1UnLoteQueDevuelveDeMenosAborta(t *testing.T) {
	e := newTestEngine(t)
	sembrar(t, e, 3)

	// Devuelve UN vector de menos, que es justo lo que produce el desfasaje silencioso.
	corto := func(textos []string) ([][]float32, error) {
		out := make([][]float32, 0, len(textos))
		for range textos[:len(textos)-1] {
			out = append(out, []float32{1, 0, 0})
		}
		return out, nil
	}

	_, err := e.EmbedBackfill(corto)
	if err == nil {
		t.Fatal("un lote que devuelve de menos DEBE abortar: si no, aparea vectores con observaciones equivocadas")
	}
	if !strings.Contains(err.Error(), "vectores") {
		t.Errorf("el error no explica el problema: %v", err)
	}
	// Y NO escribió nada: abortar a medias sería peor que no abortar.
	if n := countEmbeddingsWithModel(t, e, "static:tabla@aaaa"); n != 0 {
		t.Errorf("abortó pero ya había escrito %d vector(es): la guarda tiene que actuar ANTES de persistir", n)
	}
}

// B2 — Un lote LARGO también aborta. La guarda es sobre la CUENTA, no sobre «faltan»: de más
// también desalinea, y además delata un proveedor que no respeta el contrato.
func TestB2UnLoteQueDevuelveDeMasTambienAborta(t *testing.T) {
	e := newTestEngine(t)
	sembrar(t, e, 2)

	largo := func(textos []string) ([][]float32, error) {
		out := make([][]float32, 0, len(textos)+1)
		for i := 0; i <= len(textos); i++ {
			out = append(out, []float32{1, 0, 0})
		}
		return out, nil
	}

	if _, err := e.EmbedBackfill(largo); err == nil {
		t.Fatal("un lote que devuelve de MÁS también rompe el apareo por índice")
	}
}

// B3 — El camino feliz sigue funcionando y respeta el apareo: cada observación recibe SU vector.
// Sin esto, los dos tests de arriba se podrían satisfacer abortando siempre.
func TestB3CadaObservacionRecibeSuVector(t *testing.T) {
	e := newTestEngine(t)
	const n = 5
	sembrar(t, e, n)

	// Cada texto devuelve un vector DISTINGUIBLE, derivado de su propio contenido: si el apareo se
	// corriera una posición, el vector guardado no sería el que le toca y el chequeo lo ve.
	vistos := map[string]float32{}
	porTexto := func(textos []string) ([][]float32, error) {
		out := make([][]float32, len(textos))
		for i, txt := range textos {
			marca := float32(len(txt))
			vistos[txt] = marca
			out[i] = []float32{marca, 0, 0}
		}
		return out, nil
	}

	res, err := e.EmbedBackfill(porTexto)
	if err != nil {
		t.Fatal(err)
	}
	if res.Embedded != n {
		t.Fatalf("embebió %d de %d", res.Embedded, n)
	}
	if len(vistos) != n {
		t.Errorf("el embebedor vio %d textos distintos, esperaba %d", len(vistos), n)
	}
}

// B4 — Un vector VACÍO en una posición NO es un lote corto: es «este texto no se pudo embeber».
// Se saltea y se cuenta, sin abortar. Confundir los dos casos volvería la guarda un tapón.
func TestB4UnVectorVacioSeSalteaSinAbortar(t *testing.T) {
	e := newTestEngine(t)
	sembrar(t, e, 3)

	conHueco := func(textos []string) ([][]float32, error) {
		out := make([][]float32, len(textos))
		for i := range textos {
			if i == 1 {
				continue // vacío: posición presente, vector ausente
			}
			out[i] = []float32{1, 0, 0}
		}
		return out, nil
	}

	res, err := e.EmbedBackfill(conHueco)
	if err != nil {
		t.Fatalf("un vector vacío no debe abortar el lote: %v", err)
	}
	if res.Skipped != 1 || res.Embedded != 2 {
		t.Errorf("esperaba 1 salteada y 2 embebidas, obtuve skipped=%d embedded=%d", res.Skipped, res.Embedded)
	}
}

// B5 — Con más observaciones que el tamaño de lote, se hacen VARIAS tandas y no se pierde ninguna.
// Un off-by-one en el troceo dejaría observaciones sin vector, en silencio.
func TestB5ElTroceoNoPierdeObservaciones(t *testing.T) {
	e := newTestEngine(t)
	const n = embedBatchSize + 3
	sembrar(t, e, n)

	lotes := 0
	total := 0
	contar := func(textos []string) ([][]float32, error) {
		lotes++
		total += len(textos)
		if len(textos) > embedBatchSize {
			t.Errorf("un lote trajo %d textos, más que el tope de %d", len(textos), embedBatchSize)
		}
		out := make([][]float32, len(textos))
		for i := range textos {
			out[i] = []float32{1, 0, 0}
		}
		return out, nil
	}

	res, err := e.EmbedBackfill(contar)
	if err != nil {
		t.Fatal(err)
	}
	if total != n || res.Embedded != n {
		t.Errorf("se perdieron observaciones en el troceo: vistas=%d embebidas=%d de %d", total, res.Embedded, n)
	}
	if lotes != 2 {
		t.Errorf("esperaba 2 tandas para %d observaciones con lote %d, hubo %d", n, embedBatchSize, lotes)
	}
}

// ⚠️ B6 — UNA OBSERVACIÓN IMPOSIBLE NO PUEDE BLOQUEAR A LAS DEMÁS. Es el test del bug real: en el
// cerebro central una sola observación que el embebedor rechaza con 400 dejó al backfill parado
// TRES DÍAS, porque abortaba en la primera y la corrida resumible volvía a empezar por ella.
func TestB6UnaObservacionRechazadaNoFrenaALasDemas(t *testing.T) {
	e := newTestEngine(t)
	const n = 5
	sembrar(t, e, n)

	const toxica = "c-obs" // la que el embebedor rechaza, sola o acompañada
	intentos := 0
	conUnaImposible := func(textos []string) ([][]float32, error) {
		intentos++
		for _, txt := range textos {
			if strings.Contains(txt, toxica) {
				return nil, fmt.Errorf("status 400: the input length exceeds the context length")
			}
		}
		out := make([][]float32, len(textos))
		for i := range textos {
			out[i] = []float32{1, 0, 0}
		}
		return out, nil
	}

	res, err := e.EmbedBackfill(conUnaImposible)
	if err != nil {
		t.Fatalf("una sola observación imposible NO debe abortar la corrida: %v", err)
	}
	if res.Embedded != n-1 || res.Failed != 1 {
		t.Errorf("esperaba %d embebidas y 1 rechazada, obtuve embedded=%d failed=%d skipped=%d",
			n-1, res.Embedded, res.Failed, res.Skipped)
	}
	if intentos != 1+n {
		t.Errorf("esperaba 1 intento en lote + %d individuales, hubo %d", n, intentos)
	}
	// Y la rechazada NO se dio por hecha: sigue pendiente, así que un embebedor distinto (o el
	// mismo con más contexto) la puede levantar mañana. Marcarla como resuelta la perdería.
	if pend, err := e.countStaleEmbeddings(); err != nil || pend != 1 {
		t.Errorf("la rechazada tiene que quedar PENDIENTE: pendientes=%d err=%v", pend, err)
	}
	if got := countEmbeddingsWithModel(t, e, "static:tabla@aaaa"); got != n-1 {
		t.Errorf("esperaba %d vectores persistidos, hay %d", n-1, got)
	}
}

// ⚠️ B7 — EL EMBEBEDOR CAÍDO SIGUE SIENDO UN ERROR. Sin esta regla, el arreglo de B6 convierte
// "ollama no responde" en una corrida verde con 33 fallidas y 0 embebidas — y un verde falso no
// lo mira nadie. Si NADA se pudo embeber, se aborta.
func TestB7SiNoSeEmbebeNadaLaCorridaFalla(t *testing.T) {
	e := newTestEngine(t)
	sembrar(t, e, 3)

	caido := func([]string) ([][]float32, error) {
		return nil, fmt.Errorf("connection refused")
	}

	res, err := e.EmbedBackfill(caido)
	if err == nil {
		t.Fatal("si no se embebió NADA la corrida tiene que fallar, no reportar 3 fallidas en verde")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("el error tiene que arrastrar la causa real para que se pueda diagnosticar: %v", err)
	}
	if res.Embedded != 0 {
		t.Errorf("no debería haber embebido nada, embebió %d", res.Embedded)
	}
}

// B8 — La otra cara de B7: si el embebedor YA demostró funcionar en esta corrida, un lote que
// igual falla entero se saltea en vez de tumbar lo que faltaba. Nada se persiste, así que esas
// observaciones siguen pendientes para el próximo intento.
func TestB8UnLoteQueFallaDespuesDeFuncionarNoTumbaLaCorrida(t *testing.T) {
	e := newTestEngine(t)
	const n = embedBatchSize + 3
	sembrar(t, e, n)

	// Las tres últimas (segunda tanda) las rechaza siempre; las primeras 16 entran bien.
	ultimas := []string{
		string(rune('a'+embedBatchSize)) + "-obs",
		string(rune('a'+embedBatchSize+1)) + "-obs",
		string(rune('a'+embedBatchSize+2)) + "-obs",
	}
	rechazaLasUltimas := func(textos []string) ([][]float32, error) {
		for _, txt := range textos {
			for _, mala := range ultimas {
				if strings.Contains(txt, mala) {
					return nil, fmt.Errorf("status 400")
				}
			}
		}
		out := make([][]float32, len(textos))
		for i := range textos {
			out[i] = []float32{1, 0, 0}
		}
		return out, nil
	}

	res, err := e.EmbedBackfill(rechazaLasUltimas)
	if err != nil {
		t.Fatalf("el embebedor ya funcionó en esta corrida: un lote malo después no debe abortarla: %v", err)
	}
	if res.Embedded != embedBatchSize || res.Failed != 3 {
		t.Errorf("esperaba %d embebidas y 3 rechazadas, obtuve embedded=%d failed=%d",
			embedBatchSize, res.Embedded, res.Failed)
	}
	if pend, err := e.countStaleEmbeddings(); err != nil || pend != 3 {
		t.Errorf("las 3 rechazadas tienen que quedar pendientes: pendientes=%d err=%v", pend, err)
	}
}

// ⚠️ B9 — EL ESTADO ESTACIONARIO, QUE ES EL QUE MÁS SE VA A VER. Cuando ya sólo queda LA
// observación imposible en la cola, el lote es de UNA: el progreso de la corrida no da ninguna
// evidencia para distinguir "este texto es imposible" de "el embebedor está caído". Deducirlo
// falla acá, y falla hacia el rojo: gritaría "está caído, corré el backfill a mano" en cada
// arranque del daemon, para siempre, con un diagnóstico falso y una instrucción que no arregla
// nada. Por eso se PREGUNTA con un texto trivial en vez de adivinar.
func TestB9UnaSolaImposibleNoSeConfundeConElEmbebedorCaido(t *testing.T) {
	e := newTestEngine(t)
	sembrar(t, e, 1)

	sondas := 0
	vivoPeroLaRechaza := func(textos []string) ([][]float32, error) {
		if len(textos) == 1 && textos[0] == textoSondaEmbed {
			sondas++ // el embebedor está VIVO: contesta cualquier cosa trivial
			return [][]float32{{1, 0, 0}}, nil
		}
		return nil, fmt.Errorf("status 400: the input length exceeds the context length")
	}

	res, err := e.EmbedBackfill(vivoPeroLaRechaza)
	if err != nil {
		t.Fatalf("el embebedor contestó la sonda: está vivo, así que esto NO es una corrida fallida: %v", err)
	}
	if res.Failed != 1 || res.Embedded != 0 {
		t.Errorf("esperaba 1 rechazada y 0 embebidas, obtuve failed=%d embedded=%d", res.Failed, res.Embedded)
	}
	if sondas != 1 {
		t.Errorf("esperaba exactamente 1 sonda de vitalidad, hubo %d", sondas)
	}
	// Y sigue pendiente: mañana, con otro embebedor o más contexto, entra.
	if pend, err := e.countStaleEmbeddings(); err != nil || pend != 1 {
		t.Errorf("la rechazada tiene que quedar pendiente: pendientes=%d err=%v", pend, err)
	}
}

// B10 — La sonda NO se dispara cuando alguna del lote entró: el embebedor ya demostró estar vivo
// y preguntárselo sería un pedido de más en el camino más común de este arreglo.
func TestB10SinSondaSiAlgunaDelLoteEntro(t *testing.T) {
	e := newTestEngine(t)
	sembrar(t, e, 4)

	sondas := 0
	unaMala := func(textos []string) ([][]float32, error) {
		if len(textos) == 1 && textos[0] == textoSondaEmbed {
			sondas++
			return [][]float32{{1, 0, 0}}, nil
		}
		for _, txt := range textos {
			if strings.Contains(txt, "b-obs") {
				return nil, fmt.Errorf("status 400")
			}
		}
		out := make([][]float32, len(textos))
		for i := range textos {
			out[i] = []float32{1, 0, 0}
		}
		return out, nil
	}

	res, err := e.EmbedBackfill(unaMala)
	if err != nil {
		t.Fatal(err)
	}
	if sondas != 0 {
		t.Errorf("no hacía falta sondear: 3 de 4 entraron, el embebedor ya se probó vivo (sondas=%d)", sondas)
	}
	if res.Embedded != 3 || res.Failed != 1 {
		t.Errorf("esperaba 3 embebidas y 1 rechazada, obtuve embedded=%d failed=%d", res.Embedded, res.Failed)
	}
}

// ⚠️ B11 — «MISMO model_id» NO SIEMPRE SIGNIFICA «MISMO VECTOR». Si cambia CÓMO se embebe y no
// CON QUÉ, la procedencia no lo detecta y esas filas no las lista nadie: quedan mal para siempre.
// Es el caso real del troceo — los textos largos tenían un vector calculado sólo sobre su primer
// pedazo. EmbedBackfillAll es la salida, y por eso es explícita: re-embebe la base entera.
func TestB11BackfillAllRehaceLoQueYaTeniaVector(t *testing.T) {
	e := newTestEngine(t)
	const n = 4
	sembrar(t, e, n)

	uno := func(textos []string) ([][]float32, error) {
		out := make([][]float32, len(textos))
		for i := range textos {
			out[i] = []float32{1, 0, 0}
		}
		return out, nil
	}
	if _, err := e.EmbedBackfill(uno); err != nil {
		t.Fatal(err)
	}
	if pend, _ := e.countStaleEmbeddings(); pend != 0 {
		t.Fatalf("precondición: no debería quedar nada pendiente, quedan %d", pend)
	}

	// El backfill normal ya no ve nada que hacer: la procedencia coincide.
	res, err := e.EmbedBackfill(uno)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scanned != 0 {
		t.Errorf("el backfill normal no debería listar nada, listó %d", res.Scanned)
	}

	// El de --all sí, y las re-embebe todas.
	res, err = e.EmbedBackfillAll(uno)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scanned != n || res.Embedded != n {
		t.Errorf("--all tenía que re-embeber las %d, listó %d y embebió %d", n, res.Scanned, res.Embedded)
	}
}
