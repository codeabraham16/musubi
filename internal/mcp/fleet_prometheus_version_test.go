package mcp

// Guardas de `musubi_fleet_device_agent_stale` (A68).
//
// El cabo nació de un caso concreto: el 2026-09-01 el cerebro corría 0.130.0 y los dos Windows
// `v0.106.0-28-gdf2ec21`, veinticuatro versiones atrás. Se descubrió DE CASUALIDAD, mirando otra
// cosa. El costo fue que A67 se desplegó y no podía correr en las dos máquinas para las que se
// escribió, porque su binario no tenía la capacidad — y nada lo habría dicho.

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// latirConVersion hace latir a una máquina declarando la versión de su agente.
func latirConVersion(t *testing.T, ts string, token, version string) {
	t.Helper()
	m := &fleet.Muestra{Tomada: time.Now().UTC(), NumCPU: 4, MemTotal: 100, MemUsada: 25,
		DiscoTotal: 1000, DiscoUsado: 100, DiscoDisponible: 850}
	txt, err := m.Serializar()
	if err != nil {
		t.Fatal(err)
	}
	cuerpo := `{"version":"` + version + `","muestra":` + txt + `}`
	if code, body := postCon(t, ts+fleetHeartbeatPath, token, cuerpo); code != http.StatusOK {
		t.Fatalf("el latido de %q falló: %d %s", version, code, body)
	}
}

// serieDe devuelve el valor de una serie para una máquina, y si la línea existe.
func serieDe(salida, metrica, device string) (string, bool) {
	for _, l := range strings.Split(salida, "\n") {
		if strings.HasPrefix(l, metrica+"{") && strings.Contains(l, `device="`+device+`"`) {
			if i := strings.LastIndex(l, " "); i > 0 {
				return l[i+1:], true
			}
		}
	}
	return "", false
}

// LOS TRES ESTADOS QUE LA SERIE TIENE QUE DISTINGUIR, EN UNA SOLA FLOTA.
//
// El del medio es el que decide si la métrica sirve o es ruido: dos binarios del MISMO release
// construidos de commits distintos son lo normal —el de Windows se cruza a mano, el del cerebro se
// redespliega varias veces por día— y marcarlos atrasados dejaría la alarma encendida siempre.
//
// Sabotaje: comparar `d.AgentVer != versionCerebro` en vez del núcleo → la máquina al día pasa a 1.
func TestElAgenteAtrasadoSeDistingueDelQueSoloCambioDeCommit(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)

	alDia := enrolarDePrueba(t, s, "casa", "al-dia")
	atrasada := enrolarDePrueba(t, s, "casa", "atrasada")
	latirConVersion(t, ts.URL, alDia, "0.130.0-flota.e140e0c")   // otro commit del mismo release
	latirConVersion(t, ts.URL, atrasada, "v0.106.0-28-gdf2ec21") // el caso que abrió A68

	out := exportar(t, s, nil)

	if v, hay := serieDe(out, "musubi_fleet_device_agent_stale", "al-dia"); !hay || v != "0" {
		t.Errorf("la máquina del mismo release da agent_stale=%q (hay=%v): dos commits del mismo "+
			"release no son un agente atrasado, y marcarlos deja la alarma encendida en cada "+
			"redespliegue del cerebro\n%s", v, hay, out)
	}
	if v, hay := serieDe(out, "musubi_fleet_device_agent_stale", "atrasada"); !hay || v != "1" {
		t.Errorf("la máquina veinticuatro versiones atrás da agent_stale=%q (hay=%v): es el caso "+
			"exacto que abrió el cabo\n%s", v, hay, out)
	}
}

// UNA MÁQUINA SIN AGENTE NO TIENE VERSIÓN QUE COMPARAR, Y AUSENTE NO ES CERO.
//
// Un Tier B sondeado por SSH no corre nuestro binario. Emitir 0 diría «está al día», que es una
// afirmación que nadie midió — la misma regla que gobierna el exportador entero desde S4.
//
// Sabotaje: devolver (0, true) cuando AgentVer está vacío → la línea aparece y falla acá.
func TestUnaMaquinaSinAgenteNoDiceQueEstaAlDia(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)

	conAgente := enrolarDePrueba(t, s, "casa", "con-agente")
	enrolarDePrueba(t, s, "casa", "sin-agente") // enrolada y nunca latió: no reportó versión
	latirConVersion(t, ts.URL, conAgente, "0.130.0-flota.38a0a9f")

	out := exportar(t, s, nil)

	if _, hay := serieDe(out, "musubi_fleet_device_agent_stale", "sin-agente"); hay {
		t.Errorf("una máquina que nunca reportó versión exporta agent_stale: un 0 ahí se lee «está "+
			"al día» y nadie lo midió\n%s", out)
	}
	if _, hay := serieDe(out, "musubi_fleet_device_agent_stale", "con-agente"); !hay {
		t.Errorf("la máquina que SÍ reportó versión no exporta la serie\n%s", out)
	}
}

// SI EL CEREBRO NO SABE SU PROPIA VERSIÓN, NO SE MARCA A NADIE.
//
// Un binario construido sin `-ldflags -X main.version` queda en `dev`. Comparar contra eso pondría
// a la flota ENTERA en 1 por un problema del build propio, y una alarma que acusa a las máquinas
// equivocadas manda a alguien a revisar diez agentes sanos.
//
// Sabotaje: emitir 1 cuando el núcleo del cerebro no parsea → la línea aparece y falla acá.
func TestConElCerebroSinVersionNoSeMarcaAtrasadaANadie(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc")
	latirConVersion(t, ts.URL, tok, "0.130.0-flota.e140e0c")

	var b strings.Builder
	renderFlota(&b, s.engine, nil, time.Now(), s.sondaIntervalo, "dev")
	if strings.Contains(b.String(), "musubi_fleet_device_agent_stale") {
		t.Errorf("con el cerebro en `dev` se exporta agent_stale igual: la flota entera quedaría "+
			"marcada por un binario propio construido sin ldflags\n%s", b.String())
	}
}

// LA VERSIÓN NO PUEDE VIAJAR COMO ETIQUETA, Y ESTA ES LA GUARDA DE ESA DECISIÓN.
//
// Está escrita en el comentario de labelsDeFlota desde antes de A68: una etiqueta con la versión
// deja la serie RE-ETIQUETÁNDOSE SOLA en cada actualización, y las series viejas quedan huérfanas
// —con historia que ya nadie consulta y alertas que filtran por un valor que dejó de existir—.
// Por eso `agent_stale` es un booleano y cuál versión corre cada máquina se mira en
// `musubi_fleet_list`.
//
// Sabotaje: agregar {"agent_version", d.AgentVer} a labelsDeFlota → falla acá.
func TestLaVersionDelAgenteNoEntraComoEtiqueta(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc")
	latirConVersion(t, ts.URL, tok, "0.106.0-flota.deadbee")

	out := exportar(t, s, nil)
	for _, prohibido := range []string{"0.106.0", "agent_version", versionDePrueba} {
		if strings.Contains(out, prohibido) {
			t.Errorf("la salida de /metrics contiene %q: si la versión entra como etiqueta, la serie "+
				"se re-etiqueta sola en cada actualización y las viejas quedan huérfanas\n%s",
				prohibido, out)
		}
	}
}

// LA VERSIÓN LLEGA POR UNA OPTION, Y UNA OPTION SE OLVIDA EN SILENCIO.
//
// `internal/mcp` no puede leer la variable que el build inyecta en `main`, así que la referencia
// entra por `WithVersion`. Si un servidor nuevo —o uno viejo al que le tocan la lista de options—
// no la pasa, no falla nada: `agent_stale` simplemente DEJA DE EXISTIR para toda la flota, que se
// ve exactamente igual que «no hay ningún agente atrasado». Es el modo de fallo que este track
// persigue desde S4, y acá volvería a entrar por la puerta de atrás.
//
// Sabotaje: sacar `mcp.WithVersion(version)` de cualquiera de las dos construcciones → falla acá.
func TestTodoServidorQueSeSirveDeclaraSuVersion(t *testing.T) {
	crudo, err := os.ReadFile("../../cmd/musubi/main.go")
	if err != nil {
		t.Fatalf("no se pudo leer cmd/musubi/main.go: %v", err)
	}
	construcciones := 0
	for _, l := range strings.Split(string(crudo), "\n") {
		if !strings.Contains(l, "mcp.NewMcpServer(") {
			continue
		}
		construcciones++
		if !strings.Contains(l, "mcp.WithVersion(") {
			t.Errorf("una construcción del servidor no declara su versión:\n  %s\n"+
				"Sin ella `musubi_fleet_device_agent_stale` no se emite para NINGUNA máquina, y una "+
				"serie ausente se lee igual que «no hay ningún agente atrasado».", strings.TrimSpace(l))
		}
	}
	if construcciones == 0 {
		t.Fatal("no se encontró ninguna construcción de McpServer en cmd/musubi/main.go: la guarda " +
			"quedó mirando un archivo que cambió de forma, y una guarda que no encuentra nada pasa siempre")
	}
}
