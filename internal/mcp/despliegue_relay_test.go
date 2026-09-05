package mcp

// despliegue_relay_test.go custodia el despliegue del RELAY DE PANTALLA (RustDesk hbbs/hbbr).
//
// Hay DOS caminos para montarlo —el instalador systemd y el compose— y eso es una invitación a
// que se vayan a la deriva: alguien corrige uno, el otro queda, y elegir el despliegue por
// comodidad termina cambiando la EXPOSICIÓN del relay sin que nadie lo decidiera. Las guardas de
// acá son las cuatro cosas cuyo incumplimiento no rompe nada visible:
//
//   1. Ninguno de los dos se ata a 0.0.0.0 por defecto.
//   2. Los dos exigen la clave (`-k _`).
//   3. hbbs y hbbr comparten el directorio de datos.
//   4. El 21116 se publica también en UDP.
//
// Las últimas dos son las peores: con volúmenes separados cada servicio genera SU clave, hbbr
// rechaza a quien hbbs aceptó, y el síntoma es una conexión que se establece y se cae al segundo.
// Sin el UDP, los clientes simplemente no se registran — y el TCP contesta igual, así que todo
// «se ve bien».

import (
	"fmt"
	"math"
	"musubi/internal/fleet"
	"os"
	"regexp"
	"strings"
	"testing"
)

func leerDespliegueRelay(t *testing.T, archivo string) string {
	t.Helper()
	b, err := os.ReadFile("../../deploy/rustdesk/" + archivo)
	if err != nil {
		t.Fatalf("falta %s: el relay de pantalla no se puede desplegar: %v", archivo, err)
	}
	return string(b)
}

// TestNingunCaminoDelRelaySeAtaAlMundoPorDefecto.
//
// Un relay de pantalla en 0.0.0.0 queda a un `firewall-cmd --add-port` de estar en internet. Que
// sea posible está bien —es el «acceso híbrido» del README— pero tiene que costar una bandera
// explícita, no ser lo que pasa si no decís nada.
//
// Sabotaje que la hace fallar: cambiar el fallback de `preparar.sh` a BIND="0.0.0.0".
func TestNingunCaminoDelRelaySeAtaAlMundoPorDefecto(t *testing.T) {
	for _, archivo := range []string{"install-rustdesk-relay.sh", "preparar.sh"} {
		texto := leerDespliegueRelay(t, archivo)

		// El default tiene que venir del tailnet. Los dos lo buscan en el rango 100.64/10.
		if !strings.Contains(texto, "100") || !regexp.MustCompile(`tailscale ip|\$1==100`).MatchString(texto) {
			t.Errorf("%s no busca la IP del tailnet: perdió el default seguro", archivo)
		}
		// Y 0.0.0.0 sólo puede aparecer como algo que se PIDE, nunca como el valor al que se cae.
		for _, linea := range strings.Split(texto, "\n") {
			l := strings.TrimSpace(linea)
			if strings.HasPrefix(l, "#") || !strings.Contains(l, "0.0.0.0") {
				continue
			}
			if regexp.MustCompile(`BIND="?0\.0\.0\.0`).MatchString(l) {
				t.Errorf("%s se ata a 0.0.0.0 por defecto:\n    %s", archivo, l)
			}
		}
	}
}

// TestLosDosCaminosDelRelayExigenLaClave.
//
// Sin `-k _` cualquier cliente que apunte a esta dirección se registra, y el relay propio pasa a
// ser tan abierto como el público — con la diferencia de que uno cree que no lo es.
//
// Sabotaje que la hace fallar: quitar `-k _` del compose o del instalador.
func TestLosDosCaminosDelRelayExigenLaClave(t *testing.T) {
	// CADA SERVICIO por su nombre, no «el archivo menciona -k _». La primera versión buscaba la
	// cadena en el archivo entero y pasaba en verde con hbbr desarmado: hbbs la tenía y tapaba el
	// agujero. Un relay con hbbs exigiendo la clave y hbbr sin exigirla es exactamente el estado
	// que ninguna prueba genérica ve.
	compose := leerDespliegueRelay(t, "compose.yml")
	for _, svc := range []string{"hbbs", "hbbr"} {
		if !strings.Contains(compose, svc+" -k _") {
			t.Errorf("compose.yml no arranca %s con `-k _`: ese servicio acepta cualquier cliente que lo apunte, y nadie se entera", svc)
		}
	}
	if !strings.Contains(leerDespliegueRelay(t, "install-rustdesk-relay.sh"), "-k _") {
		t.Error("install-rustdesk-relay.sh no pasa `-k _`: el camino systemd quedaría más abierto que el de contenedores, que es justo la deriva que estas guardas persiguen")
	}
}

// TestHbbsYHbbrCompartenElDirectorioDeDatos — el fallo mudo más caro del compose.
//
// Los dos servicios tienen que leer LA MISMA clave. Con volúmenes distintos cada uno genera la
// suya al arrancar, hbbr rechaza a los clientes que hbbs aceptó, y el síntoma es una sesión que
// conecta y se corta al segundo — sin un error que apunte a la causa.
//
// Sabotaje que la hace fallar: darle a hbbr su propio `${MUSUBI_RUSTDESK_DIR}/hbbr:/root`.
func TestHbbsYHbbrCompartenElDirectorioDeDatos(t *testing.T) {
	texto := leerDespliegueRelay(t, "compose.yml")
	re := regexp.MustCompile(`(?m)^\s*-\s*(\S+):/root(:\w+)?\s*$`)
	monturas := re.FindAllStringSubmatch(texto, -1)
	if len(monturas) != 2 {
		t.Fatalf("se esperaban 2 monturas de /root (hbbs y hbbr) y se encontraron %d: el patrón cambió o falta un servicio", len(monturas))
	}
	if monturas[0][1] != monturas[1][1] {
		t.Errorf("hbbs monta %q y hbbr monta %q: cada uno va a generar su propia clave y las sesiones se van a cortar al conectar",
			monturas[0][1], monturas[1][1])
	}
	// Y con etiqueta de SELinux, o el contenedor no puede escribir su clave.
	for _, m := range monturas {
		if m[2] == "" {
			t.Errorf("la montura %s va sin `:z`: en un host con SELinux el relay arranca y no guarda nada", m[1])
		}
	}
}

// TestElRelayPublicaElUDPDel21116.
//
// El latido de los clientes va por UDP. Sin ese puerto los clientes NO se registran — y como el
// TCP del mismo número sí contesta, cualquier verificación superficial dice que todo anda.
//
// Sabotaje que la hace fallar: borrar la línea del `/udp` del compose.
func TestElRelayPublicaElUDPDel21116(t *testing.T) {
	texto := leerDespliegueRelay(t, "compose.yml")
	if !strings.Contains(texto, "21116:21116/udp") {
		t.Error("el compose no publica 21116/udp: los clientes no se registran, y el TCP contesta igual — el fallo se ve como «todo bien»")
	}
	// Y el preparar.sh tiene que VERIFICARLO, no sólo publicarlo.
	if !strings.Contains(leerDespliegueRelay(t, "preparar.sh"), "21116") ||
		!regexp.MustCompile(`ss -lnu|udp`).MatchString(leerDespliegueRelay(t, "preparar.sh")) {
		t.Error("preparar.sh no verifica el UDP: publicar un puerto y comprobar que escucha son dos cosas distintas")
	}
}

// TestElComposeDelRelayNoUsaLaRedDelHost — la decisión opuesta a la de la cadena de alertas, y a
// propósito.
//
// Prometheus usa red del host porque necesita que `127.0.0.1` signifique el host. Acá el problema
// es el inverso: con red del host, hbbs y hbbr escuchan en TODAS las interfaces. Publicar los
// puertos atados a la IP del tailnet es cierto independientemente de cómo esté el firewall — que
// es justo la propiedad que se quiere, porque el firewall de ese servidor no se puede ni leer sin
// root.
//
// Sabotaje que la hace fallar: agregarle `network_mode: host` a cualquiera de los dos servicios.
func TestElComposeDelRelayNoUsaLaRedDelHost(t *testing.T) {
	texto := leerDespliegueRelay(t, "compose.yml")
	// Sólo las líneas que NO son comentario. La primera versión de esta guarda daba rojo con el
	// compose correcto: el archivo NOMBRA `network_mode: host` en el comentario que explica por
	// qué no se usa. Una guarda que no distingue lo que el archivo HACE de lo que el archivo
	// CUENTA obliga a dejar de escribir el porqué, que es lo último que uno quiere perder.
	for _, l := range strings.Split(texto, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		if strings.Contains(l, "network_mode: host") {
			t.Error("el compose del relay usa red del host: hbbs y hbbr pasarían a escuchar en las interfaces públicas, y la única defensa quedaría siendo un firewall que ni se puede leer sin root")
		}
	}
	// Los puertos tienen que ir atados a una dirección, no publicados a secas.
	re := regexp.MustCompile(`(?m)^\s*-\s*"(\d+)?:?\d+:\d+`)
	for _, l := range strings.Split(texto, "\n") {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, `- "`) || !strings.Contains(l, "211") {
			continue
		}
		if !strings.Contains(l, "MUSUBI_RUSTDESK_BIND") {
			t.Errorf("un puerto se publica sin atarlo a MUSUBI_RUSTDESK_BIND y queda en todas las interfaces:\n    %s", l)
		}
	}
	_ = re
}

// TestPrepararGuardaLaIdentidadDelRelayYNoMienteSobreLoQueProtege.
//
// `data/id_ed25519` son 88 bytes que no cambian nunca y cuya pérdida cuesta una tarde POR MÁQUINA:
// el relay vuelve con otra clave y toda la flota deja de conectar hasta reconfigurar cada cliente
// a mano. Que se copie no alcanza — la copia vive en el MISMO disco, y un respaldo que se presenta
// como protección sin decir contra qué NO protege es peor que no tenerlo, porque alguien deja de
// buscar el de verdad.
//
// Sabotajes que la hacen fallar: sacar la copia de `preparar.sh`, o borrar la salvedad de que no
// protege contra perder el disco.
func TestPrepararGuardaLaIdentidadDelRelayYNoMienteSobreLoQueProtege(t *testing.T) {
	texto := leerDespliegueRelay(t, "preparar.sh")

	if !strings.Contains(texto, "id_ed25519") {
		t.Fatal("preparar.sh no toca la identidad del relay: si ese archivo se pierde, toda la flota deja de conectar y no hay copia en ningún lado")
	}
	// La privada tiene que quedar más cerrada que como la deja el contenedor (0644).
	if !regexp.MustCompile(`install -m 0600 .*id_ed25519\b`).MatchString(texto) {
		t.Error("la copia de la clave PRIVADA no se instala con 0600: el original queda 0644 y copiarlo tal cual esparce el permiso flojo")
	}
	// Y la salvedad, con todas las letras.
	for _, quiero := range []string{"NO protege", "BACKUP_REMOTE"} {
		if !strings.Contains(texto, quiero) {
			t.Errorf("preparar.sh no dice %q: presentaría como respaldo algo que vive en el mismo disco que el original", quiero)
		}
	}
}

// EL COLECTOR DEL RELAY TIENE QUE PRODUCIR UN REPORTE QUE EL CEREBRO ACEPTE CON TODO CAÍDO.
//
// Es la quinta cosa cuyo incumplimiento no rompe nada visible, y era la peor de las cinco.
//
// El colector contaba `atendidas` y `fallidas` como DISJUNTAS —una por puerto que contestó, la
// otra por puerto que no— y el cerebro exige que las fallidas sean un SUBCONJUNTO (Rendimiento.
// Valida). Con dos puertos caídos salía «atendidas 1, fallidas 2»; con los tres, «0 y 3». Los dos
// rechazados. Y rechazado NO es «se pierde el rendimiento»: `Salud.Valida()` falla y el UPDATE de
// last_health y last_report no se hace, así que el cerebro se queda con la última salud BUENA:
//
//	· musubi_fleet_service_up{service="relay-rustdesk"} se queda en 1 con el relay muerto, y la
//	  alerta ServicioCaido no puede disparar NUNCA;
//	· el panel dibuja el rendimiento congelado del relay sano;
//	· lo único que suena, 15 minutos tarde, acusa al COLECTOR de haber dejado de reportar —
//	  siendo que reporta cada minuto y es el cerebro el que lo tira.
//
// Y es el único caso para el que ese colector existe: un contenedor levantado que no atiende, que
// para el agente se ve idéntico a sano.
//
// LA PRUEBA MIRA EL TEXTO DEL .py PORQUE LA SUITE ES GO Y EL COLECTOR ES PYTHON, que es el mismo
// recurso que ya usan las guardas del instalador de Windows y del pin del guion de backup. Es un
// amarre pobre y es el que hay; por eso además se ejercita el contrato de verdad abajo.
//
// Sabotaje que la hace fallar: volver a contar `atendidas` sólo sobre los puertos que contestaron.
func TestElColectorDelRelayCuentaLasSondasIntentadasYNoLasQueContestaron(t *testing.T) {
	b, err := os.ReadFile("../../deploy/colectores/reportar-relay.py")
	if err != nil {
		t.Fatalf("no se pudo leer el colector del relay: %v", err)
	}
	src := string(b)
	for _, q := range []struct{ frag, porque string }{
		{`"atendidas": len(PUERTOS)`, "atendidas son las sondas INTENTADAS; contar sólo las que contestaron hace que fallidas las supere y el cerebro descarte el reporte entero"},
		{`"fallidas": len(caidos)`, "fallidas tiene que ser el subconjunto que falló de esas mismas sondas"},
	} {
		if !strings.Contains(src, q.frag) {
			t.Errorf("el colector del relay ya no dice %s: %s", q.frag, q.porque)
		}
	}
	// Y que no vuelva el contador disjunto que causaba el rechazo.
	if strings.Contains(src, "atendidas += 1") {
		t.Error("volvió el contador disjunto `atendidas += 1`: cuenta los puertos que contestaron, " +
			"no las sondas hechas, y con eso fallidas supera a atendidas en cuanto se cae un puerto")
	}
}

// Y EL CONTRATO SE EJERCITA DE VERDAD, no sólo se lee.
//
// La guarda de arriba es texto; ésta pasa por Rendimiento.Valida() y por TasaDeError(), que es lo
// que el cerebro hace con el reporte. Sin ella, alguien podría cambiar la regla del cerebro y las
// dos pruebas quedarían diciendo cosas distintas sin que ninguna se pusiera roja.
//
// Sabotaje que la hace fallar: hacer que TasaDeError devuelva ok=false con atendidas>0, o apretar
// la regla del desglose a `total >= Atendidas` (con los tres puertos caídos el desglose IGUALA a
// las sondas, así que ahí un `>=` rechazaría el reporte legítimo).
//
// Y uno que NO la hace fallar, anotado porque lo probé y me equivoqué: QUITAR el chequeo de
// subconjunto de Valida la deja en verde, y hace bien — esta prueba afirma que el reporte se
// ACEPTA, y quitar una regla sólo puede aceptar más. La que custodia ese chequeo es la de arriba,
// que mira el colector. Un doc que nombra un sabotaje que no funciona es exactamente la prueba
// decorativa contra la que existe esta costumbre, así que queda dicho cuál es cuál.
func TestElReporteDelRelayConLosTresPuertosCaidosLoAceptaElCerebro(t *testing.T) {
	const sondas = 3
	for _, c := range []struct {
		nombre  string
		caidos  int
		tasaEsp float64
	}{
		{"todo sano", 0, 0},
		{"un puerto caído", 1, 100.0 / 3},
		{"dos caídos", 2, 200.0 / 3},
		{"el relay entero caído", 3, 100},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			desglose := map[string]int{}
			for i := 0; i < c.caidos; i++ {
				desglose[fmt.Sprintf("puerto_2111%d", i)] = 1
			}
			r := &fleet.Rendimiento{
				VentanaSeg: 60, Atendidas: sondas, Fallidas: c.caidos, Desglose: desglose,
			}
			if err := r.Valida(); err != nil {
				t.Fatalf("el cerebro rechazaría el reporte con %d puerto(s) caído(s): %v — y "+
					"rechazarlo NO pierde el rendimiento: deja la última salud BUENA en la base, "+
					"así que ServicioCaido no puede disparar nunca", c.caidos, err)
			}
			tasa, hay := r.TasaDeError()
			if !hay {
				t.Fatal("no hay tasa de error con sondas intentadas > 0")
			}
			if math.Abs(tasa-c.tasaEsp) > 0.01 {
				t.Errorf("tasa de error %.2f%%, se esperaba %.2f%%", tasa, c.tasaEsp)
			}
		})
	}
}
