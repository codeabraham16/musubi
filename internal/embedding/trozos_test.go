package embedding

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"musubi/internal/config"
)

// trozos_test.go defiende el troceo del texto largo. El bug que lo motiva NO es hipotético: medido
// contra el ollama del central, un texto en cierta franja de largos se rechaza con 400, y uno más
// grande todavía entra pero TRUNCADO EN SILENCIO — dos textos que diferían en 140.000 caracteres
// devolvieron el vector idéntico (coseno 1,000000).

// motor es un embebedor de mentira que devuelve un vector DERIVADO del texto, para poder distinguir
// «se embebió esto» de «se embebió aquello», y que puede rechazar textos por encima de un tope,
// como hace el de verdad.
type motor struct {
	tope   int // 0 = sin tope; si el texto lo supera, rechaza como ollama
	vistos []string
}

func (m *motor) Embed(_ context.Context, text string) ([]float32, error) {
	m.vistos = append(m.vistos, text)
	if m.tope > 0 && len(text) > m.tope {
		return nil, fmt.Errorf("the input length exceeds the context length")
	}
	// Vector que depende del contenido: la suma de los bytes en dos cubetas. Dos textos distintos
	// dan vectores distintos, que es lo único que estos tests necesitan.
	var a, b float32
	for i := 0; i < len(text); i++ {
		if i%2 == 0 {
			a += float32(text[i])
		} else {
			b += float32(text[i])
		}
	}
	return []float32{a, b}, nil
}
func (m *motor) Dimensions() int { return 2 }
func (m *motor) Name() string    { return "motor" }

func (m *motor) masLargo() int {
	n := 0
	for _, v := range m.vistos {
		if len(v) > n {
			n = len(v)
		}
	}
	return n
}

func coseno(a, b []float32) float64 {
	var num, na, nb float64
	for i := range a {
		num += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return num / (math.Sqrt(na) * math.Sqrt(nb))
}

// ⚠️ T1 — EL CAMINO CORTO ES BIT-IDÉNTICO. Es el test más importante del archivo, y el menos
// obvio: si el troceo tocara también los textos que YA entraban, todos los vectores guardados
// quedarían incomparables con los nuevos y el recall se degradaría sin un solo error.
func TestT1TextoCortoPasaIntacto(t *testing.T) {
	m := &motor{}
	tr := newTroceado(m)

	texto := strings.Repeat("nada que trocear. ", 20)
	conTroceo, err := tr.Embed(context.Background(), texto)
	if err != nil {
		t.Fatal(err)
	}
	directo, err := m.Embed(context.Background(), texto)
	if err != nil {
		t.Fatal(err)
	}
	for i := range directo {
		if conTroceo[i] != directo[i] {
			t.Fatalf("el troceador cambió el vector de un texto corto: %v vs %v", conTroceo, directo)
		}
	}
	if len(m.vistos) != 2 || m.vistos[0] != texto {
		t.Errorf("el texto corto no llegó entero y de una sola vez al motor: %d llamadas", len(m.vistos)-1)
	}
}

// ⚠️ T2 — NINGÚN PEDIDO SUPERA EL TOPE. Es la razón de existir del archivo: el embebedor real
// rechaza (o peor, trunca en silencio) lo que se pasa.
func TestT2NingunPedidoSaleMasGrandeQueElTope(t *testing.T) {
	m := &motor{}
	tr := newTroceado(m)

	largo := strings.Repeat("cada parrafo dice algo distinto y todo cuenta. ", 2000) // ~94.000 chars
	if _, err := tr.Embed(context.Background(), largo); err != nil {
		t.Fatal(err)
	}
	if n := m.masLargo(); n > trozoInicial {
		t.Fatalf("salió un pedido de %d caracteres, más que el tope de %d", n, trozoInicial)
	}
	if len(m.vistos) < 2 {
		t.Errorf("un texto de %d caracteres tendría que haberse partido, hubo %d pedido(s)", len(largo), len(m.vistos))
	}
}

// ⚠️ T3 — EL FINAL DEL DOCUMENTO DEJA DE SER INVISIBLE. Este es el defecto que se está arreglando:
// antes el vector salía del primer pedazo, así que dos documentos con el mismo arranque y finales
// completamente distintos eran INDISTINGUIBLES para el recall. Se comprueba con la misma medida
// que lo delató en producción: el coseno.
func TestT3DosDocumentosConElMismoArranqueYaNoSonIguales(t *testing.T) {
	m := &motor{}
	tr := newTroceado(m)
	ctx := context.Background()

	arranque := strings.Repeat("la introduccion es identica en los dos documentos. ", 200)
	unoA, err := tr.Embed(ctx, arranque+strings.Repeat("el primero habla de barcos. ", 300))
	if err != nil {
		t.Fatal(err)
	}
	unoB, err := tr.Embed(ctx, arranque+strings.Repeat("el segundo habla de astronomia nocturna. ", 300))
	if err != nil {
		t.Fatal(err)
	}
	if c := coseno(unoA, unoB); c > 0.9999 {
		t.Fatalf("dos documentos con finales distintos dieron el mismo vector (coseno %.6f): el final se está perdiendo", c)
	}
}

// ⚠️ T4 — EL LÍMITE SE DESCUBRE MIDIENDO. Musubi es model-free y no tiene tokenizador, así que no
// puede saber dónde está el tope de un embebedor cualquiera. Ante un rechazo, el trozo se parte al
// medio y se reintenta: con un motor mucho más estricto que trozoInicial, igual sale vector.
func TestT4SiElMotorRechazaSeAchicaSolo(t *testing.T) {
	m := &motor{tope: 500} // MUCHO más estricto que trozoInicial
	tr := newTroceado(m)

	largo := strings.Repeat("texto que hay que achicar hasta que entre. ", 500)
	v, err := tr.Embed(context.Background(), largo)
	if err != nil {
		t.Fatalf("el troceador tendría que haberse achicado solo hasta entrar: %v", err)
	}
	if len(v) == 0 {
		t.Fatal("salió un vector vacío")
	}
	for _, visto := range m.vistos {
		if len(visto) > 500 {
			// Los intentos rechazados SÍ pueden ser grandes; lo que no puede es que se haya dado
			// por bueno un pedido de más de 500 (el motor lo habría rechazado).
			continue
		}
	}
}

// ⚠️ T5 — SI EL RECHAZO NO ES POR TAMAÑO, SE DEVUELVE EL ERROR. Sin este corte, un embebedor
// caído haría partir el texto hasta el carácter, gastando miles de pedidos para terminar igual —
// y encima taparía la causa real, que es justo lo que el backfill sabe aislar y reportar por id.
func TestT5NoSeParteHastaElInfinito(t *testing.T) {
	m := &motor{tope: 1} // rechaza absolutamente todo
	tr := newTroceado(m)

	_, err := tr.Embed(context.Background(), strings.Repeat("x", 20000))
	if err == nil {
		t.Fatal("un motor que rechaza todo tiene que producir un error, no un vector inventado")
	}
	if len(m.vistos) > 200 {
		t.Errorf("se hicieron %d pedidos: el halving no está cortando en trozoMinimo", len(m.vistos))
	}
}

// T6 — No se parte un rune al medio. UTF-8 roto puede hacer que el proveedor rechace un trozo que
// en realidad entraba, y el síntoma sería «falla a veces, según el idioma».
func TestT6NoSeParteUnRuneAlMedio(t *testing.T) {
	m := &motor{}
	tr := newTroceado(m)

	// Puro multibyte: cada carácter ocupa más de un byte, así que cualquier corte por bytes
	// ingenuo cae adentro de un rune.
	largo := strings.Repeat("ñáéíóúü…—“”", 3000)
	if _, err := tr.Embed(context.Background(), largo); err != nil {
		t.Fatal(err)
	}
	for i, visto := range m.vistos {
		if !utf8ValidoStr(visto) {
			t.Fatalf("el trozo %d quedó con UTF-8 roto", i)
		}
	}
}

func utf8ValidoStr(s string) bool {
	for _, r := range s {
		if r == 0xFFFD {
			return false
		}
	}
	return true
}

// T7 — El lote se conserva mientras ningún texto necesite troceo. Si esto se rompiera, la mejora
// de velocidad medida (1,37×) se perdería en silencio para el caso común, que es el 99%.
func TestT7ElLoteSobreviveSiNadieEsLargo(t *testing.T) {
	e := &espiaLote{devolver: -1}
	tr := newTroceado(e)

	if _, ok := tr.(BatchProvider); !ok {
		t.Fatal("el troceador dejó de implementar BatchProvider: el lote se pierde en silencio")
	}
	if _, err := EmbedBatch(context.Background(), tr, []string{"uno", "dos", "tres"}); err != nil {
		t.Fatal(err)
	}
	if !e.usoLote || len(e.lotes) != 1 || len(e.lotes[0]) != 3 {
		t.Errorf("el lote no llegó entero al motor: usoLote=%v lotes=%v", e.usoLote, e.lotes)
	}
}

// T8 — Los proveedores SIN red no se envuelven: no tienen límite de pedido y envolverlos cambiaría
// el vector de la tabla estática, que es determinista por contrato.
func TestT8SinRedNoSeTrocea(t *testing.T) {
	if _, ok := newTroceado(NoopProvider{}).(troceado); ok {
		t.Error("NoopProvider no debería envolverse")
	}
	if _, ok := newTroceado(&StaticProvider{}).(troceado); ok {
		t.Error("StaticProvider no debería envolverse: su vector es determinista por contrato")
	}
}

// ⚠️ T9 — EL ORDEN DE LAS CAPAS ES PARTE DE LA PRIVACIDAD. El troceador va DEBAJO del portero: si
// fuera al revés, un secreto que cayera justo sobre un corte quedaría partido en dos mitades que
// ninguna regla reconoce, y saldría SIN TAPAR hacia el proveedor.
func TestT9ElSecretoSeTapaAntesDePartir(t *testing.T) {
	m := &motor{}
	p, err := newGuarded(newTroceado(m), config.GatewayModeScrub)
	if err != nil {
		t.Fatal(err)
	}

	// EL SECRETO SE PONE JUSTO SOBRE EL CORTE, y adentro de un blob SIN espacios — que es
	// exactamente donde aparecen los secretos de verdad (un volcado, un JSON largo, un base64).
	// Sin espacios cerca, `limiteSeguro` corta por posición, así que el token queda partido en dos
	// mitades. Con el texto rodeado de espacios el corte se corre al espacio y el secreto NUNCA se
	// parte: ese test pasaba en cualquier orden de capas, o sea que no probaba nada.
	pre := strings.Repeat("x", trozoInicial-9) + ","
	texto := pre + tSecretoAWS + "," + strings.Repeat("y", 3000)
	if _, err := p.Embed(context.Background(), texto); err != nil {
		t.Fatal(err)
	}
	if len(m.vistos) < 2 {
		t.Fatal("no se troceó: el test no está probando lo que dice")
	}
	// Ni entero ni en pedazos: un pedazo de credencial sigue siendo material filtrado. Se busca
	// CUALQUIER tramo de 8 caracteres del secreto, y no «la primera mitad», porque el corte no
	// tiene por qué caer en el medio — de hecho acá cae dejando 8 de un lado y 12 del otro, y una
	// comprobación por mitades no lo ve. Esa calibración es la diferencia entre un test que
	// detecta la fuga y uno que la deja pasar en verde.
	const tramo = 8
	for i, visto := range m.vistos {
		if strings.Contains(visto, tSecretoAWS) {
			t.Fatalf("el secreto llegó CRUDO al motor en el trozo %d", i)
		}
		for j := 0; j+tramo <= len(tSecretoAWS); j++ {
			if strings.Contains(visto, tSecretoAWS[j:j+tramo]) {
				t.Fatalf("un tramo del secreto (%q) llegó crudo al motor en el trozo %d: se troceó ANTES de tapar",
					tSecretoAWS[j:j+tramo], i)
			}
		}
	}
}
