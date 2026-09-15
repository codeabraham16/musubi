package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// despacharConCtx despacha una tool con principal Y con un contexto propio, sin depender de *testing.T.
//
// Las tres cosas hacen falta juntas y ninguna la daba un helper existente:
//   - CON PRINCIPAL, porque exec exige la capacidad por máquina y sin ella el handler rechaza antes
//     de llegar a lo que se mide (`callAsPrincipal` lo hace, pero toma *testing.T).
//   - SIN *testing.T, porque esto corre en otra goroutine y llamar t.Fatal desde ahí es un error
//     propio: la prueba seguiría con el resultado a medias (la razón por la que existe `llamarSinT`).
//   - CON CONTEXTO PROPIO, que es lo que ninguno de los dos da. `esperarComando` respeta ctx.Done(),
//     así que cancelarlo es lo único que corta el bucle de polling sin esperar los 35 s de paciencia.
//     Sin esto la corrida SANA de la prueba del bucle tardaría eso entero.
func despacharConCtx(ctx context.Context, s *McpServer, p *Principal, tool string, args map[string]any) *RpcError {
	raw, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: raw})
	_, rpcErr := s.handleToolsCall(withPrincipal(ctx, p), params)
	return rpcErr
}

// TestEjecutarEnUnTierBNoCongelaElServidor mide el PRIMERO de los dos cuelgues de musubi_fleet_exec:
// el SSH sincrónico de un Tier B.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ESTA ES LA MÁS LARGA DE TODO EL TRINQUETE
//
// `EjecutarPorSSH` hace `cmd.Run()` — espera de verdad, no es un `Start()` que vuelve. Su techo es
// `ComandoTimeoutMax`, DIEZ MINUTOS, y el llamador elige cuánto dentro de ese rango con
// `timeout_seg`. Antes de la conversión eso corría con el candado EXCLUSIVO del despacho tomado (la
// tool no declara `readOnly`), así que un solo exec sobre un Tier B lento dejaba al servidor entero
// sin atender a nadie durante ese rato.
//
// Y POR ESO ACÁ NO HACE FALTA EL TRUCO DE `fleet_probe`. Aquella prueba tuvo que enrolar VARIAS
// máquinas porque su timeout por dispositivo es una constante del paquete (15 s) más corta que la
// paciencia de la sonda (30 s), así que un solo viaje se moría antes de que la sonda declarara
// bloqueo. Acá el timeout lo manda el llamador: UNA máquina con `timeout_seg` al tope sostiene el
// cuelgue todo lo que haga falta. El camino Tier B sale en methods_exec.go ANTES de la guarda de
// `esperaMaxExec`, así que un timeout largo no se encola: se ejecuta y se espera.
//
// EL SSH FALSO BLOQUEA HASTA QUE LA PRUEBA LO SUELTA, y esa es la parte que hace que la corrida sana
// siga siendo de milisegundos: con el candado bien puesto el escritor entra enseguida, se libera el
// centinela y el exec termina. Sin el centinela habría que esperar a que el contexto matara al ssh.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL SABOTAJE NO ES SACARLE `lock: lockSelf` AL REGISTRO
//
// Si el despachador vuelve a tomar el exclusivo, el `withReadLock` con que el handler lee el device
// pide el MISMO mutex que el despacho ya tiene —sync.RWMutex no es reentrante en ninguna dirección—
// y se traba ANTES de llegar al ssh: la prueba moriría en la espera de arranque, o sea por la
// aserción de VIVEZA y no por la del bloqueo. Un rojo por la aserción equivocada se lee igual que el
// correcto y no cubre lo mismo.
//
// El sabotaje declarado mete la red ADENTRO del `withWriteLock` que ya existe en `correrPorSSH`: es
// el defecto CON LA MARCA PUESTA —declarar `lockSelf` y encerrar el viaje igual—, que es
// exactamente lo que la guarda estructural NO puede ver, porque ella sólo mira si la tool declara la
// clase. Un solo nivel de candado, sin anidamiento.
//
// Sabotaje que la hace fallar: envolver la llamada a EjecutarPorSSH en el withWriteLock de
// correrPorSSH, en vez de dejarla afuera.
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\tres := fleet.EjecutarPorSSH(d.Address, cmd.Argv, timeout)"
// arnes: a="\tvar res fleet.ResultadoRemoto\n\ts.withWriteLock(func() { res = fleet.EjecutarPorSSH(d.Address, cmd.Argv, timeout) })"
// arnes: prueba="TestEjecutarEnUnTierBNoCongelaElServidor"
func TestEjecutarEnUnTierBNoCongelaElServidor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "router", "tier": "B", "caps": []string{"metrics", "exec"},
		"project": "infra", "address": "gio@router.local", "os": "linux",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}

	dir := t.TempDir()
	entro := filepath.Join(dir, "entro")
	soltar := filepath.Join(dir, "soltar")
	defer fleet.SSHFalsoParaTest(t, sshQueCuelga(entro, soltar))()

	// liberar TIENE que poder llamarse dos veces: una en el camino feliz y otra desde el cleanup.
	// El que importa es el segundo — si la prueba muere en un t.Fatalf, el `liberar()` del final no
	// se ejecuta y el guion se queda girando hasta que el contexto lo mate, diez minutos después.
	liberar := func() { _ = os.WriteFile(soltar, nil, 0o644) }
	t.Cleanup(liberar)

	// EL TIMEOUT VA AL TOPE, Y ES LO QUE HACE QUE ESTO MIDA EL CANDADO Y NO UN RELOJ. Con el
	// default (30 s) el ssh moriría antes de que la sonda —que espera hasta esperaMax— declarara
	// bloqueo, y la prueba quedaría VERDE con el defecto puesto: mediría el timeout, no el candado.
	// Se deriva de la constante que lo gobierna en vez de escribir «600».
	topeSeg := int(fleet.ComandoTimeoutMax.Seconds())
	if time.Duration(topeSeg)*time.Second <= esperaMax {
		t.Fatalf("ComandoTimeoutMax (%v) ya no supera la paciencia de la sonda (%v): con este techo "+
			"el ssh se muere antes de que la sonda declare bloqueo y esta prueba mediría el reloj",
			fleet.ComandoTimeoutMax, esperaMax)
	}

	ejecutado := make(chan *RpcError, 1)
	go func() {
		ejecutado <- despacharConCtx(context.Background(), s, conExec("infra"), "musubi_fleet_exec",
			map[string]any{"device": "router", "argv": []string{"uptime"}, "timeout_seg": topeSeg})
	}()

	// SE ESPERA A ESTAR ADENTRO DEL VIAJE antes de sondear. Sin esto la sonda podría correr antes de
	// que el exec alcance el ssh, y la prueba pasaría con el defecto puesto: un verde que mide el
	// momento equivocado.
	if !esperarArchivo(entro, esperaArranque) {
		t.Fatal("el exec no llegó al ssh: esta prueba no pudo empezar a medir, así que su verde no " +
			"significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_fleet_exec está colgada esperando el SSH de un Tier B")

	liberar()
	select {
	case rpcErr := <-ejecutado:
		if rpcErr != nil {
			t.Fatalf("al soltar el ssh, el exec falló: %+v", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("soltado el ssh, el exec igual no volvió: el candado quedó tomado por otra cosa")
	}
}

// TestEsperarElResultadoNoCongelaElServidor mide el SEGUNDO cuelgue de musubi_fleet_exec, y es el
// que NINGUNA guarda estructural podía encontrar.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// NO ES UNA LLAMADA DE RED, Y POR ESO SE ESCAPÓ
//
// En Tier A no hay SSH: el comando se encola y el agente lo levanta en su próximo latido. El handler
// espera el resultado releyendo la bitácora cada `esperaPasoExec` (250 ms) hasta `esperaMaxExec`
// (45 s). Eso corría ENTERO con el candado exclusivo del despacho tomado.
//
// La guarda de candado_no_cruza_la_red_test.go busca llamadas a las salidas del paquete `fleet` —
// EjecutarPorSSH, AbrirShellPorSSH, TomarMuestraRemota— y este bucle no llama a ninguna: le pega a
// la base local. O sea que la tool podía «salir del trinquete» sin que este cuelgue se tocara, y
// nadie se habría enterado. Un servidor congelado 45 s se siente igual venga de un socket o de un
// bucle contra SQLite.
//
// EL CONTEXTO SE CANCELA DESPUÉS DE SONDEAR, y es lo que mantiene barata la corrida sana:
// `esperarComando` respeta ctx.Done(), así que en cuanto la aserción pasó se corta el bucle en vez
// de esperar los 35 s de paciencia. Con el defecto puesto la prueba ya falló antes de llegar ahí.
//
// Sabotaje que la hace fallar: sostener el candado a través de TODO el bucle en vez de tomarlo y
// soltarlo por vuelta.
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\t\ts.withReadLock(func() { c, existe, err = s.engine.ComandoPorID(id) })"
// arnes: a="\t\ts.dispatchMu.RLock()\n\t\tdefer s.dispatchMu.RUnlock()\n\t\tc, existe, err = s.engine.ComandoPorID(id)"
// arnes: prueba="TestEsperarElResultadoNoCongelaElServidor"
func TestEsperarElResultadoNoCongelaElServidor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio") // Tier A

	// LA MÁQUINA TIENE QUE FIGURAR VIVA O ESTA PRUEBA MIDE EL CAMINO EQUIVOCADO. Con LastSeen en
	// cero, el handler corta en la guarda de `EnLinea` y devuelve «encolado» sin llegar nunca al
	// bucle: la prueba quedaría VERDE sin haber tocado lo que dice medir.
	d, _, err := s.engine.DevicePorNombre("casa", "pc-gio")
	if err != nil {
		t.Fatalf("DevicePorNombre: %v", err)
	}
	if _, err := s.engine.LatirDevice(d.ID, time.Now(), ""); err != nil {
		t.Fatalf("LatirDevice: %v", err)
	}

	// EL TIMEOUT VA CORTO Y DEBAJO DE esperaMaxExec A PROPÓSITO: por encima, el handler encola y
	// vuelve (methods_exec.go lo dice) sin entrar al bucle. Se deriva de la constante en vez de
	// escribir un número: si alguien baja `esperaMaxExec`, esto se entera.
	cortoSeg := int(esperaMaxExec.Seconds()) / 4
	if cortoSeg < 1 {
		cortoSeg = 1
	}

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	esperado := make(chan *RpcError, 1)
	go func() {
		// NADIE va a levantar este comando: no hay agente corriendo. Así que el handler entra al
		// bucle de relectura y se queda ahí hasta la paciencia o hasta que se cancele el contexto.
		esperado <- despacharConCtx(ctx, s, conExec("casa"), "musubi_fleet_exec",
			map[string]any{"device": "pc-gio", "argv": []string{"true"}, "timeout_seg": cortoSeg})
	}()

	// SE ESPERA A ESTAR ADENTRO DEL BUCLE. Acá no hay un proceso externo que deje una marca, así que
	// la señal es que el comando ya está ENCOLADO: el `EncolarComando` es la última cosa que el
	// handler hace antes de entrar a esperar.
	if !esperarQueHayaUnComando(s, "casa", esperaArranque) {
		t.Fatal("el exec no llegó a encolar el comando: esta prueba no pudo empezar a medir, así " +
			"que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_fleet_exec está colgada releyendo la bitácora a la espera del resultado")

	// Cortar el bucle por donde el código ya sabe cortarlo.
	cancelar()
	select {
	case rpcErr := <-esperado:
		if rpcErr != nil {
			t.Fatalf("al cancelar el contexto, el exec falló: %+v", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("cancelado el contexto, el exec igual no volvió: el candado quedó tomado por otra cosa")
	}
}

// esperarQueHayaUnComando espera a que el handler haya encolado, que es la señal de que ya entró al
// bucle de relectura. Se pregunta por la BITÁCORA y no por un reloj: dormir un rato fijo ataría la
// prueba a la velocidad de la máquina, que es cómo se llega a un rojo que sólo aparece en el runner.
func esperarQueHayaUnComando(s *McpServer, proyecto string, limite time.Duration) bool {
	fin := time.Now().Add(limite)
	for time.Now().Before(fin) {
		if cs, err := s.engine.BitacoraDeComandos(proyecto, "", 10); err == nil && len(cs) > 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
