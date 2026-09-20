package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// A126 — NINGUNA SECCIÓN DEL INFORME PUEDE TERMINAR LA CORRIDA.
//
// `verificar-despliegue.sh` salía con `exit 2` en «reglas de alerta» cuando no podía leer las
// reglas cargadas, o sea cuando Prometheus no contestaba. El VEREDICTO era el correcto —«no vi» no
// es «está bien»— pero el `exit` se llevaba puesto todo lo que venía después.
//
// MEDIDO EL 2026-09-15 SOBRE ESTE REPO, con el puerto de Prometheus cerrado:
//
//	                      antes        después
//	  secciones            5 de 10      10 de 10
//	  líneas de informe    32           61
//	  veredicto impreso    ninguno      las dos líneas
//	  código de salida     2            2
//
// El código de salida es el MISMO, y eso es lo que hace al arreglo barato: `dudoso` prende
// `SIN_VERIFICAR`, que el bloque del veredicto traduce a la misma salida 2. No se bajó ningún
// listón; se dejó de confundir «esta comparación no se puede hacer» con «esta corrida se terminó».
//
// LO QUE EL CORTE ESCONDÍA NO ES HIPOTÉTICO. De los cinco `$VAR` pegados a un carácter no-ASCII que
// mataban el guion en el bash 3.2 de macOS, CUATRO eran preexistentes y nunca se habían visto,
// porque vivían debajo de este corte (ver `TestNingunaVariableDeShellQuedaPegadaAUnCaracterNoAscii`).
// Al abrirlo aparecieron dos más, y las dos son de la misma familia: un volcado de Python crudo en
// medio del informe, y cuatro ROJOS —o sea divergencias— afirmados sobre cero información. Las tres
// pruebas de abajo custodian las tres cosas.
//
// POR QUÉ TRES PREGUNTAS Y NO UNA LISTA DE SECCIONES. Una guarda que enumerara «y también corre
// `scrapes`, y también `alertmanager`…» habría que ampliarla con cada sección nueva, y el día que
// alguien agregue la undécima nadie se va a acordar. Las tres preguntas de acá no crecen: la última
// sección se DERIVA del guion, el corte se busca entre dos anclas de CÓDIGO, y la tercera pregunta
// es sobre una propiedad («no pude preguntar» ≠ «está mal») y no sobre un inventario.
// ════════════════════════════════════════════════════════════════════════════════════════════

var reColores = regexp.MustCompile("\x1b\\[[0-9;]*m")

// sinColores saca los escapes ANSI del informe.
//
// NO ES COSMÉTICA: `titulo` imprime en negrita, así que un título llega como `\033[1mscrapes\033[0m`
// y cualquier búsqueda anclada al principio de línea NO LO ENCUENTRA. Al medir esto a mano por
// primera vez el `grep` devolvió cero secciones sobre un informe que tenía cinco, y ese cero se lee
// igual que «no corrió ninguna». Es el recorte que se lee como ausencia, adentro del instrumento.
func sinColores(s string) string { return reColores.ReplaceAllString(s, "") }

// seccionesDelInforme parte la salida en secciones y devuelve el cuerpo de cada una.
//
// El criterio es el del propio informe y no una lista escrita acá: `titulo` imprime el nombre SIN
// sangría, y todo veredicto (`verde`, `rojo`, `dudoso`, `gris`) empieza con dos espacios. Así que un
// renglón sin sangría abre sección y los sangrados son su cuerpo. Derivado de la forma, una sección
// nueva entra sola.
func seccionesDelInforme(salida string) map[string]string {
	fuera := map[string]string{}
	actual := ""
	for _, l := range strings.Split(sinColores(salida), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if !strings.HasPrefix(l, " ") {
			actual = strings.TrimSpace(l)
			if _, hay := fuera[actual]; !hay {
				fuera[actual] = ""
			}
			continue
		}
		if actual != "" {
			fuera[actual] += l + "\n"
		}
	}
	return fuera
}

// ultimaSeccionQueDeclaraElGuion devuelve el texto del ÚLTIMO `titulo "…"` del guion.
//
// SE DERIVA Y NO SE ESCRIBE. Poner «guiones derivados» a mano acá sería una copia: el día que se
// agregue una sección al final, la guarda seguiría verde comprobando que se llega a la anteúltima —
// que es exactamente la mitad del defecto que A126 cierra. Y viene de `leerDeploy`, que blanquea los
// comentarios, así que un `titulo "…"` citado en una explicación no puede hacerse pasar por el real.
func ultimaSeccionQueDeclaraElGuion(t *testing.T, guion string) string {
	t.Helper()
	titulos := regexp.MustCompile(`titulo "([^"]+)"`).FindAllStringSubmatch(guion, -1)
	// EL CONTROL. Sin esto, el día que `titulo` se renombre esta guarda no encontraría ninguno,
	// `ultima` quedaría vacía, `strings.Contains(salida, "")` daría true y la prueba pasaría en
	// verde sin haber mirado nada.
	if len(titulos) < 8 {
		t.Fatalf("se encontraron %d llamadas a `titulo \"…\"` en verificar-despliegue.sh y el informe "+
			"tiene bastantes más: o cambió la forma de declarar una sección, o este extractor dejó de "+
			"encontrarlas y estaría por afirmar sobre la nada", len(titulos))
	}
	return titulos[len(titulos)-1][1]
}

// TestElInformeLlegaHastaElFinalAunqueNadieConteste es la pregunta de A126, hecha CORRIENDO el guion
// con las dos APIs cerradas — que es la única corrida que CI puede montar y, por eso mismo, la que
// durante meses no miró nada de la mitad de abajo.
//
// Se afirma sobre la ÚLTIMA sección y sobre el veredicto, y no sobre un conteo de secciones: un
// número exige actualizar la guarda cada vez que se agregue una, y una guarda que hay que acordarse
// de actualizar es una guarda que se queda vieja en silencio.
//
// Sabotaje que la pone roja: devolver el `exit 2` a la sección «reglas de alerta» → el informe
// termina en ese título y no llega nunca a la última sección.
// arnes: archivo="deploy/verificar-despliegue.sh"
// arnes: de="  dudoso \"no se compararon las reglas archivo por archivo"
// arnes: a="  exit 2\n  dudoso \"no se compararon las reglas archivo por archivo"
func TestElInformeLlegaHastaElFinalAunqueNadieConteste(t *testing.T) {
	compuerta := guiones.Unix(t, "corre deploy/verificar-despliegue.sh entero contra un repo de prueba con las dos "+
		"APIs cerradas, que es la corrida que este guion tiene que sobrevivir de punta a punta",
		"bash", "git", "python3")

	ultima := ultimaSeccionQueDeclaraElGuion(t, leerDeploy(t, "verificar-despliegue.sh"))
	salida := correrVerificador(t, compuerta, prepararRepoDePrueba(t))
	limpia := sinColores(salida)

	if !strings.Contains(limpia, ultima) {
		t.Errorf("el informe NO llegó a su última sección (%q) con las APIs cerradas.\n"+
			"  Alguna sección terminó la corrida por su cuenta en vez de marcar lo suyo SIN VERIFICAR\n"+
			"  y seguir. Eso deja sin mirar todo lo que viene después —que es donde vivían cuatro de\n"+
			"  los cinco `$VAR` pegados que mataban el guion en macOS— y encima se lleva el veredicto.\n"+
			"  El idioma correcto es el de la sección de las recording rules del SLA:\n"+
			"      if [ -z \"$REGLAS_JSON\" ]; then dudoso \"…\"; else …; fi\n"+
			"  Informe completo:\n%s", ultima, limpia)
	}

	// EL VEREDICTO ES PARTE DEL INFORME Y NO UN ADORNO: es la única línea que le dice al operador
	// qué significa el código de salida que recibió. Con el `exit` en el medio, la corrida terminaba
	// en 2 sin una sola palabra.
	if !strings.Contains(limpia, "SIN VERIFICAR") {
		t.Errorf("el informe no imprimió el bloque del veredicto.\n"+
			"  Con las dos APIs cerradas hay eslabones que no se pudieron preguntar, así que el guion\n"+
			"  tiene que decirlo con todas las letras además de salir con 2. Un código de salida sin su\n"+
			"  explicación obliga a correr todo otra vez a mano, que es el paso manual que este guion\n"+
			"  existe para eliminar.\n  Informe completo:\n%s", limpia)
	}

}

// TestNingunaLecturaDeJsonEscupeSuVolcadoAlInforme — el segundo defecto que apareció al abrir el
// corte de A126, y la razón por la que esta pregunta es de TEXTO y no de corrida.
//
// Al dejar correr `scrapes` con Prometheus mudo apareció un traceback de Python de DIEZ LÍNEAS
// entre el título de la sección y sus veredictos: de dos invocaciones de `python3` hermanas, la de
// `JOBS_VIVOS` no silenciaba stderr y la de `AM_CFG` sí. La guarda estaba en una de dos. Un informe
// que de golpe muestra un volcado se deja de leer, y ésa es la forma más barata de que un hallazgo
// real pase desapercibido.
//
// POR QUÉ NO SE MIDE CORRIENDO, Y ESTO ES LO QUE CASI SE ME ESCAPA. Con el arreglo puesto, la
// sección `scrapes` ni siquiera invoca a `python3` cuando Prometheus no contestó —pregunta primero
// por `PROM_VIVO`—, así que en un fixture con el puerto cerrado el volcado NO PUEDE aparecer:
// sacarle el `2>/dev/null` dejaría la prueba en VERDE. Sería un sabotaje inerte sosteniendo una
// aserción inalcanzable, que es precisamente lo que el arnés advierte que enseña a confiar en una
// red que no está. Reproducirlo de verdad pediría un Prometheus falso que conteste algo que no es
// JSON; la pregunta de texto cubre las NUEVE invocaciones de una vez y no necesita ninguno.
//
// Y CONVERGE: no enumera cuáles silencian, le pregunta a todas lo mismo. Una lectura nueva entra al
// barrido sola.
//
// Sabotaje que la pone roja: sacarle el `2>/dev/null` a la invocación que arma `JOBS_VIVOS`.
// arnes: archivo="deploy/verificar-despliegue.sh"
// arnes: de="print(\"\\n\".join(sorted(vistos)))\n' 2>/dev/null)\""
// arnes: a="print(\"\\n\".join(sorted(vistos)))\n')\""
func TestNingunaLecturaDeJsonEscupeSuVolcadoAlInforme(t *testing.T) {
	lineas := strings.Split(leerDeploy(t, "verificar-despliegue.sh"), "\n")

	miradas := 0
	for n, l := range lineas {
		if !strings.Contains(l, "python3 -c '") {
			continue
		}
		// El guion embebido se cierra en la primera línea que EMPIEZA con la comilla simple: es la
		// que lleva las redirecciones y el `)"` que cierra la sustitución.
		cierre := -1
		for m := n + 1; m < len(lineas); m++ {
			if strings.HasPrefix(strings.TrimSpace(lineas[m]), "'") {
				cierre = m
				break
			}
		}
		if cierre < 0 {
			t.Errorf("deploy/verificar-despliegue.sh:%d abre un `python3 -c '` y no se encontró la línea "+
				"que lo cierra: este barrido dejó de entender la forma y no puede afirmar nada sobre él", n+1)
			continue
		}
		miradas++
		if !strings.Contains(lineas[cierre], "2>/dev/null") {
			t.Errorf("deploy/verificar-despliegue.sh:%d lee JSON con python y NO silencia su stderr "+
				"(cierra en la línea %d: %s).\n"+
				"  Cuando la respuesta no es el JSON que espera, python escribe un traceback de diez\n"+
				"  líneas EN MEDIO del informe, entre el título de la sección y sus veredictos. Lo que\n"+
				"  hay que decirle al operador es la consecuencia —la variable quedó vacía, no se pudo\n"+
				"  comparar— y eso ya lo dicen el `dudoso` o el `rojo` de abajo. El volcado sólo logra\n"+
				"  que el informe se deje de leer.\n"+
				"  Agregá `2>/dev/null` a la línea de cierre, como ya lo hacen sus hermanas.",
				n+1, cierre+1, strings.TrimSpace(lineas[cierre]))
		}
	}

	// EL CONTROL. Sin esto, el día que cambie la forma de invocar a python este barrido no
	// encontraría ninguna, no habría un solo error y el verde diría «todas silencian» en vez de «no
	// miré ninguna».
	if miradas < 5 {
		t.Fatalf("sólo se encontraron %d invocaciones de `python3 -c '` en verificar-despliegue.sh, y "+
			"hay bastantes más: cambió la forma de escribirlas y esta guarda está en verde sin haber "+
			"mirado casi nada", miradas)
	}
}

// TestNingunaSeccionDelInformePuedeTerminarLaCorrida es la MISMA pregunta que la de arriba, hecha
// sobre el texto del guion en vez de sobre una corrida — y las dos hacen falta.
//
// La de arriba mide una corrida concreta: con las dos APIs cerradas, el informe llega al final. Pero
// un `exit` puesto en una rama que ESA corrida no toma quedaría invisible ahí, y el defecto de A126
// es precisamente un `exit` en una rama que sólo se toma cuando algo no contesta. Ésta lo prohíbe en
// todas las ramas a la vez, que es la forma que converge: no enumera secciones ni condiciones,
// pregunta UNA cosa sobre el tramo entero.
//
// LAS DOS ANCLAS SON CÓDIGO Y NO PROSA. El principio es la primera llamada a `titulo`; el final, la
// primera línea que LEE el acumulador (`[ "$DIVERGE" -ne 0 ]`), que es donde empieza el veredicto.
// Anclar en el comentario `# ── El veredicto ──` no serviría: `leerDeploy` blanquea los comentarios
// —a propósito, para que una guarda no la pueda satisfacer una línea de prosa— y ese ancla no
// existiría acá adentro.
//
// Las salidas ANTERIORES al primer `titulo` quedan afuera y así tiene que ser: son las dos
// precondiciones del encabezado —no hay repo contra qué comparar, no hay `python3`— y ahí cortar es
// lo correcto, porque sin ellas ninguna sección puede contestar nada.
//
// EL SABOTAJE ES EN OTRA SECCIÓN A PROPÓSITO, y no el `exit 2` original: esta pregunta no es sobre
// «reglas de alerta», es sobre el tramo ENTERO. Saboteándola donde nació sólo se mediría que caza el
// caso que ya se conoce; puesta en `scrapes`, mide lo que dice medir.
//
// Sabotaje que la pone roja: meter un `exit 1` al final de la sección de los scrapes.
// arnes: archivo="deploy/verificar-despliegue.sh"
// arnes: de="gris \"los scrapes de sitio (scrape_config_files) no se comparan"
// arnes: a="exit 1\ngris \"los scrapes de sitio (scrape_config_files) no se comparan"
func TestNingunaSeccionDelInformePuedeTerminarLaCorrida(t *testing.T) {
	guion := leerDeploy(t, "verificar-despliegue.sh")

	primera := strings.Index(guion, `titulo "`)
	veredicto := strings.Index(guion, `[ "$DIVERGE" -ne 0 ]`)
	if primera < 0 || veredicto <= primera {
		t.Fatalf("no se pudo acotar el cuerpo del informe (primer `titulo` en %d, lectura de $DIVERGE "+
			"en %d): o se renombró `titulo`, o el bloque del veredicto cambió de forma. Esta guarda "+
			"estaría mirando un tramo que no es el que dice mirar", primera, veredicto)
	}

	// `exit` como PALABRA y en posición de comando: a principio de línea, o después de `;`, `&&`,
	// `||` o `then`. Así no lo satisface —ni lo dispara— la palabra «exit» adentro de un mensaje.
	reSalida := regexp.MustCompile(`(?:^|;|&&|\|\||\bthen\b)[[:space:]]*exit\b`)

	lineas := strings.Split(guion, "\n")
	desde := strings.Count(guion[:primera], "\n")
	hasta := strings.Count(guion[:veredicto], "\n")
	for n := desde; n < hasta && n < len(lineas); n++ {
		if reSalida.MatchString(lineas[n]) {
			t.Errorf("deploy/verificar-despliegue.sh:%d termina la corrida adentro del informe:\n"+
				"      %s\n"+
				"  Una sección que no puede contestar tiene que marcar lo suyo y DEJAR SEGUIR: `rojo` si\n"+
				"  midió y difiere, `dudoso` si no pudo preguntar. Las dos cosas las acumula el veredicto\n"+
				"  del final, que ya traduce `SIN_VERIFICAR` a la misma salida 2 — así que salir acá no\n"+
				"  cambia el código de salida, sólo borra las secciones que venían después y el veredicto\n"+
				"  mismo. Eso es A126, y costó que cuatro defectos vivieran meses sin que nadie los viera.",
				n+1, strings.TrimSpace(lineas[n]))
		}
	}
}

// TestUnServicioMudoNoSeCuentaComoDivergencia custodia la doctrina que el propio guion escribió
// cuatro veces y que el corte de A126 dejaba sin ejercitar en la mitad de abajo:
//
//	«NO PODER PARSEAR NO ES DIVERGIR, Y DECIRLO MAL MANDA A ARREGLAR LO QUE NO ESTÁ ROTO» (:1011)
//	«es `dudoso` y no `rojo` … no poder comparar no es lo mismo que comparar y que dé distinto» (:1018)
//	«Es ROJO y no amarillo: no es "no pude preguntar", es una divergencia real» (:1081)
//
// Con Prometheus mudo, `JOBS_VIVOS` quedaba VACÍO y el bucle de `scrapes` declaraba en rojo que el
// repo tiene jobs «que Prometheus NO tiene» — una afirmación sobre el servidor construida sobre cero
// información, y encima la que prendía `producción diverge del repo` en el veredicto. `PROM_VIVO` ya
// se consultaba en las otras CINCO secciones que dependen de Prometheus; `scrapes` era la sexta y la
// única que no preguntaba. La guarda estaba en cinco de seis caminos, que es el defecto dominante de
// este repo. `alertmanager` tenía la misma forma con su propio servicio.
//
// SE MIRAN DOS SECCIONES Y ESTÁ ACOTADO A PROPÓSITO: son las dos cuya ÚNICA entrada es un servicio
// remoto, así que con el servicio mudo no hay absolutamente nada que puedan haber medido. Las demás
// comparan contra el disco y sus rojos son reales aunque no conteste nadie. Un servicio nuevo traería
// su propio `*_VIVO`, y el control de abajo lo delata.
//
// EL FIXTURE TRAE `prometheus.yml` Y NO ES UN DETALLE. Sin él `JOBS_REPO` queda vacío, el bucle da
// cero vueltas y el sabotaje NO MUERDE: sería un sabotaje inerte declarado como si funcionara, que es
// lo que el arnés advierte que enseña a confiar en una red que no está.
//
// EL SABOTAJE VA SOBRE `AM_VIVO` Y NO SOBRE `PROM_VIVO`, Y AVERIGUARLO COSTÓ UNA CORRIDA.
//
// Lo natural era apagar la pregunta de `scrapes`, que es donde nació el defecto. Medido: el arnés
// aplicó ese sabotaje, compiló, y esta guarda NO SE MOVIÓ. No es que no cace — es que `scrapes`
// quedó con DOS compuertas, primero «¿contestó Prometheus?» y después «¿vino vacía la lista de
// targets?», así que sacarle una sigue terminando en `dudoso`. La defensa en profundidad está bien;
// el sabotaje estaba mal.
//
// Se declara entonces el que SÍ muerde: la mitad de Alertmanager, que tiene una sola compuerta y
// por eso es donde la ausencia de la pregunta se nota en el acto. Queda escrito acá porque un
// sabotaje inerte declarado como si funcionara enseña a confiar en una red que no está, y el
// próximo que quiera sabotear la mitad de Prometheus tiene que saber que hay que apagar las dos.
//
// Sabotaje que la pone roja: que «alertmanager» deje de preguntar si el servicio contestó.
// arnes: archivo="deploy/verificar-despliegue.sh"
// arnes: de="if [ \"$AM_VIVO\" != si ]; then"
// arnes: a="if false; then"
func TestUnServicioMudoNoSeCuentaComoDivergencia(t *testing.T) {
	compuerta := guiones.Unix(t, "corre deploy/verificar-despliegue.sh contra un repo de prueba con Prometheus y "+
		"Alertmanager cerrados, para medir qué veredicto emite sobre un servicio que nunca contestó",
		"bash", "git", "python3")

	raiz := prepararRepoDePrueba(t)
	// Dos jobs, que es lo mínimo para distinguir «no recorrió» de «recorrió y no acusó a nadie».
	prom := filepath.Join(raiz, "deploy", "prometheus")
	if err := os.MkdirAll(prom, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prom, "prometheus.yml"),
		[]byte("scrape_configs:\n  - job_name: musubi\n  - job_name: prometheus\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	secciones := seccionesDelInforme(correrVerificador(t, compuerta, raiz))

	// EL CONTROL, Y ES LA MITAD DE LA GUARDA: si el informe dejara de traer estas secciones —que es
	// justo lo que pasaba antes de A126— el bucle de abajo no encontraría un solo `✘` y esto pasaría
	// en verde sin haber mirado nada. «No hay rojos» y «no hay sección» son el mismo texto vacío.
	for _, s := range []string{"scrapes", "alertmanager"} {
		if _, hay := secciones[s]; !hay {
			t.Fatalf("el informe no trae la sección %q, así que esta guarda no puede medir nada.\n"+
				"  Secciones que sí llegaron: %v", s, clavesDelInforme(secciones))
		}
	}

	for seccion, porque := range map[string]string{
		"scrapes": "Prometheus nunca contestó, así que la lista de targets está VACÍA. Decir que el " +
			"repo declara un job «que Prometheus NO tiene» es afirmar una divergencia sobre cero " +
			"información. `PROM_VIVO` ya lo resuelve en las otras cinco secciones que dependen de él",
		"alertmanager": "Alertmanager nunca contestó, así que no hay configuración viva que leer. " +
			"Declarar rota la plantilla —o el parse_mode— sin haberla visto manda a arreglar lo que " +
			"nadie midió. Lo dice `AM_VIVO`, que es el hermano de `PROM_VIVO`",
	} {
		for _, l := range strings.Split(secciones[seccion], "\n") {
			if strings.Contains(l, "✘") {
				t.Errorf("con el servicio mudo, la sección «%s» emite una DIVERGENCIA:\n"+
					"    %s\n"+
					"  %s.\n"+
					"  El veredicto correcto es `dudoso` («?»), que el bloque final traduce igual a salida\n"+
					"  2 — así que no se pierde severidad, se gana que el operador sepa si tiene que ir a\n"+
					"  arreglar algo o a levantar un servicio.",
					seccion, strings.TrimSpace(l), porque)
			}
		}
	}
}

// clavesDelInforme nombra las secciones que llegaron, para que el fallo del control diga qué SÍ hubo.
func clavesDelInforme(secciones map[string]string) []string {
	var fuera []string
	for k := range secciones {
		fuera = append(fuera, k)
	}
	return fuera
}
