// Command musubi es la CLI y el daemon MCP de Musubi: instala el binario, prepara
// el workspace, corre mantenimiento y sirve el servidor MCP de memoria persistente.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"musubi/internal/buildid"
	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/mcp"
	"musubi/internal/memory"
)

// resolveProjectID determina el identificador de proyecto que se estampa en cada
// observación (memoria híbrida local+central): usa cfg.ProjectID si está seteado; si no,
// deriva el basename del path absoluto del workspace. Model-free y determinista.
func resolveProjectID(cfg config.Config, root string) string {
	if strings.TrimSpace(cfg.ProjectID) != "" {
		return cfg.ProjectID
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return filepath.Base(abs)
}

// version es la versión del binario. Se inyecta en el release vía
// -ldflags "-X main.version=<tag>"; en builds locales queda "dev".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		noArgs()
		return
	}

	command := os.Args[1]
	switch command {
	case "init":
		initProject()
	case "setup":
		runSetup(os.Args[2:])
	case "provision":
		runProvision(os.Args[2:])
	case "detect":
		runDetect()
	case "turn":
		runTurn()
	case "arnes":
		runArnes(os.Args[2:])
	case "receipt":
		runReceipt(os.Args[2:])
	case "precheck":
		runPrecheck()
	case "precompact":
		runPrecompact()
	case "capture":
		runCapture(os.Args[2:])
	case "ingest":
		runIngest(os.Args[2:])
	case "catalog":
		runCatalog(os.Args[2:])
	case "daemon":
		runDaemon()
	case "cerebro":
		runCerebro(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	case "maintain":
		runMaintain()
	case "backup":
		runBackup(os.Args[2:])
	case "token":
		runToken(os.Args[2:])
	case "embed":
		runEmbed(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
	case "dashboard":
		runDashboard(os.Args[2:])
	case "version", "--version", "-v":
		// `--esquema` imprime SÓLO el entero, para que un guion lo consuma sin parsear.
		//
		// La salida por default no cambia ni gana una línea: `redesplegar-cerebro.sh` la compara
		// entera contra la versión que instaló, y `construir.sh` la imprime. Un segundo renglón
		// acá rompería las dos, y el modo de falla sería un rollback automático en mitad de un
		// despliegue que en realidad salió bien.
		if len(os.Args) > 2 && os.Args[2] == "--esquema" {
			fmt.Println(memory.EsquemaEsperado())
			return
		}
		// `--json` imprime la identidad COMPLETA de este binario, derivada y no tipeada. Es lo
		// que un guion de despliegue tiene que comparar en vez de la cadena de versión: la
		// versión sola no dice a qué esquema migra ni qué catálogo expone, que son las dos cosas
		// que rompen cuando dos máquinas de la malla no corren el mismo build.
		//
		// Va como bandera y no como renglón nuevo de la salida normal por el mismo motivo que
		// --esquema: `redesplegar-cerebro.sh` compara la salida por default ENTERA.
		if len(os.Args) > 2 && os.Args[2] == "--json" {
			n, sha := mcp.CatalogFingerprint()
			b, err := json.MarshalIndent(buildid.Derive(version, n, sha), "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "no pude serializar la identidad: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(b))
			return
		}
		fmt.Printf("musubi %s\n", version)
	case "update":
		runUpdate()
	case "fetch":
		runFetch(os.Args[2:])
	case "calibrate":
		runCalibrate(os.Args[2:])
	case "conflicts":
		runConflicts(os.Args[2:])
	case "shell":
		// La terminal interactiva (S5b). Va como subcomando propio y no como una bandera de otro:
		// es lo único de la CLI que toma el control de la terminal, y eso merece verse en el nombre.
		runShell(os.Args[2:])
	case "agent":
		runAgent(os.Args[2:])
	default:
		fmt.Printf("Comando desconocido: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(cBold("Uso:") + " musubi <comando> [argumentos]")

	// section imprime un encabezado de grupo; cmd imprime un comando alineado. El
	// padding se aplica ANTES de colorear, así las secuencias ANSI no descuadran la
	// columna (cuando el color está apagado, queda igual de alineado).
	section := func(title string) { fmt.Println("\n" + cCyan(title)) }
	cmd := func(name, desc string) { fmt.Printf("  %s  %s\n", cBold(fmt.Sprintf("%-32s", name)), desc) }

	section("Instalación")
	cmd("setup [--agent <claude|cursor>]", "Inyecta Musubi en el proyecto actual (workspace + MCP + hooks)")
	cmd("init", "Inicializa solo el workspace .musubi/ (config + base de datos)")
	cmd("provision [--brain ...] [--dry-run]", "Une esta máquina al cerebro central (red + .mcp.json + verificación)")

	section("Servidor MCP")
	cmd("daemon", "Arranca el servidor MCP sobre stdin/stdout")
	cmd("cerebro", "Canal MCP (stdio) al cerebro CENTRAL: consulta en vivo, no replica")
	cmd("serve [--addr host:port]", "Servidor MCP sobre HTTP (modo servicio, opt-in; solo loopback)")

	section("Flota")
	cmd("agent [--brain <url>] [--once]", "Late contra el cerebro para que esta máquina figure en la flota")
	cmd("shell <maquina> [--project <id>]", "Terminal interactiva en una máquina de la flota (exige la capacidad `shell`, aparte de `exec`)")

	section("Memoria")
	cmd("maintain", "Fusiona casi-duplicados y archiva memorias frías")
	cmd("backup [--out <dir>]", "Snapshot consistente de la base (VACUUM INTO); imprime la ruta")
	cmd("token <new|list|revoke>", "Gestiona el registro de identidad del cerebro (tokens por-miembro)")
	cmd("doctor", "Diagnostica la memoria; 'doctor repair --check X --apply' repara")
	cmd("export [--out <ruta>]", "Vuelca un snapshot JSON (salud + tokens + grafo) para dashboards")
	cmd("dashboard [--addr ...] [--no-open]", "UI local de la memoria en vivo (solo lectura · loopback · 0 tokens)")
	cmd("calibrate", "(opt-in) Mide el estimador de tokens vs count_tokens (requiere ANTHROPIC_API_KEY)")
	cmd("conflicts backfill [--dry-run]", "Reconstruye el desglose lex/coseno de las relaciones que se guardaron sin él")
	cmd("conflicts shadow [--json]", "Lee el modo sombra: dónde el motor coincidió con el detector (y nunca lo corrigió)")

	section("Ingesta")
	cmd("ingest [--as ...] [--lang ...] [--json] <url>", "Convierte un link (video/red social/artículo) en texto; --save lo guarda en memoria")

	section("Catálogo de skills")
	cmd("catalog validate", "Valida un index.json de catálogo de skills")
	cmd("catalog merge <url> [--output <ruta>]", "Obtiene y fusiona un catálogo remoto en index.json")
	cmd("catalog harvest [--seeds ...] [--top N]", "Cosecha un catálogo estático de skills desde GitHub")

	section("Binario")
	cmd("update", "Descarga el último release, verifica el checksum y se auto-reemplaza")
	cmd("fetch <url>", "Baja una URL del tailnet a stdout (transporte de auto-update del cuerpo)")
	cmd("receipt <emit|check|show|install-hook>", "Gate de entrega: el push exige un recibo para ESTA huella del árbol")
	cmd("version", "Muestra la versión del binario")

	section("Hooks (uso interno de Claude Code)")
	cmd("detect [--hook-mode]", "Detecta el stack / SessionStart: auto-descubrimiento + priming")
	cmd("turn --hook-mode", "UserPromptSubmit: inyecta contexto relevante al prompt")
	cmd("precheck --hook-mode", "PreToolUse: gist antes de leer; radio de impacto antes de editar")
	cmd("capture --hook-mode", "Stop: captura los commits nuevos como memoria (red de seguridad)")
	cmd("precompact --hook-mode", "PreCompact: avisa de bajar lo durable ANTES de que se resuma")
}

// runMaintain corre el auto-mantenimiento de la memoria (consolidar + olvidar)
// como proceso one-shot e imprime un resumen en stdout.
func runMaintain() {
	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error al preparar workspace: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar configuración: %v\n", err)
		os.Exit(1)
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	rep, err := maintenanceCycle(engine, cfg.Maintenance)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error en el mantenimiento: %v\n", err)
		os.Exit(1)
	}
	_ = engine.MarkMaintenanceNow()

	fmt.Printf("Mantenimiento de memoria completo:\n")
	fmt.Printf("  Consolidación: %d fusionadas de %d escaneadas\n", rep.Consolidate.Merged, rep.Consolidate.Scanned)
	fmt.Printf("  Olvido: %d archivadas de %d escaneadas\n", rep.Decay.Archived, rep.Decay.Scanned)
	fmt.Printf("  Cuota: %d evictadas por techo de crecimiento\n", rep.Evicted)
	fmt.Printf("  Retención: %d purgadas\n", rep.Purged)
}

// maintenanceCycle corre el ciclo de mantenimiento completo (consolidar + olvidar +
// purgar + compactar) con la config dada, delegando en engine.Maintain. Lo usan el
// subcomando `maintain` y el auto-mantenimiento del daemon.
func maintenanceCycle(engine *memory.DbEngine, m config.MaintenanceConfig) (memory.MaintenanceReport, error) {
	return engine.Maintain(memory.MaintenanceOptions{
		DedupThreshold:         m.DedupThreshold,
		DecayHalfLifeDays:      m.DecayHalfLifeDays,
		DecayMinSalience:       m.DecayMinSalience,
		DecayMinAgeDays:        m.DecayMinAgeDays,
		DecayProtectImportance: m.DecayProtectImportance,
		DecayReinforcementK:    m.DecayReinforcementK,
		PurgeArchivedAfterDays: m.PurgeArchivedAfterDays,
		MaxActivePerProject:    m.MaxActivePerProject,
		Vacuum:                 m.Vacuum,
	})
}

// runServe arranca el servidor MCP sobre HTTP (modo servicio, Track 4). Es opt-in:
// requiere service.enabled en la config o un --addr explícito. Solo bind a loopback.
// Comparte toda la configuración del motor y las tools con el modo daemon (stdio).
func runServe(args []string) {
	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error al preparar workspace: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar configuración: %v\n", err)
		os.Exit(1)
	}

	// Overrides por flag: --addr <host:port> (o --addr=...) habilita el modo servicio
	// con esa dirección; --enable lo habilita con la addr de la config.
	svc := cfg.Service
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--addr" && i+1 < len(args):
			svc.Addr = args[i+1]
			svc.Enabled = true
			i++
		case strings.HasPrefix(args[i], "--addr="):
			svc.Addr = strings.TrimPrefix(args[i], "--addr=")
			svc.Enabled = true
		case args[i] == "--enable":
			svc.Enabled = true
		}
	}
	if !svc.Enabled {
		fmt.Fprintln(os.Stderr, "musubi serve: el modo servicio está desactivado. Activá 'service.enabled: true' en .musubi/config.yaml o pasá --addr <host:port>.")
		os.Exit(1)
	}
	if svc.Addr == "" {
		svc.Addr = config.Default().Service.Addr
	}
	if svc.RequestTimeoutSeconds == 0 {
		svc.RequestTimeoutSeconds = config.Default().Service.RequestTimeoutSeconds
	}

	// Proveedor de embeddings con auto-detección + degradación elegante (16.2f): enciende la
	// semántica si hay tabla en la ubicación estándar; si no (o ante error), recall léxico.
	embedder := resolveEmbedder(cfg, root)
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()
	// Estampar el proyecto de origen en las observaciones (memoria híbrida local+central).
	engine.SetProjectID(resolveProjectID(cfg, root))
	// Los géneros que este despliegue declara LIBRO MAYOR: los necesita el doctor para podar la
	// cola con la misma regla con la que la detección deja de llenarla.
	engine.SetLedgerPrefixes(cfg.Conflicts.LedgerPrefixes)

	// Aviso de cambio de modelo de embedding (homogeneidad de vectores): si el modelo
	// activo cambió y hay vectores viejos de otro modelo, se logea un warning.
	if embedding.Enabled(embedder) {
		// Procedencia del vector (F2.2): cada embedding que escriba este engine lleva este
		// model_id, y la búsqueda semántica sólo compara vectores de la misma procedencia
		// (regla de homogeneidad). Debe fijarse ANTES de servir pedidos.
		engine.SetVectorModelID(embedder.Name())
		engine.WarnOnEmbedModelSwitch(embedder.Name())
		// M3: además de AVISAR del hueco, cerrarlo. Re-embebe en background la memoria que no
		// tiene vector de este modelo (la previa a encender la semántica, o la de otra tabla tras
		// un cambio de checksum/modelo). Sin esto, cambiar de modelo apaga el recall semántico
		// hasta un `musubi embed backfill` manual. No bloquea el arranque.
		autoBackfill(engine, embedder)
	}

	server := mcp.NewMcpServer(engine, root, embedder, mcp.WithSourcing(cfg.Sourcing), mcp.WithMemory(cfg.Memory), mcp.WithMaintenance(cfg.Maintenance), mcp.WithGraph(cfg.Graph), mcp.WithConflicts(cfg.Conflicts), mcp.WithPipeline(cfg.Pipeline), mcp.WithMultiAgent(cfg.MultiAgent), mcp.WithQuota(cfg.Service.EffectiveQuotaPerMinute()), mcp.WithMotorQuota(cfg.Cognition.EffectiveMotorQuotaPerHour()), mcp.WithCognition(resolveCognition(cfg)), mcp.WithCognitionConfig(cfg.Cognition), mcp.WithUsageLedger(engine, cfg.UsageLedger), mcp.WithVersion(version))
	defer server.CloseLedger() // baja lo que quede en el buffer del ledger de uso (F0)

	// Un nodo que SIRVE sin sync saliente es TERMINAL — el caso del cerebro central: no tiene
	// upstream a dónde empujar. Encolar ahí dejaba una fila `pending` INMORTAL por cada
	// observación ingerida (nunca drenaban: el drain de abajo ni arranca sin sync), y hacía que
	// `sync_status` contra el cerebro reportara miles de "pendientes de envío" — una señal de
	// salud que MIENTE. Con sync configurado (un central encadenado a otro), encola normal.
	engine.SetOutboxEnabled(cfg.Sync.HasDestination())

	// Flota (S10): intervalo de sondeo, caducidad de las salidas de comandos y políticas de
	// auto-heal. La validación SINTÁCTICA de las políticas es acá y es fatal — una política mal
	// escrita no puede convertirse en una alarma que calla. La otra mitad (que su principal exista
	// y tenga con qué actuar) la hace ListenAndServeHTTP cuando el registro ya está cargado.
	if err := server.ConfigurarFlota(cfg.Fleet); err != nil {
		fmt.Fprintf(os.Stderr, "musubi serve: %v\n", err)
		os.Exit(1)
	}

	// Shutdown graceful: ctx se cancela con SIGINT/SIGTERM; ListenAndServeHTTP retorna.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Conciliar el outbox con la config ANTES de arrancar el drain: un nodo terminal (el cerebro
	// central: sync off / sin central_url) purga sus filas 'shared' huérfanas; un nodo con sync
	// real avisa del backlog. Cierra el stall silencioso (ver reconcileOutboxOnStartup).
	reconcileOutboxOnStartup(engine, cfg.Sync)

	// Cerebro híbrido F2: si el sync saliente está activo, arrancar el drain del outbox que
	// empuja las observaciones 'shared' al cerebro central. El ctx (SIGINT/SIGTERM) también
	// para el drain. Best-effort: un error de config del cliente NO impide arrancar el serve.
	startOutboxDrain(ctx, server, cfg.Sync)

	// Auto-drain del acervo de diseño (pilar Musubi Renaissance, el "molino continuo"): destila los
	// blobs ingeridos en tarjetas de a tandas chicas, sin intervención. No-op sin motor de cognición o
	// con el intervalo en 0 — sólo el central con motor lo enciende. Sale con el mismo ctx (SIGINT/SIGTERM).
	if cfg.Maintenance.AutoDistillMinutes > 0 {
		go server.RunDistillScheduler(ctx, time.Duration(cfg.Maintenance.AutoDistillMinutes*float64(time.Minute)), cfg.Maintenance.AutoDistillBatch)
	}

	// EL CEREBRO MANTIENE SU PROPIA MEMORIA. Acá no había nada, y `serve` ya recibía
	// `WithMaintenance(cfg.Maintenance)`: la config estaba cableada y el consumidor no existía.
	//
	// CÓMO SE VEÍA ESO, medido en el central el 2026-09-07. El ciclo SÍ corría —`last_maintenance`
	// marcaba 31 h contra un intervalo de 24 h, o sea el diente de sierra normal— pero lo corría
	// otro proceso: `musubi-gateway.service` (el bot de Telegram) lanza `main.py`, que lanza
	// `/usr/local/bin/musubi daemon`, y ESE daemon abre la misma base y sí arranca los cuatro
	// schedulers. La memoria del cerebro se mantenía como efecto secundario de que un bot de chat
	// estuviera vivo. Parar el bot —una operación perfectamente razonable— dejaba la memoria sin
	// consolidar, sin olvidar y sin purgar, sin que nada lo dijera.
	//
	// DOS PROCESOS NO SE PISAN: RunScheduledMaintenance toma el candado de despacho y consulta
	// MaintenanceDue contra `last_maintenance` en la BASE, así que el que llega segundo ve que no
	// corresponde y no hace nada. La coordinación es del dato, no del proceso.
	//
	// VAN SÓLO EL MANTENIMIENTO Y SU CORRIDA DE ARRANQUE, y no los otros tres schedulers del
	// daemon: el del grafo indexa el árbol CHECKOUTEADO y en un servidor no hay proyecto que
	// indexar; el de sombra es no-op salvo que alguien lo encienda. Agregarlos sería trabajo
	// programado sin nadie que lo pidió.
	if cfg.Maintenance.AutoIntervalHours > 0 {
		go func() {
			if ran, rep, mErr := server.RunScheduledMaintenance(); mErr != nil {
				fmt.Fprintf(os.Stderr, "musubi: auto-mantenimiento de arranque falló: %v\n", mErr)
			} else if ran {
				fmt.Fprintf(os.Stderr, "musubi: auto-mantenimiento: %d fusionadas, %d archivadas, %d evictadas, %d purgadas\n",
					rep.Consolidate.Merged, rep.Decay.Archived, rep.Evicted, rep.Purged)
			}
		}()
		go server.RunMaintenanceScheduler(ctx, time.Duration(cfg.Maintenance.AutoIntervalHours*float64(time.Hour)))
	}

	if err := server.ListenAndServeHTTP(ctx, svc); err != nil {
		fmt.Fprintf(os.Stderr, "musubi serve: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon() {
	root := workspaceDir()

	// Auto-inicializa el workspace si falta (robusto para uso como MCP server).
	if err := ensureWorkspace(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error al preparar workspace: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar configuración: %v\n", err)
		os.Exit(1)
	}

	// Proveedor de embeddings con auto-detección + degradación elegante (16.2f): enciende la
	// semántica si hay tabla en la ubicación estándar; si no (o ante error), recall léxico.
	embedder := resolveEmbedder(cfg, root)

	// Cargar motor de base de datos local.
	//
	// SI FALLA, EL DAEMON NO SE MUERE: ATIENDE DEGRADADO. Acá había un os.Exit(1) con el error a
	// stderr, y eso le dejaba al cliente MCP cero bytes de protocolo — medido: `initialize`
	// contra una base más nueva devolvía 0 bytes por stdout y exit 1. Para el agente eso es
	// idéntico a «musubi no está instalado», y las dos cosas piden acciones opuestas. El
	// servidor degradado habla el protocolo, lista el mismo catálogo y rechaza cada tools/call
	// nombrando la causa. Ver internal/mcp/degradado.go.
	//
	// El aviso por stderr se conserva: es lo que ve el operador en el log del cliente MCP, y
	// dejarlo de escribir sería cambiar un canal mudo por otro.
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		// ESCALÓN DE SÓLO LECTURA, ANTES DE DARSE POR VENCIDO. Si la base es más nueva pero su
		// piso declara que este binario la lee bien, se abre sin migrarla y se sirve lo que se
		// puede. Es el estado del medio: no trabaja, pero tampoco deja al agente sin memoria.
		if errors.Is(err, memory.ErrEsquemaLegible) {
			if ro, roErr := memory.NewDbEngineSoloLectura(root); roErr == nil {
				fmt.Fprintf(os.Stderr, "musubi: sirviendo en SÓLO LECTURA (las tools de lectura funcionan; las que escriben, no)\n")
				servirSoloLectura(ro, root, cfg, embedder, err)
				return
			} else {
				// La base se declaraba legible y no se pudo abrir igual: se dice y se cae al
				// modo degradado. Tragarlo dejaría un daemon degradado sin explicación, que es
				// justo lo que esta serie vino a sacar.
				fmt.Fprintf(os.Stderr, "musubi: la base se declaraba legible pero no abrió en sólo lectura: %v\n", roErr)
			}
		}
		fmt.Fprintf(os.Stderr, "musubi: sirviendo en MODO DEGRADADO (el protocolo responde; las tools no)\n")
		mcp.NewServidorDegradado(root, version, err).Start()
		return
	}
	defer engine.Close()
	// Estampar el proyecto de origen en las observaciones (memoria híbrida local+central).
	engine.SetProjectID(resolveProjectID(cfg, root))
	// Los géneros que este despliegue declara LIBRO MAYOR: los necesita el doctor para podar la
	// cola con la misma regla con la que la detección deja de llenarla.
	engine.SetLedgerPrefixes(cfg.Conflicts.LedgerPrefixes)

	// Aviso de cambio de modelo de embedding (homogeneidad de vectores): si el modelo
	// activo cambió y hay vectores viejos de otro modelo, se logea un warning.
	if embedding.Enabled(embedder) {
		// Procedencia del vector (F2.2): cada embedding que escriba este engine lleva este
		// model_id, y la búsqueda semántica sólo compara vectores de la misma procedencia
		// (regla de homogeneidad). Debe fijarse ANTES de servir pedidos.
		engine.SetVectorModelID(embedder.Name())
		engine.WarnOnEmbedModelSwitch(embedder.Name())
		// M3: además de AVISAR del hueco, cerrarlo. Re-embebe en background la memoria que no
		// tiene vector de este modelo (la previa a encender la semántica, o la de otra tabla tras
		// un cambio de checksum/modelo). Sin esto, cambiar de modelo apaga el recall semántico
		// hasta un `musubi embed backfill` manual. No bloquea el arranque.
		autoBackfill(engine, embedder)
	}

	// Chequeo de versión throttled: avisa por stderr si hay una versión nueva
	// (no descarga ni reemplaza nada). Corre en goroutine para no demorar el
	// arranque. CheckIntervalHours <= 0 lo desactiva.
	if cfg.Update.CheckIntervalHours > 0 {
		if due, derr := engine.MetaDue(metaLastUpdateCheck, cfg.Update.CheckIntervalHours); derr == nil && due {
			_ = engine.MarkMetaNow(metaLastUpdateCheck)
			go notifyIfOutdated()
		}
	}

	// Arrancar servidor MCP sobre Stdin/Stdout, con sourcing y memoria configurados.
	server := mcp.NewMcpServer(engine, root, embedder, mcp.WithSourcing(cfg.Sourcing), mcp.WithMemory(cfg.Memory), mcp.WithMaintenance(cfg.Maintenance), mcp.WithGraph(cfg.Graph), mcp.WithConflicts(cfg.Conflicts), mcp.WithPipeline(cfg.Pipeline), mcp.WithMultiAgent(cfg.MultiAgent), mcp.WithQuota(cfg.Service.EffectiveQuotaPerMinute()), mcp.WithMotorQuota(cfg.Cognition.EffectiveMotorQuotaPerHour()), mcp.WithCognition(resolveCognition(cfg)), mcp.WithCognitionConfig(cfg.Cognition), mcp.WithUsageLedger(engine, cfg.UsageLedger), mcp.WithSpoolLocal(filepath.Join(root, ".musubi", "live")), mcp.WithVersion(version))
	defer server.CloseLedger() // baja lo que quede en el buffer del ledger de uso (F0)
	// El VERTEDERO del feed en vivo: sólo acá, no en runServe. El central ya reparte por HTTP;
	// un daemon stdio no tiene por dónde sacar sus eventos y por eso el trabajo local no se veía.
	defer server.CloseSpool()

	// Auto-mantenimiento de fondo (Track 5 / T5.2): el daemon es long-running; sin esto el
	// ciclo cognitivo (consolidar/olvidar/purgar) solo correría una vez al arrancar. Dos
	// goroutines best-effort que serializan contra el dispatch vía el write-lock del server:
	//   (1) una corrida de arranque NO bloqueante (un VACUUM grande no demora el primer pedido);
	//   (2) un ticker periódico que repite el ciclo intra-sesión.
	// El ctx se cancela al retornar de runDaemon (señal o EOF de stdin), parando el ticker.
	maintCtx, stopMaint := context.WithCancel(context.Background())
	defer stopMaint()
	if cfg.Maintenance.AutoIntervalHours > 0 {
		go func() {
			if ran, rep, mErr := server.RunScheduledMaintenance(); mErr != nil {
				fmt.Fprintf(os.Stderr, "musubi: auto-mantenimiento de arranque falló: %v\n", mErr)
			} else if ran {
				fmt.Fprintf(os.Stderr, "musubi: auto-mantenimiento: %d fusionadas, %d archivadas, %d evictadas, %d purgadas\n", rep.Consolidate.Merged, rep.Decay.Archived, rep.Evicted, rep.Purged)
			}
		}()
		go server.RunMaintenanceScheduler(maintCtx, time.Duration(cfg.Maintenance.AutoIntervalHours*float64(time.Hour)))
	}
	// El grafo de código se mantiene solo (P3). Va en su PROPIO gate y no colgado del de
	// mantenimiento: son dos ciclos con costos y riesgos distintos, y quien apague el
	// mantenimiento de la memoria no está pidiendo que además se le quede rancio el grafo.
	if cfg.Maintenance.GraphIndexHours > 0 {
		go server.RunCodeGraphScheduler(maintCtx, time.Duration(cfg.Maintenance.GraphIndexHours*float64(time.Hour)))
	}
	// Auto-drain del acervo (pilar Musubi Renaissance): no-op sin motor de cognición o con el intervalo
	// en 0. Va en su propio gate, como el grafo — dos ciclos con costos distintos.
	if cfg.Maintenance.AutoDistillMinutes > 0 {
		go server.RunDistillScheduler(maintCtx, time.Duration(cfg.Maintenance.AutoDistillMinutes*float64(time.Minute)), cfg.Maintenance.AutoDistillBatch)
	}
	// El modo sombra no se pregunta acá: RunShadowWorker es un no-op si está apagado (el default).
	// Sale con el mismo contexto que el mantenimiento, así el apagado del daemon lo corta también.
	go server.RunShadowWorker(maintCtx)

	// Cerebro híbrido F2: gatear el ENQUEUE con el sync (simétrico con runServe). Sin esto el
	// daemon stdio encola 'shared' incondicionalmente (default true) y, con el sync apagado, esas
	// filas se apilan sin que el drain (que ni arranca sin sync) las toque → pending INMORTALES que
	// envejecen en silencio. Debe fijarse antes de servir pedidos (server.Start()).
	engine.SetOutboxEnabled(cfg.Sync.HasDestination())
	// Conciliar el outbox con la config: purga huérfanas de un nodo terminal / avisa de un backlog
	// real, ANTES de arrancar el drain (ver reconcileOutboxOnStartup).
	reconcileOutboxOnStartup(engine, cfg.Sync)
	// Drain del outbox (sync saliente) también en el daemon (stdio). El maintCtx (cancelado al
	// retornar de runDaemon por señal o EOF) también lo detiene.
	startOutboxDrain(maintCtx, server, cfg.Sync)

	// Capturar SIGINT/SIGTERM para graceful shutdown: el select espera hasta que el
	// servidor termine (EOF de stdin) o llegue una señal. En ambos casos se retorna
	// de runDaemon y el defer engine.Close() cierra la DB limpiamente.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		defer close(done)
		server.Start()
	}()

	select {
	case sig := <-sigs:
		fmt.Fprintf(os.Stderr, "musubi: señal %v recibida, cerrando\n", sig)
	case <-done:
	}
}

// startOutboxDrain arranca el drain del outbox (sync saliente del cerebro híbrido, F2) si está
// autoBackfill (M3) le pasa al engine el callback de vectorización para que cierre en background el
// hueco de procedencia (memoria sin vector del modelo actual). El engine se mantiene MODEL-FREE: no
// embebe, recibe el embed del caller. Compartido por runServe y runDaemon. No bloquea el arranque
// (ver DbEngine.AutoEmbedBackfill); el ctx del embed es Background porque la goroutine ya está atada
// al ciclo de vida del engine (bgWG: Close la espera).
func autoBackfill(engine *memory.DbEngine, embedder embedding.Provider) {
	engine.AutoEmbedBackfill(func(textos []string) ([][]float32, error) {
		return embedding.EmbedBatch(context.Background(), embedder, textos)
	})
}

// configurado: requiere sync.enabled, un central_url no vacío y un intervalo > 0. Construye el
// SyncClient desde cfg.Sync (resuelve el token de la env var, valida https/allow_insecure), lo
// inyecta en el server y lanza RunOutboxScheduler en su propia goroutine atada a ctx. Es
// best-effort y compartido por runServe y runDaemon: un error de construcción del cliente NO
// aborta el arranque (se avisa por stderr y el server sigue local-first). Con sync desactivado
// es un no-op total (comportamiento idéntico al de antes de F2).
// reconcileOutboxOnStartup concilia el outbox con la config de sync al arrancar, para que un stall
// SILENCIOSO sea imposible. Con backlog pendiente hay dos caminos:
//   - Nodo TERMINAL (sync off o sin central_url): esas filas no tienen destino — son huérfanas de
//     un binario viejo (antes del gate SetOutboxEnabled) o de una config que apagó el sync. Se
//     PURGAN (el contenido de las observaciones queda; sólo se descarta el intento de envío) y se
//     avisa. Es la auto-cura del cerebro central: se sana sola al reiniciar con el binario nuevo.
//   - Sync habilitado y con destino: hay backlog real; se avisa fuerte (el drain debería vaciarlo).
//
// Best-effort: cualquier error se logea por stderr y NO aborta el arranque.
func reconcileOutboxOnStartup(engine *memory.DbEngine, sync config.SyncConfig) {
	pending, _, _, err := engine.OutboxStats()
	if err != nil || pending == 0 {
		return
	}
	if !sync.HasDestination() {
		n, perr := engine.PurgeOutboxPending()
		if perr != nil {
			fmt.Fprintf(os.Stderr, "musubi: no se pudieron purgar %d fila(s) huérfanas del outbox: %v\n", pending, perr)
			return
		}
		if n > 0 {
			fmt.Fprintf(os.Stderr, "musubi: outbox — purgadas %d fila(s) 'shared' pendientes huérfanas (nodo terminal sin sync saliente: no tenían destino)\n", n)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "musubi: outbox — %d observación(es) 'shared' pendientes de enviar al central; el drain intentará vaciarlas\n", pending)
}

func startOutboxDrain(ctx context.Context, server *mcp.McpServer, cfg config.SyncConfig) {
	if !cfg.Enabled {
		return // sync saliente apagado a propósito (nodo local o terminal): no-op total.
	}
	if strings.TrimSpace(cfg.CentralURL) == "" || cfg.DrainIntervalSeconds <= 0 {
		// HABILITADO pero mal configurado: antes retornaba en silencio y, con el enqueue activo,
		// las filas 'shared' se apilaban sin que nada avisara (el stall silencioso). Ahora se grita.
		fmt.Fprintln(os.Stderr, "musubi: sync saliente HABILITADO pero mal configurado (central_url vacío o drain_interval<=0); el outbox NO va a drenar. Revisá el bloque sync de .musubi/config.yaml")
		return
	}
	client, err := mcp.NewSyncClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi: sync saliente desactivado (config inválida): %v\n", err)
		return
	}
	server.SetSyncClient(client, cfg)
	go server.RunOutboxScheduler(ctx, time.Duration(cfg.DrainIntervalSeconds)*time.Second)
	// FLOTA EN VIVO (flota.go): la telemetría de invocaciones de esta máquina viaja al feed del
	// central para que su panel muestre a la flota trabajando. Mismo token, misma frontera de
	// confianza que el sync que se configuró recién arriba; flota_vivo: false la apaga sola.
	if cfg.FlotaVivoActivo() {
		go server.RunFlotaVivo(ctx)
	}
	// Sync ENTRANTE (C5.3b): baja la memoria shared del proyecto DESDE el central. RunInboundScheduler
	// gatea internamente en team_mode (un proyecto local no baja nada). Mismo intervalo que el drain.
	go server.RunInboundScheduler(ctx, time.Duration(cfg.DrainIntervalSeconds)*time.Second)
}

// servirSoloLectura atiende con la memoria abierta en el escalón de sólo lectura.
//
// ES UNA FUNCIÓN APARTE Y NO UNA RAMA DENTRO DE runDaemon a propósito: lo que la distingue no es
// una opción de más, es TODO LO QUE NO ARRANCA. Escrito como rama, cada scheduler nuevo que
// alguien agregue abajo quedaría corriendo también acá, contra una base que no se puede escribir,
// y el error saldría del motor SQLite a los minutos y en un log que nadie mira.
//
// LO QUE NO ARRANCA, Y POR QUÉ CADA UNO:
//   - mantenimiento, grafo y destilado: los tres ESCRIBEN. Un ciclo cognitivo sobre una base que
//     no entiende del todo sería peor que no correrlo.
//   - ledger de uso y vertedero del feed: escriben también, y su valor es el registro histórico,
//     que es justo lo que no se debe ensuciar desde un binario desactualizado.
//   - outbox: encolar desde acá propagaría el malentendido a la malla.
//   - cognición: el motor no es de lectura y su cuota se lleva en la base.
//
// El único que sí va es el embedder, porque la búsqueda semántica es una LECTURA (el vector de la
// consulta se calcula en memoria y no se guarda).
func servirSoloLectura(eng *memory.DbEngine, root string, cfg config.Config, embedder embedding.Provider, causa error) {
	defer eng.Close()
	eng.SetProjectID(resolveProjectID(cfg, root))
	srv := mcp.NewMcpServer(eng, root, embedder,
		mcp.WithVersion(version),
		mcp.WithSoloLectura(causa.Error()),
		mcp.WithSourcing(cfg.Sourcing),
		mcp.WithMemory(cfg.Memory),
		mcp.WithGraph(cfg.Graph),
		mcp.WithConflicts(cfg.Conflicts),
		mcp.WithQuota(cfg.Service.EffectiveQuotaPerMinute()),
	)
	srv.Start()
}
