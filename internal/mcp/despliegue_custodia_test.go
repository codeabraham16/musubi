package mcp

// despliegue_custodia_test.go — las guardas de A73.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL CABO: LAS GUARDAS VALIDABAN EL REPO Y PRODUCCIÓN DIVERGÍA SIN QUE NADA LO DIJERA
//
// `TestLaCadenaDeAlertasSeVigilaASiMisma` pasaba en verde mientras el job `alertmanager` NO estaba
// desplegado. Sin ese scrape, `alertmanager_notifications_failed_total` no existe y
// `CadenaDeAlertasFallando` —la alerta que vigila que las alertas se entreguen— no podía
// dispararse nunca. La guarda leía `deploy/prometheus/prometheus.yml`; el servidor corría otro
// archivo. Medido el 2026-09-02: 29 reglas cargadas contra 31 en el repo.
//
// La comparación repo↔producción no la puede hacer una prueba de Go: no tiene el servidor
// delante. La hace `deploy/verificar-despliegue.sh` a pedido, y `ReglasDeFlotaSinDesplegar` /
// `ReglasDelCerebroSinDesplegar` de forma desatendida, cruzando los archivos.
//
// LO QUE ESTAS PRUEBAS CUSTODIAN ES LA MITAD QUE SÍ VIVE EN EL REPO: que los números de esa
// custodia cruzada sean los verdaderos. Un conteo escrito a mano se pudre —alguien agrega una
// alerta y no lo toca— y un conteo podrido hace sonar la alarma para siempre, que es cómo se
// apaga un canal.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// archivoDeReglas es la forma mínima de un archivo de reglas de Prometheus.
type archivoDeReglas struct {
	Groups []struct {
		Name  string `yaml:"name"`
		Rules []struct {
			Alert string `yaml:"alert"`
			Expr  string `yaml:"expr"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

func cargarReglas(t *testing.T, nombre string) (archivoDeReglas, string) {
	t.Helper()
	ruta := filepath.Join("..", "..", "deploy", nombre)
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("falta %s: %v", ruta, err)
	}
	var a archivoDeReglas
	if err := yaml.Unmarshal(crudo, &a); err != nil {
		t.Fatalf("%s no es YAML válido: %v", ruta, err)
	}
	return a, string(crudo)
}

func cuentaDeReglas(a archivoDeReglas) int {
	n := 0
	for _, g := range a.Groups {
		n += len(g.Rules)
	}
	return n
}

// exprDeAlerta devuelve la expresión de una alerta por nombre.
func exprDeAlerta(a archivoDeReglas, alerta string) (string, bool) {
	for _, g := range a.Groups {
		for _, r := range g.Rules {
			if r.Alert == alerta {
				return r.Expr, true
			}
		}
	}
	return "", false
}

// EL NÚMERO DE LA CUSTODIA CRUZADA SALE DEL REPO, NO DE LA MEMORIA DE NADIE.
//
// Cada archivo de reglas declara cuántas reglas tiene EL OTRO. Cruzado a propósito: un archivo que
// declara su propio conteo se despliega junto con el conteo, las dos mitades se mueven a la vez y
// la comprobación nunca falla. Cruzándolos, un despliegue a medias rompe la simetría en la
// dirección que sea y alguno de los dos grita.
//
// Esta prueba cierra el otro extremo: que el número declarado sea el verdadero. Sin ella, agregar
// una alerta deja la custodia cruzada disparando para siempre —la alarma que no se puede apagar
// arreglando algo— o, peor, alguien la "arregla" bajando el número y la custodia deja de custodiar.
//
// Sabotaje: agregar una alerta a cualquiera de los dos archivos sin tocar el número del otro.
func TestCadaArchivoDeReglasCustodiaElConteoDelOtro(t *testing.T) {
	base, _ := cargarReglas(t, "musubi-alerts.yml")
	flota, _ := cargarReglas(t, "musubi-alerts-flota.yml")
	// El SLA son recording rules, no alertas, pero `cuentaDeReglas` cuenta entradas de `rules:` y
	// no le importa el campo — así que el mismo mecanismo sirve para custodiarlo (A93).
	sla, _ := cargarReglas(t, "musubi-recording.yml")

	casos := []struct {
		alerta   string
		en       archivoDeReglas
		custodia archivoDeReglas
		nombre   string
	}{
		{"ReglasDeFlotaSinDesplegar", base, flota, "musubi-alerts-flota.yml"},
		{"ReglasDelCerebroSinDesplegar", flota, base, "musubi-alerts.yml"},
		// A93 · el SLA era la única de las tres familias sin nadie que la contara. Su guarda vive
		// en el archivo de FLOTA, que es el que se instala junto con él.
		{"ReglasDelSlaSinDesplegar", flota, sla, "musubi-recording.yml"},
	}
	reNumero := regexp.MustCompile(`!=\s*(\d+)`)
	for _, c := range casos {
		expr, ok := exprDeAlerta(c.en, c.alerta)
		if !ok {
			t.Errorf("no existe la alerta %s: la custodia cruzada de A73 quedó a medias, y media "+
				"custodia no avisa de un despliegue parcial en la dirección que le falta", c.alerta)
			continue
		}
		m := reNumero.FindStringSubmatch(expr)
		if m == nil {
			t.Errorf("%s ya no compara contra un número: %q", c.alerta, expr)
			continue
		}
		declarado, _ := strconv.Atoi(m[1])
		real := cuentaDeReglas(c.custodia)
		if declarado != real {
			t.Errorf("%s dice que %s tiene %d reglas y tiene %d.\n"+
				"Actualizá el número en la expresión. Si no, esa alerta queda disparando en "+
				"producción para siempre desde el próximo despliegue — y una alarma que no se apaga "+
				"arreglando algo es cómo se apaga un canal entero.",
				c.alerta, c.nombre, declarado, real)
		}
	}
}

// LA REGEX DE LA CUSTODIA TIENE QUE DISTINGUIR LOS DOS ARCHIVOS, Y NO ES OBVIO QUE LO HAGA.
//
// Prometheus etiqueta cada grupo como `<ruta del archivo>;<nombre del grupo>`, y las dos rutas se
// parecen mucho: `musubi-alerts.yml` es PREFIJO de nada, pero `.*musubi-alerts.*` matchea a los
// dos. Si la de un archivo matchea también al otro, los conteos se suman y la custodia compara
// contra un total que no es el de nadie: pasa a estar rota en verde.
//
// Sabotaje: sacarle el `\\.` o el `;` a cualquiera de las dos regex → falla acá.
func TestLaCustodiaNoConfundeUnArchivoDeReglasConElOtro(t *testing.T) {
	base, _ := cargarReglas(t, "musubi-alerts.yml")
	flota, _ := cargarReglas(t, "musubi-alerts-flota.yml")

	// Las etiquetas que Prometheus produce de verdad, medidas contra el servidor el 2026-09-02.
	etiquetas := []string{
		"/etc/prometheus/rules/musubi-alerts.yml;musubi-brain",
		"/etc/prometheus/rules/musubi-alerts.yml;musubi-watchdog",
		"/etc/prometheus/rules/musubi-alerts-flota.yml;musubi-flota",
		"/etc/prometheus/rules/musubi-alerts-flota.yml;musubi-politicas",
		"/etc/prometheus/rules/musubi-alerts-flota.yml;musubi-custodia",
		// El del SLA. Va acá porque es donde se prueba que un matcher no se lleve grupos ajenos, y
		// `musubi-recording.yml` es el nombre que MÁS se parece a los otros dos sin ser ninguno.
		"/etc/prometheus/rules/musubi-recording.yml;musubi-sla",
	}
	reMatcher := regexp.MustCompile(`rule_group=~"([^"]+)"`)

	casos := []struct {
		alerta  string
		en      archivoDeReglas
		esperar string // qué archivo tienen que matchear las etiquetas seleccionadas
	}{
		{"ReglasDeFlotaSinDesplegar", base, "musubi-alerts-flota.yml"},
		{"ReglasDelCerebroSinDesplegar", flota, "musubi-alerts.yml"},
		{"ReglasDelSlaSinDesplegar", flota, "musubi-recording.yml"},
	}
	for _, c := range casos {
		expr, ok := exprDeAlerta(c.en, c.alerta)
		if !ok {
			continue // ya lo denuncia la prueba de arriba
		}
		ms := reMatcher.FindAllStringSubmatch(expr, -1)
		if len(ms) == 0 {
			t.Errorf("%s no filtra por rule_group: compara el total de TODAS las reglas cargadas, "+
				"así que dos errores que se compensan la dejan en verde", c.alerta)
			continue
		}
		for _, m := range ms {
			// PromQL ancla las regex de matcher por completo, y el literal lleva las escapes de
			// Go. Se reproducen las dos cosas para probar lo que Prometheus va a evaluar.
			patron, err := strconv.Unquote(`"` + m[1] + `"`)
			if err != nil {
				t.Errorf("%s: el literal %q no es una cadena PromQL válida (%v) — Prometheus lo "+
					"rechaza al cargar y la regla no existe", c.alerta, m[1], err)
				continue
			}
			re, err := regexp.Compile("^(?:" + patron + ")$")
			if err != nil {
				t.Errorf("%s: %q no compila como regex: %v", c.alerta, patron, err)
				continue
			}
			var matcheadas []string
			for _, e := range etiquetas {
				if re.MatchString(e) {
					matcheadas = append(matcheadas, e)
				}
			}
			if len(matcheadas) == 0 {
				t.Errorf("%s: el matcher %q no selecciona NINGÚN grupo real. `sum()` de vacío no "+
					"devuelve nada, así que la comparación `!=` nunca se evalúa: la custodia queda "+
					"muda y en verde.", c.alerta, patron)
			}
			for _, e := range matcheadas {
				if !strings.Contains(e, c.esperar+";") {
					t.Errorf("%s: el matcher %q también agarra %q, que es del otro archivo. Los "+
						"conteos se suman y la custodia compara contra un total que no es de nadie.",
						c.alerta, patron, e)
				}
			}
		}
	}
}

// CADA ARCHIVO DE REGLAS DICE SI SE DESPLIEGA SIEMPRE O BAJO CONDICIÓN.
//
// Sin esa línea, un archivo parkeado a propósito —`musubi-alerts-altura.yml` sin su scrape,
// `musubi-alerts-backup-offhost.yml` sin destino remoto— se denuncia como divergencia, y un
// informe que denuncia lo que está bien deja de leerse a las dos semanas. La lee
// `deploy/verificar-despliegue.sh`.
//
// Sabotaje: borrar la línea `# despliegue:` de cualquiera de los cuatro archivos.
func TestCadaArchivoDeReglasDeclaraCuandoSeDespliega(t *testing.T) {
	rutas, err := filepath.Glob(filepath.Join("..", "..", "deploy", "musubi-alerts*.yml"))
	if err != nil || len(rutas) < 4 {
		t.Fatalf("se encontraron %d archivos de reglas (err=%v); el glob se rompió y la prueba no probaría nada", len(rutas), err)
	}
	reMarca := regexp.MustCompile(`(?m)^#\s*despliegue:\s*(siempre|condicional\b.*)$`)
	for _, r := range rutas {
		crudo, err := os.ReadFile(r)
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", r, err)
		}
		m := reMarca.FindSubmatch(crudo)
		if m == nil {
			t.Errorf("%s no declara `# despliegue: siempre` ni `# despliegue: condicional — <razón>`.\n"+
				"Sin eso, verificar-despliegue.sh no puede distinguir un archivo que falta de uno "+
				"parkeado a propósito, y denuncia como drift lo que es una decisión.", filepath.Base(r))
			continue
		}
		if v := strings.TrimSpace(string(m[1])); strings.HasPrefix(v, "condicional") && !strings.Contains(v, "—") {
			t.Errorf("%s se declara condicional y no dice de qué: «%s». La condición es lo único "+
				"que hace accionable el informe.", filepath.Base(r), v)
		}
	}
}

// LOS JOBS QUE SE VIGILAN SON LOS QUE EL REPO DECLARA — TODOS, NO LOS QUE ALGUIEN RECORDÓ.
//
// `ScrapeQueElRepoDeclaraYNoExiste` existe porque el job `alertmanager` estuvo sin desplegar y
// nada lo dijo. Si mañana entra un job nuevo a prometheus.yml y no entra a esta alerta, el agujero
// vuelve a abrirse EXACTAMENTE igual: nadie va a notar que ese scrape no existe hasta que haga
// falta una métrica suya.
//
// Sabotaje: agregar un `job_name:` a prometheus.yml sin sumarlo a la alerta → falla acá.
func TestLosJobsVigiladosSonLosQueElRepoDeclara(t *testing.T) {
	cfg := leerDeploy(t, "prometheus", "prometheus.yml")

	// LAS COMILLAS SIMPLES CUENTAN, Y NO CONTARLAS ERA UN AGUJERO CON DIENTES.
	//
	// La primera versión aceptaba `"?`, o sea comillas dobles o nada. `- job_name: 'altura-db'` es
	// YAML válido y es el estilo que usa media documentación de Prometheus — y quedaba INVISIBLE
	// para esta guarda. Medido el 2026-09-05, en las dos direcciones:
	//
	//   · declarar un job con comillas simples y NO sumarlo a la alerta -> la guarda pasa en VERDE.
	//     Es exactamente el agujero de A73, que es para lo que esta prueba existe.
	//   · declararlo con comillas simples Y sumarlo a la alerta (lo correcto) -> la guarda FALLA,
	//     diciendo «la alerta vigila el job "altura-db", que prometheus.yml no declara: nunca va a
	//     existir y la alerta queda encendida para siempre».
	//
	// O sea que premiaba el defecto y castigaba el arreglo, con un mensaje que manda a sacar la
	// línea correcta. Peor que no mirar.
	// SE LEE EL VALOR ENTERO Y DESPUÉS SE LE SACAN LAS COMILLAS, en vez de describir con una clase
	// de caracteres qué puede tener un nombre de job. Una clase se queda corta EN SILENCIO: con
	// `[A-Za-z0-9_.-]+`, el job `"un.job.raro@2"` se leía como `un.job.raro` —cortado en el `@`— y
	// el control de conteo tampoco lo veía, porque un nombre truncado sigue contando como uno.
	// Leer hasta el fin de la línea no puede quedarse corto.
	var declarados []string
	for _, m := range regexp.MustCompile(`(?m)^\s*-\s*job_name:\s*(.+?)\s*(?:#.*)?$`).FindAllStringSubmatch(cfg, -1) {
		declarados = append(declarados, strings.Trim(m[1], `"'`))
	}
	if len(declarados) == 0 {
		t.Fatal("no se encontró ningún job_name en prometheus.yml: el patrón se rompió y la prueba no probaría nada")
	}
	// Y EL CONTROL DE «LOS VI A TODOS», que es lo que faltaba: contar las líneas `job_name:` sin
	// interpretar el valor. Si un estilo de comillas nuevo se le escapa al regex de arriba, esto lo
	// dice en vez de dejar la diferencia en silencio — el modo de falla no era una aserción
	// equivocada, era un recorrido que no llegaba.
	if lineas := len(regexp.MustCompile(`(?m)^\s*-\s*job_name:`).FindAllString(cfg, -1)); lineas != len(declarados) {
		t.Fatalf("prometheus.yml tiene %d líneas `job_name:` y el patrón sólo pudo leer %d nombres (%s):\n"+
			"hay una forma de escribirlo que esta prueba no reconoce, así que esos jobs quedan sin vigilar\n"+
			"y la guarda lo diría en verde.", lineas, len(declarados), strings.Join(declarados, ", "))
	}

	base, _ := cargarReglas(t, "musubi-alerts.yml")
	expr, ok := exprDeAlerta(base, "ScrapeQueElRepoDeclaraYNoExiste")
	if !ok {
		t.Fatal("no existe ScrapeQueElRepoDeclaraYNoExiste: un job sin desplegar vuelve a verse igual que uno sano")
	}
	vigilados := map[string]bool{}
	for _, m := range regexp.MustCompile(`job="([^"]+)"`).FindAllStringSubmatch(expr, -1) {
		vigilados[m[1]] = true
	}

	var faltan []string
	for _, j := range declarados {
		if !vigilados[j] {
			faltan = append(faltan, j)
		}
	}
	sort.Strings(faltan)
	if len(faltan) > 0 {
		t.Errorf("prometheus.yml declara scrapes que ScrapeQueElRepoDeclaraYNoExiste no vigila: %s.\n"+
			"Es el agujero de A73 otra vez: si ese job no llega a desplegarse, todo lo que dependa "+
			"de sus métricas queda ciego y se ve en verde.", strings.Join(faltan, ", "))
	}

	// Y AL REVÉS: vigilar un job que el repo no declara hace sonar la alerta para siempre.
	decl := map[string]bool{}
	for _, j := range declarados {
		decl[j] = true
	}
	for j := range vigilados {
		if !decl[j] {
			t.Errorf("la alerta vigila el job %q, que prometheus.yml no declara: nunca va a existir "+
				"y la alerta queda encendida para siempre", j)
		}
	}
}

// TestElPinDelGuionDeBackupEsElVerdadero custodia un acoplamiento que ya mordió, y a quien lo creó.
//
// EL CABO: `install-musubi-brain.sh` verifica el sha256 de `deploy/musubi-backup.sh` contra un pin
// escrito en el propio instalador. Es la decisión correcta —un `.sha256` publicado junto al script
// lo controla el mismo que controla el script, y `main` no tiene branch protection, así que no
// verificaría nada— pero deja un número escrito a mano que se pudre en cuanto alguien toca el
// guion. Y un pin podrido no degrada: el instalador MUERE, y muere en la máquina de quien está
// levantando un cerebro nuevo, que es el peor momento para descubrirlo.
//
// PASÓ EL MISMO DÍA QUE SE CREÓ EL ACOPLAMIENTO. El commit que lo introdujo avisó por escrito a
// las otras terminales de que tocar el guion exigía actualizar el pin — y esa misma tarde otro
// commit del MISMO autor le agregó la marca `.last_snapshot` al guion sin tocar el pin. Lo cazó
// una persona leyendo, no una prueba. Por eso esta existe: el aviso escrito no es una guarda.
func TestElPinDelGuionDeBackupEsElVerdadero(t *testing.T) {
	// crudo: el sha256 tiene que ser el del ARCHIVO ENTERO, comentarios incluidos, porque es
	// lo que el instalador verifica contra el archivo en disco. Filtrarlo daría un pin que no
	// coincide con nada y una guarda que pide actualizar un número que ya está bien.
	guion, err := os.ReadFile(filepath.Join("..", "..", "deploy", "musubi-backup.sh")) // crudo: sha256 del texto entero
	if err != nil {
		t.Fatalf("no se pudo leer deploy/musubi-backup.sh: %v", err)
	}
	real := fmt.Sprintf("%x", sha256.Sum256(guion))

	inst, err := leerArchivoDeDespliegue(filepath.Join("..", "..", "deploy", "install-musubi-brain.sh"))
	if err != nil {
		t.Fatalf("no se pudo leer deploy/install-musubi-brain.sh: %v", err)
	}
	m := regexp.MustCompile(`(?m)^BACKUP_SHA256="([a-f0-9]{64})"`).FindSubmatch(inst)
	if m == nil {
		t.Fatal("install-musubi-brain.sh no declara BACKUP_SHA256=\"<sha256>\": " +
			"o se quitó la verificación del guion de backup —y entonces se instala sin verificar " +
			"un script que un timer corre como el usuario del cerebro— o cambió de forma y esta " +
			"guarda dejó de mirar donde debe")
	}
	if pin := string(m[1]); pin != real {
		t.Errorf("el pin del guion de backup quedó viejo:\n"+
			"  install-musubi-brain.sh dice: %s\n"+
			"  deploy/musubi-backup.sh es:   %s\n"+
			"Alguien editó el guion y no actualizó el pin. NO es cosmético: el instalador hace `die` "+
			"y no instala el backup, en la máquina de quien está levantando un cerebro nuevo.\n"+
			"Arreglo:  sha256sum deploy/musubi-backup.sh", pin, real)
	}
}

// TestLaVersionDeGoNoDiverge custodia el número de versión de Go, que vive en nueve lugares.
//
// EL CABO, MEDIDO EL 2026-09-03: `go.mod` declaraba `go 1.26.4` mientras los OCHO pines de los
// workflows decían `1.26.6`. Alguien ya había averiguado que hacía falta 1.26.6 y lo fijó en cada
// job — y nunca tocó `go.mod`, que es la fuente de verdad para cualquiera que compile el proyecto.
//
// La consecuencia no era estética: `govulncheck` sobre 1.26.4 encontró TRES vulnerabilidades de la
// biblioteca estándar alcanzables desde código nuestro (crypto/tls post-handshake, la complejidad
// cuadrática de net/url en `fleet.TomarMuestraDeExposicion`, y el ReadHeaderTimeout de net/http en
// `mcp.ListenAndServeHTTP`), las tres arregladas en 1.26.6. Los binarios de `release.yml` salían
// parcheados; el que se compila desde `go.mod` —el que corría en producción— no.
//
// Y lo que lo destapó fue una decisión de una línea: el job `vulns` usa `go-version-file: go.mod`
// en vez de fijar la versión como sus hermanos. Fijarla habría dado verde y tapado la deriva. Un
// verificador que no lee la fuente de verdad verifica otra cosa.
func TestLaVersionDeGoNoDiverge(t *testing.T) {
	mod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("no se pudo leer go.mod: %v", err)
	}
	m := regexp.MustCompile(`(?m)^go (\d+\.\d+(?:\.\d+)?)$`).FindSubmatch(mod)
	if m == nil {
		t.Fatal("go.mod no declara una directiva `go <version>` reconocible")
	}
	quiere := string(m[1])

	flujos, err := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if err != nil || len(flujos) == 0 {
		t.Fatalf("no se encontraron workflows: %v", err)
	}
	pin := regexp.MustCompile(`(?m)^\s*go-version:\s*'?"?(\d+\.\d+(?:\.\d+)?)'?"?\s*$`)

	vistos := 0
	for _, f := range flujos {
		crudo, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", f, err)
		}
		for _, hit := range pin.FindAllSubmatch(crudo, -1) {
			vistos++
			if got := string(hit[1]); got != quiere {
				t.Errorf("%s fija go-version: %s y go.mod declara %s.\n"+
					"No es cosmético: el job que usa `go-version-file: go.mod` compila con OTRA "+
					"versión que sus hermanos, y si la vieja tiene un CVE de la biblioteca estándar "+
					"nadie se entera —los binarios de release salen parcheados y el que se compila "+
					"desde go.mod, no.\nArreglo: que las dos digan lo mismo.",
					filepath.Base(f), got, quiere)
			}
		}
	}
	if vistos == 0 {
		t.Fatal("ningún workflow fija `go-version:`: o cambió la forma de declararlo y esta guarda " +
			"dejó de mirar donde debe, o se pasaron todos a `go-version-file` (y entonces esta " +
			"prueba sobra y hay que borrarla a conciencia, no dejarla pasando en verde sobre nada)")
	}
	t.Logf("%d pines de go-version comprobados contra go.mod (%s)", vistos, quiere)
}

// TestElPinDelGuionDeRedespliegueEsElVerdadero — el hermano del pin del backup (A111).
//
// EL CABO, MEDIDO EL 2026-09-05: de los dos guiones derivados que viven en el servidor,
// `/usr/local/bin/musubi-backup` coincidía BYTE A BYTE con `deploy/musubi-backup.sh` y
// `/home/musubi/redesplegar-cerebro.sh` tenía 9690 bytes contra 19469 del repo — con su
// verificación de la migración muerta (`[[ "$ESQUEMA" -ge 37 ]]` con la base ya en 46: vacuamente
// cierta), que pasó así en los seis redespliegues del 4 y el 5.
//
// La diferencia entre el que se mantuvo al día y el que no NO fue el cuidado de nadie: fue que uno
// lo instalaba `install-musubi-brain.sh` detrás de una compuerta de sha256 y el otro llegaba a
// mano. Es la lección de siempre —el hermano sin la guarda—, y por eso el arreglo fue darle al
// redespliegue el mismo instalador, que trae consigo el mismo pin escrito a mano, que se pudre
// igual. De ahí esta prueba.
//
// Y ACÁ EL PIN PODRIDO CUESTA MÁS QUE EN EL BACKUP: este guion reemplaza el binario del cerebro y
// se corre como root, así que un `die` del instalador deja al servidor sin ninguna forma
// verificada de actualizarse — y la salida a mano es justamente la que produjo la deriva.
func TestElPinDelGuionDeRedespliegueEsElVerdadero(t *testing.T) {
	// crudo: el sha256 tiene que ser el del ARCHIVO ENTERO, comentarios incluidos, porque es
	// lo que el instalador verifica contra el archivo en disco. Filtrarlo daría un pin que no
	// coincide con nada y una guarda que pide actualizar un número que ya está bien.
	guion, err := os.ReadFile(filepath.Join("..", "..", "deploy", "redesplegar-cerebro.sh")) // crudo: sha256 del texto entero
	if err != nil {
		t.Fatalf("no se pudo leer deploy/redesplegar-cerebro.sh: %v", err)
	}
	real := fmt.Sprintf("%x", sha256.Sum256(guion))

	inst, err := leerArchivoDeDespliegue(filepath.Join("..", "..", "deploy", "install-musubi-brain.sh"))
	if err != nil {
		t.Fatalf("no se pudo leer deploy/install-musubi-brain.sh: %v", err)
	}
	m := regexp.MustCompile(`(?m)^REDESPLIEGUE_SHA256="([a-f0-9]{64})"`).FindSubmatch(inst)
	if m == nil {
		t.Fatal("install-musubi-brain.sh no declara REDESPLIEGUE_SHA256=\"<sha256>\": o se quitó la " +
			"verificación del guion de redespliegue —y entonces se instala sin verificar el archivo " +
			"que reemplaza el binario del cerebro corriendo como root— o cambió de forma y esta " +
			"guarda dejó de mirar donde debe")
	}
	if pin := string(m[1]); pin != real {
		t.Errorf("el pin del guion de redespliegue quedó viejo:\n"+
			"  install-musubi-brain.sh dice:        %s\n"+
			"  deploy/redesplegar-cerebro.sh es:    %s\n"+
			"Alguien editó el guion y no actualizó el pin. El instalador hace `die` y el servidor "+
			"queda sin guion de redespliegue verificado — y la salida a mano es exactamente lo que "+
			"produjo A111.\nArreglo:  sha256sum deploy/redesplegar-cerebro.sh", pin, real)
	}
}

// TestCadaGuionQueSeInstalaEnElServidorSeCompara — que la tabla de guiones derivados no se quede
// corta cuando alguien agregue el tercero (A111).
//
// EL CABO ES DE UN PISO MÁS ARRIBA QUE EL DE A111. Arreglar la deriva del redespliegue agregando
// una fila a mano en `verificar-despliegue.sh` deja el mismo agujero para el PRÓXIMO guion: una
// lista escrita a mano no tiene cómo enterarse de que apareció un archivo nuevo. Es el defecto de
// A93 —`verificar-cobertura.sh` con su lista de archivos a mano y el argumento contra las listas a
// mano escrito al lado— y no se cierra escribiendo mejor la lista, se cierra derivándola.
//
// LA FUENTE MECÁNICA ES EL INSTALADOR, Y LA LISTA DE INSTALADORES TAMBIÉN SE DERIVA. Todo lo que
// llega al servidor con `install` está declarado en un guion, con su destino. La versión anterior
// de esta prueba leía UN instalador —`install-musubi-brain.sh`, nombrado a mano— y ahí quedaba su
// propio agujero: `redesplegar-cerebro.sh` TAMBIÉN tiene un `install -m` (línea 132), corre en el
// servidor con `sudo`, y un segundo `install` ahí habría llegado sin que nada lo cruzara contra su
// fuente, con esta guarda en verde. Es la lección de N-1 de N caminos, adentro de la guarda escrita
// para cerrarla.
//
// EL CONJUNTO CORRECTO SE DERIVA DE LA TABLA. Los guiones que corren EN el servidor del cerebro son
// exactamente los que `GUIONES_DERIVADOS` declara que se instalan allá. Así que los instaladores a
// mirar son: el instalador raíz, más cada `.sh` del lado izquierdo de la tabla. Si mañana alguien
// agrega un tercer guion a la tabla, entra a este barrido SOLO. No hay lista que actualizar.
//
// Los otros `install -m` del repo (`deploy/prometheus/install-musubi-prometheus.sh`,
// `deploy/docker/preparar.sh`, `deploy/rustdesk/preparar.sh`) quedan afuera por una razón y no por
// olvido: ninguno se instala en el servidor del cerebro —se corren a mano desde el repo, sobre otros
// roles— y lo que SÍ depositan desde el repo son archivos de reglas, que `verificar-despliegue.sh`
// compara por su propio camino (el barrido de `$DIR_REGLAS_ALLA`, no esta tabla).
//
// Y NO SE CUENTA LO QUE MATCHEÓ, SE EXIGE QUE NO QUEDE NINGUNO SIN PARSEAR. Una guarda que enumera
// formas no converge: el día que un `install` se escriba distinto, un regex que sólo cuenta sus
// aciertos mira hacia otro lado y se queda en verde. Acá cada línea de CÓDIGO que diga `install -m`
// tiene que resolverse a un destino; una que no se pueda parsear es un ROJO que pide enseñarle la
// forma nueva, no un silencio.
func TestCadaGuionQueSeInstalaEnElServidorSeCompara(t *testing.T) {
	verif, err := leerArchivoDeDespliegue(filepath.Join("..", "..", "deploy", "verificar-despliegue.sh"))
	if err != nil {
		t.Fatalf("no se pudo leer deploy/verificar-despliegue.sh: %v", err)
	}

	// LA TABLA SE EXTRAE ANTES Y LA MEMBRESÍA SE PREGUNTA ADENTRO DE ELLA, no en todo el archivo.
	// Preguntar `strings.Contains(verificador, ruta)` habría dejado que un COMENTARIO que nombra la
	// ruta satisficiera la guarda sin comparar nada — que es el defecto dominante del 2026-09-05
	// (siete guardas en verde sobre su propio sabotaje, todas por un texto que vivía donde no
	// decide) y sería especialmente ridículo acá, en la guarda que existe para cerrarlo.
	tabla := regexp.MustCompile(`(?s)GUIONES_DERIVADOS="(.*?)"`).FindStringSubmatch(string(verif))
	if tabla == nil {
		t.Fatal("verificar-despliegue.sh ya no declara GUIONES_DERIVADOS=\"...\": la tabla cambió de " +
			"forma y esta guarda —y el arreglo de A111— dejaron de tener dónde apoyarse")
	}

	// La tabla, parseada una vez: se usa para tres cosas distintas (derivar los instaladores,
	// preguntar membresía, y comprobar que cada archivo declarado exista).
	type filaDerivada struct{ rel, destino string }
	var filas []filaDerivada
	for _, fila := range strings.Split(tabla[1], "\n") {
		fila = strings.TrimSpace(fila)
		if fila == "" {
			continue
		}
		partes := strings.SplitN(fila, "|", 2)
		if len(partes) != 2 {
			t.Errorf("fila mal formada en GUIONES_DERIVADOS: %q (se espera `<archivo del repo>|<ruta en el servidor>`)", fila)
			continue
		}
		filas = append(filas, filaDerivada{rel: partes[0], destino: partes[1]})
	}
	if len(filas) == 0 {
		t.Fatal("GUIONES_DERIVADOS quedó sin ninguna fila parseable: esta guarda no tiene de dónde " +
			"derivar los instaladores ni contra qué preguntar membresía")
	}

	// EL LADO IZQUIERDO TIENE QUE COLGAR DE `deploy/`, y no es cosmética: ese lado es lo que el
	// verificador ABRE (`sha256sum "$REPO/$rel"`, verificar-despliegue.sh:1231), o sea que define la
	// SUPERFICIE DE LECTURA del guion. De esa superficie depende algo que no se ve desde acá: el
	// latido decide si su veredicto es creíble mirando si el árbol diverge, y «diverge» sólo importa
	// si toca lo que el guion lee. Hoy esa superficie es `{VERSION, deploy/**}`, y una fila
	// `scripts/foo.sh|/usr/local/bin/foo` la ensancharía en silencio, con todo en verde: nadie se
	// enteraría de que ahora un cambio en `scripts/` invalida el veredicto de producción.
	//
	// Esto es una REGLA que elegimos, no un hecho del mundo, y por eso va clavada acá con su razón
	// en vez de derivada de algún lado: derivarla del propio guion la volvería un espejo.
	for _, f := range filas {
		if !strings.HasPrefix(f.rel, "deploy/") {
			t.Errorf("GUIONES_DERIVADOS declara %q, que NO cuelga de `deploy/`.\n"+
				"El lado izquierdo es lo que `verificar-despliegue.sh` abre, así que agregar una fila "+
				"fuera de `deploy/` ENSANCHA la superficie de lectura del verificador — y de esa "+
				"superficie depende si el latido considera creíble su propio veredicto.\n"+
				"Arreglo: movelo bajo `deploy/`, o si de verdad tiene que vivir afuera, ensanchá la "+
				"superficie A PROPÓSITO y actualizá lo que dependa de ella.", f.rel)
		}
	}

	// LOS INSTALADORES, DERIVADOS: el raíz más cada `.sh` que la tabla instala en el servidor.
	const instaladorRaiz = "install-musubi-brain.sh"
	instaladores := []string{filepath.Join("..", "..", "deploy", instaladorRaiz)}
	for _, f := range filas {
		if strings.HasSuffix(f.rel, ".sh") {
			instaladores = append(instaladores, filepath.Join("..", "..", filepath.FromSlash(f.rel)))
		}
	}

	// LA ÚNICA EXCEPCIÓN, POR RUTA Y NO POR NOMBRE DE VARIABLE. El binario del cerebro no se compara
	// por sha contra el repo porque el repo no lo trae: se compila. Su deriva la mira la sección
	// «versión del cerebro» de `verificar-despliegue.sh`, que le pregunta `musubi version` y la cruza
	// contra VERSION. Es otra pregunta, no una menos.
	//
	// Antes esta excepción era la CADENA "BIN", el nombre de la variable en el instalador raíz. Eso
	// se rompe solo: `redesplegar-cerebro.sh` instala EXACTAMENTE el mismo archivo y lo llama
	// `$DESTINO`. Un nombre de variable es del archivo que lo escribe; la ruta es del mundo.
	const binarioDelCerebro = "/usr/local/bin/musubi"

	lineaDeInstall := regexp.MustCompile(`\binstall\s+-m\s`)
	// El destino es el ÚLTIMO argumento: `"$VAR"`, `"/ruta literal"` o `/ruta literal` sin comillas.
	destinoDeInstall := regexp.MustCompile(`\binstall\s+-m\s+[0-7]{3,4}\s+(?:-[a-zA-Z]\s+\S+\s+)*(?:"[^"]+"|\S+)\s+(?:"(\$?[A-Za-z_][A-Za-z0-9_]*|/[^"]*)"|(/\S+))\s*(?:\|\||&&|;|#|$)`)

	comprobados := 0
	for _, ruta := range instaladores {
		crudo, err := leerArchivoDeDespliegue(ruta)
		if err != nil {
			t.Errorf("no se pudo leer %s, que esta guarda deriva como instalador: %v", ruta, err)
			continue
		}
		// Las asignaciones literales DE ESE ARCHIVO, para resolver "$BACKUP_BIN" → la ruta.
		rutaDe := map[string]string{}
		for _, m := range regexp.MustCompile(`(?m)^([A-Z_]+)="(/[^"$]*)"`).FindAllSubmatch(crudo, -1) {
			rutaDe[string(m[1])] = string(m[2])
		}

		vistos := 0
		for n, linea := range strings.Split(string(crudo), "\n") {
			codigo := strings.TrimSpace(linea)
			// Las líneas de comentario que citan el comando —y hay varias, porque el instalador
			// explica por qué usa `install` y no `mv`— no instalan nada, y contarlas sería la falla
			// de siempre: una guarda satisfecha por un texto que no decide.
			if strings.HasPrefix(codigo, "#") || !lineaDeInstall.MatchString(codigo) {
				continue
			}
			vistos++
			// `install -m 0700 -d "$DIR"` crea un DIRECTORIO, no deposita un archivo del repo: no
			// hay nada que comparar contra una fuente. Se descuenta acá, nombrado, y no por omisión.
			if regexp.MustCompile(`\binstall\s+-m\s+[0-7]{3,4}\s+(?:-[a-zA-Z]\s+\S+\s+)*-d\s`).MatchString(codigo) {
				continue
			}
			m := destinoDeInstall.FindStringSubmatch(codigo)
			if m == nil {
				t.Errorf("%s:%d dice `install -m` y esta guarda NO PUDO PARSEAR su destino:\n  %s\n"+
					"No se descuenta en silencio: una forma que la guarda no entiende es exactamente "+
					"por donde se le escapa el próximo archivo que llegue al servidor sin que nadie lo "+
					"cruce contra su fuente. Enseñale la forma nueva.",
					filepath.Base(ruta), n+1, codigo)
				continue
			}
			destino := m[1]
			if destino == "" {
				destino = m[2]
			}
			if strings.HasPrefix(destino, "$") {
				v := strings.TrimPrefix(destino, "$")
				resuelto, ok := rutaDe[v]
				if !ok {
					t.Errorf("%s:%d instala en $%s y no encuentro su asignación literal en ese archivo: "+
						"no puedo saber qué ruta es, así que tampoco puedo comprobar que alguien la compare",
						filepath.Base(ruta), n+1, v)
					continue
				}
				destino = resuelto
			}

			if destino == binarioDelCerebro {
				continue
			}

			// LA CLASE, NO EL CASO: ningún destino de instalación puede colgar de /home.
			// Se pregunta ANTES que la pertenencia a la tabla a propósito: una ruta bajo /home está mal
			// aunque alguien la compare, y agregarla a la tabla no la lava — sería un rojo contestado con
			// el arreglo equivocado.
			//
			// El arreglo de A111 movió `redesplegar-cerebro.sh` del home de `musubi` a /usr/local/sbin
			// porque se corre con `sudo` y vivía en un directorio que escribe el uid 1000 — el mismo con
			// el que corre `musubi_fleet_exec`, o sea que quien alcance el canal del agente podía dejar
			// código escrito ahí y esperar al próximo redespliegue.
			//
			// MEDIDO EN EL SERVIDOR EL 2026-09-05, y esto es lo que convierte el caso en una clase: en
			// `/home/musubi` hay DIECISÉIS archivos ejecutables `.sh`/`.py` con la misma forma
			// —`arreglar-principals.py`, `backup-secrets.sh`, `b1-instalar-timer.sh`, `b1-adjudicar.sh`,
			// entre otros—. Ninguno lo corre root hoy por systemd ni por cron (verificado: las dos
			// unidades que apuntan a /home corren con `User=musubi`, y no hay nada en /etc/cron.d, en
			// /etc/crontab ni en el cron de root), así que la exposición de hoy es exactamente una: un
			// humano haciendo `sudo` sobre un archivo que el uid del agente puede reescribir. Pero nada
			// impide la siguiente, y custodiar «el redespliegue está en /usr/local/sbin» habría cerrado
			// el caso dejando la clase abierta — que es el defecto que este repo repite.
			if strings.HasPrefix(destino, "/home/") {
				t.Errorf("%s:%d instala en %s, que cuelga de /home.\n"+
					"Un archivo bajo /home lo escribe el dueño de ese home; si además se corre con `sudo` "+
					"—o lo lee algo privilegiado— es un camino de escalada, y en este servidor no es "+
					"teórico: `musubi_fleet_exec` corre como ese mismo uid.\n"+
					"Arreglo: instalalo bajo /usr/local/bin o /usr/local/sbin, que son de root",
					filepath.Base(ruta), n+1, destino)
				continue
			}

			// Se busca `|<ruta>` y dentro de `tabla[1]`: así es como la tabla la escribe —después del
			// archivo del repo— y ahí es donde la ruta DECIDE que se compare algo.
			if !strings.Contains(tabla[1], "|"+destino) {
				t.Errorf("`%s` instala %s (línea %d) y `verificar-despliegue.sh` NO lo compara contra el "+
					"repo.\nEs A111 otra vez con otro archivo: un guion que llega al servidor y que nada "+
					"cruza contra su fuente se queda viejo en silencio, y no hay daemon que lo relea —el "+
					"archivo ES lo que corre—.\nArreglo: agregá la fila `<archivo del repo>|%s` a "+
					"GUIONES_DERIVADOS en deploy/verificar-despliegue.sh",
					filepath.Base(ruta), destino, n+1, destino)
				continue
			}
			comprobados++
		}

		// EL CONTROL, POR INSTALADOR RAÍZ. Los guiones de la tabla pueden legítimamente no instalar
		// nada; el instalador raíz no, y si dejara de hacerlo con esta forma, esta guarda estaría
		// mirando donde ya no se decide.
		if strings.HasSuffix(ruta, instaladorRaiz) && vistos == 0 {
			t.Fatalf("no encontré ninguna línea de código con `install -m` en %s: o el instalador dejó "+
				"de instalar por ahí y esta guarda mira donde ya no se decide, o cambió de forma. En "+
				"cualquiera de los dos casos la lista de guiones derivados quedó sin fuente", instaladorRaiz)
		}
	}

	if comprobados == 0 {
		t.Fatal("no quedó ningún destino que comprobar después de descontar el binario del cerebro: " +
			"esta prueba estaría en verde sin haber mirado nada")
	}

	// EL CONTROL DE QUE LOS INSTALADORES SON MÁS DE UNO. La versión vieja de esta guarda leía un
	// archivo solo, y ese era el defecto. Si la derivación se rompiera —la tabla cambia de forma, los
	// `.sh` desaparecen del lado izquierdo— volveríamos a mirar uno sin que nada avise.
	if len(instaladores) < 2 {
		t.Fatalf("derivé %d instalador(es) de la tabla y tienen que ser al menos 2 (el raíz más los "+
			"guiones que la tabla instala en el servidor): la derivación se rompió y esta guarda volvió "+
			"a mirar un archivo solo, que es exactamente el agujero que vino a cerrar", len(instaladores))
	}

	// La tabla al revés: que cada archivo que declara exista. Una fila que apunta a un archivo
	// borrado compara contra la nada, y el verificador lo diría en rojo recién en el servidor.
	presentes := 0
	for _, f := range filas {
		if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(f.rel))); err != nil {
			t.Errorf("GUIONES_DERIVADOS declara %q y ese archivo no existe en el repo: la comparación "+
				"no tiene contra qué correr", f.rel)
			continue
		}
		presentes++
	}
	t.Logf("%d instaladores derivados, %d destinos comprobados contra la tabla, %d filas con archivo presente",
		len(instaladores), comprobados, presentes)
}

// TestElLatidoQueEmpujaElVerificadorEsElQueMiranLasAlertas — el acoplamiento nuevo de A115.
//
// EL CABO QUE ESTO CIERRA ES EL DE SIEMPRE, UN PISO MÁS ARRIBA. `comparar-y-latir.sh` empuja dos
// gauges y tres alertas de `musubi-alerts.yml` los miran. Los nombres viven en DOS archivos que
// nadie compara, en dos lenguajes distintos —un sobre JSON y una expresión PromQL—, así que
// renombrar la métrica en el guion no rompe nada visible: el empuje sigue andando, las alertas
// siguen cargadas, y quedan mirando una serie que ya no llega. **Y el modo de falla es el peor
// que hay acá: las tres alertas se quedan CALLADAS**, que es exactamente lo que significan cuando
// todo está bien. `ComparacionRepoServidorSinCorrer` tiene un brazo `absent(...)` y taparía el
// caso —dispararía—, pero las otras dos no, y una alarma que se apaga por renombre es la forma
// que A73 vino a cerrar.
//
// SE MIRA DONDE DECIDE, NO DONDE SE MENCIONA. En el guion los nombres salen del campo `"name"`
// del sobre OTLP, que es lo único que viaja; el encabezado los NOMBRA en prosa —explica por qué
// el timestamp va explícito— y esa mención no empuja nada. En el archivo de alertas salen de la
// expresión y no de las anotaciones. Es la lección dominante del 2026-09-05, y acá tenía dos
// puertas de entrada.
func TestElLatidoQueEmpujaElVerificadorEsElQueMiranLasAlertas(t *testing.T) {
	guion, err := leerArchivoDeDespliegue(filepath.Join("..", "..", "deploy", "comparar-y-latir.sh"))
	if err != nil {
		t.Fatalf("no se pudo leer deploy/comparar-y-latir.sh: %v", err)
	}
	alertas, err := leerArchivoDeDespliegue(filepath.Join("..", "..", "deploy", "musubi-alerts.yml"))
	if err != nil {
		t.Fatalf("no se pudo leer deploy/musubi-alerts.yml: %v", err)
	}

	// ── Lo que el guion EMPUJA: el campo "name" del sobre, en líneas que no son comentario ──
	empujadas := map[string]bool{}
	reNombre := regexp.MustCompile(`"name"\s*:\s*"(musubi_[a-z0-9_]+)"`)
	for _, linea := range strings.Split(string(guion), "\n") {
		if strings.HasPrefix(strings.TrimSpace(linea), "#") {
			continue
		}
		for _, m := range reNombre.FindAllStringSubmatch(linea, -1) {
			empujadas[m[1]] = true
		}
	}
	if len(empujadas) == 0 {
		t.Fatal("no encontré ninguna métrica en el sobre OTLP de deploy/comparar-y-latir.sh " +
			"(un campo `\"name\": \"musubi_...\"` fuera de comentarios). O el guion dejó de empujar " +
			"—y entonces las tres alertas de A115 miran una serie que nadie escribe— o cambió de " +
			"forma y esta guarda mira donde ya no se decide")
	}

	// ── Lo que las alertas MIRAN: sólo el bloque `expr:`, nunca las anotaciones ─────────────
	miradas := map[string]bool{}
	reMetrica := regexp.MustCompile(`\bmusubi_verificacion_[a-z0-9_]+`)
	reCorte := regexp.MustCompile(`^(for|labels|annotations|keep_firing_for)\s*:|^- alert:`)
	dentro := false
	for _, linea := range strings.Split(string(alertas), "\n") {
		codigo := strings.TrimSpace(linea)
		if i := strings.Index(codigo, "#"); i == 0 {
			continue
		}
		if strings.HasPrefix(codigo, "expr:") {
			dentro = true
		} else if dentro && reCorte.MatchString(codigo) {
			dentro = false
		}
		if dentro {
			for _, m := range reMetrica.FindAllString(codigo, -1) {
				miradas[m] = true
			}
		}
	}

	// ── Las dos direcciones. Cada una es un defecto distinto y se arregla distinto ──────────
	for m := range empujadas {
		if !miradas[m] {
			t.Errorf("`comparar-y-latir.sh` empuja %s y NINGUNA expresión de musubi-alerts.yml la mira.\n"+
				"Una métrica que se empuja y nadie lee es trabajo que se paga y no se cobra — y si "+
				"reemplazó a la que sí se leía, las alertas de A115 quedaron calladas mirando una "+
				"serie muerta, que es indistinguible de «todo bien».", m)
		}
	}
	for m := range miradas {
		if !empujadas[m] {
			t.Errorf("una alerta de musubi-alerts.yml mira %s y `comparar-y-latir.sh` NO la empuja.\n"+
				"Esa alerta no puede dispararse nunca: es el defecto exacto de A73 —una regla cargada "+
				"sobre una métrica que no existe se ve igual que una regla en verde—.\n"+
				"O se renombró la métrica en el guion y no acá, o la alerta se escribió contra una "+
				"serie que nadie produce.", m)
		}
	}
	t.Logf("%d métrica(s) empujada(s) y las mismas %d miradas por las alertas", len(empujadas), len(miradas))
}

// TestLasAlertasDelLatidoLeenLaSerieConLastOverTime — el defecto que sólo se vio corriéndolo.
//
// UNA SERIE EMPUJADA NO ESTÁ CASI NUNCA. Prometheus la marca rancia ~5 minutos después de cada
// empuje y desaparece del vector instantáneo. Medido el 2026-09-05 contra el servidor, el mismo
// dato consultado en cuatro instantes: a los 58 s presente; a los 308 s, 508 s y 608 s AUSENTE,
// mientras `last_over_time(...[7d])` lo encontraba en los cuatro. Y `comparar-y-latir.sh` late
// cada SEIS HORAS.
//
// ESCRITAS CON LA MÉTRICA PELADA, LAS TRES ALERTAS ESTABAN ROTAS EN LAS DOS DIRECCIONES A LA VEZ:
//
//	· `ComparacionRepoServidorSinCorrer` disparaba a los cinco minutos de CADA latido, por su
//	  brazo `absent()` — un falso positivo cada 6 h para siempre, que es cómo se enseña a ignorar
//	  un canal (los trece `MaquinaCaida` de A79).
//	· `ProduccionDivergeDelRepo` y `DespliegueConEslabonesSinVerificar` NO PODÍAN DISPARARSE
//	  NUNCA: su `for` de 30 min no llega a cumplirse antes de que la serie se evapore. Y el modo
//	  de falla es mudo — `max()` sobre una serie rancia no da 0, no da NADA, y una expresión sin
//	  resultado no alerta.
//
// NINGUNA LECTURA DEL YAML LO MOSTRABA: las tres expresiones se leen perfectamente bien y dicen
// lo que uno quiere que digan. Se vio porque se corrió y la alerta apareció disparando con el
// latido recién empujado. Por eso esta guarda es sobre la FORMA de la expresión y no sobre su
// sentido: el sentido ya era correcto.
func TestLasAlertasDelLatidoLeenLaSerieConLastOverTime(t *testing.T) {
	alertas, err := leerArchivoDeDespliegue(filepath.Join("..", "..", "deploy", "musubi-alerts.yml"))
	if err != nil {
		t.Fatalf("no se pudo leer deploy/musubi-alerts.yml: %v", err)
	}

	// Sólo el bloque `expr:`. Estas métricas se nombran también en comentarios —el que explica
	// esta misma medición las cita— y ahí no deciden nada.
	reCorte := regexp.MustCompile(`^(for|labels|annotations|keep_firing_for)\s*:|^- alert:`)
	// La forma buena: la métrica va DENTRO de last_over_time( o absent_over_time( y con rango.
	// CUALQUIER FUNCIÓN `*_over_time` VALE, y la lista de dos nombres era demasiado estrecha.
	//
	// La propiedad que esta guarda custodia es que la serie se lea sobre un RANGO y no en el
	// vector instantáneo — porque una serie empujada se pone rancia a los ~5 minutos y el latido
	// es cada 6 horas. Todas las `*_over_time` de PromQL leen un rango por construcción: exigir
	// dos nombres concretos rechaza expresiones correctas, y una guarda que grita en falso se
	// aprende a ignorar.
	//
	// Lo encontró un `max_over_time(...[30d])` legítimo: el trinquete que detecta que alguien
	// APAGÓ la exigencia de TLS después de haberla prendido necesita mirar treinta días atrás, y
	// `last_over_time` no puede contestar «¿alguna vez estuvo en 1?».
	reEnvuelta := regexp.MustCompile(`[a-z_]+_over_time\(\s*musubi_verificacion_[a-z0-9_]+\s*\[`)
	reCruda := regexp.MustCompile(`musubi_verificacion_[a-z0-9_]+`)

	dentro, vistas := false, 0
	for n, linea := range strings.Split(string(alertas), "\n") {
		codigo := strings.TrimSpace(linea)
		if strings.HasPrefix(codigo, "#") {
			continue
		}
		if strings.HasPrefix(codigo, "expr:") {
			dentro = true
		} else if dentro && reCorte.MatchString(codigo) {
			dentro = false
		}
		if !dentro {
			continue
		}
		// Se tachan las apariciones bien envueltas y se mira si sobra alguna: contar unas y otras
		// por separado daría verde en una línea que tenga una envuelta y una pelada.
		crudas := len(reCruda.FindAllString(codigo, -1))
		if crudas == 0 {
			continue
		}
		vistas += crudas
		if envueltas := len(reEnvuelta.FindAllString(codigo, -1)); envueltas < crudas {
			t.Errorf("musubi-alerts.yml:%d lee una serie del latido SIN `last_over_time`/`absent_over_time`:\n"+
				"  %s\n"+
				"Una serie EMPUJADA se pone rancia ~5 min después de cada empuje y el latido es cada 6 h, "+
				"así que en el vector instantáneo no está casi nunca. Escrita así, la alerta o dispara "+
				"en falso el 99%% del tiempo (si pregunta por `absent`) o no puede dispararse jamás "+
				"(si su `for` es más largo que los 5 min de vida de la serie). Las dos fallas son mudas "+
				"al leer el YAML: la expresión se lee bien.\n"+
				"Arreglo: envolvela — `last_over_time(<metrica>[7d])`, o `absent_over_time(<metrica>[7d])`.",
				n+1, codigo)
		}
	}
	if vistas == 0 {
		t.Fatal("no encontré ninguna serie `musubi_verificacion_*` en las expresiones de " +
			"musubi-alerts.yml: o las alertas del latido de A115 se fueron —y entonces la " +
			"comparación repo↔servidor volvió a no tener quien la vigile— o cambiaron de nombre y " +
			"esta guarda quedó mirando al vacío en verde")
	}
	t.Logf("%d lectura(s) de la serie del latido, todas envueltas", vistas)
}

// TestNadieLeDiceAlOperadorQueDecideElDirectorioDeTrabajo — A109 aplicado a la CLASE.
//
// EL CABO, Y ES SOBRE UNA GUARDA MÍA. A109 encontró que `avisarQueConfigGobierna` cerraba con
// «manda el del directorio de trabajo» y midió que es FALSO donde más importa: los daemons de esta
// máquina corren SIN `MUSUBI_HOME` pero CON `CLAUDE_PROJECT_DIR`, así que la raíz sale de la
// variable y el cwd sólo coincide. Se corrigió, y la guarda que se escribió
// —`TestElOrigenDeLaRaizSeDiceYNoSeAdivina`— prueba el COMPORTAMIENTO de `workspaceDirConOrigen`.
//
// Eso no cubre la afirmación escrita en otro lado, y por eso sobrevivió una: el check
// `config_que_gobierna` de `internal/memory/doctor_config.go` —que existe justamente para
// contestar «¿cuál config manda acá?»— terminaba con la misma frase. Encontrado el 2026-09-05
// corriéndolo, no leyéndolo. La lección aprendida de un lado y no del hermano, otra vez.
//
// SE MIRAN LOS LITERALES DE CADENA Y NO LOS COMENTARIOS, y no es por comodidad: el defecto es lo
// que se le DICE al operador. Un comentario que explica por qué la frase está prohibida —como éste—
// no manda a nadie a mirar el cwd. Mirar el archivo entero pondría en rojo su propia documentación,
// que es la trampa en la que cayó la guarda gemela de A109 en su primera corrida.
func TestNadieLeDiceAlOperadorQueDecideElDirectorioDeTrabajo(t *testing.T) {
	// SE PROHÍBE LA PROPIEDAD, NO UNA REDACCIÓN — y esto lo enseñó el sabotaje de esta misma
	// guarda. La primera versión buscaba la cadena exacta `manda el del directorio de trabajo`,
	// que es como lo decía `avisarQueConfigGobierna`. El defecto real que la motivó decía «el que
	// manda ES EL del directorio de trabajo», con dos palabras de más, y la guarda NO lo cazaba:
	// habría pasado en verde sobre la instancia que la hizo existir. Ahora se busca la relación
	// —«manda» cerca de «directorio de trabajo»— y no una frase.
	//
	// Y se juntan TODOS los literales del archivo antes de buscar, porque un mensaje largo se
	// escribe concatenado en varias líneas: partido en dos literales, ninguno contendría la
	// relación entera y el archivo pasaría entero.
	reProhibida := regexp.MustCompile(`(?i)manda[^"]{0,40}` + "directorio de " + "trabajo")
	// Un literal de cadena de Go, en una línea que no es comentario.
	reLiteral := regexp.MustCompile(`"[^"]*"`)

	raiz := filepath.Join("..", "..")
	revisados := 0
	err := filepath.WalkDir(raiz, func(ruta string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "worktrees", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		b, e := os.ReadFile(ruta)
		if e != nil {
			return nil
		}
		revisados++
		var literales []string
		primera := 0
		for n, linea := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(linea), "//") {
				continue
			}
			for _, lit := range reLiteral.FindAllString(linea, -1) {
				if primera == 0 && reProhibida.MatchString(strings.Join(append(literales, lit), " ")) {
					primera = n + 1
				}
				literales = append(literales, lit)
			}
		}
		{
			blob := strings.Join(literales, " ")
			if m := reProhibida.FindString(blob); m != "" {
				t.Errorf("%s:%d le dice al operador que decide el directorio de trabajo:\n  %s\n"+
					"Es FALSO donde más importa: un daemon sin MUSUBI_HOME y con CLAUDE_PROJECT_DIR "+
					"toma la raíz de la VARIABLE, y el cwd sólo coincide (medido en /proc/<pid>/environ, "+
					"A109). Un diagnóstico que nombra la causa equivocada manda a mirar el cwd —que se "+
					"puede cambiar— en vez de la variable, que decide.\n"+
					"Arreglo: nombrá la raíz resuelta y de dónde salió (`workspaceDirConOrigen`), o "+
					"apuntá a donde eso se dice, en vez de afirmar una causa que este código no conoce.",
					ruta, primera, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recorriendo el repo: %v", err)
	}
	if revisados < 50 {
		t.Fatalf("sólo se revisaron %d archivos .go: el recorrido no está mirando el repo y esta "+
			"guarda pasaría en verde sin haber leído nada", revisados)
	}
	t.Logf("%d archivos .go de producción revisados", revisados)
}
