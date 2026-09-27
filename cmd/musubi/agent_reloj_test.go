package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// cuerpoQueMandaElAgente hace latir al agente contra un cerebro de mentira y devuelve el cuerpo
// que mandó, leído con el tipo del contrato. El cuerpo viaja por un canal y no por una variable
// compartida: el handler corre en otra goroutine.
func cuerpoQueMandaElAgente(t *testing.T, m *fleet.Muestra) fleet.CuerpoLatido {
	t.Helper()
	recibido := make(chan []byte, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		recibido <- b
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if res := latir(ts.URL, "tok-abc", "", m); !res.ok {
		t.Fatalf("el latido falló: %+v", res)
	}
	var crudo []byte
	select {
	case crudo = <-recibido:
	case <-time.After(5 * time.Second):
		t.Fatal("el latido respondió y el cerebro de mentira no recibió nada")
	}
	if len(crudo) == 0 {
		t.Fatal("el agente mandó el latido SIN CUERPO: sin versión, sin muestra y sin hora")
	}
	var c fleet.CuerpoLatido
	if err := json.Unmarshal(crudo, &c); err != nil {
		t.Fatalf("el cuerpo no es el contrato: %v (%s)", err, crudo)
	}
	return c
}

// relojesDeLaPrueba deja la enumeración y el reloj del latido en manos de la prueba, y los
// devuelve a su lugar al terminar.
func relojesDeLaPrueba(t *testing.T, enumerar func() ([]fleet.ReporteServicio, error), reloj func() time.Time) {
	t.Helper()
	anteriorEnum, anteriorReloj := enumerarServicios, relojDelLatido
	enumerarServicios, relojDelLatido = enumerar, reloj
	reiniciarFrenos()
	t.Cleanup(func() {
		enumerarServicios, relojDelLatido = anteriorEnum, anteriorReloj
		reiniciarFrenos()
	})
}

// A133 — LA HORA DE ENVÍO SE SELLA DESPUÉS DE LO LENTO.
//
// El cerebro lee `enviado_ms` como la hora de este reloj cuando el latido salió, y le resta la
// suya. Todo lo que el agente tarde entre el sello y el envío se lee como un reloj ATRASADO. Lo
// lento del latido es la enumeración de servicios (~3 s de WMI en Windows cada vez que vence su
// caché) y la sonda de alcance, y la muestra se toma antes de las dos: reusar `m.Tomada` era lo
// cómodo porque ya estaba a mano, y en Windows habría publicado segundos de atraso inventados.
//
// SIN DORMIR: el reloj del latido contesta una hora mientras la enumeración no corrió y otra
// después, así que la prueba afirma el ORDEN de las dos cosas y no una carrera contra el reloj de
// la máquina que corre la suite.
//
// Sabotaje: sellar con la hora de la muestra.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="\tcarga.EnviadoMs = relojDelLatido().UnixMilli()"
// arnes: a="\tcarga.EnviadoMs = m.Tomada.UnixMilli()"
// arnes: colision_ok="TestUnRelojAbsurdoViajaEnElLatido"
func TestLaHoraDeEnvioSeTomaDespuesDeLoLento(t *testing.T) {
	antes := time.UnixMilli(1_790_000_000_000)
	despues := antes.Add(3 * time.Second)
	var enumerado atomic.Bool
	relojesDeLaPrueba(t,
		func() ([]fleet.ReporteServicio, error) {
			enumerado.Store(true)
			return nil, nil
		},
		func() time.Time {
			if enumerado.Load() {
				return despues
			}
			return antes
		})

	c := cuerpoQueMandaElAgente(t, &fleet.Muestra{Tomada: antes})
	if !enumerado.Load() {
		t.Fatal("control: el latido no enumeró servicios, así que esta prueba no mide nada")
	}
	if c.EnviadoMs != despues.UnixMilli() {
		t.Fatalf("enviado_ms = %d, esperaba %d (la hora de DESPUÉS de enumerar): lo que tarde el agente entre el sello y el envío el cerebro lo lee como reloj atrasado",
			c.EnviadoMs, despues.UnixMilli())
	}
}

// A133 — UN RELOJ ABSURDO VIAJA ENTERO, Y NO SE LLEVA PUESTO AL LATIDO.
//
// Es la máquina que más importa ver: una con la pila del BIOS agotada arranca en el pasado, y una
// con el NTP roto se puede ir años para cualquier lado. Hay dos formas de perderla:
//
//   - que la hora viaje como FECHA: `time.Time` no se serializa fuera de los años 0 a 9999, el
//     `json.Marshal` del latido falla, y el agente manda el latido SIN CUERPO — sin versión, sin
//     muestra y sin inventario. Por eso viaja como entero de milisegundos.
//   - que alguien descarte los relojes imposibles antes de mandarlos: el absurdo es justamente lo
//     que el cerebro necesita ver.
//
// Sabotaje: descartar la hora si no cabe en una fecha serializable.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="\tcarga.EnviadoMs = relojDelLatido().UnixMilli()"
// arnes: a="\tif t := relojDelLatido(); t.Year() <= 9999 {\n\t\tcarga.EnviadoMs = t.UnixMilli()\n\t}"
// arnes: colision_ok="TestLaHoraDeEnvioSeTomaDespuesDeLoLento"
func TestUnRelojAbsurdoViajaEnElLatido(t *testing.T) {
	absurdo := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, err := absurdo.MarshalJSON(); err == nil {
		t.Fatal("control: el año 10000 se pudo serializar como fecha, así que esta prueba no mide lo que dice")
	}
	relojesDeLaPrueba(t,
		func() ([]fleet.ReporteServicio, error) { return nil, nil },
		func() time.Time { return absurdo })

	c := cuerpoQueMandaElAgente(t, nil)
	if c.Version == "" {
		t.Error("el latido llegó sin versión: el reloj absurdo se llevó puesto el resto del cuerpo")
	}
	if c.EnviadoMs != absurdo.UnixMilli() {
		t.Fatalf("enviado_ms = %d, esperaba %d: el reloj más roto de la flota es justo el que no llega",
			c.EnviadoMs, absurdo.UnixMilli())
	}
}
