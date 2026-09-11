package mcp

// fleet_prometheus.go exporta la telemetría de la flota en formato Prometheus, para que la
// HISTORIA la guarde quien sabe guardarla. Track «Control de flota».
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA TENSIÓN QUE ESTE ARCHIVO RESUELVE, Y POR QUÉ NO ERA UN `for` SOBRE LAS FILAS
//
// S4 dejó a Musubi guardando el PRESENTE de la flota (la última muestra de cada máquina) y la
// historia explícitamente afuera: una tabla de series con 40 máquinas latiendo cada 30 s son
// 115.000 filas diarias que nadie consulta salvo para graficar, y graficar series es para lo que
// existe Prometheus — que este repo ya despliega.
//
// Pero exportar chocaba con S3. Un scrape de Prometheus presenta UNA credencial, mientras la
// compuerta de la flota es POR MÁQUINA Y POR CAPACIDAD. La salida fácil habría sido decir «/metrics
// es infraestructura, que vea todo»: eso convierte al scraper en una puerta trasera que sortea el
// eje entero, y bastaría con darle su token a alguien para leer la telemetría de todos los tenants.
//
// LA RESOLUCIÓN ES QUE EL SCRAPER NO ES UN CASO ESPECIAL: es un principal más. Se exporta
// exactamente lo que ESA credencial puede ver, con la misma PuedeSobreDevice que usa la tool. Si
// querés que Prometheus vea toda la flota, se lo declarás en principals.yaml:
//
//	- name: prometheus
//	  role: reader
//	  read: all              # ve todos los proyectos
//	  fleet:
//	    metrics: ["*"]       # ...y la telemetría de todas las máquinas
//
// Consecuencia fail-closed que conviene tener presente: con el TOKEN LEGACY (admin sin sección
// `fleet:`) no se exporta ninguna máquina. Es coherente con C1 —el rol de memoria no otorga
// capacidades de flota— y la salida lo DICE en vez de quedarse muda, para que nadie pierda una
// tarde buscando por qué el dashboard está vacío.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"musubi/internal/buildid"
	"musubi/internal/fleet"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// proyectosParaExportar es el tope de proyectos que se recorren en un scrape.
//
// Existe porque el scrape corre cada 15 s y no puede convertirse en un escaneo sin fin de la
// base. No es un límite de la flota: es un límite de cuántos TENANTS distintos se barren por
// scrape, y si alguna vez se alcanza, la salida lo dice (nunca se trunca en silencio).
const proyectosParaExportar = 64

// renderFlota agrega al exposition format las métricas de las máquinas que `p` puede ver.
//
// Recibe el principal ya resuelto por el handler: la autorización NO se decide acá, se APLICA.
// intervaloSonda entra por parámetro (y no por una constante) porque de él SE DERIVA el umbral
// de «caído» de las máquinas sin agente (S10 · I2): con el umbral fijo de 90 s, un Tier B
// sondeado cada 5 min exportaba up=0 el 97 % del tiempo y `MaquinaCaida` disparaba para siempre.
// Una alerta que grita sin parar se silencia, y con ella se silencian las que sí importaban.
//
// DESDE S11 ESTE YA NO ES EL ÚNICO CAMINO DE SALIDA: el empuje OTLP (fleet_otlp.go) exporta lo
// mismo por otra boca. Por eso la selección de máquinas, la tabla de series y el juego de labels
// viven en funciones compartidas y no adentro de este `for` — dos copias discrepan el día que
// alguien agrega un campo, y la discrepancia se descubre semanas después, cuando dos dashboards
// muestran cosas distintas.
// vidaDeRedLookup responde si el cerebro alcanza una máquina por fuera de su agente. Un `nil`
// significa «nadie midió», que es distinto de «no está»: con nil no se emite ninguna serie.
type vidaDeRedLookup func(deviceID string, ahora time.Time) (fleet.VidaDeRed, bool)

func renderFlota(b *strings.Builder, engine memory.StorageBackend, p *Principal, ahora time.Time,
	intervaloSonda time.Duration, versionCerebro string, vidaDe vidaDeRedLookup, techoServicios int) {
	vistos, truncadoProyectos, ilegible := devicesVisiblesParaMetricas(engine, p)
	// Los tres hechos viajan JUNTOS en la misma estructura que usa el empuje. Que las dos bocas
	// compartan el tipo es lo que impide que una empiece a reportar algo que la otra no.
	recorte := truncadoDeExport{Proyectos: truncadoProyectos, Ilegible: ilegible}
	// Un error leyendo las ventanas NO puede convertirse en «todas en mantenimiento» (apagaría
	// las alertas de la flota entera) ni hacer fallar el scrape. Se sigue con el mapa vacío, que
	// es el comportamiento de antes de que esto existiera.
	enMantenimiento, errMant := engine.DevicesEnMantenimiento(ahora)
	if errMant != nil {
		enMantenimiento = nil
	}

	if len(vistos) == 0 {
		// Un bloque vacío y mudo manda a alguien a depurar Prometheus cuando el problema está en
		// principals.yaml. Se dice, en un comentario que el parser ignora.
		b.WriteString("# musubi_fleet: ninguna máquina visible para esta credencial.\n")
		b.WriteString("# Las capacidades de flota NO se derivan del rol: declarálas en principals.yaml\n")
		b.WriteString("#   fleet:\n#     metrics: [\"*\"]\n")
		// Y LA SERIE SALE IGUAL, sobre todo acá: «esta credencial no ve ninguna máquina» y «el
		// almacén no se dejó leer» terminaban los dos en este mismo comentario que Prometheus
		// descarta, sin una sola serie. Con `kind="unreadable"` los dos casos dejan de ser el
		// mismo silencio.
		renderTruncado(b, recorte, techoServicios)
		renderTechos(b, techoServicios, 0, 0)
		renderReferenciaDeVersion(b, versionCerebro)
		// Y LAS BAJAS, PRECISAMENTE ACÁ. Este `return` se toma cuando no queda NINGUNA máquina
		// visible — que es exactamente lo que pasa cuando se revocó la última. Saltearlo dejaba el
		// aviso de la baja sin salir justo en el caso en que más importa: el proyecto entero se
		// apagó y sus alertas se van a resolver todas juntas, en silencio.
		renderBajasRecientes(b, engine, p, ahora)
		return
	}
	if recorte.Proyectos {
		fmt.Fprintf(b, "# musubi_fleet: se barrieron los primeros %d proyectos; hay más.\n", proyectosParaExportar)
	}

	for _, s := range seriesDeFlota(ahora, intervaloSonda, versionCerebro, enMantenimiento) {
		escribirGauge(b, vistos, s.Nombre, s.Ayuda, s.Valor)
	}
	// EL TRUNCADO DEJA DE SER UN COMENTARIO Y PASA A SER UNA SERIE (Ola 0 del plan empresa).
	//
	// Los avisos de arriba son líneas que empiezan con `#`, o sea: Prometheus las DESCARTA al
	// parsear. Estaban escritas para una persona que abriera /metrics a mano, y nadie abre
	// /metrics a mano. El resultado era el peor de los dos mundos: el sistema sabía que había
	// recortado la cobertura y no había forma de que ese hecho llegara a una alerta.
	//
	// EL RESTO DEL CUERPO SE ARMA APARTE Y SE VUELCA DESPUÉS. La serie de truncado tiene que
	// salir en este punto exacto (el orden del exposition format ya está probado) pero sus tres
	// hechos recién se conocen cuando terminaron los barridos de abajo. Bufferear cuesta un
	// strings.Builder y evita lo que había antes: un SEGUNDO barrido completo de los servicios
	// hecho sólo para recuperar un bool que el primero ya sabía.
	var cuerpo strings.Builder
	// QUÉ CORRE ADENTRO de esas máquinas (A43). Va DESPUÉS y con las mismas máquinas ya
	// compuertadas: la lista `vistos` es la que pasó por PuedeSobreDevice, y reusarla es lo que
	// evita un segundo lugar donde olvidarse la compuerta.
	truncadoSvs, ilegibleSvs, peorProyecto := renderServicios(&cuerpo, engine, vistos, ahora, techoServicios)
	recorte.Servicios = truncadoSvs
	recorte.Ilegible = recorte.Ilegible || ilegibleSvs
	// QUIÉN ESTÁ ESPERANDO UN SEGUNDO PAR DE OJOS (Ola 2). Va con las mismas máquinas ya
	// compuertadas, por lo mismo que servicios.
	recorte.Ilegible = renderAprobaciones(&cuerpo, engine, vistos, ahora) || recorte.Ilegible
	// SI EL TAILNET VE A LAS QUE NO LATEN (Ola 3). Va con las mismas máquinas ya compuertadas.
	renderVidaDeRed(&cuerpo, vistos, ahora, vidaDe)
	// LAS BAJAS RECIENTES, que no están en `vistos` justamente por estar dadas de baja.
	renderBajasRecientes(&cuerpo, engine, p, ahora)

	renderTruncado(b, recorte, techoServicios)
	// EL MARGEN, ANTES DEL CORTE. `export_truncated` avisa cuando un techo YA cortó, o sea
	// después de perder cobertura; estas dos dicen cuánto falta.
	renderTechos(b, techoServicios, proyectosDistintos(vistos), peorProyecto)
	renderReferenciaDeVersion(b, versionCerebro)
	b.WriteString(cuerpo.String())
}

// nombreVidaDeRed dice si el CEREBRO alcanza a una máquina por fuera de su agente.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA SERIE FALTA CUANDO NO SE MIDIÓ, Y ÉSE ES EL DISEÑO ENTERO
//
// Al revés que `musubi_fleet_export_truncated`, que se emite en 0 porque el 0 es un hecho
// medido, acá un 0 significa «la máquina NO está en la red» — una afirmación fuerte que manda a
// alguien a revisar hardware. Un cerebro sin acceso al tailnet, una máquina que no está en el
// tailnet, o dos pares que dicen llamarse igual, no permiten afirmar eso.
//
// Así que la ausencia de serie es «no sé», y sólo hay serie cuando hubo respuesta. La alerta que
// la consume usa `and on(...)`, que con la serie ausente simplemente no dispara: el eje se apaga
// solo donde no se puede medir, en vez de mentir.
//
// Y SÓLO APARECE PARA LAS QUE NO ESTÁN LATIENDO: en una máquina que late, la pregunta no cambia
// ninguna decisión, y una serie que existe para toda la flota invita a construir alertas sobre
// un dato que sólo se refresca para una parte.
// EL NOMBRE NO EMPIEZA CON `musubi_fleet_device_`, Y NO ES ESTILO: SI EMPEZARA, SE PIERDE.
//
// El scrape descarta `musubi_fleet_(device|service)_.*` con un `metric_relabel_configs` —está en
// deploy/prometheus/prometheus.yml— porque la telemetría POR MÁQUINA llega por el empuje OTLP y
// tener las dos copias sería duplicar. Esta serie NO viaja por OTLP: la produce el cerebro, así
// que llamarla `musubi_fleet_device_net_up` la mandaba derecho al descarte.
//
// Y el modo de falla era el peor posible: la serie no llega, la alerta que la consume usa
// `and on(...)` y con la serie ausente simplemente NO DISPARA. Todo en verde, sin un solo error,
// y el eje entero apagado. Se desplegó así y se «verificó» aceptando su ausencia como correcta
// —«toda la flota late, no tiene por qué existir»—, que es exactamente cómo se ve un drop.
//
// El nombre correcto además es más honesto: esto no lo reporta la máquina, lo mide el CEREBRO
// sobre la máquina, igual que `musubi_fleet_approval_pending`. La guarda que impide que vuelva a
// pasar es TestNingunaSerieDelCerebroCaeEnElDescarteDelScrape.
const nombreVidaDeRed = "musubi_fleet_net_up"

// seriesSoloDelScrape son las que produce EL CEREBRO y no viajan por el empuje OTLP.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// ESTA LISTA EXISTE PARA QUE UNA SERIE NUEVA NO SE PIERDA EN SILENCIO
//
// El scrape descarta `musubi_fleet_(device|service)_.*` a propósito: esa familia es telemetría
// POR MÁQUINA y llega por OTLP, así que la copia del scrape sería duplicación. Las de acá abajo
// NO tienen copia por OTLP — si el descarte las agarra, no llegan a ningún lado.
//
// Y no se nota: la alerta que las consume usa `and on(...)`, y con la serie ausente no dispara.
// Sin errores, sin logs, sin nada rojo. Pasó con `musubi_fleet_device_net_up`, que se desplegó y
// se «verificó» aceptando su ausencia como correcta — que es exactamente cómo se ve un drop.
//
// La custodia TestNingunaSerieDelCerebroCaeEnElDescarteDelScrape cruza esta lista con el regex
// del prometheus.yml, y además exige que TODA serie nombrada en este archivo esté o acá o en la
// tabla de seriesDeFlota: una lista que se puede quedar vieja no protege de nada.
var seriesSoloDelScrape = []string{
	nombreExportTruncado,
	nombreAprobPendientes,
	nombreAprobEspera,
	nombreVidaDeRed,
	// Sale de observability.go y no de este archivo, que es exactamente por lo que faltó acá
	// durante meses: la custodia leía UN archivo y esta serie vive en otro. Es la de mayor
	// consecuencia de las cinco —tres alertas cuelgan de ella y no tiene copia por OTLP—.
	nombrePoliticaAcciones,
	// Los techos y el uso: dicen CUÁNTO FALTA para que un recorte empiece, así que tienen que
	// llegar aunque el empuje esté apagado — de hecho el empuje es una de las cosas que se
	// dimensionan con ellas.
	nombreTecho,
	nombreUso,
	// Es un hecho del cerebro, no telemetría de una máquina: no viaja por el empuje.
	nombreReferenciaVersion,
	// Una máquina revocada no está en el barrido del empuje —no es visible— así que su baja sólo
	// puede llegar por el scrape.
	nombreBajaReciente,
}

// nombreBajaReciente dice que una máquina SE DIO DE BAJA hace poco, con su antigüedad.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// REVOCAR UNA MÁQUINA RESOLVÍA SUS ALERTAS EN SILENCIO.
//
// `revoked` es una BANDERA y no un DELETE, así que la fila queda — pero la máquina sale del
// export, sus series se vuelven obsoletas, y TODAS sus alertas se resuelven solas. Del otro lado
// del canal, «se arregló» y «la sacamos del inventario» llegan como el MISMO `[RESOLVED]`, sin
// una palabra que los distinga. Quien lo lee concluye que el problema se atendió — y la máquina
// con el disco lleno que se dio de baja sin arreglar queda cerrada en la cabeza de todos.
//
// ES UNA SERIE ACOTADA EN EL TIEMPO y no una bandera permanente: acompaña a las resoluciones y
// después desaparece sola. Una serie que viviera para siempre convertiría el aviso en parte del
// paisaje, que es otra forma de no decir nada.
//
// EL NOMBRE NO LLEVA `device_` A PROPÓSITO, y no es estilo: `prometheus.yml` descarta
// `musubi_fleet_(device|service)_.*` del scrape porque esa familia llega por el empuje OTLP. Esta
// serie NO puede viajar por el empuje —el empuje recorre las máquinas VISIBLES, y una máquina
// revocada no lo es— así que con ese prefijo se descartaría en el scrape y no llegaría por
// ningún lado. Es el mismo criterio que `musubi_fleet_net_up` y `musubi_fleet_approval_pending`.
// ────────────────────────────────────────────────────────────────────────────────────────────
const nombreBajaReciente = "musubi_fleet_revoked_ago_seconds"

// ventanaDeBajaReciente es cuánto tiempo se sigue anunciando una baja.
//
// Sale de la vida de una alerta, no de un gusto: tiene que cubrir con margen el `for:` de la
// alerta que la lee más el tiempo que el canal tarda en entregar, para que el aviso de la baja y
// las resoluciones que produjo lleguen juntos.
const ventanaDeBajaReciente = 24 * time.Hour

// renderBajasRecientes emite una línea por máquina dada de baja adentro de la ventana.
//
// LEE LAS REVOCADAS APARTE, y no puede hacerse metiéndolas en `vistos`: ahí adentro emitirían
// TODAS sus series —cpu, disco, servicios— y volverían a la flota como si estuvieran vivas, que
// es exactamente lo contrario de lo que esto quiere decir.
func renderBajasRecientes(b *strings.Builder, engine memory.StorageBackend, p *Principal, ahora time.Time) {
	proyectos, _, _ := proyectosVisibles(engine, p)
	// Y LOS PROYECTOS QUE YA NO TIENEN MÁQUINAS VIVAS. `proyectosVisibles` los descubre con
	// `ProyectosConDevices`, que filtra `revoked = 0`: el proyecto cuya ÚLTIMA máquina se dio de
	// baja desaparece entero, y su baja —la que más importa anunciar— no saldría por ningún lado.
	if conBajas, err := engine.ProyectosConBajasRecientes(ahora.Add(-ventanaDeBajaReciente), proyectosParaExportar); err == nil {
		ya := map[string]bool{}
		for _, q := range proyectos {
			ya[q] = true
		}
		for _, q := range conBajas {
			if !ya[q] {
				proyectos = append(proyectos, q)
			}
		}
	}
	tipoEscrito := false
	for _, proy := range proyectos {
		devices, err := engine.ListarDevices(proy, true) // true = incluir revocadas
		if err != nil {
			continue // lo ilegible ya lo cuenta el barrido principal
		}
		for _, d := range devices {
			if !d.Revoked || d.RevokedAt.IsZero() {
				continue
			}
			edad := ahora.Sub(d.RevokedAt)
			if edad < 0 || edad > ventanaDeBajaReciente {
				continue
			}
			// LA COMPUERTA ES LA DEL HISTORIAL Y NO LA NORMAL, y no es un atajo: para una máquina
			// revocada `PuedeSobreDevice` contesta SIEMPRE que no —el kill-switch de la revocación
			// es absoluto, y así tiene que seguir siendo para todo lo que TOQUE la máquina—. Lo que
			// se está publicando acá no toca nada: es el hecho de que salió del inventario, que es
			// justamente lo que hay que poder decir. `PuedeVerHistorialDeDevice` levanta ese
			// kill-switch y NADA MÁS: misma tenencia, misma concesión. Quien podía ver esa máquina
			// mientras vivía se entera de su baja; nadie más.
			if !PuedeVerHistorialDeDevice(p, d, fleet.CapMetrics) {
				continue
			}
			if !tipoEscrito {
				fmt.Fprintf(b, "# HELP %s Hace cuántos segundos se dio de baja esta máquina. Existe SÓLO durante las primeras 24 h desde la revocación, y sirve para distinguir «se arregló» de «la sacamos del inventario»: sin ella, revocar resuelve todas las alertas de la máquina y del otro lado llega el mismo [RESOLVED] que produce un arreglo.\n# TYPE %s gauge\n",
					nombreBajaReciente, nombreBajaReciente)
				tipoEscrito = true
			}
			fmt.Fprintf(b, "%s{project=%q,device=%q} %d\n", nombreBajaReciente, d.ProjectID, d.Name, int64(edad.Seconds()))
		}
	}
}

func renderVidaDeRed(b *strings.Builder, vistos []fleet.Device, ahora time.Time, vidaDe vidaDeRedLookup) {
	if vidaDe == nil {
		return
	}
	tipoEscrito := false
	for _, d := range vistos {
		v, hay := vidaDe(d.ID, ahora)
		if !hay || v == fleet.VidaNoMedida {
			continue
		}
		if !tipoEscrito {
			fmt.Fprintf(b, "# HELP %s 1 si el cerebro alcanza esta máquina por la red aunque su agente no lata. Sólo existe para máquinas que NO están latiendo, y FALTA cuando no se pudo medir: su ausencia es «no sé», nunca «no está».\n# TYPE %s gauge\n",
				nombreVidaDeRed, nombreVidaDeRed)
			tipoEscrito = true
		}
		valor := 0
		if v == fleet.VidaPresente {
			valor = 1
		}
		fmt.Fprintf(b, "%s{project=%q,device=%q} %d\n", nombreVidaDeRed, d.ProjectID, d.Name, valor)
	}
}

// nombreAprobPendientes y nombreAprobEspera cuentan las solicitudes de cuatro ojos que esperan.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ HACEN FALTA DOS SERIES Y NO ALCANZA CON CONTAR
//
// La aprobación NO VIAJA: nadie recibe una notificación, así que una solicitud que ningún
// aprobador mira vence sola y el control se degrada a una negación con demora — que es peor que
// no tenerlo, porque parece que funciona.
//
// El contador solo no alcanza para alertar: con solicitudes que entran y salen, el conteo puede
// quedarse en 1 sin que NADIE haya esperado mucho, y un `for: 10m` sobre eso dispararía por un
// flujo sano. La ESPERA MÁS VIEJA mide lo que importa —hace cuánto que hay alguien trabado— y no
// necesita `for:`.
//
// El contador sigue existiendo porque es lo que distingue «nadie espera» de «alguien acaba de
// pedir»: las dos dan una espera cercana a cero.
//
// LOS DOS SE EMITEN EN CERO cuando no hay nada pendiente, por la misma razón que
// musubi_fleet_export_truncated: una serie que sólo aparece cuando hay problema no se puede
// graficar y no se distingue de que el exportador no corrió.
//
// LAS ETIQUETAS SON SÓLO `project`: ni la máquina, ni quién pidió, ni quién puede aprobar. Un
// scrape lo lee cualquiera que llegue al endpoint, y «fulano quiere entrar al servidor de pagos»
// es exactamente la clase de dato que no tiene por qué estar ahí. Para saber QUÉ está esperando
// está musubi_fleet_approvals, que sí pasa por la compuerta.
const (
	nombreAprobPendientes = "musubi_fleet_approval_pending"
	nombreAprobEspera     = "musubi_fleet_approval_wait_seconds"
)

// topeDeAprobacionesPorProyecto es el TERCER techo del exportador, y el único que sigue sin
// perilla ni serie propia. Estaba escrito como un `200` pelado adentro de la llamada.
//
// SU DAÑO DEPENDE DE LA COMPUERTA, Y ESO NO ESTABA MEDIDO. Acá decía que pasarse sólo deja el
// conteo corto y que «la espera más vieja puede no ser la más vieja de verdad». Lo segundo es
// falso y lo primero es incompleto:
//
//   - `AprobacionesPendientes` pide `ORDER BY creada ASC LIMIT ?` (internal/memory/aprobaciones.go),
//     así que la más vieja SIEMPRE entra en la página. Con una credencial que ve todo el proyecto
//     el único daño es el conteo, que se clava en el tope.
//
//   - PERO EL TOPE LO APLICA EL ALMACÉN SOBRE EL PROYECTO ENTERO Y LA COMPUERTA CORRE DESPUÉS,
//     acá abajo, con `visibles[sol.DeviceID]`. Una credencial que ve pocas máquinas de un
//     proyecto con más de `topeDeAprobacionesPorProyecto` pendientes puede recibir una página
//     entera de solicitudes que no ve NINGUNA, y entonces sale `musubi_fleet_approval_pending 0`
//     y `musubi_fleet_approval_wait_seconds 0` — y el HELP de esa serie dice, con todas las
//     letras, «0 = no hay ninguna esperando». Ahí el techo sí hace desaparecer el hecho: es un
//     cero que significa «no sé», que es exactamente lo que este archivo existe para no emitir.
//
// QUEDA ASÍ EN ESTA RONDA, A PROPÓSITO Y ESCRITO: el arreglo no es una guarda, es un cuarto
// `kind` en `musubi_fleet_export_truncated` con su aviso y su perilla —o mejor, aplicar el tope
// después de la compuerta— y eso cambia el contrato de /metrics y del empuje OTLP, que no es algo
// que se cuele en una ronda de guardas. Anotado en specs/control-de-flota/ABIERTO.md como A123.
//
// Tener nombre es la mitad barata del arreglo: un `200` adentro de una llamada no se puede ni
// buscar.
const topeDeAprobacionesPorProyecto = 200

func renderAprobaciones(b *strings.Builder, engine memory.StorageBackend, vistos []fleet.Device, ahora time.Time) (ilegible bool) {
	// Los proyectos salen de las máquinas YA compuertadas: preguntarle al almacén por todos los
	// proyectos sería un segundo recorrido sin compuerta, que es como se exporta de más sin que
	// nadie lo note.
	// ══════════════════════════════════════════════════════════════════════════════════════
	// LOS PROYECTOS SALEN DE `vistos`, PERO LAS SOLICITUDES TAMBIÉN TIENEN QUE FILTRARSE
	//
	// La primera versión hacía sólo la mitad: sacaba los proyectos de `vistos` —ya compuertado
	// por PuedeSobreDevice— y después le pedía al almacén las pendientes DEL PROYECTO ENTERO.
	// Una credencial con `metrics: ["srv-01"]` recibía el conteo de todas las máquinas de ese
	// proyecto, incluidas las que no puede ni listar. Era exactamente «el segundo recorrido
	// donde uno se olvida la compuerta» que el comentario de al lado decía estar evitando.
	//
	// Lo encontró una revisión adversaria, no una prueba: el comentario correcto estaba escrito
	// AL LADO del código que lo contradecía, que es la forma más difícil de ver un agujero.
	//
	// Con el conjunto de ids, lo que se cuenta pasa a ser «pendientes sobre máquinas que ESTA
	// credencial ve», que es lo único que se le puede decir a quien scrapea.
	orden := make([]string, 0, 4)
	visto := map[string]bool{}
	visibles := make(map[string]bool, len(vistos))
	for _, d := range vistos {
		visibles[d.ID] = true
		if !visto[d.ProjectID] {
			visto[d.ProjectID] = true
			orden = append(orden, d.ProjectID)
		}
	}

	fmt.Fprintf(b, "# HELP %s Solicitudes de cuatro ojos esperando una segunda persona. La aprobación no viaja: si nadie mira musubi_fleet_approvals, vencen solas a los %s.\n# TYPE %s gauge\n",
		nombreAprobPendientes, fleet.VentanaDeAprobacion, nombreAprobPendientes)
	fmt.Fprintf(b, "# HELP %s Hace cuántos segundos espera la solicitud de cuatro ojos MÁS VIEJA de este proyecto. 0 = no hay ninguna esperando.\n# TYPE %s gauge\n",
		nombreAprobEspera, nombreAprobEspera)

	for _, proy := range orden {
		pendientes, err := engine.AprobacionesPendientes(proy, ahora, topeDeAprobacionesPorProyecto)
		if err != nil {
			// Un error leyendo esto NO puede romper el scrape entero: la telemetría de la flota
			// vale más que este contador. Se saltea el proyecto en vez de emitir un cero, que
			// diría «no hay nadie esperando» sin saberlo.
			//
			// PERO OMITIR LA LÍNEA TAMPOCO ALCANZA: una serie que falta no dispara nada, y se ve
			// igual que «ese proyecto no existe». Se levanta la mano por la misma serie que los
			// otros tres hermanos.
			ilegible = true
			logx.Error("export de flota: no se pudieron leer las aprobaciones pendientes de un proyecto; su serie de espera NO sale y nadie va a notar que alguien quedó trabado",
				"project", proy, "error", err,
				"serie", nombreExportTruncado+`{kind="unreadable"}`)
			continue
		}
		// La lista viene ordenada por `creada ASC`, así que la primera VISIBLE es la más vieja.
		n, espera := 0, 0.0
		for _, sol := range pendientes {
			if !visibles[sol.DeviceID] {
				continue
			}
			if n == 0 {
				if d := ahora.Sub(sol.Creada).Seconds(); d > 0 {
					espera = d
				}
			}
			n++
		}
		fmt.Fprintf(b, "%s{project=%q} %d\n", nombreAprobPendientes, proy, n)
		fmt.Fprintf(b, "%s{project=%q} %.0f\n", nombreAprobEspera, proy, espera)
	}
	return ilegible
}

// nombreExportTruncado es la serie que dice que el exportador dejó cosas afuera. Vale 1 cuando se
// recortó y 0 cuando no: acá el 0 NO es un «no medido» —se sabe con certeza que no se truncó— así
// que emitirlo es correcto y además necesario, porque una serie que sólo existe cuando hay
// problema no se puede graficar ni distinguir de «el exportador no corrió».
const nombreExportTruncado = "musubi_fleet_export_truncated"

// truncadoDeExport dice CUÁL de los dos techos del exportador cortó, POR SEPARADO.
//
// Era un solo `bool` fusionado con `truncado = truncado || truncadoSvs`, y la fusión borraba
// justo el dato accionable: los dos techos se arreglan distinto —uno es la perilla
// `fleet.services_per_project_export`, el otro es `proyectosParaExportar`, una constante de
// compilación— así que un aviso que no dice cuál se cortó manda a la perilla equivocada. Un aviso
// que nombra el techo equivocado es PEOR que no avisar: el que lo lee sube un número, no ve
// ningún cambio, y concluye que la alerta miente.
type truncadoDeExport struct {
	// Proyectos: se pasó de `proyectosParaExportar` tenants con máquinas en este barrido.
	Proyectos bool
	// Servicios: algún proyecto pasó el techo de servicios exportables.
	Servicios bool
	// Ilegible: parte de la flota NO SE PUDO LEER (la lista de proyectos, las máquinas de un
	// proyecto, sus servicios o sus aprobaciones). No es un techo: es la ausencia de medición.
	//
	// EXISTE PORQUE «FALLÉ» Y «NO HUBO CORTE» ERAN EL MISMO VALOR. Cada uno de esos errores caía
	// en un `continue` mudo y los dos bools de arriba seguían en false, así que el export
	// afirmaba `kind="services"} 0` —«medí y no recorté»— sobre proyectos que no había mirado.
	// El 0 de los otros dos `kind` sólo es una medición cuando éste vale 0.
	Ilegible bool
}

// renderTruncado emite la serie con un punto por dimensión recortable.
//
// `techoServicios` entra por parámetro y no se lee de una constante porque es CONFIGURABLE: el
// HELP tiene que nombrar el techo VIGENTE. Un HELP que dice 2000 cuando
// `fleet.services_per_project_export` vale 300 manda a quien lee la alerta a buscar 2000
// servicios que no existen. `techoServicios <= 0` es «sin techo», y entonces `kind="services"` no
// puede valer 1: se dice así en vez de nombrar un número que no rige.
func renderTruncado(b *strings.Builder, t truncadoDeExport, techoServicios int) {
	unoSi := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	fmt.Fprintf(b, "# HELP %s 1 si el exportador dejó afuera parte de la flota. `kind=projects`: se pasó de %d proyectos por scrape (techo de compilación, ver proyectosParaExportar). `kind=services`: algún proyecto pasó %s. `kind=unreadable`: NO SE PUDO LEER parte de la flota (la lista de proyectos, las máquinas de uno, sus servicios o sus aprobaciones), así que mientras valga 1 el 0 de los otros dos es «no medí» y no «no hubo corte». Lo que queda afuera NO tiene serie, así que sus alertas no pueden dispararse.\n# TYPE %s gauge\n",
		nombreExportTruncado, proyectosParaExportar, describirTechoDeServicios(techoServicios), nombreExportTruncado)
	fmt.Fprintf(b, "%s{kind=\"projects\"} %s\n", nombreExportTruncado, unoSi(t.Proyectos))
	fmt.Fprintf(b, "%s{kind=\"services\"} %s\n", nombreExportTruncado, unoSi(t.Servicios))
	fmt.Fprintf(b, "%s{kind=\"unreadable\"} %s\n", nombreExportTruncado, unoSi(t.Ilegible))
}

// nombreTecho es la serie que dice CUÁNTO ENTRA, y `nombreUso` cuánto se está usando.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LOS TRES TECHOS ESTABAN TIPEADOS EN GO Y NO LLEGABAN A PROMETHEUS.
//
// `musubi_fleet_export_truncated` avisa que un techo CORTÓ — o sea DESPUÉS de perder cobertura,
// con cero antelación. Y lo que se pierde no es un gráfico: son las series de las máquinas o los
// servicios que quedaron afuera, así que sus alertas dejan de poder dispararse. El aviso llega
// cuando el daño ya está hecho.
//
// Medido el 2026-09-10: `musubi-server` declara 56 servicios contra el techo de 64 del latido.
// Quedan OCHO. Nadie tenía forma de saberlo sin entrar a mirar, y el día que se pase, la máquina
// manda un inventario recortado que el cerebro lee como inventario COMPLETO.
//
// Con estas dos series el margen es una resta, y `TechoDeExportCerca` puede avisar al 80 % —antes
// del corte y no después—. Los techos se emiten como series y no se escriben en una alerta
// porque dos de los tres son CONFIGURABLES: un número tipeado en la regla nombraría un techo que
// no rige, que es el defecto exacto que `describirTechoDeServicios` vino a arreglar un piso más
// abajo.
// ────────────────────────────────────────────────────────────────────────────────────────────
const (
	nombreTecho = "musubi_fleet_export_limit"
	nombreUso   = "musubi_fleet_export_usage"
)

// proyectosDistintos cuenta cuántos tenants entraron al barrido, que es lo que se compara contra
// el techo de `proyectosParaExportar`.
func proyectosDistintos(vistos []fleet.Device) int {
	p := map[string]struct{}{}
	for _, d := range vistos {
		p[d.ProjectID] = struct{}{}
	}
	return len(p)
}

// nombreReferenciaVersion dice si el cerebro puede comparar la versión de un agente contra la
// suya. Es UNA serie y no una por máquina: es un hecho del CEREBRO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `agent_stale` se OMITE cuando la comparación no se puede hacer, y una de las razones está del
// lado del cerebro: si su propia versión no se puede parsear —una cadena vacía porque el build no
// la selló, o una de cuatro componentes que `NucleoDeVersion` no entiende— entonces
// `VersionDelAgenteDifiere` devuelve `comparable=false` PARA TODAS LAS MÁQUINAS. La serie
// desaparece de la flota entera y `AgenteDesactualizado` queda imposible de disparar.
//
// Omitir es lo correcto: marcar a toda la flota como atrasada sería culparla de un problema del
// build propio. Lo que faltaba es que la omisión SE PUEDA VER. Es el incidente del 2026-09-09,
// del que se arregló la causa conocida —un argumento omitido— y no la FORMA de falla, que sigue
// viva: cualquier versión que el parser no entienda apaga el eje en silencio, con todo en verde.
//
// EMITIRLA POR MÁQUINA HABRÍA SIDO EL ERROR OBVIO, y lo cazaron dos guardas apenas se intentó:
// `TestElEmpujeNoLlevaLasMetricasDelServidor` y la que compara scrape contra empuje. No es
// telemetría de una máquina; es una propiedad del exportador, como los techos.
//
// Vale 0 y no se omite: acá el 0 es un hecho que el cerebro conoce con certeza sobre SÍ MISMO
// —«no puedo parsear mi propia versión»— y no un «no se pudo medir».
// ────────────────────────────────────────────────────────────────────────────────────────────
const nombreReferenciaVersion = "musubi_fleet_version_reference_usable"

// renderReferenciaDeVersion emite la serie de arriba. Una sola línea, sin etiquetas.
func renderReferenciaDeVersion(b *strings.Builder, versionCerebro string) {
	valor := 0
	if _, ok := fleet.NucleoDeVersion(versionCerebro); ok {
		valor = 1
	}
	fmt.Fprintf(b, "# HELP %s 1 si el cerebro puede parsear SU PROPIA versión y por lo tanto comparar la de cada agente; 0 si no. Con 0, musubi_fleet_device_agent_stale se omite para la FLOTA ENTERA y AgenteDesactualizado queda imposible de disparar, sin un solo error. La versión del cerebro se sella en el build.\n# TYPE %s gauge\n", nombreReferenciaVersion, nombreReferenciaVersion)
	fmt.Fprintf(b, "%s %d\n", nombreReferenciaVersion, valor)
}

// renderTechos emite, por dimensión recortable, el techo vigente y el uso actual.
//
// `usoProyectos` y `usoServiciosMax` los mide el barrido que acaba de correr: el uso que importa
// es el del PEOR proyecto, porque el techo se aplica por proyecto y un promedio escondería
// justamente al que está por cortar.
func renderTechos(b *strings.Builder, techoServicios, usoProyectos, usoServiciosMax int) {
	fmt.Fprintf(b, "# HELP %s Techo vigente por dimensión recortable del export. `kind=projects` es una constante de compilación (proyectosParaExportar); `kind=services` es la perilla `fleet.services_per_project_export`; `kind=heartbeat_services` es cuántos servicios acepta UN latido (fleet.ServiciosPorLatido). AUSENTE cuando esa dimensión no tiene techo.\n# TYPE %s gauge\n", nombreTecho, nombreTecho)
	fmt.Fprintf(b, "%s{kind=\"projects\"} %d\n", nombreTecho, proyectosParaExportar)
	if techoServicios > 0 {
		// SE OMITE CUANDO NO HAY TECHO, y no se emite un 0: un 0 se leería como «no entra ni un
		// servicio», que es lo contrario de lo que significa. Ver la regla de este archivo.
		fmt.Fprintf(b, "%s{kind=\"services\"} %d\n", nombreTecho, techoServicios)
	}
	fmt.Fprintf(b, "%s{kind=\"heartbeat_services\"} %d\n", nombreTecho, fleet.ServiciosPorLatido)

	fmt.Fprintf(b, "# HELP %s Uso actual de cada dimensión recortable, contra el techo de %s. `kind=services` es el PEOR proyecto y no el promedio: el techo se aplica por proyecto, así que un promedio escondería justo al que está por cortar.\n# TYPE %s gauge\n", nombreUso, nombreTecho, nombreUso)
	fmt.Fprintf(b, "%s{kind=\"projects\"} %d\n", nombreUso, usoProyectos)
	fmt.Fprintf(b, "%s{kind=\"services\"} %d\n", nombreUso, usoServiciosMax)
}

// describirTechoDeServicios pone en palabras el techo VIGENTE, y es la ÚNICA fuente de esa frase.
//
// Estaba escrita adentro del Fprintf del HELP y repetida —con otro número— en el comentario que
// emite renderServicios. Dos lugares donde escribir un techo es un lugar de más: el defecto que
// le dio nombre a este cabo fue justamente que un mensaje nombrara un techo que no era el que
// cortó.
//
// El apagado NO nombra ningún número: decir «2000, pero desactivado» manda a alguien a buscar un
// corte a 2000 que no puede ocurrir. Y NINGUNO quiere decir ninguno: la frase decía «este punto
// no puede valer 1» —el 1 es el valor del gauge, no un techo— y ese dígito alcanzaba para que la
// única regla que se puede medir sobre esta rama («acá no hay ningún número que sea un techo»)
// tuviera una excepción, o sea para que no se pudiera medir. Se dice con palabras.
func describirTechoDeServicios(techoServicios int) string {
	if techoServicios <= 0 {
		return "el techo de servicios, que está DESACTIVADO (`fleet.services_per_project_export` negativo), así que este punto no puede encenderse"
	}
	return fmt.Sprintf("de %d servicios (techo `fleet.services_per_project_export`)", techoServicios)
}

// devicesVisiblesParaMetricas resuelve QUÉ máquinas ve `p`, ya ordenadas por (proyecto, nombre).
//
// Es el ÚNICO lugar donde se combinan proyectosVisibles y PuedeSobreDevice, y por eso lo comparten
// el scrape y el empuje: un segundo recorrido de la flota es un segundo lugar donde olvidarse la
// compuerta, y ese olvido no se ve —exporta de más, calladito— hasta que alguien audita.
func devicesVisiblesParaMetricas(engine memory.StorageBackend, p *Principal) (vistos []fleet.Device, truncado bool, ilegible bool) {
	proyectos, truncado, ilegible := proyectosVisibles(engine, p)
	for _, proy := range proyectos {
		devices, err := engine.ListarDevices(proy, false)
		if err != nil {
			// HERMANO DEL DE SERVICIOS: un proyecto ilegible no puede tumbar el scrape entero,
			// pero sus máquinas no se exportan y sin serie no hay alerta que las cubra. El
			// `continue` mudo dejaba eso indistinguible de «ese proyecto no tiene máquinas».
			ilegible = true
			logx.Error("export de flota: no se pudieron listar las máquinas de un proyecto; esas máquinas NO se exportan y quedan sin serie (ninguna alerta las cubre)",
				"project", proy, "error", err,
				"serie", nombreExportTruncado+`{kind="unreadable"}`)
			continue
		}
		for _, d := range devices {
			if PuedeSobreDevice(p, d, fleet.CapMetrics) {
				vistos = append(vistos, d)
			}
		}
	}
	sort.Slice(vistos, func(i, j int) bool {
		if vistos[i].ProjectID != vistos[j].ProjectID {
			return vistos[i].ProjectID < vistos[j].ProjectID
		}
		return vistos[i].Name < vistos[j].Name
	})
	return vistos, truncado, ilegible
}

// serieDeFlota es UNA métrica exportable de una máquina.
//
// La lista la produce seriesDeFlota() y la consumen LOS DOS caminos de salida —el scrape de
// /metrics y el empuje OTLP—, para que no puedan discrepar. El día que alguien agregue un campo a
// fleet.Muestra lo agrega acá y aparece en los dos lados; con dos copias aparece en uno.
type serieDeFlota struct {
	Nombre string
	Ayuda  string
	// Unidad es la unidad OTLP (UCUM). El exposition format la ignora: los nombres ya la llevan
	// en el sufijo, que es la convención de Prometheus.
	//
	// OJO CON EL "1" DE LO ADIMENSIONAL, que es la trampa de este campo: el receptor OTLP de
	// Prometheus NORMALIZA el nombre con la unidad, y a un gauge con unidad "1" le agrega el
	// sufijo `_ratio` — `musubi_fleet_device_up` llegaría como `musubi_fleet_device_up_ratio` y
	// las 12 reglas de deploy/musubi-alerts-flota.yml seguirían evaluándose sin disparar NUNCA.
	// Por eso lo adimensional viaja con unidad VACÍA, y las demás sólo declaran una unidad que el
	// nombre YA lleva (bytes, seconds, celsius, percent), que es el caso en el que la
	// normalización no agrega nada. Lo custodia TestNingunaUnidadRenombraLaSerieEnPrometheus.
	Unidad string
	// Entera decide la codificación OTLP del punto: asInt (string) o asDouble (número). En el
	// exposition format no cambia nada —formatearValor ya imprime los enteros sin decimales—,
	// así que las dos salidas siguen coincidiendo valor por valor.
	Entera bool
	Valor  func(d fleet.Device, m *fleet.Muestra) (float64, bool)
}

// seriesDeFlota devuelve las 24 series en orden estable: las TRES que salen de la fila del device
// (up, last_seen, agent_stale) y las 18 que salen de la MUESTRA.
//
// `ahora` e `intervaloSonda` entran por parámetro porque tres series son relativas al reloj (up,
// last_seen, sample_age): con un reloj por serie, `up` podría decir «viva» y `sample_age` medirse
// contra otro instante. Un solo reloj por export, y el empuje además lo usa para sellar los puntos.
// `versionCerebro` entra por lo mismo y por una razón más: `internal/mcp` no puede leer la variable
// que el build inyecta en `main`, así que la referencia contra la que se compara cada agente viaja
// desde arriba o no existe.
// `enMantenimiento` es el conjunto de máquinas con una ventana activa. Entra como parámetro y no
// se consulta adentro de cada `Valor` porque la tabla se arma UNA vez por scrape y los `Valor` se
// llaman una vez por máquina y por serie: una consulta ahí adentro serían 21 consultas por
// máquina y por scrape.
func seriesDeFlota(ahora time.Time, intervaloSonda time.Duration, versionCerebro string, enMantenimiento map[string]bool) []serieDeFlota {
	return []serieDeFlota{
		// LA VENTANA DE MANTENIMIENTO, COMO SERIE (Ola 1).
		//
		// Es lo que le permite a una regla decir «no alertes de esta máquina ahora» con la misma
		// forma con la que ya dice «no alertes de una máquina caída»:
		//
		//     unless on(project, device) (musubi_fleet_device_maintenance == 1)
		//
		// Vale 0 fuera de la ventana y no se omite, al revés que las series de medición: acá el 0
		// es un hecho que el cerebro conoce con certeza —«esta máquina NO está en mantenimiento»—
		// y no un «no se pudo medir». Y hace falta que exista: un `unless` contra una serie que
		// sólo aparece durante la ventana funciona igual, pero nadie podría graficar ni auditar
		// cuánto tiempo estuvo una máquina en mantenimiento.
		{"musubi_fleet_device_maintenance",
			"1 si la máquina tiene una ventana de mantenimiento ACTIVA. Lo declara una persona con musubi_fleet_maintenance; mientras vale 1, las políticas de auto-heal no actúan sobre ella y las reglas que la miran no alertan.",
			"", true,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				if enMantenimiento[d.ID] {
					return 1, true
				}
				return 0, true
			}},
		// SI EL TOKEN DE ESTA MÁQUINA PUEDE ROTAR (A102).
		//
		// AUSENTE CUANDO EL AGENTE NO LO DIJO, y eso no es un detalle: `0` significa «reportó que su
		// token vino por variable y NO puede rotar», que es una acusación concreta. Un agente viejo
		// no manda el campo, y publicar 0 por su silencio marcaría como defectuosa a media flota sin
		// que nadie lo haya medido. Es la regla de este plano: AUSENTE NO ES CERO.
		//
		// Con `variable` el token además queda en el ENTORNO del proceso, donde lo lee cualquier
		// proceso del mismo usuario y donde sobrevive a que alguien arregle el archivo — el mecanismo
		// exacto de A88, mirado desde el otro lado.
		{"musubi_fleet_device_token_rotable",
			"1 si el token del dispositivo vino por ARCHIVO y una rotación se puede completar; 0 si vino por variable de entorno y no. AUSENTE si el agente no lo reporta (agente viejo): un 0 inventado acusaría a la máquina de algo que nadie midió.",
			"", true,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				rotable, seSabe := fleet.CredencialRotable(d.TokenFuente)
				if !seSabe {
					return 0, false
				}
				if rotable {
					return 1, true
				}
				return 0, true
			}},
		// QUÉ CONTRATO DICE HABLAR ESTA MÁQUINA.
		//
		// ────────────────────────────────────────────────────────────────────────────────────
		// EXISTE PORQUE DOS EJES DE ESTE PLANO SE APAGAN SOLOS Y NADIE PODÍA DECIR POR QUÉ.
		//
		// `services_unknown` y `services_omitted` se OMITEN cuando el agente declara un capver
		// por debajo de `CapverConInventarioExplicado`, y eso está bien: un agente viejo no puede
		// distinguir «enumeré y no hubo error» de «no sé enumerar», así que afirmar 0 sería una
		// alerta PERDIDA — silenciosa, que es peor que una falsa.
		//
		// Pero la ausencia quedaba MUDA. Desde Prometheus, «esta máquina tiene el eje apagado
		// porque su agente es viejo» y «esta máquina no existe» se ven igual: las dos son una
		// serie que no está. Las dos alertas que cerraron A116 nacieron mudas para toda la flota
		// —el Capver subió a 2 el 2026-09-10 y los agentes se cruzan a mano— y nada lo decía.
		//
		// Con esta serie la ausencia tiene nombre: `AgenteSinContratoDeclarado` dice que el
		// agente no declara capver, y `AgenteConContratoViejo` dice que declara uno por debajo
		// del que esos ejes necesitan. Un eje apagado deja de ser silencio y pasa a ser un aviso
		// con la máquina nombrada.
		//
		// SE OMITE CON 0, y no es el cero que este plano prohíbe: `Capver = 0` significa
		// literalmente «este agente no declaró nada» —lo dice `protocolo.go`— y emitirlo como
		// número lo convertiría en «habla la versión cero», que es una afirmación que nadie hizo.
		// Ver la regla de este archivo: AUSENTE NO ES CERO.
		// ────────────────────────────────────────────────────────────────────────────────────
		{"musubi_fleet_device_capver",
			"Versión de CAPACIDADES del protocolo que declara el agente de esta máquina. NO es la versión del producto (ésa es musubi_fleet_device_agent_stale): dos builds distintos pueden hablar el mismo contrato. AUSENTE si el agente no declara ninguna —un agente anterior a la pieza, o una máquina sin agente—, porque un 0 se leería como «habla la versión cero» y lo que pasa es que nadie afirmó nada. Los ejes services_unknown y services_omitted se apagan por debajo de 2.",
			"", true,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				if d.Capver <= 0 {
					return 0, false
				}
				return float64(d.Capver), true
			}},
		// HACE CUÁNTO QUE LATE EL MISMO PROCESO.
		//
		// ────────────────────────────────────────────────────────────────────────────────────
		// DOS AGENTES SOBRE UNA FILA ERAN INDISTINGUIBLES DE UNO. La credencial del latido es de
		// la MÁQUINA y no del proceso, así que dos agentes corriendo a la vez —un servicio más
		// una corrida a mano, una instalación duplicada, un zombi del binario renombrado—
		// escriben los dos sobre la misma fila y el cerebro ve un único agente sano.
		//
		// ESTA SERIE LOS SEPARA SIN INFERIR NADA. Con UN agente, la marca se escribe al arrancar
		// y esta serie crece sin parar. Con DOS alternándose, cada latido trae un emisor distinto
		// del guardado, la marca vuelve a cero, y la serie se queda pegada al cero para siempre.
		//
		// ASÍ SE CERRÓ A92 CON EL PROBLEMA TODAVÍA PUESTO: el diagnóstico se hizo midiendo la
		// CADENCIA —un diente de sierra de 37,8 s contra 25,8 s del vecino—, una inferencia sobre
		// un efecto de segundo orden que sólo se puede hacer mirando a mano y sabiendo de
		// antemano que hay que mirar.
		//
		// AUSENTE si el agente no declara emisor (un binario anterior a la pieza) o si nunca
		// latió. Un 0 ahí sería idéntico a «dos agentes peleándose», que es justo lo contrario de
		// lo que pasa: nadie afirmó nada.
		// ────────────────────────────────────────────────────────────────────────────────────
		{"musubi_fleet_device_emitter_stable_seconds",
			"Hace cuántos segundos que late el MISMO proceso sobre esta fila. Crece mientras haya un solo agente; se queda cerca de CERO si hay dos alternándose, porque cada uno pisa la marca del otro. AUSENTE si el agente no declara su emisor (binario anterior a capver 3) — un 0 ahí sería indistinguible de dos agentes peleándose.",
			"s", false,
			func(d fleet.Device, _ *fleet.Muestra) (float64, bool) {
				if d.Emisor == "" || d.EmisorDesde.IsZero() {
					return 0, false
				}
				return ahora.Sub(d.EmisorDesde).Seconds(), true
			}},
		// CUÁNTOS SERVICIOS NO ENTRARON EN EL ÚLTIMO INVENTARIO (A116).
		//
		// SIEMPRE PRESENTE, INCLUIDO EL 0, y acá el criterio es el OPUESTO al de `token_rotable`
		// de arriba — vale explicar por qué, porque las dos reglas conviven en el mismo archivo.
		// Allá el 0 sería una acusación («no puede rotar») que nadie midió, así que la ausencia es
		// lo honesto. Acá el 0 dice «no recortó», que es lo que hace HOY un agente viejo: no se
		// afirma nada que no esté pasando. Y si esta serie desapareciera cuando el inventario está
		// completo, `absent()` no distinguiría «está completo» de «esta máquina no reporta», que es
		// justamente la confusión que esta métrica existe para deshacer.
		//
		// Un valor > 0 significa que el inventario que el cerebro tiene de esa máquina es PARCIAL,
		// y que la poda por ausencia está suspendida ahí: «lo que no vino» dejó de significar «ya
		// no corre». Sin esta serie eso sólo se ve contando a mano en la máquina correcta, que es
		// como se encontró — de casualidad, después de que una limpieza pedida por otra razón
		// cambiara los números.
		{"musubi_fleet_device_services_omitted",
			"Cuántos servicios NO entraron en el último inventario de esta máquina por el techo del latido. 0 = el inventario está completo. Mayor que 0 = el inventario del cerebro es PARCIAL y la poda por ausencia está suspendida para esa máquina.",
			"", false,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				if d.Capver < buildid.CapverConInventarioExplicado {
					return 0, false // no sabe contar lo que recortó: ausente, no cero
				}
				return float64(d.ServiciosOmitidos), true
			}},
		// EL FALLO DEL ENUMERADOR, QUE ANTES SÓLO EXISTÍA EN EL LOG DE LA MÁQUINA.
		//
		// Es el OPUESTO de `services_omitted`, no su hermano: aquél es «enumeré bien y no entró
		// todo», éste es «no pude enumerar», o sea que no viajó NADA. Del lado del cerebro ese
		// silencio era idéntico al de un inventario estable.
		//
		// MEDIDO EN `davantis-1` EL 2026-09-09: 64 alertas `ServicioSinNoticias` —una por servicio
		// conocido—, 61 horas sin reportar, con el agente vivo y mandando CPU y uptime. Sesenta y
		// cuatro alertas para UNA causa, y ninguna la nombra. Con esta serie la causa tiene UNA
		// alerta propia, y `ServicioSinNoticias` se calla para esa máquina — igual que ya se calla
		// cuando la máquina está caída o en mantenimiento, y por el mismo motivo: no son 64
		// problemas, es uno.
		//
		// EL MOTIVO NO VA COMO ETIQUETA. Es texto libre que escribe la máquina y su cardinalidad no
		// la elige nadie de este lado — sería la misma decisión que ya se tomó para el desglose de
		// servicios. Cuál es se mira en `musubi_fleet_list`, que es donde vive el texto.
		{"musubi_fleet_device_services_unknown",
			"1 si esta máquina NO PUDO enumerar sus servicios en su último latido, 0 si pudo. Cuando es 1 el inventario dejó de viajar entero —el agente manda la lista completa o no la manda— así que lo guardado no se pierde pero envejece, y a los 30 min salta `ServicioSinNoticias` por cada servicio conocido. POR QUÉ no pudo se mira en `musubi_fleet_list`: el motivo es texto libre de la máquina y como etiqueta sería cardinalidad sin techo.",
			"", false,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				// EL 0 SÓLO VALE SI LA MÁQUINA SABE DECIR QUE NO (capver >= 2).
				//
				// Un agente anterior no manda `servicios_error`, así que llega `""` — idéntico al
				// `""` de uno que enumeró bien. Devolver 0 ahí es afirmar «enumeró» sobre una
				// máquina que nunca contestó la pregunta, y `MaquinaNoPuedeEnumerar` queda VERDE
				// POR IGNORANCIA. Como la alerta dispara con `== 1`, ese falso 0 es una alerta
				// PERDIDA y no una falsa: silenciosa.
				//
				// Medido el 2026-09-09: las cuatro máquinas de la flota con la serie en 0, dos de
				// ellas con agentes que no conocen el campo, y una sin agente ninguno.
				//
				// Se OMITE en vez de inventar un tercer valor: es la regla que ya gobierna este
				// exportador —un dato ausente no es un cero—, y del lado de Prometheus «no sé» se
				// pregunta con `absent()`, que es su forma natural. Un `-1` habría que enseñárselo
				// a cada regla que la use, y la que se olvide lo lee como un número.
				if d.Capver < buildid.CapverConInventarioExplicado {
					return 0, false
				}
				if d.ServiciosError != "" {
					return 1, true
				}
				return 0, true
			}},
		{"musubi_fleet_device_up",
			"1 si la máquina dio señal de vida dentro de SU umbral, 0 si no. El umbral es por tier: 90s (3 latidos) con agente, 3x el intervalo de sondeo sin agente.",
			"", false,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				if d.EnLinea(ahora, umbralEnLineaPara(d, intervaloSonda)) {
					return 1, true
				}
				return 0, true
			}},
		{"musubi_fleet_device_last_seen_seconds",
			"Segundos desde el último latido. Ausente si la máquina nunca latió.",
			"s", false,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				if d.LastSeen.IsZero() {
					return 0, false
				}
				return ahora.Sub(d.LastSeen).Seconds(), true
			}},

		// De acá abajo, todo sale de la MUESTRA. Una máquina que no reportó no aporta ninguna de
		// estas series — y ésa es la regla central del export, ver escribirGauge.
		{"musubi_fleet_device_cpu_percent", "Uso de CPU (0-100), promedio del intervalo entre latidos. AUSENTE en el primer latido de un agente: el porcentaje es una derivada y hace falta una lectura anterior.",
			"%", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.CPUPct) })},
		{"musubi_fleet_device_cpus", "Cantidad de CPUs.",
			"", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.NumCPU), m.NumCPU > 0 })},
		{"musubi_fleet_device_memory_total_bytes", "RAM total.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.MemTotal), m.MemTotal > 0 })},
		{"musubi_fleet_device_memory_used_bytes", "RAM usada (total menos MemAvailable, no menos MemFree: el page cache no cuenta como ocupado).",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.MemUsada), m.MemTotal > 0 })},
		// AUSENTE en Windows y macOS: ninguno de los dos expone el equivalente de MemFree sin
		// mentir (ullAvailPhys es el análogo de MemAvailable, no de MemFree).
		{"musubi_fleet_device_memory_free_bytes", "RAM que el kernel no tiene asignada a nada (MemFree). NO es total menos usada: la usada sale de MemAvailable, y el page cache vive en el medio. AUSENTE en Windows y macOS, que no la exponen sin mentir.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDeBytes(m.MemLibre) })},
		{"musubi_fleet_device_swap_total_bytes", "Swap total.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.SwapTotal), m.SwapTotal > 0 })},
		{"musubi_fleet_device_swap_used_bytes", "Swap usada.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.SwapUsada), m.SwapTotal > 0 })},
		{"musubi_fleet_device_disk_total_bytes", "Tamaño del filesystem raíz.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoTotal), m.DiscoTotal > 0 })},
		{"musubi_fleet_device_disk_used_bytes", "Ocupado por archivos (como la columna Used de df).",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoUsado), m.DiscoTotal > 0 })},
		{"musubi_fleet_device_disk_available_bytes", "Lo que una aplicación todavía puede escribir (columna Avail de df). NO es total menos usado: entre medio está la reserva de root (~5%). Ésta es la serie para alertar.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoDisponible), m.DiscoTotal > 0 })},
		// AUSENTE en Windows: la carga es un concepto de UNIX y ahí no existe, así que la serie
		// no se emite en vez de emitir un 0 que se leería como «máquina ociosa».
		{"musubi_fleet_device_load1", "Carga a 1 minuto. AUSENTE en sistemas sin load average (Windows).",
			"", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load1) })},
		{"musubi_fleet_device_load5", "Carga a 5 minutos. AUSENTE en sistemas sin load average (Windows).",
			"", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load5) })},
		{"musubi_fleet_device_load15", "Carga a 15 minutos. AUSENTE en sistemas sin load average (Windows).",
			"", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load15) })},
		{"musubi_fleet_device_uptime_seconds", "Segundos desde el arranque de la máquina.",
			"s", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.UptimeSeg), m.UptimeSeg > 0 })},
		{"musubi_fleet_device_temperature_celsius", "Zona térmica preferida por tipo (CPU antes que chasis); si ninguna, la más alta plausible. AUSENTE si la máquina no expone sensor.",
			"Cel", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.TempC) })},
		// El nombre viaja en INGLÉS aunque el campo de la muestra sea `num_procesos`: adentro el
		// JSON está en castellano, y en Prometheus la convención del ecosistema es inglesa. El
		// punto de traducción es éste y ninguno más.
		{"musubi_fleet_device_processes", "Procesos, no hilos (el 4º campo de /proc/loadavg cuenta hilos y da 3 a 5 veces más). AUSENTE en macOS: contarlos ahí exigiría un fork+exec por latido en el proceso que corre en todas las máquinas.",
			"", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.NumProcesos), m.NumProcesos > 0 })},
		{"musubi_fleet_device_sample_age_seconds", "Antigüedad de la muestra. Si crece sin parar, el agente late pero dejó de medir.",
			"s", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return ahora.Sub(m.Tomada).Seconds(), !m.Tomada.IsZero() })},
		{"musubi_fleet_device_reach_up",
			"1 si esta máquina alcanza TODOS los destinos que le declararon (A67), 0 si falla alguno. AUSENTE si no tiene ninguno configurado: nadie le pidió que mirara, que no es lo mismo que no llegar. CUÁL destino falla se mira en musubi_fleet_list, no acá — sus valores los elige quien configura cada máquina y como etiqueta serían cardinalidad sin techo, la misma decisión que el desglose de servicios.",
			"", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) {
				if len(m.Alcance) == 0 {
					return 0, false
				}
				for _, s := range m.Alcance {
					if !s.Alcanza {
						return 0, true
					}
				}
				return 1, true
			})},
		// NO SALE DE LA MUESTRA: sale de la FILA del device (A68). `agent_version` la escribe
		// `LatirDevice` en cada latido y sobrevive a que la máquina se muera, así que una máquina
		// caída sigue diciendo en qué versión se quedó — que es lo que se quiere saber de ella.
		{"musubi_fleet_device_agent_stale",
			"1 si el agente corre un release distinto del cerebro, 0 si es el mismo. Compara el NÚCLEO semver y no el commit: el binario de cada máquina se cruza a mano y el del cerebro se redespliega varias veces por día, así que comparar commits dejaría a la flota entera marcada después de cada despliegue. AUSENTE en las máquinas sin agente (un Tier B sondeado por SSH no tiene versión que comparar) y AUSENTE también si el cerebro no sabe la suya: sin referencia, marcar a toda la flota sería culparla de un problema del build propio. CUÁL versión corre cada una se mira en musubi_fleet_list — ninguna de las dos viaja como etiqueta, que dejaría la serie re-etiquetándose sola en cada actualización y las viejas huérfanas.",
			"", false,
			func(d fleet.Device, _ *fleet.Muestra) (float64, bool) {
				difiere, comparable := fleet.VersionDelAgenteDifiere(d.AgentVer, versionCerebro)
				if !comparable {
					return 0, false
				}
				if difiere {
					return 1, true
				}
				return 0, true
			}},
	}
}

// deLaMuestra adapta una serie que sólo mira la MUESTRA a la firma que lleva también el device.
// Una máquina sin muestra no aporta la serie: la comprobación de nil está escrita UNA vez acá y
// no diecisiete veces, que es como se olvida en la número dieciocho.
func deLaMuestra(f func(*fleet.Muestra) (float64, bool)) func(fleet.Device, *fleet.Muestra) (float64, bool) {
	return func(_ fleet.Device, m *fleet.Muestra) (float64, bool) {
		if m == nil {
			return 0, false
		}
		return f(m)
	}
}

// escribirGauge emite un gauge con una línea por máquina.
//
// LA REGLA CENTRAL DEL EXPORT: si el valor es DESCONOCIDO, LA LÍNEA NO SE EMITE. No se emite 0.
//
// Es el mismo principio que el `null` de la tool (D1/D3), y en Prometheus importa todavía más:
// una serie ausente se dibuja como un hueco y `absent()` la puede alertar, mientras que un 0
// entra al gráfico como una medición real. Un `cpu_percent 0` durante el primer latido de cada
// agente pintaría una caída a cero en cada reinicio, y esas caídas fantasma son exactamente lo
// que hace que alguien deje de mirar un dashboard.
//
// Si ninguna máquina tiene el valor, tampoco se emiten HELP y TYPE: un bloque de cabeceras sin
// series es ruido.
func escribirGauge(b *strings.Builder, devices []fleet.Device, nombre, ayuda string,
	valor func(fleet.Device, *fleet.Muestra) (float64, bool)) {

	var cuerpo strings.Builder
	for _, d := range devices {
		v, ok := valor(d, d.UltimaMuestra)
		if !ok {
			continue
		}
		fmt.Fprintf(&cuerpo, "%s{%s} %s\n", nombre, etiquetasDe(d), formatearValor(v))
	}
	if cuerpo.Len() == 0 {
		return
	}
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s gauge\n", nombre, ayuda, nombre)
	b.WriteString(cuerpo.String())
}

// labelsDeFlota es EL juego de labels de una máquina, en orden canónico y en UN solo lugar.
//
// Cardinalidad acotada a propósito: nombre, proyecto, tier y OS. Las TAGS quedan afuera —son texto
// libre del administrador y meterlas haría explotar la cardinalidad de la serie, que es la forma
// clásica de voltear un Prometheus. Y NADA de lo que la máquina reporta de sí misma entra acá
// (versión del agente, dirección, id de RustDesk): eso la dejaría re-etiquetándose sola.
//
// etiquetasDe lo formatea para el exposition format y atributosOTLP para el empuje. Armarlos dos
// veces es cómo un renombre de `device` a `hostname` deja las 12 reglas de
// deploy/musubi-alerts-flota.yml evaluándose para siempre sin disparar nunca.
func labelsDeFlota(d fleet.Device) [4][2]string {
	return [4][2]string{
		{"device", d.Name},
		{"project", d.ProjectID},
		{"tier", string(d.Tier)},
		{"os", d.OS},
	}
}

// etiquetasDe formatea los labels para el exposition format.
func etiquetasDe(d fleet.Device) string {
	var b strings.Builder
	for i, kv := range labelsDeFlota(d) {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(kv[0])
		b.WriteByte('=')
		b.WriteString(citarLabel(kv[1]))
	}
	return b.String()
}

// citarLabel escapa un valor de label según el exposition format: backslash, comilla y salto de
// línea. No es teórico: el nombre de una máquina lo escribe un administrador, y un device llamado
// `a"b` partiría la línea en dos y corrompería TODO el scrape, no sólo esa serie.
func citarLabel(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}

// formatearValor evita la notación científica de %v para los enteros grandes (los bytes de un
// disco de 500 GB) y no arrastra decimales inútiles.
func formatearValor(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.4g", v)
}

// proyectosVisibles resuelve QUÉ proyectos barre este scrape.
//
// Un principal acotado barre el suyo y nada más. Uno read=all (la cabina, la sala de mando, el
// scraper declarado como corresponde) barre todos los que tengan máquinas. El stdio local —que
// no llega acá por HTTP, pero el seam lo admite— también.
func proyectosVisibles(engine memory.StorageBackend, p *Principal) (proyectos []string, truncado bool, ilegible bool) {
	federado := p == nil
	if p != nil {
		if read, _ := p.caps(); read == ReadAll {
			federado = true
		}
	}
	if !federado {
		if p.ProjectID == "" {
			// No es un fallo: esta credencial NO tiene proyecto, y eso está medido.
			return nil, false, false
		}
		return []string{p.ProjectID}, false, false
	}
	todos, err := engine.ProyectosConDevices(proyectosParaExportar + 1)
	if err != nil {
		// EL PEOR DE LOS TRES HERMANOS: acá no se pierde un proyecto, se pierden TODOS, y el
		// `return nil, false` decía «no hay nada que exportar y no se recortó nada» — un export
		// entero en cero que desde Prometheus se lee igual que una flota apagada.
		logx.Error("export de flota: no se pudo listar NINGÚN proyecto con máquinas; este scrape no exporta ni una máquina",
			"error", err, "serie", nombreExportTruncado+`{kind="unreadable"}`)
		return nil, false, true
	}
	if len(todos) > proyectosParaExportar {
		return todos[:proyectosParaExportar], true, false
	}
	return todos, false, false
}

// valorDe traduce el vocabulario del «no sé» del dominio (un puntero nil) al del exportador (el
// bool que decide si la línea se emite). Una sola traducción, para que ningún campo opcional
// nuevo se olvide de respetarla.
func valorDe(p *float64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return *p, true
}

// valorDeBytes es el gemelo de valorDe para los contadores de BYTES opcionales (*uint64). Existe
// por lo mismo: que la traducción del «no sé» esté escrita una vez y no se le olvide a nadie.
func valorDeBytes(p *uint64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return float64(*p), true
}
