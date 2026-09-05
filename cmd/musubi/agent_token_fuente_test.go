package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// ═════════════════════════════════════════════════════════════════════════════════════════════
// A102 · EL AGENTE DICE DE DÓNDE SALIÓ SU TOKEN, PORQUE ES EL ÚNICO QUE LO SABE
//
// Con `MUSUBI_DEVICE_TOKEN_FILE` el agente puede REESCRIBIR el archivo cuando el cerebro le ofrece
// un token nuevo en la respuesta del latido. Con `MUSUBI_DEVICE_TOKEN` no: un proceso no reescribe
// su propio entorno, así que la rotación vence siempre — y desde el cerebro esa máquina late
// exactamente igual que una que sí puede rotar.
//
// Medido el 2026-09-05 en `davantis-1`: para descubrir que su lanzador usaba la forma vieja
// (`set /p MUSUBI_DEVICE_TOKEN=<archivo`) hubo que LEER UN .cmd EN LA MÁQUINA. El agente lo sabía
// desde siempre: su `ruta` está vacía justamente en ese caso.
//
// LA DECISIÓN NO SE REPITE. `Fuente()` se DERIVA de `ruta`, que es la prueba que dejó
// `cargarCredencial` al elegir el camino. Volver a mirar el entorno acá sería una segunda decisión
// sobre lo mismo, que es la forma de defecto que este repo pasó el día arreglando: dos lugares que
// deciden igual hasta que uno cambia.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL SABOTAJE QUE DECÍA ESTE COMENTARIO NO FUNCIONA, Y LA RAZÓN VALE MÁS QUE LA PRUEBA
//
// Afirmé acá que hacer `Fuente()` mire `os.Getenv(envTokenFile)` en vez de `c.ruta` pondría algo en
// rojo. Lo corrí y salió VERDE, y al buscar el caso que las separa no existe: dadas las reglas de
// `cargarCredencial`, una credencial NO-nil con `ruta` vacía sólo se produce cuando `_FILE` está
// vacío, y una `_FILE` puesta e ilegible devuelve ERROR y nil en vez de caer a la variable. O sea que
// hoy las dos implementaciones son OBSERVACIONALMENTE EQUIVALENTES: no hay entrada que las distinga.
//
// Derivar de `ruta` sigue siendo lo correcto, pero por una razón más chica y honesta que la que
// escribí: no evita un defecto ACTUAL, evita que este lugar tenga que ACORDARSE de cambiar si algún
// día `cargarCredencial` acepta un fallback —por ejemplo, `_FILE` ilegible cayendo a la variable—.
// Ahí sí se separarían, y con `ruta` esto seguiría diciendo la verdad sin que nadie lo toque.
//
// Queda escrito en vez de borrado porque es la sexta vez en el día que un sabotaje que yo declaré no
// funciona, y todas por lo mismo: describo la consecuencia que ESPERO en vez de la que corrí.
func TestElAgenteDiceDeDondeSalioSuToken(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "device.token")
	if err := os.WriteFile(ruta, []byte("tok-de-archivo\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("por archivo", func(t *testing.T) {
		t.Setenv(envTokenFile, ruta)
		t.Setenv(envToken, "")
		c, err := cargarCredencial()
		if err != nil || c == nil {
			t.Fatalf("cargarCredencial: %v", err)
		}
		if got := c.Fuente(); got != fleet.CredencialDeArchivo {
			t.Errorf("Fuente() = %q, esperaba %q", got, fleet.CredencialDeArchivo)
		}
	})

	t.Run("por variable", func(t *testing.T) {
		t.Setenv(envTokenFile, "")
		t.Setenv(envToken, "tok-de-variable")
		c, err := cargarCredencial()
		if err != nil || c == nil {
			t.Fatalf("cargarCredencial: %v", err)
		}
		if got := c.Fuente(); got != fleet.CredencialDeVariable {
			t.Errorf("Fuente() = %q, esperaba %q — y con eso el cerebro no puede saber que esta "+
				"máquina no completa una rotación", got, fleet.CredencialDeVariable)
		}
	})

	// SIN CREDENCIAL NO SE INVENTA UNA FUENTE. Un agente sin token no late; decir «variable» acá
	// sería afirmar algo sobre una máquina que no reportó nada.
	t.Run("sin credencial", func(t *testing.T) {
		t.Setenv(envTokenFile, "")
		t.Setenv(envToken, "")
		c, _ := cargarCredencial()
		if c != nil {
			t.Fatalf("sin ninguna de las dos variables no tendría que haber credencial: %+v", c)
		}
		if got := c.Fuente(); got != "" {
			t.Errorf("Fuente() de una credencial nil = %q, esperaba vacío", got)
		}
	})
}

// Y QUE VIAJE EN EL LATIDO, que es lo único que le sirve al cerebro.
//
// Una `Fuente()` correcta que no llega al cuerpo del POST deja la pregunta igual de incontestable
// que antes — es el mismo defecto de A99, un dato que el agente sabe y que nadie del otro lado ve.
// Y el caso VACÍO importa igual: el campo se OMITE en vez de mandarse en blanco, para que un agente
// que no lo sabe se distinga de uno viejo que no lo manda… y para que el cerebro trate los dos igual.
//
// Sabotaje: sacar el `if fuenteDelToken != ""` y mandarlo siempre → el subtest del vacío se pone rojo
// porque el campo aparece.
func TestElLatidoLlevaLaFuenteDelToken(t *testing.T) {
	for _, c := range []struct {
		nombre string
		fuente string
		espera bool
	}{
		{"archivo", fleet.CredencialDeArchivo, true},
		{"variable", fleet.CredencialDeVariable, true},
		{"vacía: se omite", "", false},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			var visto map[string]any
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &visto)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer ts.Close()

			if res := latir(ts.URL, "tok", c.fuente, nil); !res.ok {
				t.Fatalf("el latido falló")
			}
			got, hay := visto["token_fuente"]
			if hay != c.espera {
				t.Fatalf("token_fuente presente = %v, esperaba %v (cuerpo: %v)", hay, c.espera, visto)
			}
			if c.espera && got != c.fuente {
				t.Errorf("token_fuente = %v, esperaba %q", got, c.fuente)
			}
		})
	}
}

// EL CUERPO SIGUE SIN LLEVAR IDENTIDAD, y este campo no es una excepción.
//
// El invariante B4/D5 dice que el dispositivo NO puede decir quién es: quién es lo decide el token,
// del lado del cerebro. `token_fuente` habla de CÓMO llegó la credencial, no de a quién pertenece —
// pero conviene custodiarlo, porque un campo nuevo en el latido es exactamente por donde se cuela
// una identidad autodeclarada.
func TestLaFuenteDelTokenNoEsUnCampoDeIdentidad(t *testing.T) {
	var crudo string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		crudo = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	if res := latir(ts.URL, "tok-secreto", fleet.CredencialDeArchivo, nil); !res.ok {
		t.Fatal("el latido falló")
	}
	// Ni el token, ni una ruta, ni un nombre de máquina: la fuente es una PALABRA del vocabulario
	// compartido y nada más. Mandar la ruta diría dónde vive la credencial de esa máquina.
	for _, prohibido := range []string{"tok-secreto", "device.token", "AppData", "/home/", "C:\\"} {
		if strings.Contains(crudo, prohibido) {
			t.Errorf("el cuerpo del latido lleva %q, que no le corresponde:\n%s", prohibido, crudo)
		}
	}
}

// UN `_FILE` ILEGIBLE FALLA CERRADO: no cae a la variable en silencio.
//
// Esta prueba nació buscando el caso que separa «derivar de `ruta`» de «mirar el entorno», y NO lo
// es: con `_FILE` ilegible `cargarCredencial` devuelve error y nil, así que `Fuente()` contesta vacío
// por su guarda de nil antes de llegar a cualquiera de las dos ramas. Se queda porque lo que SÍ
// custodia es valioso y no estaba escrito en ningún lado: que una ruta puesta y rota sea un ERROR y
// no un fallback silencioso a la variable.
//
// Si cayera a la variable, el arco entero de A102 se volvería inútil: la máquina reportaría
// «variable» —correcto— pero nadie sabría que hay una `_FILE` configurada y rota, que es un defecto
// distinto y arreglable. Y peor: la rotación seguiría venciendo con un archivo presente en el disco,
// que es el escenario donde uno mira el archivo y concluye que está todo bien.
func TestUnArchivoDeTokenIlegibleFallaCerradoYNoCaeALaVariable(t *testing.T) {
	t.Setenv(envTokenFile, filepath.Join(t.TempDir(), "no-existe.token"))
	t.Setenv(envToken, "tok-de-variable")
	c, err := cargarCredencial()
	if err == nil {
		t.Fatalf("un _FILE ilegible tiene que dar error y no caer a la variable en silencio: %+v", c)
	}
	if c != nil {
		t.Fatalf("con error no tendría que haber credencial: %+v", c)
	}
	if got := c.Fuente(); got != "" {
		t.Errorf("Fuente() = %q sobre una credencial que no se pudo abrir: el valor sale de lo que se "+
			"abrió, no de lo que el entorno declara", got)
	}
}
