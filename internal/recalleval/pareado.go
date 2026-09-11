package recalleval

import (
	"fmt"
	"math"
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
	for id, ma := range a.PorConsulta {
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
