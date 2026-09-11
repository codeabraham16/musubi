package mcp

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// UNA ROTACIÓN DE TOKEN SE ABRÍA EN LA BASE Y MORÍA EN SILENCIO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// El estado estaba guardado desde siempre —`token_sha256_nuevo` y `rotacion_vence`— y NO LO
// MIRABA NADIE: ni `musubi doctor`, ni el panel, ni una alerta. El barrido que las abandona
// devuelve un número que se escribía en un log y nada más.
//
// El modo de falla es el de siempre en este track: una operación que no terminó se ve igual que
// una que nunca empezó.
//
// ABANDONARLA NO ROMPE NADA, y eso está decidido y escrito: la rotación es HIGIENE y no
// emergencia, así que el agente se queda con el token viejo —el estado que ya había—. Lo que sí
// pasa es que la higiene no se hizo, y hasta ahora nadie se enteraba.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestUnaRotacionAbiertaSeVeYUnaCerradaNo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-rotando")
	latirComoEmisor(t, ts.URL, tok, "eeee5555")

	d, hay, err := s.engine.DevicePorNombre("casa", "pc-rotando")
	if err != nil || !hay {
		t.Fatalf("no encuentro la máquina: %v %v", hay, err)
	}

	dump := func(ahora time.Time) string {
		var b strings.Builder
		renderFlota(&b, s.engine, nil, ahora, s.sondaIntervalo, "0.140.3", nil, serviciosPorProyectoDefault)
		return b.String()
	}

	// SIN ROTACIÓN NO HAY SERIE. Una que existiera siempre —valiendo 0, por ejemplo— no se
	// distinguiría de una rotación vencida, que es justo lo que hay que poder ver.
	if strings.Contains(dump(time.Now()), nombreRotacionAbierta+"{") {
		t.Fatal("se exporta una rotación abierta cuando no hay ninguna")
	}

	vence := time.Now().Add(2 * time.Hour)
	if _, err := s.engine.AbrirRotacion(d.ID, vence); err != nil {
		t.Fatalf("abrir la rotación: %v", err)
	}

	salida := dump(time.Now())
	linea := ""
	for _, l := range strings.Split(salida, "\n") {
		if strings.HasPrefix(l, nombreRotacionAbierta+"{") {
			linea = l
		}
	}
	if linea == "" {
		t.Fatalf("una rotación abierta no se exporta: se abre en la base y muere en silencio, que es "+
			"exactamente el defecto.\n%s", salida)
	}
	if !strings.Contains(linea, `device="pc-rotando"`) {
		t.Errorf("la serie no nombra la máquina: %s", linea)
	}
	// EL VALOR ES LO QUE FALTA, no la hora de vencimiento: un timestamp absoluto obliga a restar
	// contra el reloj de quien mira, y con eso ya se equivocó `timestamp()` una vez acá.
	campos := strings.Fields(linea)
	if len(campos) != 2 {
		t.Fatalf("la línea no tiene la forma esperada: %q", linea)
	}
	if campos[1] == "0" || strings.HasPrefix(campos[1], "17") || strings.HasPrefix(campos[1], "20") {
		t.Errorf("el valor parece una hora absoluta y tiene que ser lo que FALTA: %q", linea)
	}

	// YA VENCIDA: el valor se va a negativo en vez de desaparecer. Desaparecer la haría
	// indistinguible de «no hay ninguna rotación», que es lo contrario.
	vencido := dump(time.Now().Add(3 * time.Hour))
	negativa := false
	for _, l := range strings.Split(vencido, "\n") {
		if strings.HasPrefix(l, nombreRotacionAbierta+"{") && strings.Contains(l, " -") {
			negativa = true
		}
	}
	if !negativa {
		t.Errorf("una rotación ya vencida no se exporta en negativo: desaparecer la hace "+
			"indistinguible de «no hay ninguna rotación abierta», que es lo contrario de lo que "+
			"pasa.\n%s", vencido)
	}

	// Y AL COMPLETARSE, DESAPARECE.
	if _, err := s.engine.AbandonarRotacionesVencidas(time.Now().Add(3 * time.Hour)); err != nil {
		t.Fatalf("abandonar: %v", err)
	}
	if strings.Contains(dump(time.Now().Add(3*time.Hour)), nombreRotacionAbierta+"{") {
		t.Error("la rotación se abandonó y la serie sigue: quedaría una alerta sonando sobre algo " +
			"que ya se resolvió solo")
	}
}

// Y LA ALERTA QUE LA LEE.
func TestLaAlertaDeRotacionSinCompletarExiste(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts-flota.yml")), " ")
	if !strings.Contains(reglas, nombreRotacionAbierta+" <") {
		t.Errorf("ninguna alerta lee `%s`: la rotación seguiría abriéndose en la base y muriéndose "+
			"en silencio, que es el defecto entero", nombreRotacionAbierta)
	}
}
