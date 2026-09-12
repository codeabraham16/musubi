package mcp

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/arnes"
)

// ═════════════════════════════════════════════════════════════════════════════════════════════
// LA DEUDA DE SABOTAJES SE MIDE EN CI, Y NO PUEDE CRECER EN SILENCIO
//
// El árbol promete un sabotaje por guarda y lo escribe en prosa. Medido el 2026-09-12 sobre
// `git ls-files`: **774 anclas en 163 archivos de prueba**, y el único veredicto que existió alguna
// vez fue a mano —A107 y A120 cubrieron 38, el 4,9 %— con **6 de esos 38 que NO rompían lo que
// decían romper**. O sea que la tasa medida de sabotajes que mienten es del 16 %, aplicada a menos
// de la veinteava parte del árbol.
//
// EL CABO A123 LLEVA LA CUENTA Y TAMBIÉN ENUMERA UNA FORMA: cuenta la frase canónica «Sabotaje que
// la hace fallar» y por eso declara 396 en 93 archivos. El árbol la escribe de 30 formas —incluidas
// 42 anclas EN MAYÚSCULAS que ningún grep del registro ve— así que la deuda real es **1,95× la
// registrada**, y hay **70 archivos de prueba que el registro no ve enteros**. Es el defecto
// dominante del repo aplicado a la contabilidad de su propia deuda.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE ESTA GUARDA EXIGE, Y POR QUÉ CADA COSA
//
// Correr los 774 sabotajes cuesta horas —cada uno son cuatro invocaciones de `go test`— y no entra
// en CI. Eso es `go run ./deploy/cmd/arnes -correr`, a mano y por lotes. Lo que SÍ entra en CI, en
// milisegundos, es comprobar que el corpus no se podrió:
//
//	· CERO DIRECTIVAS ROTAS. Una directiva que no se puede leer se contaría como «mecanizada» y
//	  nadie la correría: la cobertura subiría con sabotajes que no existen.
//	· CERO DIRECTIVAS QUE DEJARON DE APUNTAR. Si el `de` ya no está en el archivo —porque el código
//	  se movió— el sabotaje NO SE APLICA, y su verde se lee igual que «la guarda cubre». Es la
//	  tercera cara de «el `-run` del sabotaje escrito a mano».
//	· CERO ANCLAS SIN UBICAR. Un agujero en el enumerador se ve idéntico a un árbol sin deuda.
//	· LA DEUDA NO CRECE. Un ancla nueva sin directiva es una promesa nueva que nadie puede correr.
//
// Sabotaje que la hace fallar: agregar un `// Sabotaje: algo` a cualquier `_test.go` sin su
// `arnes:` (la deuda sube y pasa el techo), o cambiarle el `de` a cualquier directiva por un texto
// que no exista.
// `prueba=` VA DECLARADO PORQUE ESTA ANCLA FLOTA: vive en la cabecera del archivo y no pegada al
// `func Test…`, así que el lector no la puede derivar del AST. Y no la saltea en silencio — la
// denunció en su primera corrida, que es cómo apareció esta línea.
// arnes: prueba="TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre"
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\tif n := strings.Count(string(b), de); n != 1 {"
// arnes: a="\tif n := strings.Count(string(b), de); n != 1 && false {"

// anclasEnProsaAlDia es LA DEUDA MEDIDA, con su fecha. Es un hecho del mundo —cuántas promesas sin
// ejecutar tenía el árbol ese día— así que va clavado: derivarlo del árbol dejaría a esta guarda
// midiéndose contra sí misma y aceptando cualquier número.
//
// SÓLO BAJA. Si tu cambio la sube, la salida NO es subir esta constante: es escribir la directiva
// `arnes:` de tu ancla nueva, o declararla `no_mecanizable="<motivo>"` si el sabotaje literal no
// puede compilar. Subir el techo es exactamente cómo una guarda se convierte en la defensora del
// defecto que vigila — y este repo ya lo pagó con `TestG1`, que enumeraba cuatro herramientas y
// PROHIBÍA arreglar las otras seis.
const anclasEnProsaAlDia = 774

// holguraDelTecho es cuánto se deja bajar antes de exigir que el techo se ajuste.
//
// LAS DOS DIRECCIONES SON NECESARIAS Y LA SEGUNDA NO ES OBVIA. Un techo que sólo prohíbe subir se
// PODRE: se mecanizan 300 anclas, el techo sigue en 774, y la guarda deja de medir sin ponerse
// roja ni una vez. Con la holgura, un lote grande obliga a bajar el número — que es el único
// registro de que la deuda bajó.
const holguraDelTecho = 30

func TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre(t *testing.T) {
	raiz := filepath.Join("..", "..")
	c, err := arnes.Censar(raiz)
	if err != nil {
		t.Fatalf("no pude censar el árbol: %v — no medí nada", err)
	}

	// EL CONTROL VA PRIMERO: un cero acá no es «el árbol no promete sabotajes», es «este censo no
	// miró nada», y todo lo que sigue daría verde.
	if len(c.Anclas) == 0 || c.Archivos < 400 {
		t.Fatalf("el censo miró %d archivos y encontró %d anclas: eso no es un árbol sin deuda, es "+
			"un enumerador que no está mirando el repo", c.Archivos, len(c.Anclas))
	}

	if len(c.SinUbicar) > 0 {
		t.Errorf("el lector vio %d ancla/s en el texto crudo y no pudo colocarlas en el AST. Un agujero "+
			"en el enumerador se ve idéntico a un árbol sano:\n  %s",
			len(c.SinUbicar), strings.Join(c.SinUbicar, "\n  "))
	}

	if rotas := c.Rotas(); len(rotas) > 0 {
		var lineas []string
		for _, a := range rotas {
			lineas = append(lineas, a.Archivo+":"+strconv.Itoa(a.Linea)+": "+strings.Join(a.Quejas, "; "))
		}
		t.Errorf("%d directiva/s `arnes:` no se pueden leer. NO cuentan como mecanizadas a propósito: "+
			"una directiva ilegible que se cuenta como cubierta hace subir la cobertura con sabotajes "+
			"que nadie puede correr.\n  %s", len(rotas), strings.Join(lineas, "\n  "))
	}

	// ESTA ES LA QUE IMPIDE QUE EL CORPUS SE PODRA, Y ES LA RAZÓN DE QUE ESTA GUARDA VIVA EN CI.
	// Los 774 sabotajes no se pueden correr acá; que sus anclas sigan existiendo, sí.
	if males := arnes.Validar(c); len(males) > 0 {
		t.Errorf("%d directiva/s dejaron de apuntar a donde dicen. Un `de` que ya no está significa que "+
			"el sabotaje NO SE APLICA, y su verde se lee igual que «la guarda cubre»:\n  %s",
			len(males), strings.Join(males, "\n  "))
	}

	pendientes := len(c.Pendientes())
	switch {
	case pendientes > anclasEnProsaAlDia:
		t.Errorf("LA DEUDA SUBIÓ: %d anclas en prosa contra un techo de %d.\n"+
			"Agregaste %d promesa/s de sabotaje que nadie puede correr. La salida NO es subir "+
			"`anclasEnProsaAlDia`: es escribirle la directiva `arnes:` a tu ancla nueva\n"+
			"    // arnes: archivo=\"ruta/al/archivo.go\"\n"+
			"    // arnes: de=\"el literal exacto a reemplazar\"\n"+
			"    // arnes: a=\"con qué se reemplaza (vacío borra)\"\n"+
			"o declararla `no_mecanizable=\"<motivo>\"` si el sabotaje literal no puede compilar "+
			"(el caso típico: la guarda es el único lector de un import o de una variable, así que "+
			"borrarla deja `imported and not used` y el rojo sería por build roto).\n"+
			"Correlo con: go run ./deploy/cmd/arnes -detalle",
			pendientes, anclasEnProsaAlDia, pendientes-anclasEnProsaAlDia)
	case anclasEnProsaAlDia-pendientes > holguraDelTecho:
		t.Errorf("EL TECHO SE AFLOJÓ: hay %d anclas en prosa y el techo dice %d (%d de más).\n"+
			"Bajá `anclasEnProsaAlDia` a %d. Un techo que sólo prohíbe subir se podre: deja de medir "+
			"sin ponerse rojo ni una vez, y el número que baja es el único registro de que la deuda bajó.",
			pendientes, anclasEnProsaAlDia, anclasEnProsaAlDia-pendientes, pendientes)
	}

	t.Logf("%d archivos · %d anclas · %d mecanizadas · %d exentas · %d en prosa (techo %d) · cobertura %.1f %%",
		c.Archivos, len(c.Anclas), len(c.Mecanizadas()), len(c.Exentas()), pendientes, anclasEnProsaAlDia,
		100*float64(len(c.Mecanizadas()))/float64(len(c.Anclas)))
}
