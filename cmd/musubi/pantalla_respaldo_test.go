package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// configFalsa deja un RustDesk.toml de mentira y hace que el agente lo tome como el único
// candidato. Devuelve su ruta.
func configFalsa(t *testing.T, contenido string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "RustDesk.toml")
	if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envConfigRustdesk, ruta)
	// El envRespaldoPantalla lo pone rustdeskFalso, que lo usan todas las pruebas de este plano.
	return ruta
}

// cuerpoQuePisaLaConfig es el doble de `rustdesk --password`: reescribe la config como lo hace el
// binario real, que es lo que después permite MEDIR cuál archivo cambió.
func cuerpoQuePisaLaConfig(ruta string) string {
	return "printf \"enc_id = 'abc'\\npassword = 'DE-SESION'\\nsalt = 'sal'\\n\" > " + ruta + "\nexit 0\n"
}

// EL CASO QUE MOTIVÓ TODO: al cerrar, la contraseña del dueño de la máquina vuelve.
func TestAlCerrarVuelveLaContrasenaAnterior(t *testing.T) {
	ruta := configFalsa(t, "enc_id = 'abc'\npassword = 'LA-DEL-DUENO'\nsalt = 'sal'\n")
	rustdeskFalso(t, cuerpoQuePisaLaConfig(ruta))

	res := aplicarSesionPantalla(comandoRecibido{
		ID: "cmd-1", Argv: []string{"musubi:pantalla", "ses-1", "LaClaveDeSesion", "30m"}, TimeoutSeg: 30,
	})
	if res.Error != "" {
		t.Fatalf("no se pudo aplicar: %s", res.Error)
	}
	if got := leerArchivo(t, ruta); !strings.Contains(got, "DE-SESION") {
		t.Fatalf("el doble no pisó la config, la prueba no probaría nada: %s", got)
	}

	cerrarSesionPantalla("prueba")

	got := leerArchivo(t, ruta)
	if !strings.Contains(got, "password = 'LA-DEL-DUENO'") {
		t.Errorf("no volvió la contraseña del dueño:\n%s", got)
	}
	if strings.Contains(got, "DE-SESION") {
		t.Errorf("quedó puesta la contraseña de sesión:\n%s", got)
	}
	if _, hay := leerRespaldo(); hay {
		t.Error("la marca de sesión abierta quedó en disco después de cerrar")
	}
}

// LA REGRESIÓN CARA: arrancar el agente sin ninguna sesión abierta NO puede tocar la contraseña.
//
// Antes, agent.go forzaba `marcarSesionAbierta(true)` justo antes de cerrar «por las dudas», y el
// cierre de entonces scrambleaba la contraseña permanente. Cada reinicio del agente destruía la
// del dueño. Esta prueba es la que no deja que vuelva.
func TestElArranqueSinSesionAbiertaNoTocaNada(t *testing.T) {
	ruta := configFalsa(t, "enc_id = 'abc'\npassword = 'LA-DEL-DUENO'\nsalt = 'sal'\n")
	reg := rustdeskFalso(t, cuerpoQuePisaLaConfig(ruta))
	marcarSesionAbierta(false)

	cerrarSesionColgadaDeArranque()

	if llamadas := leerRegistro(t, reg); llamadas != "" {
		t.Errorf("se invocó a RustDesk sin haber ninguna sesión abierta: %q", llamadas)
	}
	if got := leerArchivo(t, ruta); !strings.Contains(got, "password = 'LA-DEL-DUENO'") {
		t.Errorf("el arranque cambió la contraseña del dueño:\n%s", got)
	}
}

// La marca en disco es la que sobrevive a la muerte del agente: un proceso NUEVO, sin nada en
// memoria, tiene que poder devolver lo que dejó puesto su encarnación anterior.
func TestUnAgenteQueVuelveRestituyeDesdeLaMarca(t *testing.T) {
	ruta := configFalsa(t, "enc_id = 'abc'\npassword = 'DE-SESION'\nsalt = 'sal'\n")
	rustdeskFalso(t, "exit 0")
	marcarSesionAbierta(false) // proceso nuevo: no sabe de ninguna sesión

	if err := guardarRespaldo(respaldoPantalla{
		Sesion: "ses-vieja", Vence: time.Now().Add(-time.Hour),
		Previas: []passPrevia{{Ruta: ruta, Linea: "password = 'LA-DEL-DUENO'", Habia: true}},
	}); err != nil {
		t.Fatal(err)
	}

	cerrarSesionColgadaDeArranque()

	if got := leerArchivo(t, ruta); !strings.Contains(got, "password = 'LA-DEL-DUENO'") {
		t.Errorf("no se restituyó desde la marca en disco:\n%s", got)
	}
}

// Cuando NO había contraseña previa no hay nada que devolver, y ahí sí la puerta se cierra con un
// valor al azar: dejar a RustDesk sin contraseña sería abrir la máquina, no cerrarla.
func TestSinContrasenaPreviaSeSigueScrambleando(t *testing.T) {
	ruta := configFalsa(t, "enc_id = 'abc'\nsalt = 'sal'\n")
	reg := rustdeskFalso(t, cuerpoQuePisaLaConfig(ruta))

	res := aplicarSesionPantalla(comandoRecibido{
		ID: "cmd-1", Argv: []string{"musubi:pantalla", "ses-1", "LaClaveDeSesion", "30m"}, TimeoutSeg: 30,
	})
	if res.Error != "" {
		t.Fatalf("no se pudo aplicar: %s", res.Error)
	}
	cerrarSesionPantalla("prueba")

	llamadas := strings.Count(leerRegistro(t, reg), "--password")
	if llamadas != 2 {
		t.Errorf("se esperaban dos --password (aplicar y scramblear), hubo %d", llamadas)
	}
}

// El blob cifrado es texto arbitrario y puede traer `$1`, `$0` o `\1`. Si la restitución pasara
// por la expansión de plantilla de regexp, esos bytes se leerían como referencias de grupo y la
// contraseña volvería CORRUPTA — un modo de falla que sólo aparece con ciertos valores.
func TestElBlobVuelveLiteralAunqueTengaDolarUno(t *testing.T) {
	crudo := "password = 'AA$1BB${2}CC\\1DD'"
	ruta := configFalsa(t, "enc_id = 'abc'\n"+crudo+"\nsalt = 'sal'\n")
	rustdeskFalso(t, cuerpoQuePisaLaConfig(ruta))

	res := aplicarSesionPantalla(comandoRecibido{
		ID: "cmd-1", Argv: []string{"musubi:pantalla", "ses-1", "LaClaveDeSesion", "30m"}, TimeoutSeg: 30,
	})
	if res.Error != "" {
		t.Fatalf("no se pudo aplicar: %s", res.Error)
	}
	cerrarSesionPantalla("prueba")

	if got := leerArchivo(t, ruta); !strings.Contains(got, crudo) {
		t.Errorf("el blob no volvió literal:\nquería: %s\ntengo:\n%s", crudo, got)
	}
}

// Un archivo que no cambió no se recuerda ni se toca: restituirle un valor idéntico sería
// reescribir sin motivo la configuración de otro programa.
func TestNoSeRecuerdaLoQueNoCambio(t *testing.T) {
	ruta := configFalsa(t, "enc_id = 'abc'\npassword = 'IGUAL'\nsalt = 'sal'\n")
	rustdeskFalso(t, "exit 0") // el doble NO toca la config

	antes := fotoDeLasConfigs()
	if got := loQueCambio(antes); len(got) != 0 {
		t.Errorf("se registró un cambio que no ocurrió: %+v", got)
	}
	_ = ruta
}

func leerArchivo(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Una marca ILEGIBLE no es una marca ausente: el archivo está, así que alguien lo escribió y lo
// más probable es que haya quedado una sesión puesta. El arranque tiene que cerrar igual, aunque
// no sepa qué restituir — dejar la contraseña de sesión viva para siempre es peor.
func TestUnaMarcaIlegibleSeTrataComoSesionAbierta(t *testing.T) {
	ruta := configFalsa(t, "enc_id = 'abc'\npassword = 'DE-SESION'\nsalt = 'sal'\n")
	reg := rustdeskFalso(t, "exit 0")
	marcarSesionAbierta(false)

	if err := os.WriteFile(rutaRespaldoPantalla(), []byte("{esto no es json"), 0o600); err != nil {
		t.Fatal(err)
	}

	cerrarSesionColgadaDeArranque()

	if !strings.Contains(leerRegistro(t, reg), "--password") {
		t.Error("con la marca ilegible no se cerró la sesión: la contraseña de sesión queda viva para siempre")
	}
	_ = ruta
}

// Y el contraste que le da sentido: SIN archivo, el arranque no toca nada. Aplicarle el mismo
// sesgo al caso ausente es el defecto original —cada reinicio destruía la contraseña del dueño—.
func TestUnaMarcaAusenteNoEsUnaSesionAbierta(t *testing.T) {
	configFalsa(t, "enc_id = 'abc'\npassword = 'LA-DEL-DUENO'\nsalt = 'sal'\n")
	reg := rustdeskFalso(t, "exit 0")
	marcarSesionAbierta(false)
	_ = os.Remove(rutaRespaldoPantalla())

	cerrarSesionColgadaDeArranque()

	if llamadas := leerRegistro(t, reg); llamadas != "" {
		t.Errorf("sin marca no se puede tocar nada, y se invocó a RustDesk: %q", llamadas)
	}
}

// La marca que NO SE PUEDE LEER, que es una rama distinta de la que tiene el JSON roto.
//
// ESTA PRUEBA EXISTE PORQUE LA DE AL LADO NO ALCANZABA. Un archivo con basura adentro se LEE
// perfecto y recién falla al parsear, así que ejercía el `Unmarshal` y dejaba el error de
// os.ReadFile sin cubrir: sabotear esa rama daba verde. Un directorio en la ruta sí hace fallar
// a ReadFile con un error que no es IsNotExist, que es exactamente el caso «está y no lo puedo
// mirar».
func TestUnaMarcaQueNoSePuedeLeerSeTrataComoSesionAbierta(t *testing.T) {
	configFalsa(t, "enc_id = 'abc'\npassword = 'DE-SESION'\nsalt = 'sal'\n")
	reg := rustdeskFalso(t, "exit 0")
	marcarSesionAbierta(false)

	// Un directorio donde va el archivo: existe, y leerlo falla.
	_ = os.Remove(rutaRespaldoPantalla())
	if err := os.MkdirAll(rutaRespaldoPantalla(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, hay := leerRespaldo(); !hay {
		t.Fatal("la marca ilegible se leyó como ausente: la prueba no probaría lo que dice")
	}

	cerrarSesionColgadaDeArranque()

	if !strings.Contains(leerRegistro(t, reg), "--password") {
		t.Error("con la marca ilegible no se cerró la sesión: la contraseña de sesión queda viva para siempre")
	}
}
