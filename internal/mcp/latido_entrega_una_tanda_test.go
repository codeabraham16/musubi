package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// UN LATIDO ENTREGA LA TANDA ENTERA Y NI UN COMANDO MÁS.
//
// La cota de `perdido` (fleet.EsperaMaxDeEntregado) cuenta la tanda con fleet.ComandosPorEntregaMax:
// da por sentado que una entrega nunca lleva más. Ninguna prueba del árbol contaba cuántos comandos
// entrega un latido. La auditoría A131 (C4-m8) puso `maxComandosPorLatido = 2 *
// fleet.ComandosPorEntregaMax` —exactamente el modo de falla que el comentario de fleet_http.go
// nombra— y fleet y mcp quedaron verdes.
//
// DÓNDE MUERDE, con la cota de hoy: cuando la tanda trae una shell. La cota se arma con UNA shell y
// nueve comandos detrás, y una shell ocupa al agente más que once comandos en su techo, así que
// veinte comandos comunes —el doble de la tanda— todavía reportan adentro de ella. Con una shell
// adelante, en cambio, lo que viaja al final espera la sesión entera y diez comandos más de los que
// la cota cuenta, y se dibuja muerto mientras espera su turno: el error caro, el que manda a relanzar
// lo que va a correr igual. La prueba no espera a que la tanda traiga una shell para decirlo: cuenta
// la entrega, que es la premisa de la cota para cualquier tanda.
//
// Acá se encola por el motor una cola de cada largo que importa —uno, justo la tanda, uno más y el
// techo de la cola, cada una en su máquina— y se manda UN latido real por HTTP: cada máquina recibe
// min(cola, tanda). El «uno más» y el techo son los que ven una entrega de más; «justo la tanda», una
// de menos. Es la prueba del CONSUMIDOR de la constante (lo que le llega al agente), no la del texto:
// comparar el literal de la constante no vería un transporte que la ignore.
//
// EXPOSICIÓN medida por la auditoría: nunca pasó. De 1.748 entregas, la más grande llevó 9 comandos.
// La condición previa sí existió: 88 comandos creados en 15 minutos sobre una máquina y 37 sobre
// otra. Con la mutación, un agente que vuelve con la cola llena se lleva el doble de la tanda de una.
//
// Sabotaje: que el latido entregue el doble de la tanda (C4-m8) → si la tanda trae una shell, lo que
// viaja al final espera la sesión entera y diez comandos de más, y se dibuja perdido mientras espera.
// arnes: archivo="internal/mcp/fleet_http.go"
// arnes: de="const maxComandosPorLatido = fleet.ComandosPorEntregaMax"
// arnes: a="const maxComandosPorLatido = 2 * fleet.ComandosPorEntregaMax"
func TestUnLatidoEntregaLaTandaEnteraYNiUnComandoMas(t *testing.T) {
	tanda := fleet.ComandosPorEntregaMax
	// EL PISO: si la cola no pudiera ser más larga que la tanda, no habría entrega de más que medir.
	if fleet.ColaMaxPorDevice <= tanda+1 {
		t.Fatalf("ColaMaxPorDevice=%d y la tanda es %d: la prueba no puede encolar más de lo que se entrega",
			fleet.ColaMaxPorDevice, tanda)
	}
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)

	for i, cola := range []int{1, tanda, tanda + 1, fleet.ColaMaxPorDevice} {
		nombre := fmt.Sprintf("pc-cola-%d", i)
		tok := enrolarConExec(t, s, "casa", nombre)
		d, ok, err := s.engine.DevicePorToken(tok)
		if err != nil || !ok {
			t.Fatalf("%s: no se resolvió el token (%v %v)", nombre, ok, err)
		}
		for j := 0; j < cola; j++ {
			if _, err := s.engine.EncolarComando(fleet.Comando{
				DeviceID: d.ID, ProjectID: "casa", Principal: "gio", Origen: fleet.OrigenPersona,
				Argv: []string{"echo", fmt.Sprint(j)}, Timeout: 30 * time.Second,
			}); err != nil {
				t.Fatalf("%s: encolar el %d de %d: %v", nombre, j+1, cola, err)
			}
		}

		codigo, cuerpo := postCon(t, ts.URL+fleetHeartbeatPath, tok, "")
		if codigo != http.StatusOK {
			t.Fatalf("%s: el latido respondió %d: %s", nombre, codigo, cuerpo)
		}
		var r fleet.RespuestaLatido
		if err := json.Unmarshal([]byte(cuerpo), &r); err != nil {
			t.Fatalf("%s: la respuesta del latido no es el contrato: %v (%s)", nombre, err, cuerpo)
		}
		quiero := min(cola, tanda)
		if len(r.Comandos) > tanda {
			t.Errorf("%s: con %d en cola, un latido entregó %d comandos y la cota de `perdido` cuenta tandas "+
				"de %d. Si esa tanda trae una shell, lo que viaja al final espera la sesión entera y más "+
				"comandos de los que la cota cuenta (%d de más), y se dibuja muerto mientras espera su turno",
				nombre, cola, len(r.Comandos), tanda, len(r.Comandos)-tanda)
		} else if len(r.Comandos) != quiero {
			t.Errorf("%s: con %d en cola, un latido entregó %d comandos y tenían que ser %d: la tanda es %d y "+
				"lo que sobra espera al próximo latido", nombre, cola, len(r.Comandos), quiero, tanda)
		}
	}
}
