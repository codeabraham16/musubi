package mcp

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// EL UMBRAL DE LA ALERTA Y LA CADENCIA DEL TIMER SON EL MISMO NÚMERO ESCRITO DOS VECES
//
// `CertificadoTLSPorVencer` dispara a los 21 días, y esos 21 días no son un gusto: son TRES
// corridas del timer de renovación, que es semanal. El margen es el punto — la primera renovación
// puede fallar y todavía quedan dos oportunidades antes de que alguien tenga que apurarse.
//
// Escrito a mano en los dos lados, el día que alguien pase el timer a mensual el umbral se queda
// en 21 días y el aviso llegaría DESPUÉS de la única corrida que quedaba. Nadie lo notaría: los
// dos archivos siguen siendo válidos por separado. Es la forma que este repo persigue con nombre
// propio, y acá se cierra derivando uno del otro.
// ════════════════════════════════════════════════════════════════════════════════════════════

// corridasDeMargen es cuántas renovaciones tienen que caber adentro del umbral de la alerta.
//
// TRES Y NO DOS: con dos, la primera falla y la segunda es la última: eso es una carrera, no un
// aviso. Con tres, una falla deja dos oportunidades y el aviso llega mientras todavía se puede
// arreglar sin apuro.
const corridasDeMargen = 3

// periodoDelTimer saca del `OnCalendar` cada cuánto dispara la renovación.
//
// SE LEE EL ARCHIVO Y NO SE ESCRIBE EL NÚMERO, que es el punto entero de esta guarda. Sólo se
// entienden las dos formas que este repo usa; cualquier otra es un `t.Fatal` y no un default,
// porque un default acá inventaría una cadencia y la comparación siguiente no mediría nada.
func periodoDelTimer(t *testing.T, archivo string) time.Duration {
	t.Helper()
	texto := leerDeploy(t, "systemd", archivo)
	m := regexp.MustCompile(`(?m)^OnCalendar=(.+)$`).FindStringSubmatch(texto)
	if m == nil {
		t.Fatalf("%s no tiene una línea `OnCalendar=`: o cambió de forma de disparar o el archivo se "+
			"movió, y esta guarda dejó de mirar lo que dice mirar", archivo)
	}
	cal := strings.TrimSpace(m[1])
	switch {
	case regexp.MustCompile(`^(Mon|Tue|Wed|Thu|Fri|Sat|Sun)\b`).MatchString(cal):
		// Un día de la semana nombrado: dispara una vez por semana.
		return 7 * 24 * time.Hour
	case strings.HasPrefix(cal, "*-*-*"):
		// Todos los días, con una o más horas listadas: el período es 24 h dividido las horas.
		horas := strings.Count(strings.Fields(cal)[1], ",") + 1
		return time.Duration(24/horas) * time.Hour
	default:
		t.Fatalf("no sé leer el `OnCalendar=%s` de %s. Antes de agregarle un caso a esta función, "+
			"pensá si el umbral de la alerta sigue teniendo sentido con esa cadencia: el número "+
			"que custodia sale de acá.", cal, archivo)
		return 0
	}
}

// TestElUmbralDeVencimientoDejaTresCorridasDeMargen — el umbral se DERIVA de la cadencia.
//
// Sabotaje que la pone roja (los dos, y son las dos direcciones del mismo error): pasar el
// `OnCalendar=` del timer a mensual dejando el umbral en 21 días, o bajar el umbral de la alerta
// a 7 días dejando el timer semanal. Los dos archivos siguen siendo válidos por separado, que es
// justamente por qué hace falta una guarda que los cruce.
func TestElUmbralDeVencimientoDejaTresCorridasDeMargen(t *testing.T) {
	periodo := periodoDelTimer(t, "musubi-tls-renovar.timer")

	reglas := leerDeploy(t, "musubi-alerts.yml")
	i := strings.Index(reglas, "- alert: CertificadoTLSPorVencer")
	if i < 0 {
		t.Fatal("no está la alerta `CertificadoTLSPorVencer`: sin ella el certificado vence en " +
			"silencio y el cerebro arranca fail-closed el día 91")
	}
	bloque := reglas[i:]
	if j := strings.Index(bloque[1:], "- alert:"); j > 0 {
		bloque = bloque[:j]
	}
	// El umbral está escrito como `< 21 * 24 * 3600` para que se lea en días sin hacer cuentas.
	m := regexp.MustCompile(`musubi_tls_certificate_expiry_seconds\s*<\s*(\d+)\s*\*\s*24\s*\*\s*3600`).FindStringSubmatch(bloque)
	if m == nil {
		t.Fatalf("no pude leer el umbral en días de `CertificadoTLSPorVencer`. Se espera la forma "+
			"`musubi_tls_certificate_expiry_seconds < N * 24 * 3600`, que es la que deja el número "+
			"legible en días sin hacer cuentas:\n%s", bloque)
	}
	dias, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	umbral := time.Duration(dias) * 24 * time.Hour

	minimo := time.Duration(corridasDeMargen) * periodo
	if umbral < minimo {
		t.Errorf("`CertificadoTLSPorVencer` avisa a los %d días y el timer renueva cada %v, así que "+
			"caben %.1f corridas adentro del aviso y tienen que caber %d.\n"+
			"  Con menos margen, la primera renovación que falle deja el aviso llegando cuando ya no "+
			"hay otra oportunidad: eso es una carrera contra el reloj, no un aviso.\n"+
			"  O subís el umbral de la alerta, o acelerás el timer. Los dos archivos son válidos por "+
			"separado, y por eso este cruce existe.",
			dias, periodo, float64(umbral)/float64(periodo), corridasDeMargen)
	}

	// Y EL MARGEN NO PUEDE SER ABSURDO POR EL OTRO LADO: un umbral mayor que la vida entera del
	// certificado dispararía desde el minuto uno, todos los días, por una renovación que anda
	// perfecto. Un canal que suena siempre es un canal apagado.
	const vidaDelCertificado = 90 * 24 * time.Hour
	if umbral >= vidaDelCertificado {
		t.Errorf("el umbral son %d días y el certificado vive %v: la alerta estaría disparada SIEMPRE, "+
			"desde el instante en que se emite, aunque la renovación funcione perfecto.",
			dias, vidaDelCertificado)
	}
}

// TestLaRenovacionDelCertificadoTieneTimerYGuion — las tres piezas existen y se nombran entre sí.
//
// UNA PIEZA SUELTA ACÁ NO FALLA: NO HACE NADA. Un `.service` sin `.timer` no dispara nunca; un
// timer que apunta a un guion que no está deja la unidad en `failed` donde nadie mira; y un guion
// sin unidad es un archivo que alguien tiene que acordarse de correr, que es exactamente la
// condición que esto vino a eliminar. Las tres o ninguna.
func TestLaRenovacionDelCertificadoTieneTimerYGuion(t *testing.T) {
	unidad := leerDeploy(t, "systemd", "musubi-tls-renovar.service")
	timer := leerDeploy(t, "systemd", "musubi-tls-renovar.timer")
	guion := leerDeploy(t, "renovar-tls.sh")

	if !strings.Contains(timer, "Persistent=true") {
		t.Error("el timer de renovación no es `Persistent=true`: un disparo que cae con la máquina " +
			"apagada NO se pospone, se PIERDE, y las semanas son lo único que separa este timer del " +
			"día 90 del certificado")
	}
	if !strings.Contains(unidad, "ExecStart=") {
		t.Error("la unidad de renovación no tiene `ExecStart=`: no corre nada")
	}
	// EL NOMBRE DEL NODO NO PUEDE TENER UN DEFAULT. Un certificado emitido para el nombre
	// equivocado no lo valida ningún cliente, y eso es peor que no tenerlo: el cerebro sirve TLS,
	// la migración se declara hecha, y cada agente falla la verificación.
	if !strings.Contains(unidad, "Environment=MUSUBI_TLS_NODO=\n") &&
		!strings.HasSuffix(strings.TrimRight(unidad, "\n"), "Environment=MUSUBI_TLS_NODO=") {
		if regexp.MustCompile(`MUSUBI_TLS_NODO=\S`).MatchString(unidad) {
			t.Error("la unidad trae un `MUSUBI_TLS_NODO` con valor: ese nombre es el que va a decir el " +
				"certificado, y uno equivocado produce un certificado que ningún cliente valida. Tiene " +
				"que quedar vacío para que el guion salga con 2 y lo diga.")
		}
	}
	// EL GUION NO PUEDE REINICIAR EL CEREBRO NI PEDIR QUE SE REINICIE: la recarga en caliente es
	// lo que hace que la renovación sirva, y un reinicio semanal escondería que sin ella no
	// serviría. Si alguien la saca, esto tiene que ponerse rojo antes de que alguien agregue el
	// reinicio como parche.
	for _, prohibido := range []string{"systemctl restart", "systemctl --user restart"} {
		if strings.Contains(guion, prohibido) || strings.Contains(unidad, prohibido) {
			t.Errorf("la renovación reinicia el cerebro (`%s`). No hace falta: `GetCertificate` relee "+
				"el par cuando cambia. Y reiniciarlo taparía el caso que importa —alguien podría creer "+
				"que renovar el archivo alcanza— además de meter un corte semanal por nada.", prohibido)
		}
	}
	// Y EL GUION NO PUEDE IMPRIMIR LA CLAVE. Es lo mismo que con el uuid del watchdog: la salida
	// de esto va al journal, se pega en un chat y se lee en una pantalla compartida.
	if strings.Contains(guion, "cat \"$KEY\"") || strings.Contains(guion, "cat $KEY") {
		t.Error("el guion imprime la clave privada: su salida va al journal y se pega en chats")
	}
}
