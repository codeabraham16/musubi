package embedding

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// trozos.go garantiza que ningún texto salga hacia el embebedor en un pedido más grande de lo que
// ese embebedor acepta, partiéndolo y promediando los vectores de las partes.
//
// ⚠️ POR QUÉ ESTO NO SE PUEDE DELEGAR EN `truncate` DEL PROVEEDOR. Medido contra el ollama del
// cerebro central (bge-m3, 2026-08-18), el comportamiento por largo NO es un umbral, es una BANDA:
//
//	≤ ~2.048 tokens ......... entra bien
//	~2.048 – num_ctx ........ HTTP 400 «the input length exceeds the context length»
//	≥ num_ctx ............... entra, TRUNCADO EN SILENCIO
//
// O sea que entradas MÁS GRANDES funcionan y una más chica falla — el síntoma que delata que leer
// el error literalmente («es muy largo») lleva a la conclusión equivocada. Y los dos extremos son
// malos: abajo se pierde la observación con un 400, arriba se guarda un vector calculado sobre el
// primer pedazo y presentado como si representara el documento entero. Se probó: dos textos que
// difieren en 140.000 caracteres devolvieron el vector IDÉNTICO (coseno 1,000000).
//
// Y subir `num_ctx` NO arregla nada: el borde de abajo no se movió al pasar de 4096 a 8192 (8.970
// → 8.921 caracteres), porque es una constante del embebedor y no una fracción del contexto. Lo
// único que cambió fue el borde de ARRIBA, o sea que subir el contexto ENSANCHA la banda muerta.
// Por eso el arreglo tiene que estar de este lado.
//
// MODEL-FREE, sin tokenizador: Musubi no puede contar tokens, así que no puede saber dónde está el
// límite de un embebedor cualquiera. En vez de cablear una constante que va a estar mal para el
// próximo modelo (o para un idioma con otra densidad de tokens), el trozo que el embebedor rechaza
// se PARTE AL MEDIO y se reintenta. El límite se descubre midiendo, que es lo mismo que hace el
// resto del sistema.

const (
	// trozoInicial es el tamaño de partida, en caracteres. Cómodo para prosa latina (~1.360 tokens
	// a 4,4 caracteres por token, bien debajo de los ~2.048 medidos) y si un idioma denso lo
	// desmiente, el halving lo corrige solo en el primer pedido.
	trozoInicial = 6000
	// trozoMinimo corta el halving. Debajo de esto, que el embebedor siga rechazando ya no es un
	// problema de tamaño: es del texto o del servicio, y hay que devolver el error en vez de
	// seguir partiendo hasta el carácter.
	trozoMinimo = 375
	// loteMaxChars acota cuánto texto va en UN pedido, sin importar cuántos textos sean.
	//
	// EL LOTE ESTABA LIMITADO POR CANTIDAD Y EL COSTO NO DEPENDE DE LA CANTIDAD, DEPENDE DEL TEXTO.
	// Medido contra el ollama del central (bge-m3, 2026-08-18), el tiempo de un pedido es lineal en
	// caracteres, ~0,5–0,8 ms cada uno:
	//
	//	 1 texto  ×  1.000 car =   1.000 car ->   0,8 s
	//	16 textos ×  1.000 car =  16.000 car ->   8,1 s
	//	16 textos ×  3.000 car =  48.000 car ->  27,8 s   <- al filo del plazo
	//	16 textos ×  6.000 car =  96.000 car ->  65,5 s   <- se pasa
	//
	// Se vio en producción: `embed backfill --all` llenó el log de `context deadline exceeded` en
	// lotes de 16, y cada uno cayó al reintento uno por uno. No se perdió nada —para eso está esa
	// red— pero el lote quedó DESACTIVADO de hecho justo para los textos grandes, y con él la
	// mejora de 1,37× que se había medido. Un tope por cantidad no puede acotar un costo que
	// depende del tamaño.
	//
	// 40.000 porque ahí el pedido tarda ~23 s medidos: entra con margen en cualquier plazo sensato
	// y sigue juntando decenas de observaciones normales en un solo viaje.
	loteMaxChars = 40000
)

// troceado envuelve un Provider y le garantiza pedidos que entran.
//
// Va DEBAJO del portero de privacidad a propósito (ver NewProvider): el texto se tapa ENTERO y
// recién después se parte. Al revés, un secreto que cayera justo sobre el corte quedaría partido
// en dos mitades que ninguna regla reconoce, y saldría sin tapar. La privacidad se decide sobre el
// texto completo; el troceo es transporte.
type troceado struct{ inner Provider }

func (t troceado) Name() string    { return t.inner.Name() }
func (t troceado) Dimensions() int { return t.inner.Dimensions() }

// Embed deja pasar intacto lo que ya entra, y sólo trocea lo que no.
//
// El camino corto es bit-idéntico al de antes de este archivo — importa que lo sea: si el troceo
// cambiara el vector de los textos normales, todos los vectores guardados quedarían incomparables
// con los nuevos y el recall se degradaría en silencio. Hay un test que lo sella.
func (t troceado) Embed(ctx context.Context, text string) ([]float32, error) {
	if len(text) <= trozoInicial {
		return t.inner.Embed(ctx, text)
	}
	return t.embedPartido(ctx, text)
}

// EmbedBatch mantiene el lote mientras ningún texto necesite troceo (el caso común, y donde está
// la mejora de velocidad medida). Con un texto largo adentro, el lote se resuelve de a uno: cada
// largo se convierte en varios pedidos, así que ya no hay un lote que respetar — y mezclar
// respuestas de tamaños distintos es justo cómo se desalinea el apareo por índice.
func (t troceado) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	hayLargo := false
	for _, tx := range texts {
		if len(tx) > trozoInicial {
			hayLargo = true
			break
		}
	}
	if !hayLargo {
		// Ningún texto necesita troceo, pero el LOTE sí puede ser demasiado para un solo pedido:
		// el costo es lineal en caracteres, no en cantidad. Se manda de a tandas acotadas.
		return t.enTandas(ctx, texts)
	}
	out := make([][]float32, len(texts))
	for i, tx := range texts {
		v, err := t.Embed(ctx, tx)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// enTandas parte el lote en pedidos de a lo sumo loteMaxChars y concatena las respuestas.
//
// La concatenación conserva el ORDEN, que es lo único que el caller puede usar para aparear cada
// vector con su observación. Un reordenamiento acá no rompería nada visible: escribiría el
// embedding de cada texto en el registro de otro.
func (t troceado) enTandas(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for inicio := 0; inicio < len(texts); {
		fin, acum := inicio, 0
		for fin < len(texts) {
			// Siempre al menos uno, aunque solo ya supere el tope: partirlo es trabajo del troceo
			// por texto, no de acá, y dejarlo afuera sería un bucle que no avanza.
			if fin > inicio && acum+len(texts[fin]) > loteMaxChars {
				break
			}
			acum += len(texts[fin])
			fin++
		}
		vecs, err := EmbedBatch(ctx, t.inner, texts[inicio:fin])
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
		inicio = fin
	}
	return out, nil
}

// embedPartido parte el texto, embebe cada trozo y promedia.
//
// El promedio de vectores normalizados es la forma estándar de representar un documento por sus
// partes, y es estrictamente mejor que lo que pasaba antes —quedarse con el primer pedazo— porque
// el final del documento deja de ser invisible para el recall.
func (t troceado) embedPartido(ctx context.Context, texto string) ([]float32, error) {
	var suma []float32
	usados := 0
	for _, trozo := range partir(texto, trozoInicial) {
		v, err := t.embedAchicando(ctx, trozo)
		if err != nil {
			return nil, err
		}
		if len(v) == 0 {
			continue // el embebedor no tuvo vector para este pedazo; no invalida a los demás
		}
		if suma == nil {
			suma = make([]float32, len(v))
		}
		if len(v) != len(suma) {
			return nil, fmt.Errorf("el proveedor %s devolvió vectores de largo distinto (%d y %d) para trozos del mismo texto",
				t.inner.Name(), len(suma), len(v))
		}
		normalizar(v)
		for i := range v {
			suma[i] += v[i]
		}
		usados++
	}
	if usados == 0 {
		return nil, nil // sin vector: el caller ya sabe distinguir «vacío» de «error»
	}
	for i := range suma {
		suma[i] /= float32(usados)
	}
	normalizar(suma)
	return suma, nil
}

// embedAchicando pide el trozo y, si el embebedor lo rechaza, lo parte al medio y promedia las dos
// mitades. Así el límite real se descubre midiendo en vez de cablearlo.
func (t troceado) embedAchicando(ctx context.Context, trozo string) ([]float32, error) {
	v, err := t.inner.Embed(ctx, trozo)
	if err == nil {
		return v, nil
	}
	if len(trozo) <= trozoMinimo {
		// Ya no se puede achicar más: el rechazo no es por tamaño. Se devuelve el error del
		// embebedor tal cual, que es lo que el backfill sabe aislar y reportar con su id.
		return nil, err
	}
	izq, der := partirEnDos(trozo)
	a, errA := t.embedAchicando(ctx, izq)
	if errA != nil {
		return nil, errA
	}
	b, errB := t.embedAchicando(ctx, der)
	if errB != nil {
		return nil, errB
	}
	return promediar(a, b), nil
}

// promediar devuelve el promedio normalizado de dos vectores, tolerando que alguno venga vacío.
func promediar(a, b []float32) []float32 {
	switch {
	case len(a) == 0:
		return b
	case len(b) == 0:
		return a
	case len(a) != len(b):
		return a // largos distintos: no se puede promediar; se conserva el primero antes que inventar
	}
	normalizar(a)
	normalizar(b)
	out := make([]float32, len(a))
	for i := range a {
		out[i] = (a[i] + b[i]) / 2
	}
	normalizar(out)
	return out
}

// normalizar lleva el vector a norma 1, in place. Un vector nulo se deja como está: dividir por
// cero produciría NaN, y un NaN adentro de un índice vectorial envenena toda comparación futura.
func normalizar(v []float32) {
	var suma float64
	for _, x := range v {
		suma += float64(x) * float64(x)
	}
	if suma == 0 {
		return
	}
	inv := float32(1 / math.Sqrt(suma))
	for i := range v {
		v[i] *= inv
	}
}

// partir corta el texto en trozos de a lo sumo max BYTES, prefiriendo cortar en un espacio.
//
// Dos cuidados que no son estéticos: (1) nunca partir un rune al medio, porque UTF-8 roto puede
// hacer que el proveedor rechace un trozo que en realidad entraba; (2) preferir el espacio, para
// no cortar palabras y degradar el vector más de lo necesario.
func partir(texto string, max int) []string {
	var out []string
	for len(texto) > max {
		corte := limiteSeguro(texto, max)
		out = append(out, texto[:corte])
		texto = strings.TrimLeft(texto[corte:], " \t\n\r")
	}
	if texto != "" {
		out = append(out, texto)
	}
	return out
}

// partirEnDos corta un trozo por la mitad, con los mismos cuidados que partir.
func partirEnDos(trozo string) (string, string) {
	corte := limiteSeguro(trozo, len(trozo)/2)
	if corte <= 0 || corte >= len(trozo) {
		corte = len(trozo) / 2
		for corte > 0 && !utf8.RuneStart(trozo[corte]) {
			corte--
		}
	}
	return trozo[:corte], strings.TrimLeft(trozo[corte:], " \t\n\r")
}

// limiteSeguro devuelve el mayor corte ≤ max que cae en un espacio, o en su defecto en un borde de
// rune. Nunca devuelve 0 para un texto no vacío: un corte de 0 haría un bucle infinito.
func limiteSeguro(texto string, max int) int {
	if max >= len(texto) {
		return len(texto)
	}
	if max <= 0 {
		max = 1
	}
	if i := strings.LastIndexAny(texto[:max], " \t\n\r"); i > 0 {
		return i
	}
	corte := max
	for corte > 0 && !utf8.RuneStart(texto[corte]) {
		corte--
	}
	if corte == 0 {
		return max // un solo rune más largo que el tope: se corta igual antes que no avanzar
	}
	return corte
}

// newTroceado envuelve p si vale la pena. NoopProvider y StaticProvider quedan afuera por lo que
// SON —el primero no embebe, el segundo es una tabla en proceso sin límite de pedido— igual que
// con el portero: así el camino sin red sigue siendo bit-idéntico por construcción.
func newTroceado(p Provider) Provider {
	switch p.(type) {
	case NoopProvider, *StaticProvider:
		return p
	default:
		return troceado{inner: p}
	}
}
