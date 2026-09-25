package fleet

// cronologia_ventana_bordes_test.go cierra los huecos que la auditoría A131 (tema T8) encontró en
// las guardas de la VENTANA y del ORDEN de la cronología: mutaciones del código que rompían el
// invariante y dejaban fleet, memory y mcp enteros en verde, más un defecto VIVO.
//
// El patrón de los siete es uno solo, y por eso estas pruebas recorren EJES y no agregan casos:
// cada guarda vieja clavaba un eje en un valor cómodo —`ahora` sin fracción, pedir de más con
// 365 días, sin duración con d == 0, el borde de la ventana mirado sólo por un lado, el empate
// entre dos time.Time idénticos, los instantes en horas enteras— y el defecto vivía en otro valor
// de ese mismo eje. Sumarle un punto a cada lista habría tapado los siete que se vieron y dejado
// abierto el octavo.

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"
)

// ahorasDePrueba son los `ahora` con los que se arma una ventana: sin fracción, y con la fracción
// mínima, media y máxima que admite un segundo. El segundo es la granularidad del almacenamiento,
// así que ésos son TODOS los casos distintos que Normalizada puede ver en una punta.
func ahorasDePrueba() []time.Time {
	base := time.Date(2026, 9, 24, 14, 42, 18, 0, time.UTC)
	return []time.Time{
		base,
		base.Add(time.Nanosecond),
		base.Add(500 * time.Millisecond),
		base.Add(time.Second - time.Nanosecond),
	}
}

// duracionesDeBorde recorre el dominio ENTERO de `d` alrededor de los tres puntos que lo parten
// —el cero, VentanaDefault y VentanaMax—, cada uno con su vecino inmediato de cada lado, más los
// extremos del int64. Sale de las constantes, no de números elegidos: si VentanaMax cambia, los
// bordes se mueven con ella.
func duracionesDeBorde() []time.Duration {
	return []time.Duration{
		math.MinInt64, -VentanaMax, -time.Hour, -time.Nanosecond,
		0,
		time.Nanosecond, time.Second,
		VentanaDefault - time.Nanosecond, VentanaDefault, VentanaDefault + time.Nanosecond,
		VentanaMax - time.Second, VentanaMax - time.Nanosecond, VentanaMax,
		VentanaMax + time.Nanosecond, VentanaMax + time.Second, VentanaMax + time.Hour,
		2 * VentanaMax, 2*VentanaMax + time.Nanosecond, 365 * 24 * time.Hour,
		math.MaxInt64,
	}
}

// VENTANAHASTA CUMPLE SU CONTRATO EN TODO EL DOMINIO DE `d`, Y LA VENTANA QUE ARMA SOBREVIVE AL
// CAMINO REAL: Normalizada y después Valida, que es el orden en que la usan las dos tools.
//
// La guarda vieja (TestVentanaHastaAplicaLosDefaults) clavaba TRES ejes a la vez y el defecto vivía
// en los tres:
//
//   - `ahora` sin mirar y la ventana SUELTA, medida con Duracion(). En producción la ventana no
//     viaja suelta: se normaliza, y normalizar redondea hacia afuera. Con `ahora` con fracción —o
//     sea siempre— la ventana del máximo salía con 720h0m1s y Valida la rechazaba. `horas: 720` y
//     `horas: 1000` devolvían -32603 en musubi_fleet_cronologia Y en musubi_fleet_contexto. Es el
//     «no un error» del doc de esa guarda, roto en el árbol sano (C2-vivo1).
//   - «pedir de más» clavado en 365 días, doce veces el máximo: un tope que recortara recién por
//     encima del doble pasaba igual, y 721 h devolvían 721 h (C2-m8).
//   - «sin duración» clavado en d == 0: una duración NEGATIVA armaba una ventana al revés (C2-m9).
//
// Y el control positivo de TestUnaVentanaInvalidaNoSeConvierteEnTraemeTodo estaba clavado en 6 h,
// así que un Valida que rechazara la ventana del máximo EXACTO —la que esta función devuelve
// cuando se pide de más— pasaba en verde (C2-m6; ver también la prueba de abajo).
//
// EXPOSICIÓN medida por la auditoría: 18 llamadas a las dos tools desde 2026-08-30 (cronologia 10
// ok y 4 con error; contexto 4 ok). Los 4 errores no se pueden atribuir: la bitácora de tools no
// guarda argumentos y el journal legible empieza después del último. El camino estaba en el
// binario desplegado desde b27c7ce.
//
// Sabotaje: recortar recién por encima del doble del máximo → pedir 721 h da 721 h y Valida la
// rechaza.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tif d > VentanaMax {"
// arnes: a="\tif d > 2*VentanaMax {"
// Sabotaje: el default sólo para d == 0 → una duración negativa arma una ventana al revés.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tif d <= 0 {"
// arnes: a="\tif d == 0 {"
func TestVentanaHastaCumpleSuContratoEnTodoElDominio(t *testing.T) {
	casos, enElTopeConFraccion := 0, 0
	for _, ahora := range ahorasDePrueba() {
		for _, d := range duracionesDeBorde() {
			casos++
			if d >= VentanaMax && ahora.Nanosecond() != 0 {
				enElTopeConFraccion++
			}
			nombre := fmt.Sprintf("VentanaHasta(%s, %v)", ahora.Format(time.RFC3339Nano), d)
			v := VentanaHasta(ahora, d)

			if !v.Hasta.Equal(ahora) {
				t.Errorf("%s movió `hasta` a %s: la punta que eligió el llamador no se toca",
					nombre, v.Hasta.Format(time.RFC3339Nano))
			}
			// El contrato del doc, cláusula por cláusula.
			switch got := v.Duracion(); {
			case d <= 0 && got != VentanaDefault:
				t.Errorf("%s dura %s: TODO lo que no es positivo es «sin duración» y da el default %s; "+
					"otra cosa arma una ventana al revés o vacía", nombre, got, VentanaDefault)
			case d > VentanaMax && got != VentanaMax:
				t.Errorf("%s dura %s: pedir de más —por poco o por mucho— tiene que dar el máximo %s",
					nombre, got, VentanaMax)
			case d > 0 && d <= VentanaMax && got != d:
				t.Errorf("%s dura %s: lo que entra en el máximo se respeta tal cual", nombre, got)
			}
			errSuelta := v.Valida()
			if errSuelta != nil {
				t.Errorf("%s armó una ventana que Valida rechaza (%v): pedir cualquier cosa da una ventana, "+
					"no un error", nombre, errSuelta)
			}
			if err := v.Normalizada().Valida(); err != nil && errSuelta == nil {
				t.Errorf("%s pasa Valida suelta pero NO después de Normalizada (%v): es el orden en que la usan "+
					"musubi_fleet_cronologia y musubi_fleet_contexto, así que la tool contesta -32603 a quien "+
					"pidió el máximo", nombre, err)
			}
		}
	}
	// EL PISO: sin el caso que destapó el defecto vivo, esta prueba mediría todo menos lo que vino a
	// medir, y seguiría en verde.
	if casos == 0 || enElTopeConFraccion == 0 {
		t.Fatalf("la tabla no recorrió el tope con `ahora` fraccionario (casos=%d, en el tope=%d)", casos, enElTopeConFraccion)
	}
}

// UNA VENTANA QUE VALIDA ACEPTÓ SIGUE ACEPTADA DESPUÉS DE NORMALIZARSE, en los tres caminos que
// arman una ventana y no sólo en el de VentanaHasta: `desde`+`hasta` explícitos y `desde` solo
// llegan con fracción igual (RFC3339 la admite, y `ahora` la trae siempre).
//
// Recorre el plano fracción-de-`desde` × duración alrededor del máximo, y por cada ventana exige
// lo que Normalizada promete: puntas en segundos enteros, idempotente, la de ARRIBA nunca hacia
// adentro, la de abajo hacia adentro SÓLO para no pasarse del tope — y lo que Valida decide: el
// máximo EXACTO entra, un nanosegundo más no.
//
// El CONTROL NEGATIVO importa tanto como el positivo: un Normalizada que recortara TODA ventana al
// máximo haría pasar la mitad de arriba y convertiría un pedido inválido en uno válido, que es
// justo lo que Valida existe para impedir.
//
// Sabotaje: sacar el recorte al tope de Normalizada → la ventana del máximo con `desde` fraccionario
// sale con 720h0m1s y la consulta la rechaza (el C2-vivo1 de vuelta).
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tif hasta.Sub(desde) > VentanaMax && v.Duracion() <= VentanaMax {"
// arnes: a="\tif false && hasta.Sub(desde) > VentanaMax && v.Duracion() <= VentanaMax {"
// Sabotaje: que Valida rechace también el máximo exacto (`>=`) → la ventana que VentanaHasta
// devuelve cuando se pide de más no pasa nunca (C2-m6).
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tif v.Duracion() > VentanaMax {"
// arnes: a="\tif v.Duracion() >= VentanaMax {"
func TestNormalizarNoInvalidaLoQueValidaAcepto(t *testing.T) {
	duraciones := []time.Duration{
		time.Hour,
		VentanaMax - time.Second, VentanaMax - time.Nanosecond, VentanaMax,
		VentanaMax + time.Nanosecond, VentanaMax + time.Second,
	}
	aceptadas, rechazadas, recortadas := 0, 0, 0
	for _, desde := range ahorasDePrueba() {
		for _, dur := range duraciones {
			v := Ventana{Desde: desde, Hasta: desde.Add(dur)}
			nombre := fmt.Sprintf("[%s, +%s)", desde.Format(time.RFC3339Nano), dur)

			errV := v.Valida()
			if dur <= VentanaMax && errV != nil {
				t.Errorf("%s dura a lo sumo el máximo y Valida la rechazó: %v", nombre, errV)
			}
			if dur > VentanaMax && errV == nil {
				t.Errorf("%s se pasa del máximo y Valida la aceptó: el tope no corta donde dice", nombre)
			}

			n := v.Normalizada()
			if n.Desde.Nanosecond() != 0 || n.Hasta.Nanosecond() != 0 {
				t.Errorf("%s normalizada quedó con fracción: %s → %s", nombre,
					n.Desde.Format(time.RFC3339Nano), n.Hasta.Format(time.RFC3339Nano))
			}
			if nn := n.Normalizada(); !nn.Desde.Equal(n.Desde) || !nn.Hasta.Equal(n.Hasta) {
				t.Errorf("%s: normalizar dos veces movió la ventana otra vez — la que se DEVUELVE no sería la que se aplica", nombre)
			}
			if n.Hasta.Before(v.Hasta) {
				t.Errorf("%s: la punta de arriba se movió hacia ADENTRO — lo que acaba de pasar queda afuera", nombre)
			}

			if errV == nil {
				aceptadas++
				if err := n.Valida(); err != nil {
					t.Errorf("%s pasó Valida y normalizada ya no (%v): la tool contesta -32603 a un pedido que "+
						"ella misma aceptó", nombre, err)
				}
				if n.Desde.After(v.Desde) {
					recortadas++
					if n.Duracion() != VentanaMax {
						t.Errorf("%s: la punta de abajo se movió hacia adentro sin estar en el tope (queda %s)", nombre, n.Duracion())
					}
				}
			} else {
				rechazadas++
				if err := n.Valida(); err == nil {
					t.Errorf("%s venía de más y normalizada pasó Valida: normalizar blanqueó un pedido inválido", nombre)
				}
			}
		}
	}
	// EL PISO: las tres ramas se tienen que haber ejercitado, o la prueba pasa sin medir la que falta.
	if aceptadas == 0 || rechazadas == 0 || recortadas == 0 {
		t.Fatalf("la tabla no recorrió las tres ramas: aceptadas=%d rechazadas=%d recortadas en el tope=%d",
			aceptadas, rechazadas, recortadas)
	}
}

// UN MOSAICO DE VENTANAS CUENTA CADA INSTANTE EXACTAMENTE UNA VEZ, y un instante fuera del mosaico
// ninguna. Es el motivo entero de que la ventana sea semiabierta.
//
// La guarda vieja (TestLaVentanaEsSemiabierta) miraba tres puntos: el `desde`, el `hasta` y uno
// ANTES del `desde`. Nunca uno DESPUÉS del `hasta`, así que un Contiene que excluyera sólo el
// instante exacto del borde (`!t.Equal(v.Hasta)`) pasaba las tres y dejaba entrar todo el futuro
// (C2-m1). Acá los puntos salen de los BORDES —cada uno con su vecino inmediato de cada lado, más
// uno lejos de cada punta del mosaico—, así que no hay un lado sin mirar.
//
// EXPOSICIÓN: cero. Contiene no tiene ningún llamador de producción; lo que decide la línea de
// tiempo es el `>= ? AND < ?` de las consultas. Por eso la otra mitad de este arreglo vive en
// internal/memory/cronologia_bordes_test.go, que usa a Contiene como ORÁCULO de esas consultas:
// con eso Contiene deja de ser una función que el arnés certifica y nadie usa, y pasa a ser la
// especificación contra la que se mide lo que corre.
//
// Sabotaje: que Contiene excluya sólo el instante exacto del `hasta` → todo lo posterior entra.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\treturn !t.Before(v.Desde) && t.Before(v.Hasta)"
// arnes: a="\treturn !t.Before(v.Desde) && !t.Equal(v.Hasta)"
// arnes: colision_ok="TestLaVentanaEsSemiabierta"
func TestUnMosaicoDeVentanasCuentaCadaInstanteUnaVez(t *testing.T) {
	t0 := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	mosaico := []Ventana{
		{Desde: t0, Hasta: t0.Add(12 * time.Hour)},
		{Desde: t0.Add(12 * time.Hour), Hasta: t0.Add(24 * time.Hour)},
		{Desde: t0.Add(24 * time.Hour), Hasta: t0.Add(36 * time.Hour)},
	}
	primero, ultimo := mosaico[0].Desde, mosaico[len(mosaico)-1].Hasta

	type punto struct {
		nombre string
		t      time.Time
		veces  int // en cuántas ventanas del mosaico tiene que caer
	}
	puntos := []punto{
		{"mucho antes del mosaico", primero.Add(-time.Hour), 0},
		{"mucho después del mosaico", ultimo.Add(time.Hour), 0},
	}
	bordes := []time.Time{primero}
	for _, v := range mosaico {
		bordes = append(bordes, v.Hasta)
	}
	for _, b := range bordes {
		for _, eps := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
			p := b.Add(eps)
			veces := 1
			if p.Before(primero) || !p.Before(ultimo) {
				veces = 0
			}
			puntos = append(puntos, punto{fmt.Sprintf("borde %s %+v", b.Format(time.RFC3339), eps), p, veces})
		}
	}
	for _, p := range puntos {
		var en []string
		for i, v := range mosaico {
			if v.Contiene(p.t) {
				en = append(en, fmt.Sprint(i))
			}
		}
		if len(en) != p.veces {
			t.Errorf("%s (%s) cayó en %d ventanas del mosaico [%s], y tiene que caer en %d: sumar los tramos "+
				"daría un total que no existe", p.nombre, p.t.Format(time.RFC3339Nano), len(en), strings.Join(en, ","), p.veces)
		}
	}
	// Y los dos lados de UNA ventana, dichos sin el mosaico: el control de que la cuenta de arriba no
	// da bien por compensación entre vecinas.
	v := mosaico[1]
	for _, c := range []struct {
		nombre string
		t      time.Time
		quiero bool
	}{
		{"justo antes del desde", v.Desde.Add(-time.Nanosecond), false},
		{"el desde", v.Desde, true},
		{"justo antes del hasta", v.Hasta.Add(-time.Nanosecond), true},
		{"el hasta", v.Hasta, false},
		{"justo después del hasta", v.Hasta.Add(time.Nanosecond), false},
		{"mucho después del hasta", v.Hasta.Add(24 * time.Hour), false},
		{"mucho antes del desde", v.Desde.Add(-24 * time.Hour), false},
	} {
		if got := v.Contiene(c.t); got != c.quiero {
			t.Errorf("Contiene(%s) = %v, esperaba %v: la ventana es [desde, hasta)", c.nombre, got, c.quiero)
		}
	}
}

// ORDENARHECHOS ES EL ORDEN TOTAL DEL INSTANTE, DESEMPATADO POR REFERENCIA — contra un oráculo
// independiente, y en todas las granularidades y representaciones de un time.Time.
//
// La guarda vieja (TestOrdenarHechosEsEstableYDelMasNuevoAlMasViejo) clavaba dos ejes:
//
//   - LA REPRESENTACIÓN: su empate era entre dos time.Time IDÉNTICOS, así que `==` y Equal daban lo
//     mismo. El mismo instante en otra zona —lo que time.Parse(RFC3339) devuelve ante un offset que
//     no es `Z`—, o con y sin lectura monotónica, NO es `==`; con `!=` en el desempate, After da
//     false para los dos lados y el orden vuelve a depender del orden de lectura (C2-m10).
//   - LA GRANULARIDAD: todos sus instantes estaban en horas enteras, así que comparar al SEGUNDO
//     (`Unix()`) pasaba. Dos hechos separados por 300 ms empataban y salía primero el más viejo
//     (C2-m11).
//
// En vez de agregar un caso por forma, esto compara contra `sort` con `Compare` y la referencia: la
// definición del orden escrita de otra manera. Cualquier comparación más gruesa que el instante, o
// sensible a la representación, se aparta del oráculo sin que haya que nombrarla.
//
// EXPOSICIÓN: cero hoy. Toda fecha guardada es RFC3339 en `Z` y al segundo (12.821 comandos, 4
// sesiones de pantalla, 1 de shell), así que en producción `==`, Equal y Unix() dan lo mismo. Lo
// que lo mantiene inofensivo es una COSTUMBRE del almacenamiento, no una regla de este código: el
// día que alguien escriba con RFC3339Nano o con otro offset, esta prueba ya cubre ese caso.
//
// Sabotaje: desempatar con `!=` en vez de Equal → el mismo instante en dos zonas no empata.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\t\tif !hs[i].Cuando.Equal(hs[j].Cuando) {"
// arnes: a="\t\tif hs[i].Cuando != hs[j].Cuando {"
// Sabotaje: ordenar al SEGUNDO → dos hechos del mismo segundo quedan en el orden de lectura.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\t\t\treturn hs[i].Cuando.After(hs[j].Cuando)"
// arnes: a="\t\t\treturn hs[i].Cuando.Unix() > hs[j].Cuando.Unix()"
func TestOrdenarHechosEsElOrdenTotalDelInstante(t *testing.T) {
	// Un instante en segundo entero QUE CONSERVA la lectura monotónica: Add la conserva, Truncate y
	// Round la tiran. Así la representación «monotónica» es un eje más.
	ahora := time.Now()
	base := ahora.Add(-time.Duration(ahora.Nanosecond()))

	// Todas las granularidades, de a una: la mitad cae en el MISMO segundo que `base`.
	deltas := []time.Duration{
		0, time.Nanosecond, time.Microsecond, time.Millisecond, 300 * time.Millisecond,
		time.Second - time.Nanosecond, time.Second, time.Second + time.Nanosecond, time.Hour,
	}
	// El mismo instante escrito de cuatro formas que NO son `==` entre sí.
	representaciones := []struct {
		nombre string
		de     func(time.Time) time.Time
	}{
		{"monotonico", func(x time.Time) time.Time { return x }},
		{"utc", func(x time.Time) time.Time { return x.UTC() }},
		{"art", func(x time.Time) time.Time { return x.In(time.FixedZone("ART", -3*3600)) }},
		{"cero-otro-puntero", func(x time.Time) time.Time { return x.In(time.FixedZone("", 0)) }},
	}

	var hechos []Hecho
	for i, d := range deltas {
		for j, r := range representaciones {
			// LA REFERENCIA CRECE CON EL TIEMPO a propósito: si una comparación más gruesa convierte
			// en empate dos instantes distintos, el desempate por referencia pone PRIMERO al más
			// viejo, y el oráculo lo ve. Con la referencia al revés, ese defecto quedaría tapado.
			hechos = append(hechos, Hecho{Cuando: r.de(base.Add(d)), Referencia: fmt.Sprintf("d%02d-%d-%s", i, j, r.nombre)})
		}
	}

	oraculo := append([]Hecho(nil), hechos...)
	sort.SliceStable(oraculo, func(i, j int) bool {
		if c := oraculo[i].Cuando.Compare(oraculo[j].Cuando); c != 0 {
			return c > 0
		}
		return oraculo[i].Referencia < oraculo[j].Referencia
	})
	quiero := referenciasDe(oraculo)

	// El orden de ENTRADA no puede importar: la lista es la misma sea cual sea el orden de lectura
	// de las fuentes. Se prueban tres, incluido el inverso, que es el que destapa un desempate que
	// cae al orden de lectura.
	entradas := map[string][]Hecho{
		"como se armaron": append([]Hecho(nil), hechos...),
		"invertida":       invertida(hechos),
		"intercalada":     intercalada(hechos),
	}
	for nombre, entrada := range entradas {
		OrdenarHechos(entrada)
		for i := range entrada {
			if entrada[i].Referencia != oraculo[i].Referencia {
				t.Errorf("con la entrada %s, OrdenarHechos se aparta del orden total del instante en la posición %d: "+
					"puso %s (%s) donde va %s (%s) — la lista se reordena según cómo se leyó\n obtuve %s\nquería %s",
					nombre, i, entrada[i].Referencia, entrada[i].Cuando.Format(time.RFC3339Nano),
					oraculo[i].Referencia, oraculo[i].Cuando.Format(time.RFC3339Nano), referenciasDe(entrada), quiero)
				break
			}
		}
	}
	// EL PISO: sin empates entre representaciones distintas y sin instantes distintos dentro del
	// mismo segundo, la tabla no ejercitaría ninguno de los dos ejes que vino a destrabar.
	mismoSegundoDistinto, empateEntreZonas := 0, 0
	for i := range hechos {
		for j := range hechos {
			if i >= j {
				continue
			}
			a, b := hechos[i].Cuando, hechos[j].Cuando
			if a.Equal(b) && a != b {
				empateEntreZonas++
			}
			if !a.Equal(b) && a.Unix() == b.Unix() {
				mismoSegundoDistinto++
			}
		}
	}
	if empateEntreZonas == 0 || mismoSegundoDistinto == 0 {
		t.Fatalf("la tabla no ejercita los dos ejes: empates entre representaciones=%d, instantes distintos en el mismo segundo=%d",
			empateEntreZonas, mismoSegundoDistinto)
	}
}

func referenciasDe(hs []Hecho) string {
	refs := make([]string, len(hs))
	for i, h := range hs {
		refs[i] = h.Referencia
	}
	return strings.Join(refs, " ")
}

func invertida(hs []Hecho) []Hecho {
	out := make([]Hecho, len(hs))
	for i, h := range hs {
		out[len(hs)-1-i] = h
	}
	return out
}

// intercalada toma los pares y después los impares: un orden que no es ni el original ni su inverso.
func intercalada(hs []Hecho) []Hecho {
	out := make([]Hecho, 0, len(hs))
	for i := 0; i < len(hs); i += 2 {
		out = append(out, hs[i])
	}
	for i := 1; i < len(hs); i += 2 {
		out = append(out, hs[i])
	}
	return out
}
