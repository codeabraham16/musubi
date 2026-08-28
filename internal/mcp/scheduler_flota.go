package mcp

// scheduler_flota.go es EL LATIDO PROPIO DEL TRACK. Track «Control de flota», S10.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE FALTABA, DICHO EN UNA LÍNEA: hasta acá, todo lo que Musubi hacía con la flota lo hacía
// porque ALGUIEN LLAMÓ UNA TOOL. Un sistema que sólo responde cuando se le pregunta no puede
// enterarse de nada a las 4 de la mañana.
//
// Tres trabajos, un solo ticker (I1):
//
//	1. SONDEAR   — ir a buscar a los que no tienen agente. `musubi_fleet_probe` existía desde S7b
//	               y se podía colgar de un cron, pero nadie lo llamaba solo: un Tier B figuraba
//	               caído aunque estuviera perfecto, porque la única prueba de vida posible es que
//	               vayamos a buscarlo (A19).
//	2. PODAR     — vaciar las salidas viejas de comandos. `PodarSalidasDeComandos` existía y
//	               estaba probada desde S5, y no la llamaba nadie: las salidas no caducaban (A11).
//	3. ACTUAR    — las políticas de auto-heal (A10): reaccionar, no sólo avisar.
//
// UN SOLO TICKER Y NO TRES, y tampoco colgado del mantenimiento de la memoria que ya existe. Esto
// último era la opción fácil y habría atado la caducidad de datos de la flota a
// `maintenance.auto_interval_hours` — un número que alguien puede poner en 0 por razones de
// memoria sin sospechar que con eso apaga otra cosa, en otro subsistema, para siempre.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"musubi/internal/config"
	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// sondasEnParalelo es cuántos dispositivos se miden a la vez dentro de un barrido.
//
// El tope existe por los dos lados: sin él, 40 máquinas son 40 conexiones SSH simultáneas
// saliendo del cerebro; con 1, un barrido secuencial de 40 × 15 s tarda 10 minutos y no entra en
// un tick de 5. Cuatro es el punto donde un barrido grande entra cómodo sin parecer un escaneo.
const sondasEnParalelo = 4

// podaCadaTanto es cada cuánto se vacían las salidas viejas. La poda NO va en cada tick: es un
// UPDATE sobre la tabla de comandos y el tick es de minutos. Una vez por hora alcanza de sobra
// para una retención que se mide en días.
const podaCadaTanto = time.Hour

// ConfigurarFlota deja el servidor listo para su latido propio y valida lo que se puede validar
// SIN el registro de principals delante: la sintaxis de cada política y el destino del empuje OTLP.
//
// La validación va en DOS TIEMPOS y no es una complicación gratuita: esta mitad no necesita el
// registro y por eso puede correr en el arranque de cualquier entrypoint, incluso uno sin archivo
// de principals. La otra mitad —que el principal exista y tenga con qué actuar— la hace
// vincularRegistroDeFlota cuando el registro ya está cargado. Las dos son de ARRANQUE (I12): de
// las dos formas de enterarse de que una política está mal escrita, «el servidor no arranca» es
// mucho más barata que «el disco se llenó igual y nadie sabe por qué».
func (s *McpServer) ConfigurarFlota(cfg config.FleetConfig) error {
	s.sondaIntervalo = cfg.EffectiveProbeInterval()
	s.retencionSalidasDias = cfg.EffectiveOutputRetentionDays()

	politicas := make([]fleet.Politica, 0, len(cfg.Policies))
	vistos := make(map[string]bool, len(cfg.Policies))
	for _, pc := range cfg.Policies {
		pol := fleet.Politica{
			Nombre:    pc.Name,
			Principal: pc.Principal,
			Cuando:    fleet.Condicion(pc.When),
			Supera:    pc.Threshold,
			Sobre:     pc.Devices,
			Hacer:     pc.Run,
			Cooldown:  time.Duration(pc.CooldownMinutes * float64(time.Minute)),
		}
		if err := pol.Validar(); err != nil {
			return err
		}
		// Nombres repetidos: el cooldown y las métricas se llevan POR NOMBRE, así que dos
		// políticas homónimas compartirían las dos cosas — una taparía a la otra sin que nada
		// fallara. Es justo el error que sólo se descubre cuando ya importa.
		if vistos[pol.Nombre] {
			return fmt.Errorf("hay dos políticas llamadas %q: el cooldown y las métricas se llevan por nombre, así que se pisarían entre sí", pol.Nombre)
		}
		vistos[pol.Nombre] = true
		politicas = append(politicas, pol)
	}
	s.politicas = politicas
	if err := s.configurarEmpujeOTLP(cfg.OTLP); err != nil {
		return err
	}
	// El cooldown persistido se recupera ACÁ, antes de que el scheduler pueda arrancar: si se
	// cargara después del primer tick, ese tick correría sin cooldowns y el reinicio seguiría
	// siendo una ventana para actuar de más — que es justo lo que A24 vino a cerrar.
	s.cargarCooldowns()
	return nil
}

// vincularRegistroDeFlota ata las políticas al registro de principals y termina de validarlas.
//
// Se guarda el REGISTRO y no los principales ya resueltos porque el registro se recarga en
// caliente cada 10 s: una política tiene que resolver a su principal en CADA evaluación, para que
// revocar a alguien en principals.yaml apague, en el mismo instante, también lo que actuaba en su
// nombre. Sin esto habría que acordarse de apagar la política en un segundo lugar — y nadie se
// acuerda de un segundo lugar.
func (s *McpServer) vincularRegistroDeFlota(lookup principalResolver) error {
	s.buscarPrincipal = lookup
	for _, pol := range s.politicas {
		if err := s.validarPrincipalDePolitica(pol, lookup); err != nil {
			return err
		}
	}
	if err := s.validarPrincipalDeEmpuje(lookup); err != nil {
		return err
	}
	// Los avisos de intérpretes salen acá, una vez, con el resto del arranque (I10b).
	switch reg := lookup.(type) {
	case *reloadableRegistry:
		avisarDeInterpretes(reg.cur.Load())
	case *PrincipalRegistry:
		avisarDeInterpretes(reg)
	}
	return nil
}

// validarPrincipalDePolitica comprueba lo que sólo se puede saber con el registro delante.
func (s *McpServer) validarPrincipalDePolitica(pol fleet.Politica, lookup principalResolver) error {
	if lookup == nil {
		return fmt.Errorf("política %q: hay políticas configuradas pero no hay registro de principals (principals.yaml). Una política actúa con la autoridad de alguien: sin registro no hay a quién nombrar", pol.Nombre)
	}
	pr, existe := lookup.porNombre(pol.Principal)
	if !existe {
		return fmt.Errorf("política %q: el principal %q no existe en principals.yaml", pol.Nombre, pol.Principal)
	}
	// Un principal SIN ninguna concesión de `exec` deja a la política garantizadamente muerta: va
	// a evaluar, va a dar positivo y no va a poder hacer nada. Descubrirlo durante un incidente es
	// la peor hora posible para descubrirlo.
	if len(pr.Fleet[fleet.CapExec]) == 0 {
		return fmt.Errorf("política %q: el principal %q no tiene ninguna concesión `exec` en su sección `fleet:`, así que la política nunca podría actuar. Las políticas NO tienen autoridad propia: sólo la de su principal", pol.Nombre, pol.Principal)
	}
	return nil
}

// configurarEmpujeOTLP guarda la configuración del empuje y valida lo que se puede validar SIN el
// registro delante: que haya principal nombrado, que el timeout entre en el intervalo, y que el
// destino sea un destino (esquema, host, sin credencial en la URL).
//
// Es la MISMA validación en dos tiempos que las políticas, y por la misma razón: esta mitad corre
// en cualquier entrypoint, incluso uno sin principals.yaml. La otra —que el principal exista y
// tenga con qué exportar— la hace validarPrincipalDeEmpuje cuando el registro ya está cargado.
func (s *McpServer) configurarEmpujeOTLP(cfg config.OTLPPushConfig) error {
	s.empujeCfg = cfg
	s.empujador = nil
	if !cfg.Activo() {
		return nil // apagado: ni endpoint, o un intervalo negativo (el apagado explícito)
	}
	if strings.TrimSpace(cfg.Principal) == "" {
		return fmt.Errorf("fleet.otlp.endpoint está configurado pero fleet.otlp.principal está vacío. El empuje exporta CON LA AUTORIDAD DE ALGUIEN y no hay default posible: el default sería «todos los tenants». Nombrá un principal de principals.yaml que tenga `fleet: {metrics: [...]}`")
	}
	if to, iv := cfg.EffectiveTimeout(), cfg.EffectiveInterval(); to >= iv {
		return fmt.Errorf("fleet.otlp.timeout_seconds (%s) no puede ser mayor o igual que interval_seconds (%s): cada empuje lento se comería el siguiente tick y el lazo pasaría la vida salteando. Bajá el timeout o subí el intervalo", to, iv)
	}
	emp, err := nuevoEmpujadorOTLP(cfg)
	if err != nil {
		return err
	}
	s.empujador = emp
	return nil
}

// validarPrincipalDeEmpuje comprueba lo que sólo se puede saber con el registro delante.
//
// ES UN ERROR DE ARRANQUE, no un warning. Un empuje que nombra a alguien que no existe queda mudo
// para siempre —y mudo es exactamente igual que «todo tranquilo» desde afuera—, así que nadie se
// entera hasta un incidente en el que los gráficos están vacíos.
func (s *McpServer) validarPrincipalDeEmpuje(lookup principalResolver) error {
	if !s.empujeCfg.Activo() {
		return nil
	}
	nombre := strings.TrimSpace(s.empujeCfg.Principal)
	ejemplo := fmt.Sprintf("Declaralo en principals.yaml:\n  - name: %s\n    token_sha256: \"<sha256 de su token>\"\n    role: reader\n    read: all\n    fleet:\n      metrics: [\"*\"]", nombre)
	if lookup == nil {
		return fmt.Errorf("el empuje OTLP nombra al principal %q pero no hay registro de principals. %s", nombre, ejemplo)
	}
	pr, existe := lookup.porNombre(nombre)
	if !existe {
		return fmt.Errorf("el empuje OTLP nombra al principal %q, que no existe en principals.yaml. %s", nombre, ejemplo)
	}
	// Sin ninguna concesión `metrics` el empuje queda garantizadamente vacío: va a resolver, va a
	// barrer y no va a ver una sola máquina (C1 — el rol NO otorga capacidades de flota, ni
	// siquiera el de admin). Un POST con cero series cada 30 s es peor que no empujar: parece que
	// anda.
	if len(pr.Fleet[fleet.CapMetrics]) == 0 {
		return fmt.Errorf("el empuje OTLP nombra al principal %q, que no tiene ninguna concesión `metrics` en su sección `fleet:`, así que no exportaría ni una máquina. Las capacidades de flota NO se derivan del rol. %s", nombre, ejemplo)
	}
	return nil
}

// avisarDeInterpretes logea las allowlists que permiten un intérprete (I10b).
func avisarDeInterpretes(reg *PrincipalRegistry) {
	if reg == nil {
		return
	}
	for i := range reg.principals {
		for _, aviso := range avisosDeInterpretes(reg.principals[i]) {
			logx.Warn("allowlist de flota: " + aviso)
		}
	}
}

// RunFlotaScheduler corre el barrido en un ticker hasta que ctx se cancela. interval<=0 lo
// desactiva. Pensado para su propia goroutine; bloquea hasta la cancelación.
//
// NO TOMA dispatchMu, por el mismo motivo que RunOutboxScheduler: el barrido hace I/O de red
// (segundos por máquina) y tomar el candado global congelaría todas las tools mientras un router
// se toma sus quince segundos para no contestar.
func (s *McpServer) RunFlotaScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.barrerFlotaUnaVez(ctx)
		}
	}
}

// barrerFlotaUnaVez hace UN barrido completo: sondear, evaluar políticas, podar.
func (s *McpServer) barrerFlotaUnaVez(ctx context.Context) {
	// I5 — un barrido que sigue corriendo no arranca otro. Con 40 máquinas por SSH, dos barridos
	// solapados son 80 conexiones simultáneas; y si un tick tarda más que el intervalo (una red
	// lenta, un dispositivo colgado), solaparse es la regla y no la excepción.
	if !s.flotaBusy.CompareAndSwap(false, true) {
		logx.Warn("flota: el barrido anterior sigue corriendo; se saltea este tick (¿el intervalo es más corto que lo que tarda un barrido?)")
		return
	}
	defer s.flotaBusy.Store(false)

	inicio := time.Now()
	proyectos, err := s.engine.ProyectosConDevices(proyectosParaExportar)
	if err != nil {
		logx.Error("flota: no se pudieron listar los proyectos con dispositivos", "error", err)
		return
	}
	var sondeados, fallidos, acciones int
	for _, proy := range proyectos {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ok, mal := s.sondearProyecto(ctx, proy)
		sondeados += ok
		fallidos += mal
		acciones += s.aplicarPoliticas(proy, time.Now())
	}
	podadas := s.podarSalidasSiToca(time.Now())
	s.podarEstadoDePoliticasSiToca()
	// Los techos de las sesiones de shell los aplica EL CEREBRO (S5b · T5). Si dependieran de la
	// máquina remota, una máquina comprometida se los saltearía — y esa máquina es justamente
	// aquélla de la que uno se protege al ponerle un techo a una sesión de shell.
	if n := s.cerrarShellsVencidas(time.Now()); n > 0 {
		logx.Info("flota: sesiones de shell cerradas por vencimiento", "sesiones", n)
	}

	// Se anuncia sólo cuando HUBO trabajo, como el resto del scheduler: un log por tick en un
	// daemon que vive días es ruido que entierra la línea que sí importa.
	if sondeados+fallidos+acciones > 0 || podadas > 0 {
		logx.Info("flota: barrido",
			"sondeados", sondeados, "sin_respuesta", fallidos,
			"acciones", acciones, "salidas_podadas", podadas,
			"dur", time.Since(inicio).String())
	}
}

// sondearProyecto mide los dispositivos sin agente de un proyecto, en paralelo acotado.
//
// CORRE SIN PRINCIPAL, y eso NO lo convierte en una puerta lateral (I4). Escribe en la base; no
// le devuelve nada a nadie. Todo camino de LECTURA —la tool, el panel, /metrics— sigue pasando por
// PuedeSobreDevice con la credencial de quien pregunta. Que el cerebro sepa algo no es que vos
// puedas verlo. Lo que sí se respeta es el eje del APARATO (`Permite`), que no habla de personas:
// una máquina a la que no se le concedió `metrics` no se mide, la pida quien la pida.
func (s *McpServer) sondearProyecto(ctx context.Context, proyecto string) (ok, fallidos int) {
	devices, err := s.engine.ListarDevices(proyecto, false)
	if err != nil {
		logx.Error("flota: no se pudieron listar los dispositivos", "proyecto", proyecto, "error", err)
		return 0, 0
	}
	var aSondear []fleet.Device
	for _, d := range devices {
		// Tier A tiene agente propio: sondearlo sería ir a buscar lo que ya viene solo, por un
		// camino que además no existe (nada le entra a esa máquina).
		if d.Tier == fleet.TierAgente {
			continue
		}
		// iOS se saltea SIN INTENTAR: el techo es de la plataforma, no del cable, y un error de
		// adb por tick mandaría a alguien a depurar lo que no se puede arreglar.
		if fleet.EsIOS(d.OS) {
			continue
		}
		if !d.Permite(fleet.CapMetrics) {
			continue
		}
		aSondear = append(aSondear, d)
	}
	if len(aSondear) == 0 {
		return 0, 0
	}

	ahora := time.Now()
	var mu sync.Mutex
	var wg sync.WaitGroup
	cola := make(chan fleet.Device)
	obreros := sondasEnParalelo
	if len(aSondear) < obreros {
		obreros = len(aSondear)
	}
	for i := 0; i < obreros; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range cola {
				fila := s.sondearUno(d, ahora)
				mu.Lock()
				if bien, _ := fila["ok"].(bool); bien {
					ok++
				} else {
					fallidos++
				}
				mu.Unlock()
			}
		}()
	}
	for _, d := range aSondear {
		select {
		case <-ctx.Done():
			close(cola)
			wg.Wait()
			return ok, fallidos
		case cola <- d:
		}
	}
	close(cola)
	wg.Wait()
	return ok, fallidos
}

// podarSalidasSiToca vacía stdout/stderr de los comandos viejos, como mucho una vez por hora.
//
// I6 — NO BORRA LA FILA. Qué se ejecutó, quién y cuándo es permanente; lo que caduca es el
// CONTENIDO de la salida, que es donde aparecen rutas, hostnames y de vez en cuando algo que no
// debería estar ahí. Son dos retenciones distintas porque son dos riesgos distintos: perder la
// auditoría es un problema de gobierno, guardar salidas para siempre es uno de privacidad.
func (s *McpServer) podarSalidasSiToca(ahora time.Time) int64 {
	if s.retencionSalidasDias <= 0 {
		return 0 // desactivado explícitamente: las salidas no caducan
	}
	if !s.ultimaPoda.IsZero() && ahora.Sub(s.ultimaPoda) < podaCadaTanto {
		return 0
	}
	s.ultimaPoda = ahora
	n, err := s.engine.PodarSalidasDeComandos(s.retencionSalidasDias, ahora)
	if err != nil {
		logx.Error("flota: no se pudieron podar las salidas viejas", "error", err)
		return 0
	}
	return n
}

// podarEstadoDePoliticasSiToca borra las filas de cooldown de políticas que ya no existen.
//
// Cuelga de la misma cadencia que la poda de salidas: es una limpieza, no una operación de cada
// tick. Sin esto, cada política que alguien renombra o saca deja su fila para siempre — una tabla
// que sólo crece, alimentada por un archivo que se edita a mano, y que después nadie se anima a
// limpiar porque no sabe si importa.
func (s *McpServer) podarEstadoDePoliticasSiToca() {
	if len(s.politicas) == 0 {
		return // con la lista vacía la poda es un no-op deliberado: ver PodarEstadoDePoliticas
	}
	if !s.ultimaPoda.Equal(s.ultimaPodaDePoliticas) {
		s.ultimaPodaDePoliticas = s.ultimaPoda
	} else {
		return
	}
	vivas := make([]string, 0, len(s.politicas))
	for _, p := range s.politicas {
		vivas = append(vivas, p.Nombre)
	}
	if n, err := s.engine.PodarEstadoDePoliticas(vivas); err != nil {
		logx.Error("políticas: no se pudo podar el estado viejo", "error", err)
	} else if n > 0 {
		logx.Info("políticas: cooldowns de políticas que ya no existen, borrados", "filas", n)
	}
}
