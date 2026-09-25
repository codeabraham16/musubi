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
// nombra— y fleet y mcp quedaron verdes: con veinte en la tanda, el último espera a diecinueve y se
// dibuja muerto mientras espera.
//
// Acá se encola por el motor una cola de cada largo que importa —uno, justo la tanda, uno más y el
// techo de la cola, cada una en su máquina— y se manda UN latido real por HTTP: cada máquina recibe
// min(cola, tanda). El «uno más» y el techo son los que ven una entrega de más; «justo la tanda», una
// de menos. Es la prueba del CONSUMIDOR de la constante (lo que le llega al agente), no la del texto:
// comparar el literal de la constante no vería un transporte que la ignore.
//
// EXPOSICIÓN medida por la auditoría: nunca pasó. De 1.748 entregas, la más grande llevó 9 comandos.
// La condición previa sí existió: 88 comandos creados en 15 minutos sobre una máquina y 37 sobre
// otra. Con la mutación, un agente que vuelve con la cola llena se lleva hasta ColaMaxPorDevice de una.
//
// Sabotaje: que el latido entregue el doble de la tanda (C4-m8) → el último de la tanda espera a
// diecinueve y se dibuja perdido mientras espera.
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
				"de %d: el último espera a los %d de adelante, más de lo que la cota le concede, y se "+
				"dibuja muerto mientras espera su turno", nombre, cola, len(r.Comandos), tanda, len(r.Comandos)-1)
		} else if len(r.Comandos) != quiero {
			t.Errorf("%s: con %d en cola, un latido entregó %d comandos y tenían que ser %d: la tanda es %d y "+
				"lo que sobra espera al próximo latido", nombre, cola, len(r.Comandos), quiero, tanda)
		}
	}
}
