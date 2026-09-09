package mcp

// El guion de redespliegue verificaba la migración contra un número TIPEADO A MANO.

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
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
	//
	// LAXO EN LA FORMA, DURO EN EL HECHO. Antes esto buscaba el `regex:` hasta 4 líneas por
	// encima del `action: drop` y, si no lo encontraba, hacía `t.Skip`. Las dos mitades estaban
	// mal y se tapaban entre sí: meter cuatro comentarios entre las dos líneas —un NO-OP para
	// Prometheus, el descarte sigue igual de activo— hacía que el parser no viera el regex, y el
	// Skip apagaba la prueba ENTERA reportando PASS. Medido.
	//
	// Ahora la presencia del descarte se detecta APARTE de poder leer su regex, y no encontrar el
	// regex de un descarte que existe es un fallo duro: es el parser roto, no «no aplica».
	var activo string
	var hayDescarte bool
	lineas := strings.Split(string(promYml), "\n")
	for i, l := range lineas {
		if !strings.Contains(l, "action: drop") || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		hayDescarte = true
		// Hacia atrás hasta el borde del bloque —otro `- ` de la lista, u otro `action:`— en vez
		// de un tope de líneas, que es lo que el relleno rompía. Los comentarios se saltean.
		for j := i - 1; j >= 0; j-- {
			tr := strings.TrimSpace(lineas[j])
			if strings.HasPrefix(tr, "#") || tr == "" {
				continue
			}
			if strings.HasPrefix(tr, "regex:") {
				activo = strings.Trim(strings.TrimSpace(strings.TrimPrefix(tr, "regex:")), `"`)
				break
			}
			if strings.HasPrefix(tr, "- ") || strings.HasPrefix(tr, "action:") {
				break
			}
		}
	}
	if hayDescarte && activo == "" {
		t.Fatal("hay un `action: drop` activo en prometheus.yml y no pude leer su `regex:`. NO es «no aplica»: es que este parser dejó de encontrarlo, y mientras tanto el descarte sigue descartando. Arreglá el parser antes de creerle al verde")
	}

	// (1) Lo que sólo sale por el scrape no puede caer en el descarte. Sólo esta mitad depende de
	// que exista un descarte; la (2) corre siempre.
	if activo != "" {
		re, err := regexp.Compile("^(?:" + activo + ")$")
		if err != nil {
			t.Fatalf("el regex del descarte no compila (%q): %v", activo, err)
		}
		for _, n := range seriesSoloDelScrape {
			if re.MatchString(n) {
				t.Errorf("el cerebro emite %q SÓLO por el scrape y el descarte (%s) lo agarra: esa serie no llega a Prometheus, y la alerta que la consuma no va a disparar nunca — sin un solo error", n, activo)
			}
		}
	} else {
		t.Log("no hay descarte activo en el scrape: la mitad (1) no aplica. La (2) corre igual.")
	}

	// (2) Y no puede haber series sin declarar.
	//
	// ════════════════════════════════════════════════════════════════════════════════════════
	// ESTA MITAD LEÍA UN ARCHIVO Y ESTABA DOBLEMENTE CIEGA
	//
	// Antes hacía `os.ReadFile("fleet_prometheus.go")` y buscaba `"(musubi_fleet_[a-z_]+)"`.
	// No custodiaba lo que decía custodiar, por dos motivos a la vez:
	//
	//   1. Leía UN archivo de los veinte que emiten series de flota. `nombrePoliticaAcciones`
	//      sale de observability.go, así que era invisible para esta guarda.
	//   2. Y aunque hubiera leído el archivo correcto, el patrón exigía que el nombre fuera una
	//      cadena ENTERA entre comillas. Una serie emitida con `Fprintf("%s{policy=%q}", ...)`
	//      o nombrada en su línea `# HELP` no matchea ese patrón NUNCA.
	//
	// Consecuencia medida: ensanchar el descarte del scrape a `musubi_fleet_.*` —el error exacto
	// que ya se cometió el 2026-08-31— pasaba en VERDE, y se llevaba puesta la única serie de la
	// familia sin copia por OTLP, con tres alertas colgando.
	//
	// Ensanchar el grep no es el arreglo: `musubi_fleet_*` es TAMBIÉN el prefijo de las tools de
	// flota del registry, así que barrer el paquete inunda de falsos positivos que no son series.
	// La única fuente de verdad es la SALIDA, donde un nombre de tool no aparece jamás. Así que
	// se renderiza /metrics igual que el handler de http.go: los tres renders, en ese orden.
	declaradas := map[string]bool{}
	for _, n := range seriesSoloDelScrape {
		declaradas[n] = true
	}
	for _, s := range seriesDeFlota(time.Now(), time.Minute, "dev", nil) {
		declaradas[s.Nombre] = true
	}

	srv := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	maquinaConMuestra(t, srv, "casa", "pc-gio", *muestraDePrueba(), ahora)
	// Sin política sembrada `renderPoliticas` corta antes de emitir, y la serie que esta guarda
	// dejó pasar durante meses volvería a ser invisible — ahora por falta de datos en vez de por
	// el patrón. Sembrar es lo que hace que la prueba EJERZA el camino que dice cubrir.
	srv.metrics.sembrarPoliticas([]string{"nginx-vivo"})

	var render strings.Builder
	render.WriteString(srv.metrics.render(srv.engine))
	renderFlota(&render, srv.engine, ptrPrincipal(principalDePrometheus()), ahora, srv.sondaIntervalo, versionDePrueba, nil)
	srv.renderEmpuje(&render, ahora)

	// En el formato de exposición el nombre aparece de tres formas: al principio de una muestra,
	// detrás de `# HELP` y detrás de `# TYPE`. Se toman las tres.
	emitidas := map[string]bool{}
	for _, l := range strings.Split(render.String(), "\n") {
		l = strings.TrimSpace(l)
		if resto, ok := strings.CutPrefix(l, "# HELP "); ok {
			l = resto
		} else if resto, ok := strings.CutPrefix(l, "# TYPE "); ok {
			l = resto
		} else if strings.HasPrefix(l, "#") || l == "" {
			continue
		}
		if i := strings.IndexAny(l, "{ "); i >= 0 {
			l = l[:i]
		}
		if strings.HasPrefix(l, "musubi_fleet_") {
			emitidas[l] = true
		}
	}

	// PISO: si el render se rompe o deja de cubrir un camino, el mapa queda corto y el bucle de
	// abajo no recorre nada. Una prueba que pasa sobre cero series es el mismo agujero de antes.
	if len(emitidas) < 5 {
		t.Fatalf("sólo se emitieron %d series musubi_fleet_* en el render; la prueba no probaría nada", len(emitidas))
	}
	// PIN DE LA REGRESIÓN: ésta es la serie que la versión anterior no podía ver, por vivir en
	// otro archivo y salir por Fprintf. Si deja de aparecer acá, volvimos al agujero exacto.
	if !emitidas[nombrePoliticaAcciones] {
		t.Errorf("%s no aparece en el render: es la única serie de flota sin copia por OTLP, y es la que esta guarda no veía. Si el render dejó de cubrir observability.go, el agujero volvió", nombrePoliticaAcciones)
	}

	for n := range emitidas {
		if !declaradas[n] {
			t.Errorf("/metrics emite %q y no está declarada ni en seriesDeFlota (viaja por OTLP) ni en seriesSoloDelScrape: nadie sabe si el descarte se la lleva", n)
		}
	}
}
