package mcp

import (
	"math"
	"strings"
	"testing"
)

// C-ESC1 · TODA FORMA TIENE SUS NÚMEROS.
//
// El bloque de escala existe para que el brief no sea mudo justo donde hay que decidir. Una forma sin
// registro lo deja mudo otra vez, y en silencio: `registrosDe` cae al default y el brief sale con los
// números de otra densidad, sin avisar. El modo de falla es agregar la decimotercera forma y no
// acordarse de esta tabla; entonces esto tiene que romper el banco, no el diseño de alguien.
func TestEscalaTodaFormaTieneRegistro(t *testing.T) {
	for clave := range formasDeDiseno {
		reg, ok := registroDeForma[clave]
		if !ok {
			t.Errorf("la forma %q no tiene registro de escala: el brief le serviría los números de otra densidad", clave)
			continue
		}
		if _, existe := registrosDeEscala[reg]; !existe {
			t.Errorf("la forma %q apunta al registro %q, que no existe", clave, reg)
		}
	}
	// Y al revés: un registro declarado en el orden de emisión que no exista deja el bloque emitiendo
	// un encabezado con la tabla vacía — el mismo error que ya destapó el sabotaje del repliegue en
	// `formasPara`.
	for _, reg := range ordenRegistros {
		if _, ok := registrosDeEscala[reg]; !ok {
			t.Errorf("ordenRegistros nombra %q y no está en registrosDeEscala", reg)
		}
	}
	if len(ordenRegistros) != len(registrosDeEscala) {
		t.Errorf("hay %d registros y el orden de emisión nombra %d: alguno no se emitiría nunca",
			len(registrosDeEscala), len(ordenRegistros))
	}
}

// C-ESC2 · LA ESCALA ES UNA ESCALA.
//
// El valor de servir números es que formen un sistema: si los escalones no guardan la razón que el
// propio brief declara, lo que se sirve es una lista de tamaños lindos, que es exactamente lo que el
// bloque vino a reemplazar. Se verifica contra la razón declarada, con la tolerancia del redondeo al
// medio píxel —que es real y se acepta— y no contra una constante escrita a mano.
func TestEscalaLosPasosGuardanLaRazon(t *testing.T) {
	for clave, r := range registrosDeEscala {
		pasos := pasosDeEscala(r)
		if len(pasos) != escalaPasosAbajo+escalaPasosArriba+1 {
			t.Fatalf("%s: %d escalones, se esperaban %d", clave, len(pasos), escalaPasosAbajo+escalaPasosArriba+1)
		}
		for i := 1; i < len(pasos); i++ {
			// Se mide la razón ENTRE ESCALONES IMPRESOS, que es lo que ve quien compone, y se pide
			// que quede dentro del 3 % de la declarada.
			//
			// La primera versión de este test comparaba `pasos[i]` contra `pasos[i-1]*razón` con
			// tolerancia 0,26 y salía roja con la escala CORRECTA: el redondeo al medio píxel no
			// encadena —18,5 viene de redondear 18,72 hacia abajo y 22,5 de redondear 22,46 hacia
			// arriba, así que el salto impreso se estira 1,4 %— y yo había razonado la tolerancia
			// como si los errores no se sumaran. El 3 % es holgado para el redondeo y sigue siendo
			// duro contra una escala rota: meter 15 donde va 15,5 da 1,154 sobre 1,20, un 3,8 %.
			medida := pasos[i] / pasos[i-1]
			if desvio := math.Abs(medida/r.Razon - 1); desvio > 0.03 {
				t.Errorf("%s: del escalón %d al %d la razón impresa es %.4f y la declarada %.4f (%.1f %% de desvío) — la escala dejó de ser una escala",
					clave, i-1, i, medida, r.Razon, desvio*100)
			}
			if pasos[i] <= pasos[i-1] {
				t.Errorf("%s: el escalón %d (%v) no es mayor que el anterior (%v)", clave, i, pasos[i], pasos[i-1])
			}
		}
		// El cuerpo tiene que estar EN la escala: es el escalón desde el que se calcula todo, y si
		// el redondeo lo moviera, el brief declararía una base que no aparece en sus propios números.
		if cuerpo := pasos[escalaPasosAbajo]; cuerpo != r.Base {
			t.Errorf("%s: la base declarada es %v y en la escala figura %v", clave, r.Base, cuerpo)
		}
	}
}

// C-ESC3 · LA RAZÓN QUE SE ESCRIBE ES LA QUE SE USA.
//
// Este invariante nació de un bug real: `numPx` redondea a UNA decimal porque es la resolución del
// píxel, y con eso la razón 1,25 salía impresa «1,20» al lado de unos pasos que sí eran 1,25. El
// brief quedaba declarando un sistema distinto del que mostraba — y quien compone, si necesita un
// escalón más, lo recalcula con el número equivocado. El valor tranquilizador y el valor verdadero
// se habían separado sin que nada se pusiera rojo.
func TestEscalaLaRazonImpresaEsLaReal(t *testing.T) {
	for clave, r := range registrosDeEscala {
		txt := escalaPara(formasDeRegistro(clave))
		impresa := "razón " + numRazon(r.Razon)
		if !strings.Contains(txt, impresa) {
			t.Fatalf("%s: no se encontró %q en el bloque", clave, impresa)
		}
		// La razón impresa, aplicada al cuerpo, tiene que dar el escalón siguiente de la escala
		// impresa. Es la comprobación que el bug pasaba por alto: 13 × 1,20 = 15,6 → 15,5 ✔ pero
		// 14 × 1,20 = 16,8 → 17, y la escala decía 17,5.
		pasos := pasosDeEscala(r)
		leida := parseComa(t, numRazon(r.Razon))
		if math.Abs(redondearMedio(r.Base*leida)-pasos[escalaPasosAbajo+1]) > 0.26 {
			t.Errorf("%s: con la razón IMPRESA (%s) el escalón siguiente daría %v, y el brief muestra %v",
				clave, numRazon(r.Razon), redondearMedio(r.Base*leida), pasos[escalaPasosAbajo+1])
		}
	}
}

// C-ESC4 · EL BLOQUE NUNCA SALE MUDO.
//
// `shape` puede venir vacío —hay ejes que son propiedades y no esqueletos, y ese vacío es una
// respuesta— pero la escala no: una pregunta sobre la paleta también se compone sobre una pantalla.
// El bloque tiene que traer números aunque no haya forma, y tiene que traer los de CADA registro que
// abarcan las candidatas: servir la tabla de una densidad para una forma de otra es peor que no
// servir ninguna, porque parece una decisión.
func TestEscalaSiempreTraeNumeros(t *testing.T) {
	casos := [][]string{
		nil,                          // eje sin formas (color, a11y, tipografía)
		{"tabla-densa"},              // un solo registro
		{"tabla-densa", "narrativa"}, // dos extremos
		{"tablero-un-numero", "monitor-procesos", "lista-priorizada"}, // los tres
	}
	for _, formas := range casos {
		txt := escalaPara(formas)

		// LOS ESPERADOS SE DERIVAN A MANO, NO CON `registrosDe`. La primera versión los pedía con
		// `registrosDe(formas)` y el sabotaje «emitir siempre los tres» la dejó VERDE: el test estaba
		// comparando la salida contra la misma función que la produce, así que los dos lados se
		// movían juntos. Es el control que mide el proxy en vez de la cosa.
		esperados := map[string]bool{}
		for _, f := range formas {
			esperados[registroDeForma[f]] = true
		}
		if len(esperados) == 0 {
			esperados[registroPorDefecto] = true
		}
		var regs []string
		for r := range esperados {
			regs = append(regs, r)
		}
		if len(regs) == 0 {
			t.Fatalf("formas=%v: no se esperó ningún registro", formas)
		}
		for _, reg := range regs {
			r := registrosDeEscala[reg]
			if !strings.Contains(txt, "REGISTRO "+r.Nombre) {
				t.Errorf("formas=%v: falta la tabla del registro %q, que le toca a una de las candidatas", formas, reg)
			}
		}
		// Y ningún registro de más: emitir los tres siempre gasta presupuesto que le sale al corpus.
		for clave, r := range registrosDeEscala {
			if !contiene(regs, clave) && strings.Contains(txt, "REGISTRO "+r.Nombre) {
				t.Errorf("formas=%v: se emitió el registro %q y ninguna candidata cae ahí", formas, clave)
			}
		}
		// Números concretos: el bloque entero existe para que el brief deje de ser adjetivos. Si sale
		// sin px, salió mudo.
		if n := strings.Count(txt, " px"); n < 3 {
			t.Errorf("formas=%v: el bloque trae %d menciones de px — volvió a ser prosa", formas, n)
		}
		if !strings.Contains(txt, "4,5:1") || !strings.Contains(txt, "ms ") {
			t.Errorf("formas=%v: falta el núcleo universal (contraste y duraciones)", formas)
		}
	}
}

// C-ESC5 · EL BRIEF NO SIRVE DOS ESCALAS QUE SE CONTRADICEN.
//
// Lo que motivó todo esto: `principles` decía «usá una escala (11/12/13/15/18/24/30)» mientras el
// bloque nuevo sirve 13 · 15,5 · 18,5 · 22,5 · 27 · 32,5. Dos sistemas rivales en el mismo brief, y
// quien compone elige el que agarra primero — que es justo el mecanismo por el que un motor con más
// material entrega un diseño peor. El núcleo estático NO puede volver a fijar tamaños ni la rejilla.
func TestEscalaLosPrincipiosNoFijanNumeros(t *testing.T) {
	// Se buscan los sospechosos concretos de la versión vieja, no un patrón genérico: un test que
	// prohibiera «cualquier dígito» se rompería con la numeración de los propios principios y
	// terminaría desactivado.
	for _, prohibido := range []string{"11/12/13/15/18/24/30", "8/12/16/24/32/40", "GRILLA 4pt", "múltiplos de 4"} {
		if strings.Contains(designPrinciples, prohibido) {
			t.Errorf("designPrinciples vuelve a fijar números (%q) y contradice el bloque `scale`", prohibido)
		}
	}
	if !strings.Contains(designPrinciples, "'scale'") {
		t.Error("designPrinciples no manda al bloque `scale`: sin el puntero, el agente no sabe de dónde salen los números")
	}
	// El emit de painter traía su propia tercera escala («título 22–30, subtítulo 14–18…»).
	if strings.Contains(designEmitPainter, "22–30") || strings.Contains(designEmitPainter, "13–15") {
		t.Error("designEmitPainter vuelve a traer su propia escala tipográfica")
	}
}

// formasDeRegistro devuelve una forma cualquiera que caiga en el registro pedido, para poder pedir el
// bloque de UNA densidad. Determinista: recorre `ordenRegistros` sobre el mapa de formas ordenado por
// la propia tabla, no por el orden de iteración del mapa.
func formasDeRegistro(reg string) []string {
	var mejor string
	for clave, r := range registroDeForma {
		if r == reg && (mejor == "" || clave < mejor) {
			mejor = clave
		}
	}
	if mejor == "" {
		return nil
	}
	return []string{mejor}
}

// parseComa lee un número escrito a la castellana. Vive acá y no en el código de producción a
// propósito: la prueba tiene que leer lo que el brief IMPRIME, no reusar el float del que salió.
func parseComa(t *testing.T, s string) float64 {
	t.Helper()
	var ent, dec float64
	partes := strings.SplitN(s, ",", 2)
	for _, c := range partes[0] {
		ent = ent*10 + float64(c-'0')
	}
	if len(partes) == 2 {
		div := 1.0
		for _, c := range partes[1] {
			div *= 10
			dec += float64(c-'0') / div
		}
	}
	return ent + dec
}
