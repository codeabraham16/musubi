package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// sondeoSinT despacha musubi_fleet_probe CON principal desde una goroutine.
//
// No se reusa `llamarSinT`: ése despacha con un contexto pelado, y la sonda exige `metrics` por
// máquina (PuedeSobreDevice). Sin principal las máquinas se saltearían por falta de permiso,
// el sondeo volvería en microsegundos sin tocar el ssh, y esta prueba mediría la nada.
//
// Y no se reusa `callAsPrincipal` porque toma *testing.T: llamar t.Fatal desde otra goroutine es
// un error propio —la prueba seguiría corriendo con el resultado a medias—, que es la misma razón
// por la que existe `llamarSinT`.
func sondeoSinT(s *McpServer, p *Principal, args map[string]any) *RpcError {
	raw, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_fleet_probe", Arguments: raw})
	_, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
	return rpcErr
}

// sshQueCuelga arma el cuerpo del `ssh` falso que NO contesta hasta que se lo suelta.
//
// Es el equivalente de `centralQueCuelga` para esta tool, y tiene que ser un archivo y no un canal
// porque el doble corre en OTRO PROCESO: `EjecutarPorSSH` hace exec.CommandContext sobre el guion,
// así que lo único que comparten la prueba y el falso es el filesystem.
//
//   - `: > entro` avisa que el viaje empezó. Lo hacen todas las máquinas y es idempotente: alcanza
//     con que la primera esté adentro para que la sonda concurrente signifique algo.
//   - el `while` espera a que la prueba cree `soltar`.
//   - recién entonces escupe la lectura buena, para que la corrida SANA termine en milisegundos y
//     el sondeo llegue hasta el latido, que es donde vive el `withWriteLock` que se está midiendo.
func sshQueCuelga(entro, soltar string) string {
	return fmt.Sprintf(`: > %q
while [ ! -f %q ]; do sleep 0.05; done
cat <<'EOF'
%s
EOF`, entro, soltar, lecturaProcFalsa())
}

// esperarArchivo espera a que el falso deje su marca, y distingue el timeout del éxito.
func esperarArchivo(ruta string, limite time.Duration) bool {
	fin := time.Now().Add(limite)
	for time.Now().Before(fin) {
		if _, err := os.Stat(ruta); err == nil {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestSondearLaFlotaNoCongelaElServidor mide el EFECTO de `lockSelf` en musubi_fleet_probe: que el
// candado del despacho no cruce los viajes a los dispositivos.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ EL MOLDE DE LAS CUATRO ANTERIORES NO SERVÍA
//
// promote, install_skill, list_skills y codegraph_index cuelgan un central HTTP y le ponen al
// cliente un timeout de 120 s, más largo que la paciencia de la sonda (`esperaMax`, 30 s). Eso es
// lo único que hace que midan el CANDADO y no el timeout.
//
// Acá no se puede: el viaje no es HTTP sino un fork+exec de ssh, y su techo —
// `sondaTimeoutPorDispositivo`, 15 s— es una CONSTANTE DEL PAQUETE que una prueba no puede subir.
// Con una sola máquina colgada, el contexto de exec.CommandContext la mata a los 15 s, el candado
// se suelta, el escritor entra a los ~15 s y la sonda —que tolera 30— nunca declara bloqueo: la
// prueba quedaría VERDE con el defecto puesto. Es exactamente la trampa que promote documenta,
// con el agravante de que el número no está a mano para corregirlo.
//
// LA SALIDA ES SUMAR VIAJES, NO ALARGAR UNO. Se enrolan varios Tier B —cuántos lo DERIVA la prueba
// de las dos constantes, ver abajo; hoy son tres—: el bucle de `toolFleetProbe` es secuencial, así
// que con el defecto puesto el exclusivo se sostiene 3 × 15 s = 45 s, que SÍ supera los 30 de la
// sonda. Ninguna de las dos mitades alcanza sola —el bloqueo liberable da la corrida sana en
// milisegundos, las máquinas de más dan el cuelgue que sobrevive a la sonda— y por eso el molde
// anterior no se pudo copiar.
//
// Los 15 s son el timeout EFECTIVO y está medido, no supuesto: `EjecutarPorSSH` reescribe el valor
// recibido si sale de rango (`timeout <= 0 || timeout > ComandoTimeoutMax`), y con
// ComandoTimeoutMax = 10 min y ComandoTimeoutDefault = 30 s, 15 s pasa intacto. Si alguien mueve
// esas constantes, el que se rompe es este cálculo.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ HACE FALTA SI YA ESTÁ LA GUARDA ESTRUCTURAL
//
// candado_no_cruza_la_red_test.go mira la FORMA: que la tool declare `lockSelf`. Una tool puede
// declararlo y meter la red adentro de un `withWriteLock` igual — el defecto CON LA MARCA PUESTA,
// que la estructural no ve. Esto mide el EFECTO: se cuelgan dispositivos de verdad y se exige que
// otro escritor entre igual.
//
// LA SONDA ES ESCRITORA a propósito: una de sólo lectura conviviría con un RLock y pasaría igual.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL SABOTAJE NO ES SACARLE `lock: lockSelf` AL REGISTRO, Y ESO COSTÓ DOS VUELTAS EN OTRAS TOOLS
//
// Si el despachador vuelve a tomar el exclusivo, el `withReadLock` con que `toolFleetProbe` lista
// los dispositivos pide el MISMO
// mutex que el despacho ya tiene tomado —sync.RWMutex no es reentrante en ninguna dirección— y el
// handler se traba ANTES de llegar al ssh. La prueba moriría en la espera de arranque («el sondeo
// no llegó a ningún dispositivo»), o sea por la aserción de VIVEZA y no por la del bloqueo. Un rojo
// por la aserción equivocada se lee igual que el correcto y no cubre lo mismo.
//
// Por eso el sabotaje revierte el arreglo AL DEFECTO ORIGINAL en un solo nivel de candado: cambiar
// el `withReadLock` por el Lock+defer que sostiene el exclusivo sobre el bucle entero. Es lo que
// había antes de la conversión, sin anidamiento, y el cuelgue dura TODAS las máquinas.
//
// EFECTO SECUNDARIO CONOCIDO DEL SABOTAJE, escrito para que no sorprenda: tras el t.Fatalf de la
// sonda, el cleanup suelta el centinela, alguna máquina llega al latido y su `withWriteLock` se
// traba contra el Lock del propio handler. La goroutine del sondeo queda colgada para siempre. No
// impide que `go test` termine —una goroutine huérfana no retiene al proceso— y el mutex es del
// servidor de ESTA prueba, así que no contamina a las demás.
//
// Sabotaje que la hace fallar: revertir el `withReadLock` de toolFleetProbe al `dispatchMu.Lock()`
// con `defer` que sostenía el candado exclusivo sobre el bucle entero.
// arnes: archivo="internal/mcp/methods_sonda.go"
// arnes: de="\ts.withReadLock(func() { devices, err = s.engine.ListarDevices(proyecto, false) })"
// arnes: a="\ts.dispatchMu.Lock()\n\tdefer s.dispatchMu.Unlock()\n\tdevices, err = s.engine.ListarDevices(proyecto, false)"
// arnes: prueba="TestSondearLaFlotaNoCongelaElServidor"
func TestSondearLaFlotaNoCongelaElServidor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// EL NÚMERO SE DERIVA DE LAS DOS CONSTANTES QUE DECIDEN EL VEREDICTO, Y NO SE ESCRIBE.
	//
	// Hoy da tres —30 s de paciencia ÷ 15 s por máquina, más una para pasarse— y escribir «3» sería
	// una copia: el día que alguien baje `sondaTimeoutPorDispositivo` a 5 s, tres máquinas serían
	// 15 s de cuelgue contra 30 de espera y ESTA PRUEBA QUEDARÍA VERDE CON EL DEFECTO PUESTO, sin
	// que nadie la tocara. Derivarlo la deja adaptándose sola: con 5 s pediría siete.
	maquinas := int(esperaMax/sondaTimeoutPorDispositivo) + 1
	// Y EL TOPE NO PUEDE RECORTAR EN SILENCIO. `toolFleetProbe` sondea como mucho
	// `sondaMaxDispositivos`: si la derivación pidiera más, las de sobra se saltearían, el cuelgue
	// duraría menos de lo calculado y el verde volvería a no significar nada — pero por un camino
	// que no se ve. Que lo diga en vez de dejarlo pasar.
	if maquinas > sondaMaxDispositivos {
		t.Fatalf("esta prueba necesita %d máquinas para que el cuelgue (%v) supere la paciencia de "+
			"la sonda (%v), y el tope por llamada es %d: el sondeo saltearía las de sobra y el "+
			"cuelgue quedaría corto", maquinas, time.Duration(maquinas)*sondaTimeoutPorDispositivo,
			esperaMax, sondaMaxDispositivos)
	}
	for i := 0; i < maquinas; i++ {
		n := fmt.Sprintf("tierb-%d", i)
		if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
			"name": n, "tier": "B", "caps": []string{"metrics"}, "project": "infra",
			"address": n + ".local", "os": "linux"}); e != nil {
			t.Fatalf("enrolando %s: %+v", n, e)
		}
	}
	t.Logf("%d máquinas Tier B enroladas: con el defecto puesto el exclusivo se sostendría %v, "+
		"contra los %v que tolera la sonda", maquinas,
		time.Duration(maquinas)*sondaTimeoutPorDispositivo, esperaMax)

	dir := t.TempDir()
	entro := filepath.Join(dir, "entro")
	soltar := filepath.Join(dir, "soltar")
	defer fleet.SSHFalsoParaTest(t, sshQueCuelga(entro, soltar))()

	// liberar TIENE que poder llamarse dos veces: una en el camino feliz y otra desde el cleanup.
	// El camino que importa es el segundo — cuando la prueba muere en un t.Fatalf, el `liberar()`
	// del final NO se ejecuta y los guiones quedarían girando en su `while` hasta que el contexto
	// los mate: una cola de `sondaTimeoutPorDispositivo` por máquina después del veredicto.
	liberar := func() { _ = os.WriteFile(soltar, nil, 0o644) }
	t.Cleanup(liberar)

	sondeado := make(chan *RpcError, 1)
	go func() {
		sondeado <- sondeoSinT(s, conMetrics("infra"), map[string]any{})
	}()

	// SE ESPERA A ESTAR ADENTRO DEL VIAJE antes de sondear. Sin esto la sonda podría correr antes
	// de que el sondeo alcance el primer ssh, y la prueba pasaría con el defecto puesto: un verde
	// que mide el momento equivocado.
	if !esperarArchivo(entro, esperaArranque) {
		t.Fatal("el sondeo no llegó a ningún dispositivo: esta prueba no pudo empezar a medir, " +
			"así que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_fleet_probe está colgada recorriendo la flota")

	// SOLTAR ES LO ÚNICO QUE TERMINA LOS VIAJES dentro de un plazo razonable. Sin esto los guiones
	// esperan a que el contexto los mate y la corrida sana tardaría el cuelgue entero —lo mismo que
	// tarda con el defecto puesto— en vez de milisegundos.
	liberar()
	select {
	case rpcErr := <-sondeado:
		if rpcErr != nil {
			t.Fatalf("al soltar los dispositivos, el sondeo falló: %+v", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("soltados los dispositivos, el sondeo igual no volvió: el candado quedó tomado por otra cosa")
	}
}
