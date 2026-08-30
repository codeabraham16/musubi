package mcp

// Guarda contra una clase entera de bug SILENCIOSO: una serie que entra a Prometheus con OTRO
// nombre del que declara, y las reglas que la consultan quedan sin poder dispararse.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE PASÓ, Y POR QUÉ NO SE VEÍA
//
// El receptor OTLP de Prometheus AGREGA la unidad canónica al nombre cuando el nombre no termina
// en ella. `musubi_fleet_service_latency_p95_ms` con `Unit: "ms"` entró como
// `musubi_fleet_service_latency_p95_ms_milliseconds`, y `ServicioLento` —que consulta el nombre
// declarado— no podía dispararse nunca.
//
// Medido en producción el 2026-08-30. Estuvo escondido porque DOS omisiones se tapaban entre sí:
// las reglas de la fase 4 no estaban desplegadas, y ningún servicio reportaba latencia todavía.
// Con cualquiera de las dos presente, el síntoma seguía siendo silencio — que se ve igual que
// «todo bien».
//
// Y NO ES LA PRIMERA VEZ: en el TSDB quedó `musubi_fleet_device_cpus_bytes`, un fantasma sin
// muestras desde hace horas. Un binario anterior declaraba la CANTIDAD DE NÚCLEOS con unidad
// `By`, y Prometheus lo ingirió como bytes. Se arregló en algún momento sin que nadie escribiera
// por qué — o sea que el mecanismo se descubrió, se corrigió y se olvidó. Por eso esto es una
// prueba y no un comentario.
//
// Y ESTE CAMINO NO TIENE OTRA RED: el scrape de `/metrics` DESCARTA `musubi_fleet_.*` a propósito
// (para que cada dato tenga un solo productor), así que si el empuje nombra mal una serie, esa
// serie no existe en ningún lado.

import (
	"strings"
	"testing"
	"time"
)

// unidadCanonica es cómo Prometheus expande cada unidad de OTLP al sufijar el nombre. Sólo están
// las que este repo usa: agregar una unidad nueva sin agregarla acá hace fallar la prueba, que es
// exactamente lo que tiene que pasar.
var unidadCanonica = map[string]string{
	"s":  "seconds",
	"ms": "milliseconds",
	"By": "bytes",
	"1":  "ratio",
	// Estas dos las agregó la propia prueba al escribirla: se negó a suponer y las señaló. Se
	// verificaron CONTRA EL PROMETHEUS DE PRODUCCIÓN antes de darlas por buenas —
	// `musubi_fleet_device_cpu_percent` y `..._temperature_celsius` existen con su nombre intacto—
	// y pasan porque el nombre ya termina en la forma canónica, así que el receptor deduplica.
	"%":   "percent",
	"Cel": "celsius",
}

// NINGUNA SERIE PUEDE CAMBIAR DE NOMBRE AL ENTRAR POR OTLP.
//
// La regla es simple: o la unidad va vacía —y entonces el nombre manda, que es la convención de
// Prometheus y la del repo— o el nombre YA TERMINA en la forma canónica de esa unidad, y el
// receptor no toca nada.
//
// Sabotaje: devolverle `"ms"` a la serie de latencia → falla acá nombrando la serie y el sufijo
// que le agregarían.
func TestNingunaSerieCambiaDeNombreAlEntrarPorOTLP(t *testing.T) {
	ahora := time.Now()
	revisar := func(nombre, unidad string) {
		t.Helper()
		if unidad == "" {
			return
		}
		canon, conocida := unidadCanonica[unidad]
		if !conocida {
			t.Errorf("la serie %q declara la unidad %q, que esta prueba no sabe expandir: agregala a unidadCanonica y verificá qué le hace el receptor OTLP al nombre",
				nombre, unidad)
			return
		}
		if !strings.HasSuffix(nombre, "_"+canon) {
			t.Errorf("la serie %q declara unidad %q: Prometheus la va a ingerir como %q y toda regla que consulte %q queda MUDA.\n  Arreglo: dejá la unidad vacía (el nombre ya la lleva) o renombrá la serie a %q",
				nombre, unidad, nombre+"_"+canon, nombre, nombre+"_"+canon)
		}
	}

	n := 0
	for _, s := range seriesDeFlota(ahora, time.Minute) {
		revisar(s.Nombre, s.Unidad)
		n++
	}
	for _, s := range seriesDeServicio() {
		revisar(s.Nombre, s.Unidad)
		n++
	}
	// Si las tablas dejaran de encontrarse, la prueba pasaría VACÍA y en verde — el modo de fallo
	// más peligroso que puede tener un barrido.
	if n < 15 {
		t.Fatalf("sólo se revisaron %d series; el barrido no está mirando donde cree", n)
	}
	t.Logf("%d series revisadas, ninguna cambia de nombre al entrar", n)
}
