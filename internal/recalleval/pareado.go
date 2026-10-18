package recalleval

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// pareado.go compara dos configuraciones CONSULTA POR CONSULTA, en vez de por el promedio.
//
// POR QUÉ NO ALCANZA EL PROMEDIO. Dos brazos pueden empatar en MRR y haber reordenado medio corpus:
// el promedio no distingue "mejoró parejo" de "mejoró muchísimo en tres consultas y empeoró en
// veinte". Son dos resultados distintos y se deciden distinto — el segundo pide ruteo selectivo, no
// cambiar el default.
//
// ★ ESTO REPORTA, NO DECIDE, y la distinción no es cosmética. La propuesta original incluía un GATE
// que fallaba si una config empeoraba más consultas de las que mejoraba. Un juez lo implementó y lo
// midió contra el fixture: con 12 consultas y 10-12 empates por métrica, los pares discordantes eran
// 0-4, y un signo-test exacto necesita ~10-2 para p<0,05. La misma corrida daba ROJO por RR, ROJO
// por R@1 y VERDE por R@10: el veredicto lo elegía quien llamaba a la función. Un gate así es una
// moneda con cara de criterio. Por eso acá se devuelve el conteo y el detalle, y la decisión queda
// afuera, donde alguien puede mirar CUÁLES consultas se movieron.
//
// La otra mitad de esa refutación también quedó: el paper que la propuesta citaba usa el porcentaje
// degradado para justificar RUTEO SELECTIVO, no para rechazar la técnica.

// ComparacionPareada es el resultado de enfrentar dos configuraciones consulta por consulta.
type ComparacionPareada struct {
	A, B       string  // nombres de las dos configuraciones (B se mide CONTRA A)
	Metrica    string  // qué se comparó: "rr", "recall@k", "ndcg@k"
	Gana       int     // consultas donde B supera a A
	Pierde     int     // consultas donde B queda por debajo de A
	Empata     int     // consultas sin diferencia
	DeltaMedio float64 // promedio de (B - A) sobre TODAS las consultas comparadas
	// PeoresCaidas son las consultas donde B más empeoró, peor primero. Es lo accionable: el
	// promedio dice cuánto, esto dice DÓNDE mirar.
	PeoresCaidas []CaidaConsulta
	// MejoresSubas, simétrico.
	MejoresSubas []CaidaConsulta
	// Deltas son los (B - A) de TODAS las consultas comparadas, empates incluidos, ORDENADOS POR
	// EL ID DE LA CONSULTA. Son la materia prima de PSigno e IntervaloBootstrap.
	//
	// El orden no es cosmético: PorConsulta es un mapa, y Go lo recorre en un orden distinto en cada
	// corrida. Un bootstrap que remuestrea por posición sobre un slice armado en ese orden daría otro
	// intervalo con la MISMA semilla, y una medición que no se repite no se puede auditar.
	Deltas []float64
}

// CaidaConsulta es una consulta y cuánto se movió entre los dos brazos.
type CaidaConsulta struct {
	Consulta string
	Delta    float64
	A, B     float64
}

// CompararPareado enfrenta dos Scores consulta por consulta sobre una métrica.
//
// metrica: "rr" | "recall" | "ndcg". Para recall/ndcg hay que dar el k; para "rr" se ignora.
// Sólo se comparan las consultas presentes en LOS DOS lados: comparar contra una consulta que un
// brazo no corrió sería inventar un dato.
func CompararPareado(a, b Scores, metrica string, k int) (ComparacionPareada, error) {
	if a.PorConsulta == nil || b.PorConsulta == nil {
		return ComparacionPareada{}, fmt.Errorf("falta el detalle por consulta: comparar de a pares necesita Scores.PorConsulta de los dos brazos")
	}
	valor := func(m MetricasConsulta) (float64, error) {
		switch strings.ToLower(metrica) {
		case "rr":
			return m.RR, nil
		case "recall":
			v, ok := m.RecallAtK[k]
			if !ok {
				return 0, fmt.Errorf("no se midió recall@%d", k)
			}
			return v, nil
		case "ndcg":
			v, ok := m.NDCGAtK[k]
			if !ok {
				return 0, fmt.Errorf("no se midió ndcg@%d", k)
			}
			return v, nil
		}
		return 0, fmt.Errorf("métrica desconocida: %q (usá rr, recall o ndcg)", metrica)
	}

	nombre := metrica
	if metrica != "rr" {
		nombre = fmt.Sprintf("%s@%d", metrica, k)
	}
	out := ComparacionPareada{A: a.Config, B: b.Config, Metrica: nombre}
	var suma float64
	var n int
	var movidas []CaidaConsulta
	ids := make([]string, 0, len(a.PorConsulta))
	for id := range a.PorConsulta {
		ids = append(ids, id)
	}
	sort.Strings(ids) // ver Deltas: el orden del mapa cambia entre corridas
	for _, id := range ids {
		ma := a.PorConsulta[id]
		mb, ok := b.PorConsulta[id]
		if !ok {
			continue
		}
		va, err := valor(ma)
		if err != nil {
			return ComparacionPareada{}, err
		}
		vb, err := valor(mb)
		if err != nil {
			return ComparacionPareada{}, err
		}
		d := vb - va
		suma += d
		n++
		out.Deltas = append(out.Deltas, d)
		switch {
		case d > 1e-12:
			out.Gana++
		case d < -1e-12:
			out.Pierde++
		default:
			out.Empata++
		}
		if math.Abs(d) > 1e-12 {
			movidas = append(movidas, CaidaConsulta{Consulta: id, Delta: d, A: va, B: vb})
		}
	}
	if n > 0 {
		out.DeltaMedio = suma / float64(n)
	}
	sort.Slice(movidas, func(i, j int) bool { return movidas[i].Delta < movidas[j].Delta })
	const tope = 5
	for i := 0; i < len(movidas) && i < tope; i++ {
		out.PeoresCaidas = append(out.PeoresCaidas, movidas[i])
	}
	for i := len(movidas) - 1; i >= 0 && len(out.MejoresSubas) < tope; i-- {
		if movidas[i].Delta > 0 {
			out.MejoresSubas = append(out.MejoresSubas, movidas[i])
		}
	}
	return out, nil
}

// String rinde la comparación en una forma legible, con la advertencia de tamaño incluida cuando
// los pares discordantes son pocos — porque leer "gana 3, pierde 1" como si fuera un veredicto es
// exactamente el error que este archivo existe para no cometer.
func (c ComparacionPareada) String() string {
	var b strings.Builder
	discordantes := c.Gana + c.Pierde
	fmt.Fprintf(&b, "%s vs %s — %s: gana %d · pierde %d · empata %d · delta medio %+.4f\n",
		c.B, c.A, c.Metrica, c.Gana, c.Pierde, c.Empata, c.DeltaMedio)
	if discordantes < 10 {
		fmt.Fprintf(&b, "  ⚠ sólo %d consultas discordantes: con esta cantidad el signo del conteo es ruido, no señal. Mirá las consultas, no el marcador.\n", discordantes)
	}
	for _, x := range c.PeoresCaidas {
		fmt.Fprintf(&b, "  ↓ %-50s %+.4f  (%.4f → %.4f)\n", x.Consulta, x.Delta, x.A, x.B)
	}
	for _, x := range c.MejoresSubas {
		fmt.Fprintf(&b, "  ↑ %-50s %+.4f  (%.4f → %.4f)\n", x.Consulta, x.Delta, x.A, x.B)
	}
	return b.String()
}

// PSigno es el p-valor del TEST DE SIGNO EXACTO, a dos colas, sobre las consultas discordantes:
// bajo la hipótesis nula (B no es ni mejor ni peor que A), cada consulta que se movió tiene la misma
// chance de subir que de bajar, así que las que suben siguen una Binomial(n, 1/2) con n = Gana +
// Pierde. Los empates no entran: no dicen nada sobre el sentido del cambio.
//
// Es la cuenta que el encabezado de este archivo cita («un signo-test exacto necesita ~10-2 para
// p<0,05»), ahora hecha en vez de estimada a ojo. Sigue sin decidir nada: devuelve un número, y
// cuál umbral usar —y sobre qué métrica— lo elige quien lee, a la vista.
//
// Sin discordantes devuelve 1: no hay evidencia de nada, que no es lo mismo que evidencia de
// igualdad.
func (c ComparacionPareada) PSigno() float64 {
	n := c.Gana + c.Pierde
	if n == 0 {
		return 1
	}
	k := min(c.Gana, c.Pierde)
	// Con logaritmos para no desbordar: C(n, i) revienta un float64 mucho antes de las mil consultas.
	lnFact := func(x int) float64 {
		v, _ := math.Lgamma(float64(x) + 1)
		return v
	}
	cola := 0.0
	for i := 0; i <= k; i++ {
		cola += math.Exp(lnFact(n) - lnFact(i) - lnFact(n-i) - float64(n)*math.Ln2)
	}
	return math.Min(1, 2*cola)
}

// IntervaloBootstrap es el intervalo de confianza del DeltaMedio por bootstrap de percentiles:
// remuestrea las consultas con reposición `reps` veces, toma el delta medio de cada remuestra y
// devuelve los percentiles que dejan (1-nivel)/2 afuera a cada lado.
//
// Es DETERMINISTA: la semilla es explícita y Deltas viene ordenado por id de consulta. Con la
// misma comparación y la misma semilla sale el mismo intervalo en cualquier corrida.
//
// Con pocas consultas (12 del dorado, dos docenas del fixture real) el intervalo es tosco y tiende
// a quedar ANGOSTO: el bootstrap no puede inventar la variabilidad que la muestra no vio. Sirve para
// ver si el signo del delta es firme o cuelga de un par de consultas, no como una cota precisa.
//
// Sin deltas devuelve (0, 0).
func (c ComparacionPareada) IntervaloBootstrap(reps int, semilla int64, nivel float64) (lo, hi float64) {
	n := len(c.Deltas)
	if n == 0 {
		return 0, 0
	}
	if reps < 1 {
		reps = 1
	}
	rng := rand.New(rand.NewSource(semilla))
	medias := make([]float64, reps)
	for r := range medias {
		suma := 0.0
		for range n {
			suma += c.Deltas[rng.Intn(n)]
		}
		medias[r] = suma / float64(n)
	}
	sort.Float64s(medias)
	alfa := (1 - nivel) / 2
	iLo := int(math.Floor(alfa * float64(reps)))
	iHi := int(math.Ceil((1-alfa)*float64(reps))) - 1
	iLo = max(0, min(iLo, reps-1))
	iHi = max(iLo, min(iHi, reps-1))
	return medias[iLo], medias[iHi]
}
