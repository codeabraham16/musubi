package mcp

// La señal que distingue «máquina apagada» de «el agente no corre».

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// conTailnet sustituye la consulta al tailnet y limpia el latch al terminar.
func conTailnet(t *testing.T, pares []fleet.ParDeTailnet, err error) {
	t.Helper()
	previo := tailnetPares
	tailnetPares = func(context.Context) ([]fleet.ParDeTailnet, error) { return pares, err }
	vidaRedDeshabilitada.Store(false)
	t.Cleanup(func() { tailnetPares = previo; vidaRedDeshabilitada.Store(false) })
}

func deviceCaido(t *testing.T, s *McpServer, nombre string) fleet.Device {
	t.Helper()
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": "A", "caps": []string{"metrics"}, "project": "casa", "os": "windows",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	d, _, _ := s.engine.DevicePorNombre("casa", nombre)
	return d // recién enrolada: nunca latió, así que está «caída»
}

// SI EL TAILNET LA VE, LO QUE ESTÁ CAÍDO ES EL AGENTE. Es la razón de ser del eje: `up == 0`
// mandaba a revisar el hardware de una máquina encendida.
//
// Sabotaje: que medirVidaDeRed guarde VidaAusente cuando el par está EnLinea.
func TestSiElTailnetVeLaMaquinaLoCaidoEsElAgente(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := deviceCaido(t, s, "davantis-1")
	conTailnet(t, []fleet.ParDeTailnet{{Nombre: "davantis-1", EnLinea: true}}, nil)
	ahora := time.Now()

	s.medirVidaDeRed(context.Background(), []fleet.Device{d}, ahora)
	v, hay := s.vidaDeRedDe(d.ID, ahora)
	if !hay || v != fleet.VidaPresente {
		t.Fatalf("vida = %v (medida=%v), esperaba presente", v, hay)
	}

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora, s.sondaIntervalo, versionDePrueba, s.vidaDeRedDe)
	if !strings.Contains(b.String(), nombreVidaDeRed+`{project="casa",device="davantis-1"} 1`) {
		t.Fatalf("la serie no dice que la máquina está en la red:\n%s", b.String())
	}
}

// Y SI NO LA VE, SÍ ES LA MÁQUINA. El control positivo del de arriba: sin esto, un eje que
// contestara siempre «presente» pasaría igual.
func TestSiElTailnetNoLaVeLaSerieDiceCero(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := deviceCaido(t, s, "davantis-1")
	conTailnet(t, []fleet.ParDeTailnet{{Nombre: "davantis-1", EnLinea: false}}, nil)
	ahora := time.Now()

	s.medirVidaDeRed(context.Background(), []fleet.Device{d}, ahora)
	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora, s.sondaIntervalo, versionDePrueba, s.vidaDeRedDe)
	if !strings.Contains(b.String(), nombreVidaDeRed+`{project="casa",device="davantis-1"} 0`) {
		t.Fatalf("la serie no dice que la máquina no está:\n%s", b.String())
	}
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// LO QUE NO SE PUDO MEDIR NO EMITE SERIE. Es el diseño entero: un 0 acá afirma «la máquina no
// está en la red», y eso manda a alguien a revisar hardware. Un cerebro sin `tailscale`, o una
// máquina que no está en el tailnet, no permiten afirmarlo.
//
// Sabotaje: emitir 0 cuando fleet.VidaNoMedida, o dejar que medirVidaDeRed guarde algo cuando
// la consulta al tailnet falla.
func TestLoQueNoSePudoMedirNoEmiteSerie(t *testing.T) {
	casos := []struct {
		nombre string
		pares  []fleet.ParDeTailnet
		err    error
	}{
		{"no hay tailscale en esta máquina", nil, errors.New("exec: \"tailscale\": not found")},
		{"la máquina no está en el tailnet", []fleet.ParDeTailnet{{Nombre: "otra", EnLinea: true}}, nil},
		{"dos pares dicen ser la misma", []fleet.ParDeTailnet{
			{Nombre: "davantis-1", EnLinea: true}, {Nombre: "davantis-1", EnLinea: false}}, nil},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			d := deviceCaido(t, s, "davantis-1")
			conTailnet(t, c.pares, c.err)
			ahora := time.Now()

			s.medirVidaDeRed(context.Background(), []fleet.Device{d}, ahora)
			if _, hay := s.vidaDeRedDe(d.ID, ahora); hay {
				t.Fatal("se guardó una medición que no se pudo hacer")
			}
			var b strings.Builder
			renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora, s.sondaIntervalo, versionDePrueba, s.vidaDeRedDe)
			if strings.Contains(b.String(), nombreVidaDeRed) {
				t.Fatalf("se emitió la serie sin haber medido: un 0 acá dice «la máquina no está»\n%s", b.String())
			}
		})
	}
}

// UNA MEDICIÓN VIEJA TAMPOCO SE EMITE. Si `tailscale` deja de contestar, el último 1 conocido
// seguiría diciendo «la máquina está viva» sobre algo que nadie volvió a mirar — el mismo
// congelamiento que hace que Prometheus no borre la serie de una máquina muerta, pero fabricado
// por nosotros.
//
// Sabotaje: sacar la comparación contra vidaRedVigencia.
func TestUnaMedicionViejaNoSeEmite(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := deviceCaido(t, s, "davantis-1")
	conTailnet(t, []fleet.ParDeTailnet{{Nombre: "davantis-1", EnLinea: true}}, nil)
	ahora := time.Now()

	s.medirVidaDeRed(context.Background(), []fleet.Device{d}, ahora)
	if _, hay := s.vidaDeRedDe(d.ID, ahora); !hay {
		t.Fatal("no se guardó la medición fresca")
	}
	if _, hay := s.vidaDeRedDe(d.ID, ahora.Add(vidaRedVigencia+time.Minute)); hay {
		t.Fatal("una medición vencida se sigue publicando: dice que la máquina está viva sin que nadie lo haya vuelto a mirar")
	}
}

// CUANDO LA MÁQUINA VUELVE A LATIR, LA SERIE DESAPARECE. Si se quedara, `up == 1` y `net_up == 1`
// convivirían para siempre y la alerta que los cruza tendría un dato que ya no se refresca.
//
// Sabotaje: sacar el olvidarVidaDeRed de medirVidaDeRedDeLosCaidos.
func TestCuandoVuelveALatirLaSerieDesaparece(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := deviceCaido(t, s, "gio")
	conTailnet(t, []fleet.ParDeTailnet{{Nombre: "gio", EnLinea: true}}, nil)
	ahora := time.Now()

	s.medirVidaDeRedDeLosCaidos(context.Background(), ahora)
	if _, hay := s.vidaDeRedDe(d.ID, ahora); !hay {
		t.Fatal("una máquina que nunca latió tenía que medirse")
	}
	// Ahora late.
	if _, err := s.engine.LatirDevice(d.ID, ahora, ""); err != nil {
		t.Fatalf("latido: %v", err)
	}
	s.medirVidaDeRedDeLosCaidos(context.Background(), ahora)
	if _, hay := s.vidaDeRedDe(d.ID, ahora); hay {
		t.Fatal("la máquina volvió a latir y la medición de red sigue publicándose")
	}
}

// UN TIER B NO ENTRA EN ESTE EJE. No corre agente por definición, así que «el agente no está
// corriendo» no es una hipótesis sobre él: emitir la serie ahí diría algo sobre una causa que no
// aplica, y le daría a alguien un motivo para no ir a mirar la sonda que sí lo explica.
func TestUnTierBNoEntraEnLaVidaDeRed(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "router", "tier": "B", "caps": []string{"metrics"},
		"project": "casa", "address": "gio@router.local", "os": "linux",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	d, _, _ := s.engine.DevicePorNombre("casa", "router")
	conTailnet(t, []fleet.ParDeTailnet{{Nombre: "router", EnLinea: true}}, nil)
	ahora := time.Now()

	s.medirVidaDeRedDeLosCaidos(context.Background(), ahora)
	if _, hay := s.vidaDeRedDe(d.ID, ahora); hay {
		t.Fatal("se midió la vida de red de un Tier B: ese eje no habla de él")
	}
}
