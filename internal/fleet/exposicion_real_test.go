package fleet

// exposicion_real_test.go es la prueba OPT-IN contra un endpoint de verdad.
//
// No corre en CI y no debe: necesita una credencial y una red. Corre a mano, en la máquina donde
// esa credencial vive, y existe porque el fixture no puede probar lo único que el fixture no
// tiene — que el viaje entero funciona: TLS, el header, el código de estado, el tamaño real de la
// respuesta y el formato tal como lo manda el servidor hoy, no el día que se recortó el fixture.
//
// Es la misma decisión que fleet_otlp_real_test.go: lo que sólo se puede verificar contra la cosa
// real se verifica contra la cosa real, y se dice que es opt-in en vez de fingir que la suite lo
// cubre.
//
//	MUSUBI_EXPOSICION_URL=https://…/metrics \
//	MUSUBI_EXPOSICION_AUTH="Bearer …" \
//	MUSUBI_EXPOSICION_MONTAJE=/data \
//	go test ./internal/fleet -run TestContraUnEndpointDeVerdad -v

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestContraUnEndpointDeVerdad(t *testing.T) {
	url := os.Getenv("MUSUBI_EXPOSICION_URL")
	if url == "" {
		t.Skip("opt-in: poné MUSUBI_EXPOSICION_URL para correrla")
	}
	d := DestinoExposicion{
		URL:          url,
		Autorizacion: os.Getenv("MUSUBI_EXPOSICION_AUTH"),
		Montaje:      os.Getenv("MUSUBI_EXPOSICION_MONTAJE"),
	}

	var cpu ContadorCPUExportado
	m1, err := TomarMuestraDeExposicion(d, &cpu, 20*time.Second)
	if err != nil {
		t.Fatalf("el raspado falló: %v", err)
	}
	if m1.CPUPct != nil {
		t.Errorf("la primera lectura trajo CPU %v%%: no hay contra qué restar", *m1.CPUPct)
	}
	if m1.MemTotal == 0 || m1.MemUsada == 0 {
		t.Errorf("la memoria no se midió: total=%d usada=%d", m1.MemTotal, m1.MemUsada)
	}
	if m1.DiscoTotal == 0 {
		t.Errorf("el disco no se midió con montaje %q: revisá que ese punto de montaje exista en el endpoint", d.Montaje)
	}
	if err := m1.Valida(); err != nil {
		t.Errorf("la muestra real no es válida: %v", err)
	}

	// SEGUNDA LECTURA DE VERDAD, y la espera NO es «un ratito»: tiene que superar el CACHÉ DEL
	// ENDPOINT. La primera versión esperaba 3 s y fallaba siempre — no por un bug, sino porque el
	// endpoint devuelve la misma respuesta cacheada y el contador no se mueve. Medido contra el
	// real: refresca cada ~62 s (tres ciclos: 62, 66, 62). Con menos que eso, esta prueba
	// afirmaría que la derivada está rota cuando lo que pasa es que no hubo dos lecturas.
	//
	// La derivada de CPU es lo único que no se puede afirmar con un solo viaje, y es también lo
	// que más fácil se rompe sin que nada avise: un contador que no acumula bien da un porcentaje
	// plausible.
	espera := 75 * time.Second
	if v := os.Getenv("MUSUBI_EXPOSICION_ESPERA"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			espera = d
		}
	}
	t.Logf("esperando %s a que el endpoint refresque su caché…", espera)
	time.Sleep(espera)
	m2, err := TomarMuestraDeExposicion(d, &cpu, 20*time.Second)
	if err != nil {
		t.Fatalf("el segundo raspado falló: %v", err)
	}
	if m2.CPUPct == nil {
		t.Fatal("la segunda lectura no dio porcentaje de CPU: el contador no está acumulando")
	}
	if *m2.CPUPct < 0 || *m2.CPUPct > 100 {
		t.Errorf("porcentaje de CPU fuera de rango: %v", *m2.CPUPct)
	}
	// El load se imprime DESREFERENCIADO o no se imprime: un %v sobre un *float64 escribe la
	// dirección, que es ruido con forma de dato.
	load := "null"
	if m2.Load1 != nil {
		load = strconv.FormatFloat(*m2.Load1, 'f', 2, 64)
	}
	t.Logf("host real · cpu=%.2f%% mem=%d/%d disco=%d/%d libre=%d num_cpu=%d load1=%s uptime=%d",
		*m2.CPUPct, m2.MemUsada, m2.MemTotal, m2.DiscoUsado, m2.DiscoTotal, m2.DiscoDisponible,
		m2.NumCPU, load, m2.UptimeSeg)
}
