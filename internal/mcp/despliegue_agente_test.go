package mcp

// despliegue_agente_test.go custodia el drop-in que le abre al agente lo que necesita para
// enumerar contenedores (`deploy/systemd/musubi-agente-contenedores.conf`).
//
// Es un archivo de configuración de tres líneas útiles, y las tres tienen un modo de fallo que
// no se ve el día que se escribe:
//
//   1. Las rutas de /run llevan `-`. /run/user/1000 lo crea logind, y este unit puede arrancar
//      antes en un reboot. Un ReadWritePaths sobre una ruta ausente NO degrada: hace fallar el
//      arranque de la unidad entera. Sin el `-`, el agente deja de existir después del próximo
//      corte de luz, meses después de que alguien escribiera esta línea.
//   2. La ruta del home NO lleva `-`. Ésa sí tiene que estar: si el store de contenedores no
//      existe, el silencio es la respuesta equivocada.
//   3. Está el store del home. Es donde `podman ps` abre db.sql y sus locks en modo escritura,
//      y sin ella el drop-in existe, el unit arranca, y el inventario sigue sin salir — que es
//      exactamente el estado que este archivo vino a terminar.
//
// Lo que esta prueba NO hace es afirmar que el drop-in alcanza. Eso se mide en la máquina, con
// strace, y quedó anotado en el propio archivo.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func leerDropInDelAgente(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../deploy/systemd/musubi-agente-contenedores.conf")
	if err != nil {
		t.Fatalf("falta el drop-in del agente: sin él el inventario de contenedores no sale: %v", err)
	}
	return string(b)
}

var reRutaEscribible = regexp.MustCompile(`(?m)^ReadWritePaths=(-?)(\S+)`)

// LAS RUTAS DE /run SON OPCIONALES Y LA DEL HOME NO, Y LA ASIMETRÍA ES EL PUNTO.
//
// Sabotaje que la hace fallar: sacarle el `-` a `-/run/user/1000/libpod`, o ponérselo a la ruta
// del home.
func TestElDropInDelAgenteNoPuedeImpedirQueElAgenteArranque(t *testing.T) {
	conf := leerDropInDelAgente(t)
	rutas := reRutaEscribible.FindAllStringSubmatch(conf, -1)
	if len(rutas) == 0 {
		t.Fatal("el drop-in no concede ninguna ruta escribible: no sirve para nada")
	}
	vistas := map[string]bool{}
	for _, m := range rutas {
		opcional, ruta := m[1] == "-", m[2]
		vistas[ruta] = true
		switch {
		case strings.HasPrefix(ruta, "/run/") && !opcional:
			t.Errorf("%s no lleva `-`: logind puede no haberla creado todavía en un arranque, y "+
				"un ReadWritePaths sobre una ruta ausente NO degrada, hace fallar la unidad entera", ruta)
		case !strings.HasPrefix(ruta, "/run/") && opcional:
			t.Errorf("%s lleva `-`: si el store de contenedores no está, callarse es la respuesta "+
				"equivocada — el inventario quedaría mudo sin que nada lo diga", ruta)
		}
	}
	// La del home es la que de verdad desbloquea `podman ps`: ahí están db.sql y los locks. Un
	// drop-in con las dos de /run y sin ésta arranca, no falla, y no arregla nada.
	if !vistas["/home/musubi/.local/share/containers"] {
		t.Errorf("falta el store de contenedores del home: %v", vistas)
	}
}

// EL DROP-IN NO PUEDE DESHACER EL BLINDAJE QUE VINO A ESQUIVAR.
//
// La forma cómoda de que `podman ps` ande es borrar ProtectHome o ProtectSystem, o apagar
// NoNewPrivileges. Cualquiera de las tres funciona, ninguna se nota, y las tres cambian el
// agente de «lee /proc» a «puede lo que pueda el usuario». La excepción tiene que seguir siendo
// una excepción enumerada, no una puerta abierta.
//
// Sabotaje que la hace fallar: agregar `ProtectHome=no` al drop-in.
func TestElDropInAbreRutasYNoApagaElBlindaje(t *testing.T) {
	conf := leerDropInDelAgente(t)
	sinComentarios := []string{}
	for _, l := range strings.Split(conf, "\n") {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "#") {
			sinComentarios = append(sinComentarios, t)
		}
	}
	cuerpo := strings.Join(sinComentarios, "\n")
	for _, prohibida := range []string{"ProtectHome", "ProtectSystem", "NoNewPrivileges", "PrivateTmp"} {
		if strings.Contains(cuerpo, prohibida+"=") {
			t.Errorf("el drop-in toca %s: eso no es abrir una ruta, es apagar el blindaje. "+
				"Si de verdad hace falta, va en la unidad base y con su motivo escrito", prohibida)
		}
	}
}
