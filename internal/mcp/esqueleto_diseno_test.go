package mcp

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// C-ESQ1 · TODA FORMA SE PUEDE DIBUJAR.
//
// Una forma sin esqueleto no rompe nada: `bocetosDe` la saltea. Y ése es el problema — el usuario
// pide tres bocetos, recibe dos, y nada se pone rojo. El modo de falla es agregar la decimotercera
// forma y olvidarse del dibujo, igual que con el registro de escala.
func TestEsqueletoTodaFormaSeDibuja(t *testing.T) {
	for clave := range formasDeDiseno {
		if _, ok := esqueletosDeForma[clave]; !ok {
			t.Errorf("la forma %q no tiene esqueleto: pediría tres bocetos y llegarían dos, sin aviso", clave)
			continue
		}
		if svg := bocetoDe(clave); svg == "" {
			t.Errorf("la forma %q tiene esqueleto y no dibujó nada", clave)
		}
	}
	for clave := range esqueletosDeForma {
		if _, ok := formasDeDiseno[clave]; !ok {
			t.Errorf("hay esqueleto para %q y esa forma no existe en el catálogo", clave)
		}
	}
	// Y los tres de un pedido real llegan completos.
	if got := bocetosDe(candidatasDeForma("tabla", nil, intencionDeDiseno{})); len(got) != designFormasPropuestas {
		t.Errorf("llegaron %d bocetos y se proponen %d formas", len(got), designFormasPropuestas)
	}
}

// Acepta comilla simple o doble a propósito: el invariante es sobre la GEOMETRÍA, no sobre el
// delimitador. La primera versión sólo miraba comillas dobles y, al pasar el SVG a comillas simples
// —para que no se escapen dentro del JSON—, reportó «0 cajas, prácticamente vacío» en las doce formas.
// El test falló fuerte, que está bien, pero el mensaje señalaba al dibujo cuando el roto era él.
var reRect = regexp.MustCompile(`<rect[^>]*?x=['"]([-\d.]+)['"] y=['"]([-\d.]+)['"] width=['"]([-\d.]+)['"] height=['"]([-\d.]+)['"]`)

// C-ESQ2 · NINGÚN BOCETO SE SALE DEL MARCO NI DIBUJA EN LA NADA.
//
// La aritmética de cajas es donde se cuela el error silencioso: un peso mal repartido saca una caja
// del viewBox y el SVG sigue siendo válido, así que no falla nada — sólo se ve mal. Y una caja de
// ancho negativo o NaN simplemente NO SE DIBUJA: el boceto sale incompleto sin un solo error, que es
// el mismo modo de falla que ya nos comió 586 sinapsis en el panel.
func TestEsqueletoNadaSeSaleNiDesaparece(t *testing.T) {
	for clave := range formasDeDiseno {
		svg := bocetoDe(clave)
		rects := reRect.FindAllStringSubmatch(svg, -1)
		if len(rects) < 3 {
			t.Errorf("%s: el boceto tiene %d cajas — está prácticamente vacío", clave, len(rects))
			continue
		}
		for _, m := range rects {
			num := func(s string) float64 {
				v, err := strconv.ParseFloat(s, 64)
				if err != nil {
					t.Fatalf("%s: coordenada ilegible %q (¿NaN?)", clave, s)
				}
				return v
			}
			x, y, w, h := num(m[1]), num(m[2]), num(m[3]), num(m[4])
			if w <= 0 || h <= 0 {
				t.Errorf("%s: caja de %.1f×%.1f — no se dibuja y no avisa", clave, w, h)
			}
			// Tolerancia de medio píxel por el redondeo a una decimal del formato.
			if x < -0.5 || y < -0.5 || x+w > bocetoAncho+0.5 || y+h > bocetoAlto+0.5 {
				t.Errorf("%s: caja fuera del marco (%.1f,%.1f %.1f×%.1f) en %d×%d",
					clave, x, y, w, h, bocetoAncho, bocetoAlto)
			}
		}
		if !strings.HasPrefix(svg, "<svg viewBox=") || !strings.HasSuffix(svg, "</svg>") {
			t.Errorf("%s: el SVG no está cerrado o no declara viewBox — sin viewBox el navegador lo dibuja de 300×150", clave)
		}
		// LAS COMILLAS SON SIMPLES, y no es cosmética: el SVG viaja dentro de un string JSON, donde
		// cada comilla doble se escapa y cuesta el doble. Medido contra el pedido real de las notas
		// del CRM, con comillas dobles los tres bocetos pesaban 4.357 tokens y hacían que el brief se
		// pasara del tope duro.
		if strings.Contains(svg, `="`) {
			t.Errorf("%s: el SVG usa comillas dobles; dentro del JSON se escapan y cuestan el doble", clave)
		}
		// currentColor y NADA de color propio: el boceto es estructura, y una paleta acá se cruzaría
		// con la marca del proyecto.
		if strings.Contains(svg, "#") || strings.Contains(svg, "rgb(") {
			t.Errorf("%s: el boceto declara un color propio; tiene que heredar con currentColor", clave)
		}
	}
}

// C-ESQ3 · LA DENSIDAD DEL BOCETO SALE DE `scale`, NO DE UNA CONSTANTE.
//
// Es el nudo entre las dos capas y lo que hace que el boceto sirva: si las filas fueran un número
// escrito a mano, el boceto compacto y el editorial se verían iguales y la decisión de densidad
// quedaría invisible justo en el paso que existe para mostrarla.
func TestEsqueletoLasFilasSalenDelRegistro(t *testing.T) {
	const alto = 130.0
	comp := filasQueEntran(alto, registrosDeEscala["compacto"])
	est := filasQueEntran(alto, registrosDeEscala["estandar"])
	edit := filasQueEntran(alto, registrosDeEscala["editorial"])
	if !(comp > est && est > edit) {
		t.Errorf("las filas no siguen al alto de fila del registro: compacto %d, estándar %d, editorial %d", comp, est, edit)
	}
	// Y LA PROPORCIÓN TIENE QUE SER LA REAL, no sólo el orden. Es lo que el tope rompe sin que se
	// note: con factorMiniatura=4 el registro compacto quedaba clavado en el tope de 16, así que la
	// razón dibujada salía 1,48 en vez de la verdadera 1,50 — el orden se mantenía y una prueba de
	// «compacto > editorial» pasaba verde igual. La primera versión de este test miraba sólo el orden
	// y dejaba pasar exactamente eso.
	razonReal := float64(registrosDeEscala["editorial"].Fila) / float64(registrosDeEscala["compacto"].Fila)
	razonDibujada := float64(comp) / float64(edit)
	if d := razonDibujada/razonReal - 1; d > 0.06 || d < -0.06 {
		t.Errorf("la proporción dibujada es %.3f y los altos de fila piden %.3f (filas %d/%d/%d): el tope está recortando la densidad",
			razonDibujada, razonReal, comp, est, edit)
	}

	// Y el dibujo tiene que USARLA. `tabla-densa` es compacta y su banda de filas ocupa 9 de 11.
	svg := bocetoDe("tabla-densa")
	// Cada fila dibuja tres cajas; se cuentan las de la banda de filas por su altura repetida.
	if n := strings.Count(svg, `<rect`); n < 3*est {
		t.Errorf("el boceto de la tabla densa tiene %d cajas: no llega ni a las %d filas del registro más suelto",
			n, est)
	}
}

// C-ESQ4 · LOS BOCETOS SE PAGAN SÓLO CUANDO SE PIDEN, Y LA NOTA VIAJA CON ELLOS.
//
// Cuestan presupuesto y no sirven en un pedido desde cero, así que van apagados por default — el
// mismo criterio que `keep` y `change`. Y la nota sin los bocetos es peor que nada: le dice al agente
// que muestre algo que no llegó.
func TestEsqueletoSoloCuandoSePiden(t *testing.T) {
	cands := candidatasDeForma("tabla", nil, intencionDeDiseno{})

	if got := bocetosSiSePiden(false, cands); got != nil {
		t.Errorf("sin pedirlos llegaron %d bocetos", len(got))
	}
	if got := notaSiHayBocetos(false, cands); got != "" {
		t.Error("sin bocetos llegó la nota que dice cómo mostrarlos")
	}
	if got := bocetosSiSePiden(true, cands); len(got) == 0 {
		t.Fatal("se pidieron bocetos y no llegó ninguno")
	}
	if got := notaSiHayBocetos(true, cands); got == "" {
		t.Error("llegaron los bocetos y no la nota: tres SVG sueltos se leen como decoración y se ignoran")
	}
	// Sin formas candidatas no hay nada que dibujar, y entonces tampoco nota.
	if got := notaSiHayBocetos(true, nil); got != "" {
		t.Error("sin formas llegó la nota igual")
	}

	// Cada boceto se identifica: sin nombre y sin «gana en», elegir entre tres wireframes parecidos
	// se vuelve una decisión estética, que es justo lo que este paso saca del medio.
	for _, b := range bocetosSiSePiden(true, cands) {
		if b.Nombre == "" || b.Registro == "" || b.SVG == "" {
			t.Errorf("boceto incompleto: %+v", struct{ F, N, R string }{b.Forma, b.Nombre, b.Registro})
		}
		if b.GanaEn == "" && len(destacaDe(b.Forma)) > 0 {
			t.Errorf("%s destaca en algo y el boceto no lo dice", b.Forma)
		}
	}
}

// C-ESQ5 · EL MISMO PEDIDO DA EL MISMO DIBUJO.
//
// Un boceto que cambia entre corridas no se puede comparar contra el de al lado ni contra el de ayer,
// y arruina la única razón por la que los dibuja el motor y no quien compone.
func TestEsqueletoEsDeterminista(t *testing.T) {
	for clave := range formasDeDiseno {
		primero := bocetoDe(clave)
		for i := 0; i < 50; i++ {
			if got := bocetoDe(clave); got != primero {
				t.Fatalf("%s: la corrida %d dio un SVG distinto", clave, i)
			}
		}
	}
}
