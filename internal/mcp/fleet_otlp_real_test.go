package mcp

// fleet_otlp_real_test.go cierra A40: hasta acá, TODO lo que se sabía del empuje OTLP salía de
// receptores de prueba escritos por nosotros mismos.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ UN RECEPTOR DE MENTIRA NO ALCANZA, POR MÁS PRUEBAS QUE HAYA
//
// Las veinticuatro pruebas de fleet_otlp_test.go verifican que el sobre tiene la forma que
// NOSOTROS creemos que pide la especificación. Si esa creencia está equivocada —un campo con el
// nombre cambiado, un uint64 mandado como número en vez de string, una unidad que hace que
// Prometheus renombre la serie— las veinticuatro siguen en verde y el empuje no funciona en
// ningún lado. Es exactamente el modo de fallo que este repo persigue: verde por el motivo
// equivocado.
//
// Esta prueba habla con un Prometheus DE VERDAD. No corre sola porque no puede: necesita un
// proceso externo. Se enciende con una variable, y lo que verifica es lo único que un doble no
// puede decirte — que del otro lado quedó una serie consultable, con sus etiquetas y su valor.
//
//	MUSUBI_OTLP_REAL=http://127.0.0.1:9099 go test ./internal/mcp/ -run TestContraUnPrometheusDeVerdad -v
//
// TIENE UN COSTO Y CONVIENE SABERLO: cada corrida empuja una máquina con nombre irrepetible, así
// que deja una serie muerta en ese Prometheus hasta que la retención se la lleve. Es a propósito
// —dos corridas seguidas no se pueden confundir, y una serie vieja no puede hacer pasar a una
// corrida que no empujó— pero significa que no es una prueba para correr en un bucle.
//
// Ese Prometheus tiene que correr con `--web.enable-otlp-receiver`, que NO viene por defecto.
// Si falta, el POST devuelve 404 y la prueba lo dice con esas palabras en vez de dejarte
// mirando un error de red.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
)

// TestContraUnPrometheusDeVerdadAceptaElSobreYQuedaConsultable.
//
// Sabotaje que la hace fallar: darle una `unit` no vacía a las series (poner `Unit: "By"` en
// vez de `serie.Unidad` en armarPayloadOTLP).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LOS DOS SABOTAJES SE MIDIERON, Y EL RESULTADO CORRIGIÓ LO QUE YO CREÍA
//
// La primera versión de este comentario decía que mandar `timeUnixNano` como NÚMERO en vez de
// string también rompía contra Prometheus. Es falso, y se comprobó: Prometheus 3.1.0 lo acepta
// igual. A esa desviación de la especificación la atajan las pruebas con receptor de mentira,
// no ésta.
//
// El de la unidad va justo al revés, y es el que justifica que esta prueba exista:
//
//	sabotaje              receptor de mentira      Prometheus de verdad
//	timeUnixNano número   ROJO                     verde (lo tolera)
//	unit no vacía         VERDE, las 24            ROJO
//
// Con una unidad, Prometheus RENOMBRA la serie —`musubi_fleet_device_up` pasa a
// `musubi_fleet_device_up_bytes`—, contesta 200, y la consulta no encuentra nada. Las
// veinticuatro pruebas de fleet_otlp_test.go siguen en verde porque el sobre que revisan es
// impecable: el que cambia de opinión es el otro lado. Eso es exactamente lo que un doble no
// puede decirte, y por eso esta prueba no es redundante con las otras.
func TestContraUnPrometheusDeVerdadAceptaElSobreYQuedaConsultable(t *testing.T) {
	base := strings.TrimRight(os.Getenv("MUSUBI_OTLP_REAL"), "/")
	if base == "" {
		t.Skip("MUSUBI_OTLP_REAL sin definir: esta prueba necesita un Prometheus 3.x de verdad con --web.enable-otlp-receiver")
	}

	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	// Un nombre irrepetible por corrida: dos corridas seguidas contra el mismo Prometheus no
	// se pueden confundir, y una serie vieja no puede hacer pasar a una corrida que no empujó.
	maquina := fmt.Sprintf("prueba-real-%d", ahora.UnixNano())
	maquinaConMuestra(t, s, "casa", maquina, muestraSana(42, ahora), ahora)

	p := ptrPrincipal(principalDePrometheus())
	cuerpo, puntos, _, err := armarPayloadOTLP(s.engine, p, ahora, s.sondaIntervalo)
	if err != nil {
		t.Fatalf("no se pudo armar el payload: %v", err)
	}
	if puntos == 0 {
		t.Fatal("el payload salió sin puntos: no habría nada que verificar del otro lado")
	}

	emp, err := nuevoEmpujadorOTLP(config.OTLPPushConfig{
		Endpoint:  base + "/api/v1/otlp/v1/metrics",
		Principal: "prometheus",
	})
	if err != nil {
		t.Fatalf("no se pudo configurar el empujador: %v", err)
	}
	if err := emp.enviar(t.Context(), cuerpo); err != nil {
		if strings.Contains(err.Error(), "404") {
			t.Fatalf("el receptor OTLP contestó 404: a ese Prometheus le falta --web.enable-otlp-receiver. Error: %v", err)
		}
		t.Fatalf("Prometheus RECHAZÓ el sobre — las pruebas con receptor de mentira no lo habrían visto: %v", err)
	}

	// Aceptar el POST no es lo mismo que haber guardado la serie. Prometheus puede devolver 200
	// y descartar puntos (fuera de ventana, nombre inválido). Lo que importa es que se pueda
	// CONSULTAR, que es para lo que se empuja.
	consulta := fmt.Sprintf(`musubi_fleet_device_up{device=%q}`, maquina)
	var valor string
	for intento := 0; intento < 15; intento++ {
		if v, ok := consultarPrometheus(t, base, consulta); ok {
			valor = v
			break
		}
		time.Sleep(time.Second)
	}
	if valor == "" {
		t.Fatalf("Prometheus aceptó el sobre (200) y quince segundos después la serie %s NO se puede consultar: el empuje se ve exitoso y no deja nada", consulta)
	}
	if valor != "1" {
		t.Errorf("la serie llegó con valor %q y se empujó un 1: el valor se corrompió en el camino", valor)
	}

	// Y las ETIQUETAS: una serie que llega sin project es una serie que nadie puede atribuir a
	// su tenant, que es lo único que hace seguro compartir un Prometheus (B9).
	if v, ok := consultarPrometheus(t, base, fmt.Sprintf(`musubi_fleet_device_up{device=%q,project="casa"}`, maquina)); !ok || v != "1" {
		t.Error("la serie llegó SIN la etiqueta project: sin ella dos tenants comparten Prometheus y no hay forma de separar sus alertas")
	}
	t.Logf("Prometheus aceptó %d puntos y devolvió %s = %s con su project", puntos, consulta, valor)
}

// consultarPrometheus hace una consulta instantánea y devuelve el valor de la primera serie.
func consultarPrometheus(t *testing.T, base, expr string) (string, bool) {
	t.Helper()
	resp, err := http.Get(base + "/api/v1/query?query=" + url.QueryEscape(expr))
	if err != nil {
		t.Fatalf("no se pudo consultar a Prometheus: %v", err)
	}
	defer resp.Body.Close()
	crudo, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var r struct {
		Data struct {
			Result []struct {
				Value []any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if json.Unmarshal(crudo, &r) != nil || len(r.Data.Result) == 0 || len(r.Data.Result[0].Value) < 2 {
		return "", false
	}
	v, _ := r.Data.Result[0].Value[1].(string)
	return v, v != ""
}
