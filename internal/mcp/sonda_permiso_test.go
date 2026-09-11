package mcp

import (
	"os"
	"strings"
	"testing"
)

// permisoDeLaSonda es la variable que autoriza a pegarle al cerebro de producción. Es una
// constante para que la guarda que custodia este opt-in mire el MISMO nombre que el código usa, y
// no una copia que se puede quedar vieja.
const permisoDeLaSonda = "MUSUBI_SONDA_CONTRA_PRODUCCION"

// laSondaTienePermiso es la DECISIÓN, extraída para que se pueda probar.
//
// Adentro del test era ingobernable: la única forma de verificar que el opt-in no está invertido
// habría sido un grep sobre el código, y una guarda de grep sobre la condición que decide es
// justo lo que este repo dejó de aceptar. Extraída, la prueba es de comportamiento: se le ponen
// las credenciales SIN el permiso y tiene que decir que no.
func laSondaTienePermiso() (string, bool) {
	if strings.TrimSpace(os.Getenv(permisoDeLaSonda)) != "" {
		return "", true
	}
	return "la sonda le pega 209 veces al cerebro de PRODUCCIÓN, así que hace falta un permiso " +
		"explícito: exportá " + permisoDeLaSonda + "=1. Tener las credenciales en el entorno NO " +
		"alcanza — es el estado normal de esta máquina, y leerlo como autorización es cómo una " +
		"corrida de rutina terminó pegándole 209 veces a producción.", false
}

// EL OPT-IN DE LA SONDA ESTABA INVERTIDO: LA CONDICIÓN DE DISPARO ERA EL ESTADO NORMAL.
//
// `TestSondaDiseno` le pega 209 veces al cerebro de PRODUCCIÓN con la credencial viva. Se
// disparaba cuando había `MUSUBI_CENTRAL_URL` y `MUSUBI_TOKEN` en el entorno — que es exactamente
// cómo está la terminal del operador todo el día. O sea que «tengo con qué» se leía como «me
// autorizaron», y cualquier `go test ./internal/mcp` de rutina salía a la red de producción.
//
// El corredor de sabotajes era el peor caso: llama a `go test` con un patrón ANCHO a propósito,
// así que seleccionaba la sonda sin que nadie la nombrara.
func TestLaSondaNoSeAutorizaSolaPorTenerCredenciales(t *testing.T) {
	t.Run("con las credenciales puestas y SIN el permiso, no corre", func(t *testing.T) {
		t.Setenv("MUSUBI_CENTRAL_URL", "https://ejemplo.invalido")
		t.Setenv("MUSUBI_TOKEN", "no-es-una-credencial-de-verdad")
		t.Setenv(permisoDeLaSonda, "")

		motivo, hay := laSondaTienePermiso()
		if hay {
			t.Fatal("la sonda se dio permiso a sí misma por tener credenciales en el entorno: ése es " +
				"el estado normal de la máquina del operador, no una autorización")
		}
		// El motivo tiene que NOMBRAR la variable: un skip que no dice cómo habilitarlo se
		// convierte, a la tercera vez que alguien lo lee, en una prueba que «nunca corre».
		if !strings.Contains(motivo, permisoDeLaSonda) {
			t.Errorf("el motivo del skip no dice qué variable hay que poner: %q", motivo)
		}
	})

	// LA OTRA DIRECCIÓN. Sin esto, la guarda la satisface una sonda que NUNCA corre — y una sonda
	// que no se puede encender es lo mismo que no tenerla.
	t.Run("con el permiso explícito, corre", func(t *testing.T) {
		t.Setenv(permisoDeLaSonda, "1")
		if _, hay := laSondaTienePermiso(); !hay {
			t.Fatal("con el permiso puesto la sonda sigue sin correr: no hay forma de encenderla")
		}
	})
}

// EL CORREDOR DE SABOTAJES NO LE PASA LAS CREDENCIALES A `go test`.
//
// Es el cinturón de lo de arriba: el arreglo durable vive en la prueba, pero el corredor es quien
// selecciona pruebas con un patrón ancho, y no tiene por qué llevar secretos adentro.
func TestElCorredorDeSabotajesNoLeHeredaLasCredencialesAGoTest(t *testing.T) {
	guion := leerDeploy(t, "pruebas", "sabotaje.sh")

	if !strings.Contains(guion, "sinCredenciales()") {
		t.Fatal("sabotaje.sh perdió `sinCredenciales`: vuelve a correr `go test` con el entorno de la " +
			"sesión, y ahí `MUSUBI_TOKEN` está puesta")
	}
	// LAS DOS INVOCACIONES, NO UNA. Es el defecto dominante de este repo —la cautela en un camino
	// y no en su hermano— y acá los hermanos son literalmente dos líneas seguidas: `corrida` es la
	// que corre CON el sabotaje puesto y `controlVerboso` la que corre sin él. Limpiar sólo una
	// deja la mitad de las corridas pegándole a producción.
	for _, fn := range []string{"corrida()", "controlVerboso()"} {
		i := strings.Index(guion, fn)
		if i < 0 {
			t.Fatalf("no encuentro `%s` en sabotaje.sh: ¿cambió de forma?", fn)
		}
		linea := guion[i:]
		if j := strings.IndexByte(linea, '\n'); j > 0 {
			linea = linea[:j]
		}
		if !strings.Contains(linea, "sinCredenciales") {
			t.Errorf("`%s` llama a `go test` SIN limpiar el entorno:\n    %s\n"+
				"  Su hermano sí lo limpia, y ésa es la forma exacta en que una cautela se queda a "+
				"medias en este repo.", fn, strings.TrimSpace(linea))
		}
	}
}

// EL CORREDOR NO ACEPTA HABER MEDIDO CERO.
//
// Contaba los subtests que pasaron, los imprimía, y seguía igual con CERO — el falso verde que
// esta herramienta existe para cazar, adentro de la herramienta. Un `--- SKIP` termina con `ok` y
// código 0, así que un control que se saltea entero pasaba por «compila y pasa»; después el
// sabotaje también se salteaba y se reportaba como «EL SABOTAJE NO LA PONE EN ROJO». La
// herramienta acusaba a la guarda de estar hueca cuando la verdad era que nadie midió nada.
func TestElCorredorDeSabotajesRechazaHaberMedidoCero(t *testing.T) {
	guion := leerDeploy(t, "pruebas", "sabotaje.sh")

	cuenta := strings.Index(guion, `N="$(grep -c`)
	if cuenta < 0 {
		t.Fatal("sabotaje.sh ya no cuenta los subtests que pasaron: sin ese número no hay forma de " +
			"distinguir un verde de mil pruebas de un verde de cero")
	}
	// LA COMPUERTA, Y QUE SEA UNA COMPUERTA. Que el número se lea no alcanza: tiene que cortar.
	// Es la misma propiedad posicional que custodia la comparación de inodos del redespliegue —
	// leer un valor y seguir igual es no haberlo leído.
	// EL CORTE VA POR CÓDIGO Y NO POR UN COMENTARIO, y esto lo aprendí acá mismo: la primera
	// versión cortaba en el título `# ── 2 · SABOTAJE`, que es una línea de comentario — y
	// `leerDeploy` las blanquea. El marcador no existía, el corte no pasaba nunca, y `cola` se
	// comía el guion entero: los `exit 1` de las secciones de abajo satisfacían la aserción, así
	// que sacarle el `exit 1` A ESTA compuerta salía VERDE. Una guarda delimitada por algo que el
	// filtro borra es una guarda sin delimitar.
	cola := guion[cuenta:]
	if k := strings.Index(cola, `bash -c "$SABOTAJE"`); k > 0 {
		cola = cola[:k]
	} else {
		t.Fatal("no encuentro dónde empieza la aplicación del sabotaje: sin ese corte esta guarda " +
			"mira el guion entero y la satisface cualquier `exit 1` de más abajo")
	}
	if !strings.Contains(cola, "-eq 0") {
		t.Error("el corredor cuenta los subtests y NO compara ese número contra cero: seguiría " +
			"adelante con un control que no ejecutó una sola prueba")
	}
	if !strings.Contains(cola, "exit 1") {
		t.Error("el corredor detecta el cero y no CORTA: informar y seguir deja el mismo desenlace " +
			"que no haberlo detectado, y encima con un diagnóstico que culpa a la guarda")
	}
}
