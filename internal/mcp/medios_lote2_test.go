package mcp

// medios_lote2_test.go custodia cuatro reparaciones del barrido adversario que comparten forma:
// en las cuatro el código AFIRMABA algo que no podía sostener, y ninguna daba error.

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// LA LIMPIEZA DE COOLDOWNS CORRE AUNQUE LA RETENCIÓN DE SALIDAS ESTÉ APAGADA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// DOS RETENCIONES DISTINTAS ATADAS POR UN RELOJ, Y EL RELOJ LO DABA CUERDA UNA SOLA
//
// `podarEstadoDePoliticasSiToca` comparaba contra `s.ultimaPoda` «para no repetir el reloj». Pero
// quien le da cuerda a ese campo es `podarSalidasSiToca`, que arranca con
//
//	if s.retencionSalidasDias <= 0 { return 0 }   // desactivado explícitamente
//
// y ese `return` está ANTES de la asignación. En un despliegue que apaga la retención de salidas
// —una configuración soportada, que tiene su propio comentario explicándola— `ultimaPoda` se
// queda en cero para siempre y esta poda NO CORRE NUNCA. La tabla de cooldowns crece sin techo
// por una condición que habla de otra cosa, y no hay ningún error que lo diga.
//
// Sabotaje que lo hace fallar: volver a comparar contra `s.ultimaPoda` en vez del reloj propio.
func TestLaPodaDeCooldownsNoDependeDeLaRetencionDeSalidas(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// LA RETENCIÓN DE SALIDAS, APAGADA. Es el escenario entero de la prueba.
	s.retencionSalidasDias = 0
	s.politicas = []fleet.Politica{{Nombre: "viva"}}

	for _, p := range []string{"viva", "borrada-del-archivo"} {
		if err := s.engine.MarcarDisparoDePolitica(p, "dev-1", "", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}

	// Con la retención apagada, este camino no toca el reloj — que es justamente el punto.
	if n := s.podarSalidasSiToca(time.Now()); n != 0 {
		t.Fatalf("la poda de salidas hizo algo con la retención en 0: %d", n)
	}
	if !s.ultimaPoda.IsZero() {
		t.Fatal("ultimaPoda avanzó con la retención apagada: la prueba ya no reproduce el escenario")
	}

	s.podarEstadoDePoliticasSiToca(time.Now())

	// La fila de la política que ya no existe tiene que haberse ido; la viva, quedarse.
	quedan := politicasConEstado(t, s)
	if quedan["borrada-del-archivo"] {
		t.Error("la poda de cooldowns NO corrió: con la retención de salidas apagada la tabla crece sin techo, y nada lo dice")
	}
	if !quedan["viva"] {
		t.Error("la poda se llevó el cooldown de una política que SÍ existe")
	}
}

// Y el otro lado: no corre dos veces seguidas. Sin esto, «tiene reloj propio» podría ser «no
// tiene reloj», que es un DELETE sobre la tabla en cada tick del barrido.
func TestLaPodaDeCooldownsRespetaSuPropiaCadencia(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.politicas = []fleet.Politica{{Nombre: "viva"}}
	ahora := time.Now()

	s.podarEstadoDePoliticasSiToca(ahora)
	primera := s.ultimaPodaDePoliticas
	if primera.IsZero() {
		t.Fatal("la primera poda no dejó marca de reloj")
	}
	s.podarEstadoDePoliticasSiToca(ahora.Add(time.Minute))
	if !s.ultimaPodaDePoliticas.Equal(primera) {
		t.Error("podó de nuevo un minuto después: es un DELETE por tick sobre una tabla que casi nunca cambia")
	}
	s.podarEstadoDePoliticasSiToca(ahora.Add(podaCadaTanto + time.Second))
	if s.ultimaPodaDePoliticas.Equal(primera) {
		t.Error("pasada la cadencia no volvió a podar: la limpieza quedaría hecha una sola vez por vida del proceso")
	}
}

// politicasConEstado lee qué políticas tienen fila de cooldown. Se lee con CooldownsDePoliticas
// —la misma puerta que usa el motor— y NO podando: la primera versión de este helper llamaba a
// PodarEstadoDePoliticas para «ver qué quedaba», o sea que BORRABA para medir y devolvía true
// siempre. La prueba daba rojo con el arreglo puesto, que es la peor clase de falso negativo:
// dice que el arreglo no anda cuando el que no anda es el que mide.
func politicasConEstado(t *testing.T, s *McpServer) map[string]bool {
	t.Helper()
	cds, err := s.engine.CooldownsDePoliticas()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for politica := range cds {
		out[politica] = true
	}
	return out
}

// UN SEGUNDO REPORTE DEL MISMO COMANDO NO PISA EL RESULTADO DEL PRIMERO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA CONDICIÓN VIVÍA DONDE SE COMPRUEBA Y NO DONDE SE ESCRIBE
//
// `GuardarResultado` leía el estado, comprobaba que no fuera `terminado` y DESPUÉS hacía el
// `UPDATE ... WHERE id = ?`. Son dos viajes a la base y entre uno y otro no hay nada: dos
// reportes del mismo comando —un agente que reintenta porque no vio el ACK, o el que quedó
// terminando cuando el servicio se reinició— pasaban los DOS por la comprobación y el segundo
// pisaba el resultado del primero. Es la misma forma que el crítico de `CancelarMantenimiento`.
//
// ESTA PRUEBA NACIÓ ROTA Y VALE CONTARLO. La primera versión lanzaba 24 reportes concurrentes y
// comprobaba que la salida guardada tuviera «forma de un solo reporte». No servía: con el defecto
// puesto cada escritura es completa y coherente —simplemente gana la última—, así que la forma
// era válida en los dos mundos y el sabotaje quedaba VERDE. Y el `if` temprano tapaba al WHERE,
// que es la guarda de verdad: se podía romper el WHERE sin que nada se pusiera rojo.
//
// Por eso el arreglo sacó el atajo y dejó UNA sola guarda. Ahora esto se puede probar de la
// única forma que prueba algo: en secuencia, mirando quién sobrevive.
//
// Sabotaje que lo hace fallar: sacar `AND estado != ?` del WHERE.
func TestDosReportesDelMismoComandoNoSePisan(t *testing.T) {
	s, d := servidorConMaquina(t)
	nuevoComando := func() fleet.Comando {
		t.Helper()
		c, err := s.engine.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "gio",
			Argv: []string{"echo", "hola"}, Timeout: 30 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	salidaDe := func(id string) (fleet.Comando, string) {
		t.Helper()
		c, ok, err := s.engine.ComandoPorID(id)
		if err != nil || !ok {
			t.Fatalf("no se pudo releer el comando (ok=%v): %v", ok, err)
		}
		return c, c.Stdout
	}

	// ── EL CASO QUE IMPORTA, determinista ───────────────────────────────────────────────────
	c := nuevoComando()
	cero, uno := 0, 1
	if err := s.engine.GuardarResultado(d.ID, c.ID, &cero, "EL-PRIMERO", "", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// El segundo reporte NO devuelve error —el agente no tiene por qué reintentar algo guardado—
	// pero tampoco cambia nada.
	if err := s.engine.GuardarResultado(d.ID, c.ID, &uno, "EL-SEGUNDO", "pisado", "roto", time.Now().UTC()); err != nil {
		t.Fatalf("un reporte tardío devolvió error en vez de irse en silencio: %v", err)
	}
	leido, salida := salidaDe(c.ID)
	if salida != "EL-PRIMERO" {
		t.Errorf("el segundo reporte PISÓ el resultado del primero: %q. La bitácora promete append-once y un resultado que ya se leyó no puede cambiar", salida)
	}
	if leido.ExitCode == nil || *leido.ExitCode != 0 {
		t.Errorf("el código de salida lo escribió el segundo reporte: %v", leido.ExitCode)
	}
	if leido.Stderr != "" || leido.Error != "" {
		t.Errorf("el segundo reporte escribió en los otros campos: stderr=%q error=%q", leido.Stderr, leido.Error)
	}

	// ── Y QUE LA GUARDA NO SEA UN CANDADO: el PRIMER reporte sí tiene que entrar ────────────
	c2 := nuevoComando()
	if err := s.engine.GuardarResultado(d.ID, c2.ID, &cero, "UNICO", "", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if leido2, salida2 := salidaDe(c2.ID); salida2 != "UNICO" || leido2.Estado != fleet.EstadoTerminado {
		t.Errorf("el primer reporte no se guardó: salida=%q estado=%q", salida2, leido2.Estado)
	}

	// ── Y en paralelo tampoco: acá no se puede saber QUIÉN gana, pero sí que gana UNO SOLO y
	//    que su resultado queda entero.
	c3 := nuevoComando()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = s.engine.GuardarResultado(d.ID, c3.ID, &cero, fmt.Sprintf("reporte-%02d", n), "", "", time.Now().UTC())
		}(i)
	}
	wg.Wait()
	leido3, salida3 := salidaDe(c3.ID)
	if !strings.HasPrefix(salida3, "reporte-") || len(salida3) != len("reporte-00") {
		t.Errorf("la salida guardada no es la de un solo reporte: %q", salida3)
	}
	if leido3.Estado != fleet.EstadoTerminado {
		t.Errorf("el comando no quedó terminado: %q", leido3.Estado)
	}
}
