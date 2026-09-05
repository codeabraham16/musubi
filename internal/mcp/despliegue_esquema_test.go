package mcp

// El guion de redespliegue verificaba la migración contra un número TIPEADO A MANO.

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"musubi/internal/memory"
)

// LA COMPROBACIÓN DE MIGRACIÓN NO PUEDE VOLVER A QUEDARSE VIEJA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ESTO ES UNA PRUEBA Y NO UN COMENTARIO
//
// `deploy/redesplegar-cerebro.sh` decía `[[ "$ESQUEMA" -ge 37 ]]`, escrito cuando la última
// migración era la 37. Entre la 37 y la 44 esa línea siguió pasando y dejó de verificar nada:
// 44 ≥ 37 es cierto, y también lo sería con la migración cortada en la 40. Nadie lo notó porque
// **una comprobación que no puede ponerse roja se ve idéntica a una que funciona**.
//
// Un comentario que diga «acordate de actualizar esto» tiene el mismo destino que el número que
// reemplaza. La única forma de que no vuelva a pasar es que el guion DERIVE el número del binario
// —`musubi version --esquema`— y que algo se ponga rojo si alguien vuelve a tipearlo.
//
// Sabotaje: volver a poner un `-ge 44` en el guion, o sacarle el `version --esquema`.
func TestElRedespliegueNoTipeaLaVersionDeEsquema(t *testing.T) {
	crudo, err := os.ReadFile("../../deploy/redesplegar-cerebro.sh")
	if err != nil {
		t.Fatalf("no pude leer el guion de redespliegue: %v", err)
	}
	guion := string(crudo)

	// TIENE QUE ESTAR EN UNA SUSTITUCIÓN QUE CAPTURE LA SALIDA, NO EN CUALQUIER LUGAR DEL TEXTO.
	//
	// `strings.Contains` a secas pasaba en verde con el sabotaje declarado. `version --esquema`
	// aparece TRES veces en el guion: en un comentario que explica el arreglo, en el mensaje de
	// aviso de cuando el binario no sabe contestar, y —la única que decide algo— en la asignación
	// de `$ESPERADO`. Cambiar esa asignación por `ESPERADO=46`, que es exactamente el defecto que
	// esta prueba existe para prohibir, dejaba las otras dos en pie y la guarda en verde.
	// Medido el 2026-09-05; cuarto caso del día de un texto que NOMBRA la cautela y la satisface.
	//
	// La propiedad es estructural: el guion tiene que EJECUTAR al binario y quedarse con lo que
	// contesta, o sea `$(… version --esquema …)`. Un comentario y un mensaje no ejecutan nada.
	invoca := regexp.MustCompile(`\$\([^)]*version --esquema`)
	if !invoca.MatchString(codigoDe(guion)) {
		t.Error("el guion ya no EJECUTA `version --esquema` para saber a qué esquema apunta el binario\n" +
			"(nombrarlo en un comentario o en un mensaje no cuenta): la verificación de la migración\n" +
			"volvió a depender de que alguien se acuerde de actualizar un número")
	}

	// Un número comparado contra `$ESQUEMA` es exactamente la forma que se quiere prohibir. No se
	// prohíben los dígitos en general —el guion tiene timeouts y modos de archivo— sino la
	// comparación del esquema contra una constante.
	tipeado := regexp.MustCompile(`\$ESQUEMA"?\s*(-ge|-eq|-gt|==|!=)\s*"?[0-9]+`)
	if m := tipeado.FindString(guion); m != "" {
		t.Errorf("el esquema se compara contra un número tipeado (%q): eso es lo que se quedó viejo entre la 37 y la 44 sin que nadie lo viera", m)
	}
}

// Y EL BINARIO TIENE QUE SABER DECIRLO. Si `EsquemaEsperado` dejara de seguir a las migraciones,
// el guion verificaría contra un número equivocado con toda confianza.
//
// Sabotaje: que EsquemaEsperado devuelva una constante.
func TestElBinarioDiceElEsquemaAlQueApunta(t *testing.T) {
	// La lista de migraciones es la única fuente: se compara contra la MAYOR versión declarada,
	// leída del propio archivo, para que agregar una migración sin tocar nada más rompa esto si
	// EsquemaEsperado dejara de derivarse.
	crudo, err := os.ReadFile("../memory/migrations.go")
	if err != nil {
		t.Fatalf("no pude leer migrations.go: %v", err)
	}
	versiones := regexp.MustCompile(`(?m)^\s+version:\s+(\d+),`).FindAllStringSubmatch(string(crudo), -1)
	if len(versiones) == 0 {
		t.Fatal("no encontré ninguna migración declarada: el regex de esta prueba quedó viejo")
	}
	mayor := 0
	for _, v := range versiones {
		n := 0
		for _, c := range v[1] {
			n = n*10 + int(c-'0')
		}
		if n > mayor {
			mayor = n
		}
	}
	if got := memory.EsquemaEsperado(); got != mayor {
		t.Errorf("EsquemaEsperado() = %d y la migración más alta declarada es %d: el guion de despliegue verificaría contra un número equivocado", got, mayor)
	}
}

// COMPILAR PARA OTRA PLATAFORMA NO PUEDE TERMINAR EN ROJO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// UN GUION QUE REPORTA FRACASO CUANDO TUVO ÉXITO ENSEÑA A IGNORAR SU CÓDIGO DE SALIDA
//
// `construir.sh` corría `"$SALIDA" version` incondicionalmente. Con `GOOS=windows` —que es como
// se arma el agente de Windows desde este servidor— eso muere en «cannot execute binary file», y
// con `set -e` el guion sale en rojo DESPUÉS de haber escrito el binario perfectamente. Ni
// siquiera llegaba a imprimir el sha256, que es lo único que el otro lado necesita para comprobar
// que le llegó lo que se compiló.
//
// Y el código de salida de ese guion es el que decide si un despliegue sigue.
//
// Sabotaje: sacar el `if` que compara GOOS/GOARCH con los del host.
func TestConstruirNoIntentaCorrerUnBinarioDeOtraPlataforma(t *testing.T) {
	crudo, err := os.ReadFile("../../deploy/construir.sh")
	if err != nil {
		t.Fatalf("no pude leer construir.sh: %v", err)
	}
	guion := string(crudo)

	// LA COMPARACIÓN TIENE QUE SER UNA CONDICIÓN, NO UNA MENCIÓN.
	//
	// `strings.Contains(guion, "GOHOSTOS")` se satisface con la línea que calcula `DESTINO_OS`
	// —que nombra `GOHOSTOS` como valor por defecto— y con cualquier comentario. Medido el
	// 2026-09-05 con el sabotaje declarado: cambiar el `if` entero por `if true; then` deja la
	// guarda en VERDE, y eso es exactamente el defecto que existe para prohibir — en una
	// compilación cruzada intenta correr un binario de Windows en Linux y la corrida termina en
	// rojo sobre un binario que salió bien.
	//
	// La propiedad es que exista una CONDICIÓN que compare las dos dimensiones —sistema y
	// arquitectura— del destino contra las del host.
	condicion := regexp.MustCompile(`(?m)^\s*if\s.*DESTINO_OS.*GOHOSTOS.*DESTINO_ARCH.*GOHOSTARCH`)
	if !condicion.MatchString(codigoDe(guion)) {
		t.Error("construir.sh ya no CONDICIONA nada a que la plataforma destino sea la del host\n" +
			"(nombrar GOHOSTOS en el valor por defecto de DESTINO_OS no alcanza): una compilación\n" +
			"cruzada vuelve a intentar ejecutar el binario y termina en rojo sobre algo que salió bien")
	}
	// El `sha256sum` tiene que quedar FUERA del condicional: es lo que el otro lado usa para
	// verificar, y perderlo en una compilación cruzada es perder justo el dato del caso remoto.
	i := strings.Index(guion, "GOHOSTOS")
	j := strings.LastIndex(guion, "sha256sum")
	if i < 0 || j < i {
		t.Error("el sha256sum quedó antes de la comprobación de plataforma: en una compilación cruzada no se imprimiría")
	}
	// Y la comprobación tiene que envolver a `version`, no a otra cosa.
	tramo := guion[i:]
	if k := strings.Index(tramo, "sha256sum"); k < 0 || !strings.Contains(tramo[:k], `"$SALIDA" version`) {
		t.Error("`$SALIDA version` no quedó adentro del condicional de plataforma")
	}
}

// NINGUNA SERIE QUE PRODUCE EL CEREBRO PUEDE CAER EN EL DESCARTE DEL SCRAPE.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL MODO DE FALLA ES EL PEOR QUE HAY: TODO VERDE Y EL EJE APAGADO
//
// `deploy/prometheus/prometheus.yml` descarta `musubi_fleet_(device|service)_.*` en el scrape,
// porque esa familia es telemetría POR MÁQUINA y llega por el empuje OTLP: la copia del scrape
// sería duplicación. Las series que produce EL CEREBRO no tienen copia por OTLP, así que si una
// se llama con ese prefijo, se pierde.
//
// Y no se nota: la alerta que la consume usa `and on(...)` y con la serie ausente NO DISPARA. Sin
// errores, sin logs, sin nada rojo. Pasó con `musubi_fleet_device_net_up` — se desplegó y se
// «verificó» aceptando su ausencia como correcta, que es exactamente cómo se ve un drop.
//
// SON DOS COMPROBACIONES Y LA SEGUNDA ES LA QUE SOSTIENE A LA PRIMERA:
//  1. ninguna serie de `seriesSoloDelScrape` cae en el regex activo;
//  2. TODA serie nombrada en el exportador está declarada en algún lado —o viaja por OTLP (la
//     tabla de seriesDeFlota) o es sólo del scrape—. Sin esto, agregar una serie y olvidarse de
//     la lista devuelve el agujero intacto, y una lista que se queda vieja no protege de nada.
//
// Sabotaje: renombrar nombreVidaDeRed a `musubi_fleet_device_*`; o agregar una serie al
// exportador sin declararla; o cambiar el regex del prometheus.yml sin mirar el exportador.
func TestNingunaSerieDelCerebroCaeEnElDescarteDelScrape(t *testing.T) {
	promYml, err := os.ReadFile("../../deploy/prometheus/prometheus.yml")
	if err != nil {
		t.Fatalf("no pude leer prometheus.yml: %v", err)
	}
	// El regex ACTIVO, no el comentado: una línea de ejemplo no descarta nada.
	var activo string
	lineas := strings.Split(string(promYml), "\n")
	for i, l := range lineas {
		if !strings.Contains(l, "action: drop") || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		for j := i - 1; j >= 0 && j > i-5; j-- {
			tr := strings.TrimSpace(lineas[j])
			if strings.HasPrefix(tr, "regex:") && !strings.HasPrefix(tr, "#") {
				activo = strings.Trim(strings.TrimSpace(strings.TrimPrefix(tr, "regex:")), `"`)
			}
		}
	}
	if activo == "" {
		t.Skip("no hay ningún descarte activo en el scrape: esta guarda no aplica")
	}
	re, err := regexp.Compile("^(?:" + activo + ")$")
	if err != nil {
		t.Fatalf("el regex del descarte no compila (%q): %v", activo, err)
	}

	// (1) Lo que sólo sale por el scrape no puede caer en el descarte.
	for _, n := range seriesSoloDelScrape {
		if re.MatchString(n) {
			t.Errorf("el cerebro emite %q SÓLO por el scrape y el descarte (%s) lo agarra: esa serie no llega a Prometheus, y la alerta que la consuma no va a disparar nunca — sin un solo error", n, activo)
		}
	}

	// (2) Y no puede haber series sin declarar. Se comparan los literales del exportador contra
	// la unión de las dos declaraciones.
	declaradas := map[string]bool{}
	for _, n := range seriesSoloDelScrape {
		declaradas[n] = true
	}
	for _, s := range seriesDeFlota(time.Now(), time.Minute, "dev", nil) {
		declaradas[s.Nombre] = true
	}
	fuente, err := os.ReadFile("fleet_prometheus.go")
	if err != nil {
		t.Fatalf("no pude leer el exportador: %v", err)
	}
	nombres := regexp.MustCompile(`"(musubi_fleet_[a-z_]+)"`).FindAllStringSubmatch(string(fuente), -1)
	if len(nombres) < 3 {
		t.Fatalf("sólo se detectaron %d series en el exportador; el patrón se rompió y esta prueba no probaría nada", len(nombres))
	}
	for _, m := range nombres {
		if !declaradas[m[1]] {
			t.Errorf("el exportador nombra %q y no está declarada ni en seriesDeFlota (viaja por OTLP) ni en seriesSoloDelScrape: nadie sabe si el descarte se la lleva", m[1])
		}
	}
}
