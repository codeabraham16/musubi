package mcp

// fleet_export_techos_cableado_test.go cierra los agujeros que dejó la primera tanda de guardas
// de los techos del exportador. Todos MEDIDOS: alguien reprodujo el defecto de una forma que la
// guarda no miraba y la suite quedó en verde.
//
// El patrón de los cinco es el mismo y conviene nombrarlo: LA GUARDA MIRABA LA FUNCIÓN Y NO EL
// CABLE, o miraba EL TEXTO y no EL NÚMERO.
//
//  1. La prueba de la perilla llamaba a `renderFlota` pasándole el techo a mano, así que el
//     cableado `s.techoServiciosPorProyecto → /metrics` nunca se ejercitaba.
//  2. El default del constructor podía ponerse en 0 —que significa SIN TECHO— y nada lo notaba.
//  3. El aviso del empuje se asertaba por el NOMBRE de la perilla, así que imprimir el número
//     equivocado al lado de ese nombre pasaba en verde. Es el mismo defecto que le da nombre al
//     cabo, un nivel más adentro.
//  4. El rearme del aviso vivía en dos ramas `else` que se podían borrar sin que nada se moviera.
//  5. El `# HELP` podía volver a la constante mientras el corte lo hacía otro número.

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// metricsDeVerdad hace el GET a /metrics con la credencial de Prometheus y devuelve el cuerpo.
// No hay atajo: el invariante que se prueba es el CABLE entre el campo del servidor y la boca.
func metricsDeVerdad(t *testing.T, s *McpServer) string {
	t.Helper()
	reg := registroDePrueba(prometheusConToken("token-de-prometheus"))
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, registry: reg}))
	t.Cleanup(ts.Close)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer token-de-prometheus")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics devolvió %d", resp.StatusCode)
	}
	crudo, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("leyendo /metrics: %v", err)
	}
	return string(crudo)
}

func prometheusConToken(token string) Principal {
	p := principalDePrometheus()
	p.hash = hashToken(token)
	return p
}

// flotaDesdeYAML parsea la config como la parsea el cerebro al arrancar. Entrar por un literal de
// Go acá sería saltearse la mitad del camino que se está probando.
func flotaDesdeYAML(t *testing.T, texto string) config.FleetConfig {
	t.Helper()
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(texto), &cfg); err != nil {
		t.Fatalf("el YAML de prueba no parsea: %v", err)
	}
	return cfg.Fleet
}

// ── (1) LA BOCA PRINCIPAL OBEDECE LA PERILLA ────────────────────────────────────────────────
//
// `internal/mcp/http.go` le pasa `s.techoServiciosPorProyecto` a renderFlota. Se cambió esa
// expresión por la constante default y la suite entera quedó en VERDE: `services_per_project_
// export: 300` dejaba de cambiar NADA en /metrics —el tirón de Prometheus, la boca que el RUNBOOK
// manda a mirar— y nadie se enteraba. La guarda de la ronda 1 llamaba a `renderFlota` pasándole
// el techo por parámetro, así que probaba la función y no el cable.
//
// Es el HERMANO de fleet_otlp.go:508, que sí estaba cubierto: el mismo campo, la otra boca.
//
// ESTA PRUEBA ENTRA POR EL PRINCIPIO Y SALE POR EL FINAL: texto YAML → config.Config →
// ConfigurarFlota → servidor → GET /metrics por HTTP → líneas del exposition format.
//
// Sabotaje que la pone roja: en http.go, cambiar `s.techoServiciosPorProyecto` por
// `serviciosPorProyectoDefault` (o por cualquier otra cosa que no sea el campo).
func TestLaPerillaGobiernaElMetricsDeVerdadYNoSoloAlRenderFlota(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	serviciosDePrueba(t, s, d, 6, ahora)

	// CON LA PERILLA EN 3, POR LA BOCA DE VERDAD, salen 3.
	if err := s.ConfigurarFlota(flotaDesdeYAML(t, "fleet:\n  services_per_project_export: 3\n")); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	salida := metricsDeVerdad(t, s)
	if n := len(lineasDe(salida, "musubi_fleet_service_up{")); n != 3 {
		t.Errorf("`services_per_project_export: 3` y por /metrics salieron %d series de servicio (tenían que salir 3): la perilla llega al campo del servidor pero el handler no la usa, así que en producción no cambia nada", n)
	}
	if !strings.Contains(salida, nombreExportTruncado+`{kind="services"} 1`) {
		t.Errorf("cortó el techo de servicios y /metrics no lo declara:\n%s", bloqueDeTruncado(salida))
	}

	// Y CON EL TECHO DESACTIVADO salen los seis. El caso importa aparte: un handler clavado en la
	// constante 2000 pasaría el caso de arriba si la prueba usara un techo mayor a 6.
	if err := s.ConfigurarFlota(flotaDesdeYAML(t, "fleet:\n  services_per_project_export: -1\n")); err != nil {
		t.Fatalf("ConfigurarFlota sin techo: %v", err)
	}
	salida = metricsDeVerdad(t, s)
	if n := len(lineasDe(salida, "musubi_fleet_service_up{")); n != 6 {
		t.Errorf("con `services_per_project_export: -1` (sin techo) por /metrics salieron %d series y tenían que salir 6", n)
	}
	if !strings.Contains(salida, nombreExportTruncado+`{kind="services"} 0`) {
		t.Errorf("sin techo la serie tiene que decir 0 (medido y completo):\n%s", bloqueDeTruncado(salida))
	}
	// Y EL `# HELP` QUE SALE POR LA BOCA REAL tiene que hablar del techo vigente: con el techo
	// desactivado no puede seguir nombrando el 3 del scrape anterior. Se mira el NÚMERO y no la
	// frase, por lo mismo que el resto de este archivo.
	if enHelp(salida, 3) {
		t.Errorf("con el techo desactivado el # HELP de /metrics todavía nombra el 3 del scrape anterior:\n%s", bloqueDeTruncado(salida))
	}
}

// enHelp responde si el # HELP de la serie de truncado nombra ese número.
func enHelp(salida string, n int) bool {
	for _, l := range lineasDe(salida, "# HELP "+nombreExportTruncado) {
		if tieneNumero(numerosDe(l), n) {
			return true
		}
	}
	return false
}

func bloqueDeTruncado(salida string) string {
	var out []string
	for _, l := range strings.Split(salida, "\n") {
		if strings.Contains(l, nombreExportTruncado) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// ── (2) EL SERVIDOR NACE CON TECHO ──────────────────────────────────────────────────────────
//
// `server.go` arranca el campo en `config.Default().Fleet.EffectiveServicesPerProjectExport()` y
// no en el cero del struct, porque acá CERO SIGNIFICA SIN TECHO. Se puso ese campo en 0 y todo
// quedó verde: un servidor que nunca llama a ConfigurarFlota —un entrypoint sin sección `fleet:`,
// media suite de pruebas— exportaba sin ningún límite de cardinalidad, que es exactamente lo que
// el comentario de al lado dice estar evitando.
//
// LA ASERCIÓN ES DERIVADA, NO LITERAL: no dice «2000», dice «lo que el default efectivo diga, y
// que sea un techo». Subir el default por config.go no la rompe; apagarlo sí.
//
// Sabotaje que la pone roja: `techoServiciosPorProyecto: 0` en el constructor.
func TestElServidorNaceConTechoYNoSinTecho(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	esperado := config.Default().Fleet.EffectiveServicesPerProjectExport()

	if esperado <= 0 {
		t.Fatalf("el DEFAULT de la configuración ya no es un techo (%d): con <= 0 el export no tiene ningún límite de cardinalidad", esperado)
	}
	if s.techoServiciosPorProyecto != esperado {
		t.Errorf("un servidor recién construido tiene techo %d y el default efectivo es %d. Un entrypoint sin sección `fleet:` nunca llama a ConfigurarFlota: con 0 ahí, exporta sin ningún límite y la única señal es la factura de cardinalidad de Prometheus.",
			s.techoServiciosPorProyecto, esperado)
	}
	// Y el techo con el que nace tiene que SER un techo, no el «sin techo» escrito con otro
	// número: es la mitad que un `0` aprovecha.
	if s.techoServiciosPorProyecto <= 0 {
		t.Errorf("el servidor nace con techo %d, que en este código significa SIN TECHO", s.techoServiciosPorProyecto)
	}
}

// ── (3) EL AVISO IMPRIME EL NÚMERO QUE CORTÓ ────────────────────────────────────────────────
//
// La guarda de la ronda 1 exigía que el mensaje NOMBRARA la perilla
// (`services_per_project_export`). Se cambió el VALOR que acompaña a esa clave por la constante
// default y quedó verde: el aviso decía `techo=2000` cuando el techo que cortó era 2.
//
// ES EL DEFECTO QUE LE DA NOMBRE AL CABO EN OTRA FORMA. El de la ronda 1 era «el aviso nombra el
// techo equivocado»; éste es «el aviso nombra el número equivocado». La aserción de texto no lo
// ve porque el texto está bien: lo que miente es el número de al lado.
//
// SE PARSEA EL PAR CLAVE=VALOR Y SE COMPARA CONTRA EL TECHO VIGENTE. No se compara contra «2»:
// contra `s.techoServiciosPorProyecto`, que es lo que decidió el corte.
//
// Sabotaje que la pone roja: en fleet_otlp.go, cambiar `s.techoServiciosPorProyecto` por
// `serviciosPorProyectoDefault` en el argumento de `techo_servicios_por_proyecto`.
func TestElAvisoDelEmpujeImprimeElNumeroDelTechoQueCorto(t *testing.T) {
	destino := nuevoReceptor(t, http.StatusOK)
	s := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	ahora := time.Now()
	d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	serviciosDePrueba(t, s, d, 5, ahora)

	// Un techo distinto del default Y distinto de `proyectosParaExportar`: si coincidiera con
	// alguno, un aviso que imprimiera el número equivocado seguiría pasando.
	const techo = 2
	if techo == serviciosPorProyectoDefault || techo == proyectosParaExportar {
		t.Fatalf("el techo de la prueba (%d) coincide con otra constante del exportador y la haría inútil", techo)
	}
	s.techoServiciosPorProyecto = techo

	var log bytes.Buffer
	restaurar := logx.Capturar(&log)
	s.empujarUnaVez(context.Background(), ahora)
	restaurar()
	texto := log.String()

	valor, ok := valorDeClaveEnLog(texto, "techo_servicios_por_proyecto")
	if !ok {
		t.Fatalf("el aviso del techo de servicios no imprime `techo_servicios_por_proyecto`; quien lo lea no sabe contra qué número se cortó:\n%s", texto)
	}
	if valor != strconv.Itoa(s.techoServiciosPorProyecto) {
		t.Errorf("cortó el techo %d y el aviso dice %s. Nombrar la perilla correcta con el número equivocado es la MISMA falla que nombrar la perilla equivocada: el que lee compara ese número contra su config, ve que no coincide, y descarta el aviso.\n%s",
			s.techoServiciosPorProyecto, valor, texto)
	}

	// EL HERMANO: el aviso de proyectos tiene que imprimir SU número, que es la constante.
	s2 := prepararEmpuje(t, destino.URL, registroDePrueba(principalDePrometheus()), nil)
	s2.techoServiciosPorProyecto = 0
	for i := 0; i < proyectosParaExportar+1; i++ {
		maquinaConMuestra(t, s2, fmt.Sprintf("tenant-%03d", i), "server", *muestraDePrueba(), ahora)
	}
	log.Reset()
	restaurar = logx.Capturar(&log)
	s2.empujarUnaVez(context.Background(), ahora)
	restaurar()

	valor, ok = valorDeClaveEnLog(log.String(), "proyectos_por_scrape")
	if !ok {
		t.Fatalf("el aviso del techo de proyectos no imprime `proyectos_por_scrape`:\n%s", log.String())
	}
	if valor != strconv.Itoa(proyectosParaExportar) {
		t.Errorf("el techo de proyectos es %d y el aviso dice %s", proyectosParaExportar, valor)
	}
}

// valorDeClaveEnLog saca el valor de un par `clave=valor` de una línea del TextHandler de slog.
// Mira la FORMA del registro (el par) y no el texto del mensaje, que es lo que el sabotaje deja
// intacto.
func valorDeClaveEnLog(texto, clave string) (string, bool) {
	re := regexp.MustCompile(`(?m)\b` + regexp.QuoteMeta(clave) + `=("[^"]*"|\S+)`)
	m := re.FindStringSubmatch(texto)
	if m == nil {
		return "", false
	}
	return strings.Trim(m[1], `"`), true
}

// ── (4) EL REARME DE LOS AVISOS ─────────────────────────────────────────────────────────────
//
// Se borraron las dos ramas `else` que hacen el `Delete` del aviso dado y quedó verde. Un corte
// que se resuelve deja el aviso mudo PARA SIEMPRE, así que el próximo corte del mismo techo pasa
// en silencio — y eso no se ve, porque lo que falta es una línea de log que nadie está esperando.
// El commit de la ronda 1 afirmaba «los dos avisos se rearman al resolverse» y no había nada que
// lo sostuviera.
//
// LA PRUEBA MANEJA LA CONDICIÓN, NO LA RAMA: lleva al empuje a cada condición, verifica que el
// aviso quedó dado, la resuelve, y exige que el aviso se haya rearmado Y que un segundo corte
// vuelva a hablar. Un `else` borrado, un `Delete` con la clave mal escrita o un rearme puesto
// después de un `return` fallan todos igual.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// RONDA 3 — LA TABLA ESCRITA A MANO SIEMPRE LE FALTA EL PRÓXIMO, Y EL PRÓXIMO YA HABÍA NACIDO
//
// La ronda 2 dejó esta tabla con CINCO filas y en el mismo commit agregó un SEXTO aviso al
// empuje (`empuje_truncado_ilegible`). Devolver ese call site a la forma vieja —
// `if cond { avisarUnaVez(k, …) }`, sin rearme— dejaba el paquete entero en VERDE. Es
// literalmente el defecto dominante del repo (la guarda que está en N-1 de N caminos), con el
// agravante de que el hermano descubierto lo agregó el mismo commit que escribió la guarda.
//
// AGREGAR LA SEXTA FILA NO ARREGLA NADA: le faltaría el séptimo. Lo que cierra el agujero es
// DERIVAR la lista. `clavesDeAvisoDelArchivo` parsea el AST de fleet_otlp.go y saca TODAS las
// claves de `avisarUnaVez`/`avisoMientras` que emite ese archivo —en `empujarUnaVez` o en
// cualquier helper suyo—, y `TestTodoAvisoDelEmpujeTieneCasoDeRearme` exige que el conjunto
// derivado y el de la tabla sean EXACTAMENTE el mismo. Un aviso nuevo nace ROJO con el nombre de
// su clave y su línea en el mensaje; una fila que sobra también, así que la tabla tampoco se
// pudre en la otra dirección.
//
// Y LO QUE NO SE PUDO DERIVAR ES ROJO, NO VERDE: si la clave de un call site no es un literal
// (ni un literal concatenado, que es como se arma `empuje_permanente:`), la prueba falla
// diciendo el archivo y la línea. Un parser que se calla ante lo que no entiende es una guarda
// que se apaga sola con la próxima forma.

// casoDeRearme es UN aviso del empuje con la manera de provocarlo y la de resolverlo.
type casoDeRearme struct {
	clave string
	// prefijo: la clave que termina en `avisosDados` es `clave + algo variable` (el texto del
	// error). Se busca por prefijo y no por igualdad.
	prefijo bool
	// nuevo devuelve un par romper/arreglar CON SU PROPIO ESTADO, porque el ciclo se recorre dos
	// veces: romper, arreglar, romper. Un `romper` que no se puede repetir mediría media prueba
	// (que el mapa se limpió) y no la que importa (que el aviso vuelve a salir).
	nuevo func() parDeRearme
}

type parDeRearme struct {
	romper, arreglar func(t *testing.T, s *McpServer)
	// destino, si no es nil, es el receptor OTLP que este caso necesita en lugar del que contesta
	// 200 siempre. Lo devuelve `nuevo` porque el estado del receptor es parte del estado del caso.
	destino func(t *testing.T) string
}

// hayAvisoConClave dice si `avisosDados` tiene esa clave (o alguna que empiece con ella).
func hayAvisoConClave(s *McpServer, c casoDeRearme) bool {
	if !c.prefijo {
		_, dado := s.avisosDados.Load(c.clave)
		return dado
	}
	hay := false
	s.avisosDados.Range(func(k, _ any) bool {
		if clave, ok := k.(string); ok && strings.HasPrefix(clave, c.clave) {
			hay = true
			return false
		}
		return true
	})
	return hay
}

// casosDeRearmeDelEmpuje es LA LISTA DE CASOS, y que esté completa no lo sostiene la memoria de
// quien la lee: lo sostiene TestTodoAvisoDelEmpujeTieneCasoDeRearme contra el AST del código.
func casosDeRearmeDelEmpuje(ahora time.Time) []casoDeRearme {
	return []casoDeRearme{
		{
			clave: "empuje_truncado_servicios",
			nuevo: func() parDeRearme {
				preparado := false
				return parDeRearme{
					romper: func(t *testing.T, s *McpServer) {
						if !preparado {
							d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
							serviciosDePrueba(t, s, d, 5, ahora)
							preparado = true
						}
						s.techoServiciosPorProyecto = 2
					},
					arreglar: func(t *testing.T, s *McpServer) { s.techoServiciosPorProyecto = 0 },
				}
			},
		},
		{
			clave: "empuje_truncado_proyectos",
			nuevo: func() parDeRearme {
				preparado, n, ultimo := false, 0, ""
				return parDeRearme{
					romper: func(t *testing.T, s *McpServer) {
						if !preparado {
							// Justo en el techo: todavía NO trunca.
							for i := 0; i < proyectosParaExportar; i++ {
								maquinaConMuestra(t, s, fmt.Sprintf("tenant-%03d", i), "server", *muestraDePrueba(), ahora)
							}
							preparado = true
						}
						n++
						ultimo = fmt.Sprintf("tenant-extra-%02d", n)
						maquinaConMuestra(t, s, ultimo, "server", *muestraDePrueba(), ahora)
					},
					// Así se resuelve de verdad: se va un tenant y el barrido vuelve a entrar.
					arreglar: func(t *testing.T, s *McpServer) {
						if ok, err := s.engine.RevocarDevice(ultimo, "server"); err != nil || !ok {
							t.Fatalf("revocar %s/server: ok=%v err=%v", ultimo, ok, err)
						}
					},
				}
			},
		},
		{
			// EL SEXTO HERMANO, el que la tabla escrita a mano no tenía. No es un techo: se
			// provoca con un almacén que no se deja leer y se resuelve devolviéndolo a la
			// normalidad, sin tocar ninguna perilla.
			clave: "empuje_truncado_ilegible",
			nuevo: func() parDeRearme {
				var sano memory.StorageBackend
				return parDeRearme{
					romper: func(t *testing.T, s *McpServer) {
						if sano == nil {
							d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
							serviciosDePrueba(t, s, d, 4, ahora)
							sano = s.engine
						}
						s.engine = almacenQueNoSeDejaLeer{StorageBackend: sano, rompe: "servicios", proyecto: "casa"}
					},
					arreglar: func(t *testing.T, s *McpServer) { s.engine = sano },
				}
			},
		},
		{
			clave: "empuje_vacio",
			nuevo: func() parDeRearme {
				n, ultimo := 0, ""
				return parDeRearme{
					// Sin ninguna máquina visible, el sobre sale vacío y no se manda.
					romper: func(t *testing.T, s *McpServer) {
						if ultimo == "" {
							return
						}
						if ok, err := s.engine.RevocarDevice("casa", ultimo); err != nil || !ok {
							t.Fatalf("revocar casa/%s: ok=%v err=%v", ultimo, ok, err)
						}
						ultimo = ""
					},
					arreglar: func(t *testing.T, s *McpServer) {
						n++
						ultimo = fmt.Sprintf("pc-%02d", n)
						maquinaConMuestra(t, s, "casa", ultimo, *muestraDePrueba(), ahora)
					},
				}
			},
		},
		{
			clave: "empuje_sin_concesion",
			nuevo: func() parDeRearme {
				preparado := false
				return parDeRearme{
					romper: func(t *testing.T, s *McpServer) {
						if !preparado {
							maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
							preparado = true
						}
						sinCap := principalDePrometheus()
						sinCap.Fleet = map[fleet.Cap][]string{}
						s.buscarPrincipal = registroDePrueba(sinCap)
					},
					arreglar: func(t *testing.T, s *McpServer) {
						s.buscarPrincipal = registroDePrueba(principalDePrometheus())
					},
				}
			},
		},
		{
			clave: "empuje_sin_principal",
			nuevo: func() parDeRearme {
				preparado := false
				return parDeRearme{
					romper: func(t *testing.T, s *McpServer) {
						if !preparado {
							maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
							preparado = true
						}
						otro := principalDePrometheus()
						otro.Name = "otro-cualquiera"
						s.buscarPrincipal = registroDePrueba(otro)
					},
					arreglar: func(t *testing.T, s *McpServer) {
						s.buscarPrincipal = registroDePrueba(principalDePrometheus())
					},
				}
			},
		},
		{
			// EL SÉPTIMO, y el único que NO se rearma con `avisoMientras`: su clave lleva pegado
			// el texto del error, así que el rearme es `rearmarAvisos("empuje_permanente:")` y
			// vive DESPUÉS del envío exitoso. Otro mecanismo, y por eso hay que medirlo igual: es
			// el que se puede borrar sin tocar ninguna condición.
			clave:   "empuje_permanente:",
			prefijo: true,
			nuevo: func() parDeRearme {
				var estado atomic.Int64
				estado.Store(http.StatusOK)
				preparado := false
				return parDeRearme{
					destino: func(t *testing.T) string {
						ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							_, _ = io.Copy(io.Discard, r.Body)
							w.WriteHeader(int(estado.Load()))
						}))
						t.Cleanup(ts.Close)
						return ts.URL
					},
					romper: func(t *testing.T, s *McpServer) {
						if !preparado {
							maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
							preparado = true
						}
						// 404: Prometheus corriendo sin --web.enable-otlp-receiver. Permanente.
						estado.Store(http.StatusNotFound)
					},
					arreglar: func(t *testing.T, s *McpServer) { estado.Store(http.StatusOK) },
				}
			},
		},
	}
}

func TestLosAvisosDelEmpujeSeRearmanAlResolverse(t *testing.T) {
	ahora := time.Now()
	for _, c := range casosDeRearmeDelEmpuje(ahora) {
		t.Run(c.clave, func(t *testing.T) {
			p := c.nuevo()
			url := ""
			if p.destino != nil {
				url = p.destino(t)
			} else {
				url = nuevoReceptor(t, http.StatusOK).URL
			}
			s := prepararEmpuje(t, url, registroDePrueba(principalDePrometheus()), nil)

			p.romper(t, s)
			s.empujarUnaVez(context.Background(), ahora)
			if !hayAvisoConClave(s, c) {
				t.Fatalf("la condición de %q no dejó el aviso dado: la prueba no está midiendo lo que cree", c.clave)
			}

			p.arreglar(t, s)
			s.empujarUnaVez(context.Background(), ahora)
			if hayAvisoConClave(s, c) {
				t.Fatalf("%q siguió marcado como avisado después de resolverse: el aviso queda MUDO para siempre y el próximo episodio del mismo problema pasa en silencio", c.clave)
			}

			// Y EL SEGUNDO EPISODIO TIENE QUE HABLAR. Sin esto la prueba se conforma con que el
			// mapa esté limpio, que es la mitad interna del hecho; lo que importa es que la
			// próxima vez la línea de log salga.
			p.romper(t, s)
			var log bytes.Buffer
			restaurar := logx.Capturar(&log)
			s.empujarUnaVez(context.Background(), ahora)
			restaurar()
			if log.Len() == 0 {
				t.Errorf("el mismo problema volvió a ocurrir y el empuje no dijo nada: %q se rearmó en el mapa pero el aviso no volvió a salir", c.clave)
			}
		})
	}
}

// TestTodoAvisoDelEmpujeTieneCasoDeRearme ES LA GUARDA DE LA GUARDA.
//
// No prueba el empuje: prueba que la tabla de arriba cubra TODOS los avisos que el empuje emite.
// El conjunto de la derecha sale del AST del archivo, así que el aviso que se agregue mañana nace
// con su fila exigida y con su clave y su línea escritas en el mensaje del fallo.
func TestTodoAvisoDelEmpujeTieneCasoDeRearme(t *testing.T) {
	enElCodigo := clavesDeAvisoDelArchivo(t, "fleet_otlp.go")
	if len(enElCodigo) == 0 {
		t.Fatal("no se encontró NI UN aviso en fleet_otlp.go: o los avisos se mudaron de archivo —y entonces esta guarda dejó de mirar donde se decide— o el parser dejó de reconocer las llamadas. Las dos cosas son un agujero, y ninguna es un verde")
	}

	enLaTabla := map[string]bool{}
	for _, c := range casosDeRearmeDelEmpuje(time.Now()) {
		if enLaTabla[c.clave] {
			t.Errorf("`casosDeRearmeDelEmpuje` tiene %q dos veces", c.clave)
		}
		enLaTabla[c.clave] = true
	}

	for _, k := range enElCodigo {
		if !enLaTabla[k.clave] {
			t.Errorf("el empuje emite el aviso %q (%s) y NINGÚN caso de `casosDeRearmeDelEmpuje` lo lleva a su condición:\n"+
				"  nadie está midiendo que se rearme al resolverse, así que se puede volver a la forma sin rearme —`if cond { avisarUnaVez(…) }`— con la suite entera en verde.\n"+
				"  Agregale una fila con su `romper` y su `arreglar`; si la clave se arma concatenando, marcá `prefijo: true`.", k.clave, k.pos)
		}
		delete(enLaTabla, k.clave)
	}
	for k := range enLaTabla {
		t.Errorf("`casosDeRearmeDelEmpuje` tiene un caso para %q y fleet_otlp.go no emite ningún aviso con esa clave:\n"+
			"  o la clave se renombró —y entonces la fila está midiendo un aviso que no existe, o sea nada— o el aviso se borró y la fila sobra.", k)
	}
}

// claveDeAviso es un call site de `avisarUnaVez`/`avisoMientras` con su clave ya derivada.
type claveDeAviso struct {
	clave string
	pos   string
}

// clavesDeAvisoDelArchivo saca del AST todas las claves de aviso que emite un archivo del paquete.
//
// SE PARSEA, NO SE BUSCA UN TEXTO. Un grep por `avisoMientras("` le erra a la llamada que gofmt
// parte en dos líneas, a la que usa comillas invertidas y a la que concatena; y peor: un grep que
// no matchea devuelve «no hay avisos», que es verde. Acá lo que no se entiende es ROJO — si la
// clave de una llamada no se puede derivar, la prueba falla con el archivo y la línea.
func clavesDeAvisoDelArchivo(t *testing.T, archivo string) []claveDeAviso {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, archivo, nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear %s: %v", archivo, err)
	}
	vistas := map[string]bool{}
	var out []claveDeAviso
	ast.Inspect(f, func(n ast.Node) bool {
		llamada, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := llamada.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "avisarUnaVez" && sel.Sel.Name != "avisoMientras" {
			return true
		}
		if len(llamada.Args) == 0 {
			t.Errorf("%s: %s sin argumentos", fset.Position(llamada.Pos()), sel.Sel.Name)
			return true
		}
		clave, ok := literalIzquierdo(llamada.Args[0])
		if !ok {
			// ROJO EXPLÍCITO. Un aviso cuya clave no se puede leer del código es un aviso que
			// esta guarda no le puede exigir a la tabla; callarse acá sería exactamente la fuga
			// que la guarda existe para cerrar.
			t.Errorf("%s: no pude derivar la clave de este %s.\n"+
				"  Mientras no se pueda leer del código, NADIE está exigiendo que ese aviso se rearme al resolverse.\n"+
				"  Usá un literal —o un literal concatenado, que se lee como prefijo— de primer argumento.", fset.Position(llamada.Pos()), sel.Sel.Name)
			return true
		}
		if vistas[clave] {
			return true
		}
		vistas[clave] = true
		out = append(out, claveDeAviso{clave: clave, pos: fset.Position(llamada.Pos()).String()})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].clave < out[j].clave })
	return out
}

// literalIzquierdo devuelve el literal de string de una expresión, o el literal MÁS A LA IZQUIERDA
// de una concatenación (`"empuje_permanente:" + err.Error()` ⇒ `empuje_permanente:`), que es el
// prefijo con el que la clave entra a `avisosDados`.
func literalIzquierdo(e ast.Expr) (string, bool) {
	for {
		switch v := e.(type) {
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return "", false
			}
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				return "", false
			}
			return s, true
		case *ast.BinaryExpr:
			if v.Op != token.ADD {
				return "", false
			}
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		default:
			return "", false
		}
	}
}

// ── (5) EL `# HELP` Y EL COMENTARIO IMPRIMEN EL TECHO VIGENTE ───────────────────────────────
//
// Los dos volvieron a la constante default y quedó verde, incluida la rama que dice que el techo
// está DESACTIVADO. Un HELP que dice 2000 cuando la perilla vale 300 manda a quien lee la alerta
// a buscar 2000 servicios que no existen.
//
// LA ASERCIÓN NO BUSCA UN TEXTO: EXTRAE LOS NÚMEROS y exige que el conjunto sea exactamente el
// que corresponde. Con techo vigente: {proyectosParaExportar, techo}. Con el techo desactivado:
// {proyectosParaExportar} y NINGÚN otro — porque nombrar un número al lado de la palabra
// «desactivado» es peor que no nombrarlo. Una lista de textos prohibidos («que no diga 2000»)
// siempre le erra a la próxima forma; el conjunto de números no.
//
// Sabotaje que la pone roja: en describirTechoDeServicios, devolver la constante en cualquiera de
// las dos ramas.
func TestElHelpYElComentarioImprimenElTechoVigenteYNoLaConstante(t *testing.T) {
	// DOS techos, ninguno igual a ninguna constante del exportador. Comparar dos rendidos es lo
	// que hace que la prosa del HELP —que también tiene dígitos: «1 si el exportador…»— se
	// cancele sola, sin tener que enumerar qué números son de la prosa y cuáles del techo.
	const techoA, techoB = 317, 4321
	for _, n := range []int{techoA, techoB} {
		if n == serviciosPorProyectoDefault || n == proyectosParaExportar {
			t.Fatalf("el techo de prueba %d coincide con una constante del exportador y haría inútil la comparación", n)
		}
	}

	help := func(techo int) string {
		var b strings.Builder
		renderTruncado(&b, truncadoDeExport{Servicios: techo > 0}, techo)
		lineas := lineasDe(b.String(), "# HELP "+nombreExportTruncado)
		if len(lineas) != 1 {
			t.Fatalf("techo %d: se esperaba UN # HELP y salieron %d", techo, len(lineas))
		}
		return lineas[0]
	}

	numsA, numsB := numerosDe(help(techoA)), numerosDe(help(techoB))
	// (a) Cada rendido nombra SU techo y no el otro. Un HELP clavado en la constante no nombra
	//     ninguno de los dos y cae acá.
	if !tieneNumero(numsA, techoA) {
		t.Errorf("con el techo vigente en %d el # HELP no nombra ese número (nombra %v): quien lea la alerta va a buscar un corte en un número que no rige.\n%s", techoA, numsA, help(techoA))
	}
	if tieneNumero(numsA, techoB) {
		t.Errorf("el # HELP del techo %d nombra %d, que no rige:\n%s", techoA, techoB, help(techoA))
	}
	if !tieneNumero(numsB, techoB) {
		t.Errorf("con el techo vigente en %d el # HELP no nombra ese número (nombra %v):\n%s", techoB, numsB, help(techoB))
	}
	if tieneNumero(numsB, techoA) {
		t.Errorf("el # HELP del techo %d nombra %d, que no rige:\n%s", techoB, techoA, help(techoB))
	}

	// (b) LA RAMA DEL APAGADO, que es la que más fácil se pasa por alto y la que el sabotaje
	//     también tocó: sin techo, el HELP no puede introducir NINGÚN número propio. Lo que le
	//     queda permitido es exactamente lo que los dos rendidos de arriba tienen en común (la
	//     prosa y el techo de proyectos), y eso se calcula, no se enumera.
	comun := numerosComunes(numsA, numsB)
	for _, n := range numerosDe(help(0)) {
		if !tieneNumero(comun, n) {
			t.Errorf("con el techo DESACTIVADO el # HELP nombra %d, que no es ni la prosa ni el techo de proyectos. Decir un número al lado de «desactivado» manda a alguien a buscar un corte que no puede ocurrir:\n%s", n, help(0))
		}
	}

	// (c) EL COMENTARIO DEL BLOQUE DE SERVICIOS ES EL HERMANO: mismo número, otro archivo, y el
	//     sabotaje tocó los dos. Acá la línea no tiene prosa con dígitos, así que se puede exigir
	//     el conjunto exacto.
	for _, techo := range []int{2, 4} {
		s := newTestServer(t, embedding.NoopProvider{})
		ahora := time.Now()
		d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
		serviciosDePrueba(t, s, d, techo+1, ahora)
		var c strings.Builder
		if _, _ = renderServicios(&c, s.engine, []fleet.Device{d}, ahora, techo); true {
			com := lineasDe(c.String(), "# musubi_fleet_service:")
			if len(com) != 1 {
				t.Fatalf("techo %d: el comentario del recorte salió %d veces", techo, len(com))
			}
			if got := numerosDe(com[0]); !mismosNumeros(got, []int{techo}) {
				t.Errorf("con el techo en %d el comentario del bloque de servicios nombra %v:\n%s", techo, got, com[0])
			}
		}
	}
}

func tieneNumero(xs []int, n int) bool {
	for _, x := range xs {
		if x == n {
			return true
		}
	}
	return false
}

func numerosComunes(a, b []int) []int {
	var out []int
	for _, x := range a {
		if tieneNumero(b, x) {
			out = append(out, x)
		}
	}
	return out
}

var soloDigitos = regexp.MustCompile(`\d+`)

func numerosDe(linea string) []int {
	var out []int
	for _, m := range soloDigitos.FindAllString(linea, -1) {
		n, err := strconv.Atoi(m)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

func mismosNumeros(a, b []int) bool {
	a = append([]int(nil), a...)
	b = append([]int(nil), b...)
	sort.Ints(a)
	sort.Ints(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
