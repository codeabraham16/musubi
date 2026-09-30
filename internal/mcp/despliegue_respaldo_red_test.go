package mcp

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// EL RESPALDO ESPERA A LA RED ANTES DE COPIAR, IGUAL QUE SU HERMANO (A139).
//
// EL INCIDENTE, MEDIDO EL 2026-09-29. La laptop arrancó a las 10:16:44 y a las 10:21:12 los dos
// timers `Persistent=true` de esta máquina dispararon JUNTOS. El wifi recién se conectó a las
// 10:21:49. Esto fue lo que dejó cada uno en el journal:
//
//	musubi-comparar         10:21:12  · musubi-server todavía no contesta; espero hasta 300s
//	                        10:21:54  · seguí a los 42s                      → anduvo
//	musubi-respaldo-local   10:21:13  ssh: connect to host 100.79.126.62 port 22: Network is unreachable
//	                        10:21:13  [musubi-backup] ERROR: rsync falló     → la unidad en failed
//
// Misma máquina, mismo segundo, mismo servidor. La única diferencia era la espera, que
// `comparar-y-latir.sh` ya tenía y `musubi-backup.sh` no: la lección aprendida de un lado y no
// del hermano. El arreglo es la MISMA espera —la misma sonda, el mismo techo, la misma variable—
// y no una parecida, porque dos esperas que difieren en un detalle se reparan por separado.
//
// SE MIDE CONDUCTA Y NO TEXTO. El arnés corre el guion de verdad con el mundo falseado: delante del
// PATH van un `ssh`, un `rsync`, un `date` y un `sleep` que comparten un reloj en un archivo, así
// que cinco minutos de espera se prueban en un segundo y la «red» levanta a la hora que diga cada
// caso. Un `grep` de `MUSUBI_ESPERA_RED` sobre el guion lo satisfaría el comentario que explica la
// espera, y seguiría verde con la espera rota.
//
// EL OTRO CORTE, QUE NO ES DE ESTE ARREGLO Y POR ESO DEFINE UN CASO. El 2026-09-21 a las 03:25:44
// el respaldo murió con «Connection timed out» sin arranque de por medio: tailscaled no llegaba a
// nada desde antes de la 01:00 hasta las 05:15. Ninguna espera razonable salva cuatro horas, y
// está bien que así sea — por eso, al vencerse, el guion SIGUE y la copia falla por el camino de
// siempre (`die_offhost`, `.last_offhost_error`). Una espera que fallara por su cuenta sería un
// segundo camino de error que nadie vigila.
//
// Sabotaje que la hace fallar: que la espera sea el default del guion. Corrido a mano, un destino
// caído tiene que fallar rápido; sólo la unidad opta por esperar.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="${MUSUBI_ESPERA_RED:-0}"
// arnes: a="${MUSUBI_ESPERA_RED:-300}"
// Y la otra dirección: la misma expansión sin los dos puntos da lo mismo para todo valor que la
// unidad pueda poner, y tiene que quedar en verde.
// arnes: arreglo_de="${MUSUBI_ESPERA_RED:-0}"
// arnes: arreglo_a="${MUSUBI_ESPERA_RED-0}"
//
// Sabotaje que la hace fallar: no esperar nunca, que es el guion de antes de A139.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="if [ \"$ESPERA_RED\" -gt 0 ] 2>/dev/null; then"
// arnes: a="if false; then"
//
// Sabotaje que la hace fallar: que la espera vencida corte el respaldo en vez de seguir.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="para que el fallo se VEA\"\n          break"
// arnes: a="para que el fallo se VEA\"\n          die_offhost \"la red no volvió\""
//
// Sabotaje que la hace fallar: sondear el destino entero, ruta incluida, en vez de su host.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="HOST_RED=\"$(host_ssh_del_destino \"$BACKUP_REMOTE\")\""
// arnes: a="HOST_RED=\"$BACKUP_REMOTE\""
//
// Sabotaje que la hace fallar: sondear con cualquier método, rclone incluido, que no va por ssh.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="if [ \"$BACKUP_METHOD\" = rsync ]; then"
// arnes: a="if true; then"
//
// Sabotaje que la hace fallar: olvidar que «host::módulo» es el daemon de rsync.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="  case \"$resto\" in :*) return 0 ;; esac\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: olvidar que una «/» antes del «:» es una ruta local.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="|*/*)"
// arnes: a=")"
//
// Sabotaje que la hace fallar: olvidar que sin «:» el destino es una ruta local.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="  [ \"$host\" != \"$d\" ] || return 0\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: olvidar que `rsync://` es el daemon de rsync.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="  case \"$d\" in rsync://*) return 0 ;; esac\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: partir un IPv6 entre corchetes con la regla del primer «:».
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="  case \"$host\" in *'['*) return 0 ;; esac\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: sondear cada segundo en vez de cada cinco, que en cinco minutos son
// trescientas conexiones ssh contra un host que no contesta.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="        sleep 5\n"
// arnes: a="        sleep 1\n"
//
// Sabotaje que la hace fallar: que el techo se pase de largo un sondeo.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="-ge \"$ESPERA_RED\" ]; then"
// arnes: a="-gt \"$ESPERA_RED\" ]; then"
//
// Sabotaje que la hace fallar: esperar sin decir cuánto, que deja al journal sin explicar por qué
// la copia tardó.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="if [ \"$AVISADO\" -eq 1 ]; then log \"· seguí a los"
// arnes: a="if false; then log \"· seguí a los"
//
// Sabotaje que la hace fallar: que la marca del snapshot se escriba al salir, después de la
// espera, que es lo mismo que ve el arnés si la espera se muda arriba del snapshot.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="date -u +%Y-%m-%dT%H:%M:%SZ > \"$BACKUP_LOCAL_DIR/.last_snapshot\" || log \"no se pudo escribir la marca .last_snapshot\""
// arnes: a="trap 'date -u +%Y-%m-%dT%H:%M:%SZ > \"$BACKUP_LOCAL_DIR/.last_snapshot\"' EXIT"
//
// Sabotaje que la hace fallar: avisar en cada sonda, que en cinco minutos son sesenta líneas.
// arnes: archivo="deploy/musubi-backup.sh"
// arnes: de="if [ \"$AVISADO\" -eq 0 ]; then"
// arnes: a="if true; then"
//
// Sabotaje que la hace fallar: borrar del arnés uno de los destinos que no se sondean.
// arnes: archivo="deploy/pruebas/respaldo-espera-la-red.sh"
// arnes: de="sin_sonda local  rsync  \"copias\""
// arnes: a=": sin_sonda local  rsync  \"copias\""
//
// Sabotaje que la hace fallar: dejarle al arnés un solo valor que no es una espera.
// arnes: archivo="deploy/pruebas/respaldo-espera-la-red.sh"
// arnes: de="for v in 0 -5 abc 300s \"\"; do"
// arnes: a="for v in abc; do"
func TestElRespaldoEsperaLaRedAntesDeCopiar(t *testing.T) {
	compuerta := guiones.Exigir(t, "corre deploy/pruebas/respaldo-espera-la-red.sh, que ejercita el guion "+
		"de respaldo con un reloj falso", "bash", "mktemp", "env", "timeout", "awk", "sed", "grep", "cut",
		"wc", "tr", "du", "find", "date")

	arnes := filepath.Join("..", "..", "deploy", "pruebas", "respaldo-espera-la-red.sh")
	guion := filepath.Join("..", "..", "deploy", "musubi-backup.sh")
	salida, err := compuerta.Comando("bash", arnes, guion).CombinedOutput()
	if err != nil {
		t.Fatalf("el arnés del respaldo falló: %s\n%s", primeraQueja(salida), salida)
	}
	// CONTROL DE QUE EJERCITÓ CADA COSA QUE DICE: una señal por cada «✓» del arnés, y tantos «✓»
	// como señales. Si alguien borra un caso, el arnés sigue saliendo en 0 y esto es lo único que lo
	// nota. Una señal por BLOQUE no alcanzaba: la revisión de A139 borró seis de los siete destinos
	// que no se sondean y tres de los cuatro valores que no son una espera, y la prueba seguía verde.
	senales := []string{
		"el reloj falso avanza",
		"control: con la red arriba",
		"sin MUSUBI_ESPERA_RED no espera",
		"sondea cada 5 s y copia apenas el host contesta",
		"el registro dice que esperó y cuánto",
		"avisa que espera una sola vez",
		"sondea el host con su usuario y sin la ruta",
		"la espera corre después del snapshot",
		"al vencerse la espera sigue igual",
		"si la red nunca vuelve, falla por el camino de siempre",
		"rclone a «remoto:musubi»: no sondea",
		"la espera no aplica a ese destino",
		"rsync a «servidor::modulo»: no sondea",
		"rsync a «rsync://servidor/modulo»: no sondea",
		"rsync a «./copias:viejas»: no sondea",
		"rsync a «copias»: no sondea",
		"rsync a «respaldo@[2001:db8:0:0:0:0:0:1]:/srv/copias»: no sondea",
		"cp a «",
		"MUSUBI_ESPERA_RED=«0» no espera",
		"MUSUBI_ESPERA_RED=«-5» no espera",
		"MUSUBI_ESPERA_RED=«abc» no espera",
		"MUSUBI_ESPERA_RED=«300s» no espera",
		"MUSUBI_ESPERA_RED=«» no espera",
	}
	for _, senal := range senales {
		if !strings.Contains(string(salida), senal) {
			t.Fatalf("el arnés terminó en 0 pero no dijo %q, así que no ejercitó lo que dice:\n%s", senal, salida)
		}
	}
	controles := 0
	for _, linea := range strings.Split(string(salida), "\n") {
		if strings.HasPrefix(linea, "  ✓ ") {
			controles++
		}
	}
	if controles != len(senales) {
		t.Fatalf("el arnés dijo %d «✓» y la prueba conoce %d señales: se borró un caso, o se agregó uno "+
			"sin sumarle acá su señal:\n%s", controles, len(senales), salida)
	}
	if !strings.Contains(string(salida), "TODO OK") {
		t.Fatalf("el arnés terminó en 0 sin decir TODO OK:\n%s", salida)
	}
}

// LA UNIDAD DE UN GUION QUE SABE ESPERAR LA RED TIENE QUE PEDIRLE QUE ESPERE.
//
// La espera de los dos guiones vale 0 por defecto, a propósito: corridos a mano, un destino caído
// tiene que fallar rápido. Así que el arreglo de A139 vive en DOS lugares, el guion y la línea
// `Environment=MUSUBI_ESPERA_RED=…` de su unidad, y la prueba de arriba mide sólo el primero: le
// pasa la variable ella misma. Borrar la línea de la unidad dejaría esa prueba en verde y al
// respaldo de vuelta en el incidente, sin que nada lo dijera — una guarda apagada por una variable
// que nadie setea.
//
// SE DERIVA Y NO SE ENUMERA. No hay una lista de «las unidades que esperan»: se recorre cada
// unidad cuyo `ExecStart` corre un guion del repo, y si ese guion LEE la variable, la unidad tiene
// que ponerla en un número mayor que cero. Una unidad nueva con un guion que sepa esperar entra
// sola.
//
// Sabotaje que la hace fallar: dejar la espera del respaldo en cero desde su unidad.
// arnes: archivo="deploy/systemd/musubi-respaldo-local.service"
// arnes: de="Environment=MUSUBI_ESPERA_RED=300"
// arnes: a="Environment=MUSUBI_ESPERA_RED=0"
// arnes: colision_ok="TestLaUnidadDeUnGuionQueSabeEsperarLaRedLePideQueEspere"
// Y la otra dirección: entre comillas, que para systemd es la misma asignación, tiene que quedar en
// verde.
// arnes: arreglo_de="Environment=MUSUBI_ESPERA_RED=300"
// arnes: arreglo_a="Environment=\"MUSUBI_ESPERA_RED=300\""
//
// Sabotaje que la hace fallar: escribir la espera como systemd escribe una duración, `300s`. El
// guion no la toma como número y no espera nada. La guarda leía sólo los dígitos del principio y
// la daba por buena; lo encontró la revisión de A139. El sabotaje de arriba pisa esta misma línea
// y lo declara; los dos caen con motivos distintos, porque el mensaje cita el valor entero.
// arnes: archivo="deploy/systemd/musubi-respaldo-local.service"
// arnes: de="Environment=MUSUBI_ESPERA_RED=300"
// arnes: a="Environment=MUSUBI_ESPERA_RED=300s"
//
// Sabotaje que la hace fallar: una segunda asignación más abajo, en cero. systemd aplica la
// ÚLTIMA, y la guarda leía la primera: quedaba en verde con una unidad que no espera. Lo
// encontraron los tres jueces de la revisión de A139.
// arnes: archivo="deploy/systemd/musubi-respaldo-local.service"
// arnes: de="Environment=MUSUBI_HOME=@REPO@\n"
// arnes: a="Environment=MUSUBI_HOME=@REPO@\nEnvironment=MUSUBI_ESPERA_RED=0\n"
// Y la otra dirección: con sangría y con espacios alrededor del `=`, que systemd saca, es la misma
// línea y tiene que quedar en verde. La guarda de antes la buscaba al principio de la línea, y la
// daba por ausente.
// arnes: arreglo_de="Environment=MUSUBI_ESPERA_RED=300"
// arnes: arreglo_a="  Environment = MUSUBI_ESPERA_RED=300"
//
// Sabotaje que la hace fallar: un `Environment=` vacío más abajo. Para systemd borra todas las
// asignaciones anteriores, y no nombra la variable.
// arnes: archivo="deploy/systemd/musubi-respaldo-local.service"
// arnes: de="StandardOutput=journal"
// arnes: a="Environment=\nStandardOutput=journal"
func TestLaUnidadDeUnGuionQueSabeEsperarLaRedLePideQueEspere(t *testing.T) {
	unidades, err := filepath.Glob(filepath.Join("..", "..", "deploy", "systemd", "*.service"))
	if err != nil || len(unidades) == 0 {
		t.Fatalf("no encontré ninguna unidad en deploy/systemd/ (%v): el glob dejó de funcionar", err)
	}
	execStart := regexp.MustCompile(`(?m)^ExecStart=.*?@REPO@/deploy/([^\s"']+)`)
	leeLaEspera := regexp.MustCompile(`\$\{?MUSUBI_ESPERA_RED\b`)

	var revisadas []string
	for _, u := range unidades {
		b, err := leerArchivoDeDespliegue(u)
		if err != nil {
			t.Fatalf("leer %s: %v", u, err)
		}
		unidad := string(b)
		for _, m := range execStart.FindAllStringSubmatch(unidad, -1) {
			if !leeLaEspera.MatchString(leerDeploy(t, strings.Split(m[1], "/")...)) {
				continue
			}
			revisadas = append(revisadas, filepath.Base(u)+" → deploy/"+m[1])
			if n, pide := esperaQuePideLaUnidad(unidad); n <= 0 {
				t.Errorf("%s corre deploy/%s, que sabe esperar la red, y NO le pide que espere "+
					"(%s).\n"+
					"  El guion espera sólo con un entero de segundos mayor que cero, y vale 0 por "+
					"defecto: sin eso la unidad copia antes de que levante el tailnet, que es el "+
					"incidente del 2026-09-29 (A139). `300s`, como escribe systemd una duración, "+
					"tampoco le sirve.\n"+
					"  Se arregla con UNA línea `Environment=MUSUBI_ESPERA_RED=300` en la unidad, y "+
					"ninguna otra que nombre la variable.",
					filepath.Base(u), m[1], pide)
			}
		}
	}
	if len(revisadas) == 0 {
		t.Fatal("ninguna unidad corre un guion del repo que lea MUSUBI_ESPERA_RED: o cambió la forma de " +
			"escribir el ExecStart, o la de leer la variable — en los dos casos esta guarda dejó de mirar")
	}
	t.Logf("unidades revisadas: %s", strings.Join(revisadas, " · "))
}

// esperaQuePideLaUnidad devuelve la espera que el guion va a ver, en segundos, y cómo la pide la
// unidad, para el mensaje. Un número <= 0 es «no espera».
//
// SE LEE COMO LA LEE SYSTEMD, Y GANA LA ÚLTIMA ASIGNACIÓN (systemd.exec(5)). La guarda tomaba la
// primera `Environment=` que nombrara la variable: una segunda línea en 0 debajo de la de 300, un
// `Environment=` vacío —que borra todo lo anterior— o un `UnsetEnvironment=` la dejaban en verde
// con una unidad que no espera. Lo encontraron los tres jueces de la revisión de A139.
//
// NO SE MODELA CUÁL DE VARIAS GANA: LA VARIABLE SE NOMBRA UNA SOLA VEZ en `[Service]`. Una segunda
// mención es rojo, sea otra asignación, un `UnsetEnvironment=`, un `PassEnvironment=` o un `env`
// en el `ExecStart`, así que no hay una lista de formas que se pueda quedar corta. Lo único que
// pisa la asignación sin nombrarla es el `Environment=` vacío, y ése se mira aparte.
//
// LO QUE NO VE. Las variables de un `EnvironmentFile=` le ganan a `Environment=`, y el del respaldo
// vive fuera del repo. Y una línea continuada con `\` no se une con la siguiente: cada pedazo
// cuenta sus menciones igual, así que eso sólo puede costar un rojo de más.
func esperaQuePideLaUnidad(unidad string) (int, string) {
	nombra := regexp.MustCompile(`\bMUSUBI_ESPERA_RED\b`)
	// EL VALOR SE TOMA ENTERO Y SE LEE COMO LO LEE EL GUION: un entero, que `[ "$X" -gt 0 ]`
	// acepta. Con `(\d+)` a secas, `300s` pasaba por 300 y el guion no esperaba nada.
	asigna := regexp.MustCompile(`(?:^|\s)["']?MUSUBI_ESPERA_RED=([^\s"']*)`)
	n, pide := -1, "no la declara en `Environment=`"
	veces, seccion := 0, ""
	var menciones []string
	for _, linea := range strings.Split(unidad, "\n") {
		// systemd.syntax(7): los espacios de las puntas no cuentan, y `#` o `;` abren un
		// comentario.
		linea = strings.TrimSpace(linea)
		if linea == "" || strings.HasPrefix(linea, "#") || strings.HasPrefix(linea, ";") {
			continue
		}
		if strings.HasPrefix(linea, "[") {
			seccion = linea
			continue
		}
		if seccion != "[Service]" {
			continue
		}
		if k := len(nombra.FindAllStringIndex(linea, -1)); k > 0 {
			veces += k
			menciones = append(menciones, "«"+linea+"»")
		}
		// Los espacios alrededor del `=` tampoco cuentan.
		clave, valor, ok := strings.Cut(linea, "=")
		if !ok || strings.TrimSpace(clave) != "Environment" {
			continue
		}
		valor = strings.TrimSpace(valor)
		if valor == "" && veces > 0 {
			n, pide = -1, pide+", y un `Environment=` vacío más abajo borra todo lo anterior"
		} else if p := asigna.FindStringSubmatch(valor); p != nil {
			n, pide = -1, "MUSUBI_ESPERA_RED="+strconv.Quote(p[1])
			if v, err := strconv.Atoi(p[1]); err == nil {
				n = v
			}
		}
	}
	if veces > 1 {
		return -1, fmt.Sprintf("la nombra %d veces: %s. systemd aplica la última, y una segunda "+
			"asignación, un `UnsetEnvironment=` o un `env` en el `ExecStart` la pisan sin que se vea",
			veces, strings.Join(menciones, " · "))
	}
	return n, pide
}
