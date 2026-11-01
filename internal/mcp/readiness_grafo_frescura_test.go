package mcp

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// readiness_grafo_frescura_test.go cubre que la dimensión `grafo` de musubi_readiness mida si el
// índice SIRVE y no sólo si existe.
//
// La regresión que estos tests existen para cazar: la dimensión se titula «El código está indexado»
// y lo verificaba con dos chequeos de VOLUMEN —nodos > 0 y aristas > 0—, así que puntuaba 1.0 con un
// solo nodo y una sola arista. Los tres indicadores de si el índice coincide con el disco los
// calculaba `graphFreshness` y los leía únicamente `musubi_map`. Medido el 2026-09-05 en el repo
// real: 213 de 798 `.go` sin un solo nodo, `internal/fleet` entero entre ellos, y la dimensión daba
// 1.0 igual.
//
// ⚠️ TODOS COMPRUEBAN EL DENOMINADOR ANTES QUE EL NUMERADOR, por la trampa de la mutación vacua que
// este repo ya midió: sembrar un fingerprint distinto no prueba nada si el archivo no entró al
// cómputo —un `stored` vacío da cero stale y el test pasa sin medir—, y el montaje tampoco mide
// nada si cae por la rama del árbol fuera de alcance. Las dos cosas se afirman primero.

// nombreAlDia es el chequeo que este cambio agrega. En una constante porque cuatro tests lo buscan
// por nombre: si se renombra, las cuatro fallas dicen lo mismo y en un solo lugar.
const nombreAlDia = "el índice está al día"

// grafoSembrado arma un servidor con `archivos` .go en disco, todos indexados, y le rompe el
// fingerprint guardado a los primeros `rotos` (quedan STALE: el nodo dice una cosa y el disco otra).
// conAristas=false deja el grafo como una lista de símbolos, que es el caso donde el chequeo de
// volumen de aristas falla y se puede ver si el puntaje de la dimensión se mueve.
func grafoSembrado(t *testing.T, archivos, rotos int, conAristas bool) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })

	raiz := t.TempDir()
	files := make([]string, 0, archivos)
	nodes := make([]memory.GraphNode, 0, archivos)
	for i := 0; i < archivos; i++ {
		rel := fmt.Sprintf("a%02d.go", i)
		fp := escribirGo(t, raiz, rel, fmt.Sprintf("package p\n\nfunc F%02d() {}\n", i))
		if i < rotos {
			fp = "fingerprint-que-no-coincide-con-el-disco"
		}
		files = append(files, rel)
		nodes = append(nodes, memory.GraphNode{
			Key: rel + "#func:F", Kind: "func", Name: fmt.Sprintf("F%02d", i),
			Path: rel, SrcFingerprint: fp,
		})
	}
	var edges []memory.GraphEdge
	if conAristas && archivos >= 2 {
		edges = []memory.GraphEdge{{
			FromKey: nodes[0].Key, ToKey: nodes[1].Key, Kind: "calls",
			Confidence: 1, Provenance: "test", SrcPath: files[0],
			SrcFingerprint: nodes[0].SrcFingerprint,
		}}
	}
	if err := engine.UpsertPackageGraph(files, nodes, edges); err != nil {
		t.Fatal(err)
	}
	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})
	// DENOMINADOR: por este camino la frescura no se mide y el test no probaría lo que declara.
	if s.arbolFueraDeAlcance() {
		t.Fatal("el árbol salió FUERA DE ALCANCE en un montaje que mide frescura: " +
			"el enriquecido se iría por la rama «no aplica» y el test quedaría vacuo")
	}
	return s
}

// dimGrafo devuelve la dimensión `grafo` del reporte, o falla. Que exista es parte del denominador.
func dimGrafo(t *testing.T, rep memory.ReadinessReport) memory.ReadinessDimension {
	t.Helper()
	for _, d := range rep.Dimensions {
		if d.Key == "grafo" {
			return d
		}
	}
	t.Fatalf("el reporte no trae la dimensión `grafo` (dimensiones: %d)", len(rep.Dimensions))
	return memory.ReadinessDimension{}
}

// chequeo busca un chequeo por nombre; nil si no está.
func chequeo(d memory.ReadinessDimension, nombre string) *memory.ReadinessCheck {
	for i := range d.Checks {
		if d.Checks[i].Name == nombre {
			return &d.Checks[i]
		}
	}
	return nil
}

// fraccion recalcula el puntaje con SU PROPIA aritmética a propósito: un test que le pidiera el
// número a la misma función que verifica estaría comparando la implementación consigo misma.
func fraccion(cs []memory.ReadinessCheck) float64 {
	if len(cs) == 0 {
		return 0
	}
	ok := 0
	for _, c := range cs {
		if c.Pass {
			ok++
		}
	}
	return float64(ok) / float64(len(cs))
}

// tolerancia: el reporte redondea los puntajes a dos decimales (2/3 se publica como 0.67), así que
// la comparación no puede ser exacta ni atarse a la fórmula concreta del redondeo.
const tolerancia = 0.006

// I1 — UN GRAFO PODRIDO NO PUNTÚA 1.0. Es el agujero completo, en un test.
//
// Montaje: un solo archivo, indexado con un fingerprint que no coincide con el disco, y una arista.
// Los dos chequeos de volumen pasan (hay nodos, hay aristas), así que lo único que puede bajar el
// puntaje es el chequeo nuevo. ANTES de este cambio este mismo estado daba 1.0.
//
// Sabotaje que lo pone en rojo: fijar `Pass: true` en el chequeo de frescura.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="Pass: peor <= readinessGrafoFrescuraMax,"
// arnes: a="Pass: true,"
// arnes: colision_ok="TestElUmbralDeFrescuraEsProporcional"
func TestGrafoPodridoNoPuntuaPerfecto(t *testing.T) {
	s := grafoSembrado(t, 2, 2, true)
	rep := readinessComo(t, s, nil)
	d := dimGrafo(t, rep)

	// DENOMINADOR: la dimensión tiene que estar observada y traer el chequeo, o lo de abajo no mide.
	if !d.Observed {
		t.Fatal("la dimensión no quedó observada: hay nodos y aristas sembrados, así que esto " +
			"significa que el grafo no se escribió y el test no está midiendo frescura")
	}
	alDia := chequeo(d, nombreAlDia)
	if alDia == nil {
		t.Fatalf("no está el chequeo %q: la dimensión sigue mirando sólo volumen (chequeos: %+v)",
			nombreAlDia, d.Checks)
	}

	// NUMERADOR.
	if alDia.Pass {
		t.Errorf("el chequeo de frescura PASÓ con los 2 archivos del grafo desactualizados "+
			"(100 %%): %s", alDia.Why)
	}
	if d.Score >= 1.0 {
		t.Errorf("la dimensión `grafo` puntúa %.2f con el índice entero podrido. Es el agujero "+
			"que este cambio cierra: «El código está indexado» se verificaba contando nodos",
			d.Score)
	}
	if stale, ok := d.Evidence["stale"]; !ok || stale.(float64) != 2 {
		t.Errorf("evidence[stale] = %v, esperaba 2", d.Evidence["stale"])
	}
}

// I2 — EL PUNTAJE REFLEJA EL CHEQUEO, en la dimensión Y en el global.
//
// El bug más probable de esta implementación: agregar el chequeo y no recalcular. El engine ya
// calculó los puntajes sobre los dos chequeos viejos, así que el chequeo nuevo aparece en rojo y el
// número se queda en 1.0 — un chequeo de adorno, que es `guarda-definida-y-desconectada` un nivel
// más adentro.
//
// Se afirma la CONSISTENCIA en las dos alturas, y cada mitad cae por su lado:
//   - (a) el puntaje de la dimensión es la fracción de SUS chequeos → caza "no recalculé la dimensión"
//   - (b) el global es el promedio de las dimensiones → caza "recalculé la dimensión y no el global"
//
// Sabotaje: borrar la llamada a rep.Recalcular() (a), o recalcular sólo dim.Score sin el global (b).
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="\trep.Recalcular()\n}"
// arnes: a="\t// recalculo quitado a proposito\n}"
func TestPuntajeReflejaLaFrescura(t *testing.T) {
	s := grafoSembrado(t, 2, 2, true)
	rep := readinessComo(t, s, nil)
	d := dimGrafo(t, rep)

	// DENOMINADOR: tres chequeos, uno de ellos el nuevo y en rojo. Sin esto, (a) y (b) se cumplen
	// trivialmente sobre la dimensión vieja de dos chequeos verdes.
	if len(d.Checks) != 3 {
		t.Fatalf("la dimensión tiene %d chequeos, esperaba 3 (los dos de volumen + la frescura): %+v",
			len(d.Checks), d.Checks)
	}
	if alDia := chequeo(d, nombreAlDia); alDia == nil || alDia.Pass {
		t.Fatalf("el montaje no dejó el chequeo de frescura en rojo; sin un chequeo que falle, " +
			"un puntaje sin recalcular no se distingue de uno recalculado")
	}

	// (a) el puntaje de la dimensión sale de sus propios chequeos.
	if esperado := fraccion(d.Checks); math.Abs(d.Score-esperado) > tolerancia {
		t.Errorf("dimensión `grafo`: Score = %.4f, pero sus chequeos dan %.4f (%d de %d cumplidos). "+
			"El chequeo quedó de adorno: está en rojo y el número no bajó",
			d.Score, esperado, len(d.Checks)-1, len(d.Checks))
	}

	// (b) el global es el promedio de las dimensiones.
	var suma float64
	for _, dd := range rep.Dimensions {
		suma += dd.Score
	}
	esperadoGlobal := suma / float64(len(rep.Dimensions))
	if math.Abs(rep.Score-esperadoGlobal) > tolerancia {
		t.Errorf("reporte: Score = %.4f, pero el promedio de sus %d dimensiones es %.4f. "+
			"Se recalculó la dimensión y no el reporte",
			rep.Score, len(rep.Dimensions), esperadoGlobal)
	}
}

// I3 — LA EVIDENCIA TRAE LOS CINCO CRUDOS.
//
// Es la regla que `musubi_readiness` declara en su propia nota: «el puntaje es opinable, la
// evidencia no». Un porcentaje ya es una opinión —lleva el umbral adentro—; quien no esté de
// acuerdo con el 5 % tiene que poder recalcular con el suyo, y para eso necesita los números y sus
// DOS denominadores, que son distintos entre sí.
//
// Montaje con 21 archivos y 1 roto a propósito: ahí el crudo (1) y la proporción (0,0476) no se
// parecen, así que guardar uno en lugar del otro es visible. Con stale=0 los dos valdrían 0 y el
// test no distinguiría nada.
//
// Sabotaje: guardar pStale/pGhosts/pMissing en vez de los contadores.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="dim.Evidence[\"stale\"] = stale"
// arnes: a="dim.Evidence[\"stale\"] = proporcion(stale, enGrafo)"
func TestEvidenciaDeFrescuraTraeLosCrudos(t *testing.T) {
	s := grafoSembrado(t, 21, 1, true)
	d := dimGrafo(t, readinessComo(t, s, nil))

	// DENOMINADOR: sin el chequeo no hay enriquecido, y la evidencia podría venir de otra cosa.
	if chequeo(d, nombreAlDia) == nil {
		t.Fatalf("no está el chequeo %q: el enriquecido no corrió", nombreAlDia)
	}

	esperado := map[string]float64{
		"stale": 1, "ghosts": 0, "missing": 0,
		"archivos_en_grafo": 21, "archivos_en_disco": 21,
	}
	for clave, quiero := range esperado {
		bruto, ok := d.Evidence[clave]
		if !ok {
			t.Errorf("falta evidence[%q]: sin el crudo, el umbral no se puede recalcular", clave)
			continue
		}
		got, ok := bruto.(float64) // JSON: todo número vuelve float64
		if !ok {
			t.Errorf("evidence[%q] = %v (%T), esperaba un número", clave, bruto, bruto)
			continue
		}
		if got != quiero {
			t.Errorf("evidence[%q] = %v, esperaba %v. Si vino 0,0476 es la PROPORCIÓN: "+
				"la evidencia son los crudos, el porcentaje va en el Why", clave, got, quiero)
		}
	}
}

// I4 — DONDE LA FRESCURA NO APLICA, NO SE AGREGA EL CHEQUEO.
//
// En el cerebro central el grafo es federado y sus archivos no están en este disco: medir
// fingerprints marcaría todo como fantasma. Pero la salida fácil es igual de mala: agregar el
// chequeo, que pasaría gratis y SUBIRÍA el puntaje por no haber mirado. Es el cero ambiguo que
// `profundidadDe` ya documentó — «el valor de fallo disfrazado de valor tranquilizador».
//
// Montaje SIN aristas a propósito: así la dimensión vale 1/2 = 0.5, y un chequeo regalado la
// llevaría a 2/3 = 0.67. Con los dos chequeos de volumen en verde el puntaje ya sería 1.0 y el
// sabotaje no movería el número — el test pasaría sin medir el daño que declara.
//
// Sabotaje: borrar el `return` de la rama arbolFueraDeAlcance. Lo que cae NO es el chequeo —la
// guarda de los dos denominadores lo frena igual, y por eso este sabotaje quedaba VERDE— sino el
// MOTIVO, que pasa a decir que no hay archivos cuando lo que pasa es que el árbol es federado.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="marcaría todo como fantasma\"\n\t\treturn\n\t}"
// arnes: a="marcaría todo como fantasma\"\n\t}"
func TestDondeNoAplicaNoSeAgregaElChequeo(t *testing.T) {
	s := grafoSembrado(t, 2, 0, false)
	s.arbolAusente.Store(true) // el central: el árbol no está en este disco
	d := dimGrafo(t, readinessComo(t, s, nil))

	// DENOMINADOR: la dimensión está observada (hay nodos) y le falta el chequeo de aristas, que es
	// lo que deja el puntaje en 0.5 y hace visible un chequeo regalado.
	if !d.Observed {
		t.Fatal("la dimensión no quedó observada: el montaje siembra nodos, así que el grafo no se escribió")
	}
	if len(d.Checks) != 2 {
		t.Fatalf("la dimensión tiene %d chequeos, esperaba los 2 de volumen y NINGUNO de frescura "+
			"(el árbol no está en disco: no hay nada que medir): %+v", len(d.Checks), d.Checks)
	}

	// NUMERADOR.
	if alDia := chequeo(d, nombreAlDia); alDia != nil {
		t.Errorf("se agregó %q con el árbol fuera de alcance, y pasó = %v. Un chequeo que pasa "+
			"porque no se midió nada sube el puntaje POR NO MIRAR", nombreAlDia, alDia.Pass)
	}
	if math.Abs(d.Score-0.5) > tolerancia {
		t.Errorf("la dimensión puntúa %.2f, esperaba 0.50 (1 de 2 chequeos de volumen). "+
			"0.67 significa que ganó un tercer chequeo gratis", d.Score)
	}
	// EL MOTIVO TIENE QUE SER EL DE ESTA RAMA, Y NO SÓLO ESTAR. Que exista no alcanza: hay DOS
	// guardas en serie que publican `frescura_no_medida` con textos distintos, y la de abajo
	// SOBRESCRIBE a ésta si el flujo le llega. Verificar sólo la presencia dejaba el sabotaje de
	// esta rama en VERDE —lo cazó `arnes -overlay`, que `-validar` no ve—: quitarle el `return` no
	// deja pasar el chequeo, porque `graphFreshness` tiene su PROPIA guarda de
	// `arbolFueraDeAlcance()` (methods_codegraph.go) y vuelve con los cinco en cero, así que el
	// flujo cae en la guarda de los dos denominadores y sale por ahí. El chequeo sigue sin
	// agregarse, los contadores siguen sin publicarse, y lo único que cambia es el DIAGNÓSTICO.
	//
	// Y eso es justo lo que no da igual. «No hay un solo archivo contra el que medir» le dice al
	// operador que indexe; «el árbol no está en este disco» le dice que no hay nada que hacer. Dos
	// remedios opuestos para el mismo campo del tablero, y el equivocado manda a trabajar al que no
	// tiene trabajo. Un invariante con dos guardas no se custodia mirando lo que las dos comparten:
	// se custodia mirando lo que las distingue.
	motivo, _ := d.Evidence["frescura_no_medida"].(string)
	switch {
	case motivo == "":
		t.Error("falta evidence[frescura_no_medida]: no medir es un hecho del reporte, " +
			"y callarlo deja la dimensión indistinguible de una donde la frescura dio bien")
	case !strings.Contains(motivo, "no está en este disco"):
		t.Errorf("evidence[frescura_no_medida] = %q. El árbol está fuera de alcance, así que el "+
			"motivo tiene que ser ÉSE y no el de los denominadores en cero: el primero no tiene "+
			"remedio y el segundo manda a indexar", motivo)
	}
	// Y no inventa podredumbre: los contadores no se publican donde no se midieron.
	for _, clave := range []string{"stale", "ghosts", "missing"} {
		if v, ok := d.Evidence[clave]; ok {
			t.Errorf("evidence[%q] = %v con el árbol fuera de alcance: un cero ahí se lee "+
				"como «se midió y está sano»", clave, v)
		}
	}
	// `Observed` NO se toca: la dimensión sí fue observada, lo que falta es un sub-aspecto. En false
	// la mandaría a Unobserved con score 0 por regla, castigando a un central sano.
	for _, k := range rep_unobserved(t, s) {
		if k == "grafo" {
			t.Error("`grafo` quedó en Unobserved: hay nodos medidos, y un central sano no " +
				"puede puntuar 0 por una frescura que no corresponde medirle")
		}
	}
}

// rep_unobserved vuelve a pedir el reporte y devuelve sus dimensiones sin dato.
func rep_unobserved(t *testing.T, s *McpServer) []string {
	t.Helper()
	return readinessComo(t, s, nil).Unobserved
}

// I5 — LOS DOS CHEQUEOS DE VOLUMEN QUEDAN INTACTOS.
//
// El contrato de musubi_readiness lo consumen la cabina y el CRM; esto es ADITIVO. Un consumidor
// que agrupe por nombre de chequeo se rompe en silencio si uno se renombra, y «en silencio» es el
// problema: el JSON sigue siendo válido y el tablero muestra una categoría vacía.
//
// Sabotaje: renombrar cualquiera de los dos.
// arnes: archivo="internal/memory/readiness.go"
// arnes: de="{Name: \"hay símbolos indexados\", Pass: nodos > 0"
// arnes: a="{Name: \"hay simbolos indexados\", Pass: nodos > 0"
func TestLosChequeosDeVolumenQuedanIntactos(t *testing.T) {
	s := grafoSembrado(t, 2, 2, true)
	d := dimGrafo(t, readinessComo(t, s, nil))

	// DENOMINADOR: el enriquecido corrió. Si no, los dos de volumen están intactos por no haber
	// pasado nada, y el test no mide que SOBREVIVEN al enriquecido.
	if chequeo(d, nombreAlDia) == nil {
		t.Fatalf("no está el chequeo %q: el enriquecido no corrió y este test no mide nada", nombreAlDia)
	}

	for _, nombre := range []string{"hay símbolos indexados", "hay relaciones, no sólo una lista"} {
		c := chequeo(d, nombre)
		if c == nil {
			t.Errorf("desapareció el chequeo %q: el cambio tenía que ser aditivo", nombre)
			continue
		}
		if !c.Pass {
			t.Errorf("el chequeo %q falló con nodos y aristas sembrados (%s): se le cambió "+
				"la semántica, no sólo el nombre", nombre, c.Why)
		}
	}
	if d.Evidence["nodos"] == nil || d.Evidence["aristas"] == nil {
		t.Error("el enriquecido pisó la evidencia de volumen (nodos/aristas)")
	}
}

// I6 — EL UMBRAL ES PROPORCIONAL, NO BINARIO.
//
// Con tolerancia cero, crear un archivo y no indexarlo todavía tira abajo la dimensión, y un
// chequeo que está en rojo durante el trabajo normal se aprende a ignorar. El 5 % está derivado de
// 300 commits de este repo: el peor tocó 65 de 1.395 archivos (4,66 %).
//
// Montaje: 1 de 21 desactualizados = 4,76 %, justo abajo del tope. El chequeo PASA y el Why lo dice.
//
// Sabotaje: volver el chequeo binario (`stale == 0 && ghosts == 0 && missing == 0`).
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="Pass: peor <= readinessGrafoFrescuraMax,"
// arnes: a="Pass: stale == 0 && ghosts == 0 && missing == 0,"
// arnes: colision_ok="TestGrafoPodridoNoPuntuaPerfecto"
func TestElUmbralDeFrescuraEsProporcional(t *testing.T) {
	s := grafoSembrado(t, 21, 1, true)
	d := dimGrafo(t, readinessComo(t, s, nil))

	alDia := chequeo(d, nombreAlDia)
	if alDia == nil {
		t.Fatalf("no está el chequeo %q", nombreAlDia)
	}
	// DENOMINADOR: que haya 1 stale de 21. Con 0 stale el chequeo pasa igual y el test sería vacuo
	// — probaría que un grafo sano pasa, no que el umbral tolera un archivo.
	if v, ok := d.Evidence["stale"].(float64); !ok || v != 1 {
		t.Fatalf("evidence[stale] = %v, esperaba exactamente 1: sin un archivo desactualizado "+
			"este test no distingue un umbral proporcional de uno binario", d.Evidence["stale"])
	}

	if !alDia.Pass {
		t.Errorf("el chequeo falló con 1 de 21 desactualizados (4,76 %%, tope 5 %%): el umbral "+
			"quedó binario, y entonces está en rojo cada vez que alguien toca un archivo. Why: %s",
			alDia.Why)
	}
	if math.Abs(d.Score-1.0) > tolerancia {
		t.Errorf("la dimensión puntúa %.2f con el grafo sano dentro del umbral, esperaba 1.00", d.Score)
	}
	// El umbral viaja en el texto: es lo que lo hace discutible sin ir a buscarlo al código.
	if !strings.Contains(alDia.Why, "tope") {
		t.Errorf("el Why no dice contra qué umbral se midió: %q", alDia.Why)
	}
}

// I7 — UNA DIMENSIÓN SIN DATO NO GANA EL CHEQUEO. (Apareció al implementar, no estaba en el design.)
//
// Si el grafo nunca se indexó, la dimensión vale 0 por la regla de oro de readiness: «una señal no
// observada puntúa cero, no se saltea; no medir no puede subir el puntaje». Agregarle el chequeo de
// frescura la rompe por otro lado: la dimensión empieza a PUBLICAR un chequeo y cinco contadores
// sobre un índice que no existe. La guarda protege el CONTRATO —una dimensión sin dato no trae
// chequeos, ni en verde ni en rojo— y de paso el costo: medir frescura es leer y hashear el árbol
// entero para un veredicto que ya está cerrado.
//
// ⚠️ NO protege el PUNTAJE, aunque este comentario lo afirmó hasta que un panel adversarial lo midió
// (2026-11-01). Decía que sin la guarda la dimensión terminaría «puntuando 1.00». Ese 1.00 es
// INALCANZABLE: sin nodos el grafo no declara archivos, así que `enGrafo == 0` y el loop de
// `graphFreshness` deja `missing == enDisco`, o sea el eje en 1.0 —ROJO SEGURO— si hay algo en
// disco, y si no hay nada corta la guarda de los dos denominadores en cero. Medido: Score=0.00 en
// base Y con el sabotaje. ESTE TEST CAE POR `len(Checks)`, no por el puntaje.
//
// Montaje: grafo vacío y un árbol con UN `.go` indexable que ese grafo jamás vio. El árbol vacío
// —que es lo que este test montaba— no servía: con los dos denominadores en cero la guarda de abajo
// corta antes, el chequeo no se agrega por otro motivo, y el sabotaje dejaba la suite en verde.
//
// Sabotaje: borrar la guarda `if !dim.Observed { return }`.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="\tif !dim.Observed {"
// arnes: a="\tif false && !dim.Observed {"
func TestDimensionSinDatoNoGanaElChequeo(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	raiz := t.TempDir() // el árbol TIENE un .go indexable que el grafo (vacío) jamás vio
	escribirGo(t, raiz, "sin_indexar.go", "package p\n\nfunc X() {}\n")
	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})

	// DENOMINADOR 1: por la rama del árbol fuera de alcance el chequeo tampoco se agregaría, y el
	// test pasaría por el motivo equivocado — mediría I4, no esto.
	if s.arbolFueraDeAlcance() {
		t.Fatal("el árbol salió fuera de alcance: este test tiene que llegar a la guarda de " +
			"`Observed`, no quedarse en la de alcance")
	}

	rep := readinessComo(t, s, nil)
	d := dimGrafo(t, rep)

	// DENOMINADOR 2: la dimensión tiene que estar SIN DATO, que es la condición que se prueba.
	if d.Observed {
		t.Fatalf("la dimensión quedó observada con el grafo vacío: %+v", d.Evidence)
	}

	// DENOMINADOR 3: el árbol tiene que traer un indexable que el grafo no vio, o la guarda de los
	// dos denominadores en cero corta antes y el chequeo falta por OTRO motivo — el test pasaría por
	// la razón equivocada, que es justo lo que le pasaba. Se mide llamando a `graphFreshness`
	// directo y NO leyendo la evidencia: en esta rama la guarda corta antes de publicarla.
	_, _, missing, enGrafo, enDisco := s.graphFreshness(s.scopedCtx(context.Background()))
	if enGrafo != 0 || enDisco < 1 || missing < 1 {
		t.Fatalf("el montaje no reproduce el estado: enGrafo=%d (esperaba 0), enDisco=%d y missing=%d "+
			"(esperaba >= 1 los dos). Hace falta un .go en la raíz que el grafo nunca haya visto",
			enGrafo, enDisco, missing)
	}

	// NUMERADOR.
	if len(d.Checks) != 0 {
		t.Errorf("una dimensión sin dato trae %d chequeos: %+v. La guarda de `Observed` no está "+
			"cortando: con el grafo vacío no hay nada contra qué medir, y publicar el chequeo convierte "+
			"«no medí» en un veredicto", len(d.Checks), d.Checks)
	}
	if d.Score != 0 {
		t.Errorf("una dimensión SIN DATO puntúa %.2f. La regla del archivo es que no observado "+
			"vale 0: no medir no puede subir el puntaje. Ojo: este aserto es el CINTURÓN, no el "+
			"custodio de la guarda — el chequeo que se publica sin ella sale en rojo, así que el "+
			"puntaje sigue en 0 y el que cae es el de len(Checks)", d.Score)
	}
	hayGrafo := false
	for _, k := range rep.Unobserved {
		if k == "grafo" {
			hayGrafo = true
		}
	}
	if !hayGrafo {
		t.Error("`grafo` salió de Unobserved sin que nadie lo indexara")
	}
}

// grafoSinNadaQueMedir monta el estado de los dos denominadores en cero: un nodo SIN archivo
// —GraphStatsCtx lo cuenta, así que nodos > 0 y la dimensión queda observada, pero
// GraphFileFingerprintsCtx lo descarta por `path != ”`, así que el grafo declara CERO archivos— y
// un árbol en disco sin un solo `.go`. Lo comparten los dos custodios de esa rama.
func grafoSinNadaQueMedir(t *testing.T) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	nodo := memory.GraphNode{Key: "#func:HuerfanoSinArchivo", Kind: "func", Name: "HuerfanoSinArchivo"}
	if err := engine.UpsertPackageGraph(nil, []memory.GraphNode{nodo}, nil); err != nil {
		t.Fatal(err)
	}
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}) // árbol sin un solo .go

	// Por la rama del árbol fuera de alcance el chequeo tampoco se agregaría, y los dos custodios
	// pasarían midiendo I4 en vez de lo suyo.
	if s.arbolFueraDeAlcance() {
		t.Fatal("el árbol salió fuera de alcance: este montaje tiene que llegar más allá de esa guarda")
	}
	// Y los dos denominadores tienen que venir en cero, que es la condición que se prueba. Se mide
	// llamando a `graphFreshness` directo y NO leyendo la evidencia: esa rama ya no la publica, justo
	// para no afirmar un cero que puede no haberse medido.
	if _, _, _, enGrafo, enDisco := s.graphFreshness(s.scopedCtx(context.Background())); enGrafo != 0 || enDisco != 0 {
		t.Fatalf("esperaba los dos denominadores en cero y llegaron archivos_en_grafo=%d "+
			"archivos_en_disco=%d: el montaje no reproduce el estado sin nada que medir", enGrafo, enDisco)
	}
	return s
}

// I8 — SIN UN SOLO ARCHIVO CONTRA EL QUE MEDIR, EL CHEQUEO TAMPOCO SE GANA.
//
// Es la SEGUNDA PUERTA al premio por no medir, y la guarda de `Observed` no la cubre. `GraphStatsCtx`
// cuenta TODOS los nodos, pero `GraphFileFingerprintsCtx` filtra `WHERE path != ”`
// (internal/memory/codegraph.go), así que un grafo cuyos nodos no declaran archivo da `nodos > 0`
// —y entonces `Observed` es true y la guarda de I7 deja pasar— con CERO archivos para comparar. No es
// hipotético: este repo tiene hoy un nodo con `path = ”`, encontrado midiendo el costo de esta misma
// función. El mismo estado llega por otra vía: si la consulta de fingerprints falla (base bloqueada),
// `graphFreshness` devuelve todo en cero, los dos denominadores incluidos.
//
// Con los dos denominadores en cero, las tres proporciones son 0 POR CONSTRUCCIÓN —`proporcion` lo
// hace a propósito, para no publicar un NaN— y el chequeo pasaría sin haber mirado un archivo.
//
// Montaje: un nodo con `Path` vacío (lo cuenta GraphStats, lo filtra GraphFileFingerprints) y un
// árbol en disco sin un solo `.go`.
//
// Sabotaje: borrar la guarda de los dos denominadores en cero.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="\tif enGrafo == 0 && enDisco == 0 {"
// arnes: a="\tif false && enGrafo == 0 && enDisco == 0 {"
func TestSinArchivosMediblesNoGanaElChequeo(t *testing.T) {
	s := grafoSinNadaQueMedir(t)
	rep := readinessComo(t, s, nil)
	d := dimGrafo(t, rep)

	// DENOMINADOR 2: la dimensión TIENE que estar observada, o este test es I7 otra vez.
	if !d.Observed {
		t.Fatalf("la dimensión quedó NO observada: con nodos = %v la guarda de `Observed` ya corta "+
			"y este test no distingue la segunda puerta de la primera", d.Evidence["nodos"])
	}
	// (el DENOMINADOR 3 —los dos en cero— lo verifica `grafoSinNadaQueMedir` por `graphFreshness`.)

	if c := chequeo(d, nombreAlDia); c != nil {
		t.Errorf("el chequeo %q se agregó con CERO archivos contra los que medir (Pass=%v, Why=%q): "+
			"pasa gratis y sube el puntaje POR NO HABER MIRADO, que es el premio que `readiness` "+
			"existe para impedir", nombreAlDia, c.Pass, c.Why)
	}
	if math.Abs(d.Score-fraccion(d.Checks)) > tolerancia {
		t.Errorf("el puntaje (%.2f) no corresponde a los chequeos presentes (%.2f)",
			d.Score, fraccion(d.Checks))
	}
	if _, hay := d.Evidence["frescura_no_medida"]; !hay {
		t.Errorf("no se declaró POR QUÉ no hay chequeo de frescura: un chequeo que desaparece sin "+
			"explicación se lee como que la dimensión no lo necesita. Evidencia: %v", d.Evidence)
	}
}

// I9 — DONDE LA FRESCURA NO SE MIDIÓ, SUS CONTADORES NO SE PUBLICAN. (Lo pidió el panel adversarial
// de `verify`; la rama hermana —árbol fuera de alcance— ya lo cumplía y ésta no.)
//
// La regla no es de forma. En esta rama los cinco ceros son EL CERO AMBIGUO que todo el cambio existe
// para desarmar: `graphFreshness` tiene un retorno temprano por falla de la consulta que ocurre ANTES
// de caminar el disco, así que por esa vía `archivos_en_disco: 0` no es una medición sino un valor de
// fallo, y `missing: 0` tampoco. Desde `enriquecerGrafo` las dos situaciones son indistinguibles, así
// que el diagnóstico va en `frescura_no_medida` —que nombra las dos— y no en un número que un
// consumidor automático lee como «cero problemas». Un lector humano ve el texto; una alerta, no.
//
// Montaje: el mismo de I8, los dos denominadores en cero.
//
// Sabotaje: devolver una de las cinco asignaciones a donde estaba antes de este cambio, o sea
// arriba de la guarda.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="\tstale, ghosts, missing, enGrafo, enDisco := s.graphFreshness(scoped)"
// arnes: a="\tstale, ghosts, missing, enGrafo, enDisco := s.graphFreshness(scoped)\n\tdim.Evidence[\"archivos_en_disco\"] = enDisco"
func TestSinArchivosMediblesNoPublicaContadores(t *testing.T) {
	s := grafoSinNadaQueMedir(t)
	d := dimGrafo(t, readinessComo(t, s, nil))

	// DENOMINADOR: el motivo tiene que estar declarado, o esto no es la rama que se prueba.
	motivo, hay := d.Evidence["frescura_no_medida"]
	if !hay {
		t.Fatalf("no es la rama sin nada que medir: falta `frescura_no_medida`. Evidencia: %v", d.Evidence)
	}

	// NUMERADOR: ni uno de los cinco.
	for _, clave := range []string{"stale", "ghosts", "missing", "archivos_en_grafo", "archivos_en_disco"} {
		if v, publicado := d.Evidence[clave]; publicado {
			t.Errorf("evidence[%q] = %v se publicó en la rama donde la frescura NO se midió: ese cero "+
				"puede ser un valor de fallo (la consulta vuelve antes de caminar el disco) y se lee "+
				"como «cero problemas». El motivo ya está declarado: %q", clave, v, motivo)
		}
	}
}
