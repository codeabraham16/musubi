package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"musubi/internal/buildid"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// DOS AGENTES LATIENDO SOBRE UNA FILA ERAN INDISTINGUIBLES DE UNO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// La credencial del latido es de la MÁQUINA y no del proceso, así que dos agentes corriendo a la
// vez —un servicio más una corrida a mano, una instalación duplicada, un zombi del binario
// renombrado— escriben los dos sobre la misma fila. El cerebro ve un único agente sano latiendo
// el doble de seguido.
//
// ASÍ SE CERRÓ A92 CON EL PROBLEMA TODAVÍA PUESTO. El diagnóstico se hizo midiendo la CADENCIA
// —un diente de sierra de 37,8 s contra 25,8 s del vecino—, o sea una inferencia estadística
// sobre un efecto de segundo orden. Eso sólo se puede hacer mirando a mano y sabiendo de antemano
// que hay que mirar, que es la definición de un eje que no existe.
//
// LA MARCA SÓLO SE MUEVE CUANDO EL EMISOR CAMBIA, y ahí está todo. Con un agente envejece; con
// dos alternándose vuelve a cero en cada latido y se queda pegada al cero para siempre.
// ────────────────────────────────────────────────────────────────────────────────────────────

func latirComoEmisor(t *testing.T, ts, token, emisor string) {
	t.Helper()
	cuerpo, err := json.Marshal(fleet.CuerpoLatido{
		Version: "0.140.3-prueba", Capver: buildid.Capver, Emisor: emisor,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code, body := postCon(t, ts+fleetHeartbeatPath, token, string(cuerpo)); code != http.StatusOK {
		t.Fatalf("latido de %q: %d %s", emisor, code, body)
	}
}

func TestLaMarcaDelEmisorSoloSeMueveCuandoCambiaElProceso(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc")

	d, _, _ := s.engine.DevicePorNombre("casa", "pc")

	t.Run("un solo agente: la marca se pone una vez y no se toca más", func(t *testing.T) {
		latirComoEmisor(t, ts.URL, tok, "aaaa1111")
		primera, _, _ := s.engine.DevicePorNombre("casa", "pc")
		if primera.Emisor != "aaaa1111" {
			t.Fatalf("el emisor no se guardó: %+v", primera.Emisor)
		}
		if primera.EmisorDesde.IsZero() {
			t.Fatal("se guardó el emisor sin su marca de cuándo empezó: sin ella no hay serie que exportar")
		}
		// Tres latidos más del MISMO proceso no pueden mover la marca. Si la movieran, un agente
		// solo se vería exactamente igual que dos peleándose — el defecto, producido por la
		// detección misma.
		//
		// LA ESPERA NO ES DECORATIVA. La marca se guarda en RFC3339, o sea con resolución de
		// SEGUNDO: sin esperar, una reescritura escribe el mismo texto y la prueba no puede verla.
		// Medido — el sabotaje «sacar el WHERE que evita pisar la marca» pasaba en VERDE sin esto.
		time.Sleep(1100 * time.Millisecond)
		for i := 0; i < 3; i++ {
			latirComoEmisor(t, ts.URL, tok, "aaaa1111")
		}
		despues, _, _ := s.engine.DevicePorNombre("casa", "pc")
		if !despues.EmisorDesde.Equal(primera.EmisorDesde) {
			t.Errorf("la marca se movió con un latido del MISMO emisor (%v → %v): así la serie "+
				"marcaría cero siempre y un agente solo se vería como dos",
				primera.EmisorDesde, despues.EmisorDesde)
		}
	})

	t.Run("dos agentes alternándose: la marca vuelve a cero en cada latido", func(t *testing.T) {
		latirComoEmisor(t, ts.URL, tok, "bbbb2222")
		uno, _, _ := s.engine.DevicePorNombre("casa", "pc")
		time.Sleep(1100 * time.Millisecond) // la marca tiene resolución de segundo (RFC3339)
		latirComoEmisor(t, ts.URL, tok, "cccc3333")
		dos, _, _ := s.engine.DevicePorNombre("casa", "pc")
		if !dos.EmisorDesde.After(uno.EmisorDesde) {
			t.Errorf("con DOS emisores distintos la marca no se movió (%v → %v): son exactamente el "+
				"caso que esta pieza existe para detectar", uno.EmisorDesde, dos.EmisorDesde)
		}
	})

	_ = d
}

// LA SERIE SE OMITE CUANDO EL AGENTE NO DECLARA EMISOR, y no vale cero.
//
// Un 0 sería INDISTINGUIBLE de «dos agentes peleándose», que es lo contrario de lo que pasa: un
// binario anterior a capver 3 simplemente no dice nada, y nadie afirmó nada sobre él.
func TestLaSerieDelEmisorSeOmiteSiElAgenteNoLoDeclara(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "vieja")

	// Un agente viejo: late sin emisor.
	cuerpo, _ := json.Marshal(fleet.CuerpoLatido{Version: "0.100.0", Capver: 2})
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tok, string(cuerpo)); code != http.StatusOK {
		t.Fatalf("latido: %d %s", code, body)
	}

	var b strings.Builder
	renderFlota(&b, s.engine, nil, time.Now(), s.sondaIntervalo, "0.140.3", nil, serviciosPorProyectoDefault)
	for _, l := range strings.Split(b.String(), "\n") {
		if strings.HasPrefix(l, "musubi_fleet_device_emitter_stable_seconds{") {
			t.Errorf("se exportó la serie del emisor para un agente que no lo declara: %s\n"+
				"  Un valor ahí es indistinguible de «dos agentes peleándose», que es lo contrario "+
				"de lo que pasa — nadie afirmó nada sobre esa máquina.", l)
		}
	}
}

// Y LA ALERTA QUE LA LEE.
func TestLaAlertaDeDosAgentesExiste(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts-flota.yml")), " ")
	if !strings.Contains(reglas, "musubi_fleet_device_emitter_stable_seconds <") {
		t.Error("ninguna alerta lee `musubi_fleet_device_emitter_stable_seconds`: la serie diría que " +
			"hay dos agentes peleándose por una máquina y nadie estaría escuchando")
	}
}
