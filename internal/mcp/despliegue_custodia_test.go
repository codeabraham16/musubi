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
	guion, err := os.ReadFile(filepath.Join("..", "..", "deploy", "musubi-backup.sh"))
	if err != nil {
		t.Fatalf("no se pudo leer deploy/musubi-backup.sh: %v", err)
	}
	real := fmt.Sprintf("%x", sha256.Sum256(guion))

	inst, err := os.ReadFile(filepath.Join("..", "..", "deploy", "install-musubi-brain.sh"))
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
