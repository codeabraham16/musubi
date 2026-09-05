package fleet

// exposicion_test.go ejercita el parseo del formato de exposición CONTRA EL ENDPOINT REAL.
//
// El fixture (`testdata/exposicion-supabase.txt`) es un recorte literal de la base gestionada que
// motivó este camino: la notación científica, el orden de las líneas, las etiquetas y las
// familias que FALTAN son las que manda el endpoint de verdad. Sólo se reemplazó la referencia
// del proyecto. Un fixture inventado habría tenido `node_boot_time_seconds` —porque uno lo
// escribe pensando en node_exporter completo— y la ausencia de esa métrica es justamente el caso
// que más importa acá.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// cuerpoMinimo es el endpoint más chico que Musubi acepta como host: el PAR de memoria. Menos
// que esto no alcanza para medir nada, y la compuerta de ParsearExposicion lo rechaza.
const cuerpoMinimo = "node_memory_MemTotal_bytes 1.926144e+09\nnode_memory_MemAvailable_bytes 1.2061696e+09\n"

func fixtureExposicion(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/exposicion-supabase.txt")
	if err != nil {
		t.Fatalf("falta el fixture del endpoint real: %v", err)
	}
	return string(b)
}

// EL DISCO QUE SE MIDE ES EL QUE SE PIDIÓ, Y ELEGIR MAL NO ROMPE NADA VISIBLE.
//
// El endpoint publica DOS sistemas de archivos: `/` es el sistema operativo (76 GB, al 12 %) y
// `/data` es el volumen de la base (8,4 GB, el que se llena y el que hay que vigilar). Los dos
// son números creíbles, así que quedarse con el equivocado produce un panel que se ve bien y una
// alerta que no salta nunca: el disco de la base llegaría al 100 % con la raíz mostrando 12 %.
//
// Sabotaje que la hace fallar: ignorar `montaje` y quedarse con la primera fila de cada familia.
func TestElMontajePedidoEsElQueSeMide(t *testing.T) {
	texto := fixtureExposicion(t)

	datos, ok := ParsearExposicion(texto, "/data")
	if !ok {
		t.Fatal("no se reconoció el endpoint real")
	}
	raiz, ok := ParsearExposicion(texto, "/")
	if !ok {
		t.Fatal("no se reconoció el endpoint real pidiendo la raíz")
	}

	mData := MuestraDesdeExposicion(datos, nil, time.Now())
	mRaiz := MuestraDesdeExposicion(raiz, nil, time.Now())

	if mData.DiscoTotal != 8416882688 {
		t.Errorf("el disco de /data quedó en %d, se esperaba 8416882688", mData.DiscoTotal)
	}
	if mRaiz.DiscoTotal != 76938919936 {
		t.Errorf("el disco de / quedó en %d, se esperaba 76938919936", mRaiz.DiscoTotal)
	}
	if mData.DiscoTotal == mRaiz.DiscoTotal {
		t.Fatal("los dos montajes dieron el mismo número: el punto de montaje no se está mirando")
	}

	// USADO SALE DE `free` Y NO DE `avail`. Es la regla de las tres columnas de df: la reserva
	// para root no es ni usado ni disponible. Con avail, el usado de /data saldría 422.735.872 en
	// vez de 301.101.056 — un 40 % más, plausible, y equivocado en todos los paneles a la vez.
	if mData.DiscoUsado != 301101056 {
		t.Errorf("el usado de /data quedó en %d, se esperaba 301101056 (total menos free)", mData.DiscoUsado)
	}
	if mData.DiscoDisponible != 7994146816 {
		t.Errorf("el disponible de /data quedó en %d, se esperaba 7994146816", mData.DiscoDisponible)
	}
}

// LA MEMORIA USADA SALE DE MemAvailable, Y LA LIBRE ES OTRA COSA.
//
// Es la misma asimetría que el colector de Linux ya documenta, y acá está a un typo de distancia:
// las dos métricas se llaman casi igual. Con MemFree, esta base daría 88 % de RAM usada estando
// sana, y el umbral de 95 % que hoy vigila el colector viejo empezaría a sonar sin motivo.
//
// Sabotaje que la hace fallar: calcular MemUsada como `MemTotal - MemFree`.
func TestLaMemoriaDelEndpointSaleDeAvailableYNoDeFree(t *testing.T) {
	l, ok := ParsearExposicion(fixtureExposicion(t), "/data")
	if !ok {
		t.Fatal("no se reconoció el endpoint real")
	}
	m := MuestraDesdeExposicion(l, nil, time.Now())

	if m.MemTotal != 1926144000 {
		t.Errorf("MemTotal quedó en %d", m.MemTotal)
	}
	if m.MemUsada != 719974400 { // 1926144000 - 1206169600
		t.Errorf("MemUsada quedó en %d, se esperaba 719974400 (total menos available)", m.MemUsada)
	}
	if m.MemLibre == nil || *m.MemLibre != 227942400 {
		t.Errorf("MemLibre quedó en %v, se esperaba 227942400 (MemFree, que NO es la contracara de usada)", m.MemLibre)
	}
	if m.SwapTotal != 1073737728 || m.SwapUsada != 125218816 {
		t.Errorf("el swap quedó en total=%d usada=%d", m.SwapTotal, m.SwapUsada)
	}
}

// LO QUE EL ENDPOINT NO PUBLICA SE QUEDA SIN MEDIR, Y NO SE RELLENA CON EL RELOJ DE ACÁ.
//
// El endpoint real NO trae `node_boot_time_seconds` ni `node_time_seconds`. La tentación es
// obvia y está mal: usar el reloj del cerebro como «ahora». Los relojes de dos máquinas difieren,
// y contra una nube gestionada la deriva puede ser de minutos — el uptime saldría con ese error
// encima, o negativo, y nadie lo notaría porque un uptime siempre parece plausible.
//
// Es la disciplina de todo el track un nivel más abajo: ausente no es cero, y tampoco es
// «estimalo con lo que tengas a mano».
//
// Sobre el sabotaje: el que primero se anotó acá —«calcular UptimeSeg con ahora.Unix()»— NO
// hacía fallar nada, porque el endpoint real no trae NINGUNA de las dos métricas y el resultado
// seguía siendo 0. Por eso el caso del reloj está abajo, con un texto armado que sí trae la mitad:
// es la única forma en que el atajo produce un número, y ahí se ve que produce uno absurdo.
//
// Sabotaje que la hace fallar: escribir un valor por defecto cuando la familia no vino.
func TestLoQueElEndpointNoPublicaNoSeInventa(t *testing.T) {
	texto := fixtureExposicion(t)
	if strings.Contains(texto, "node_boot_time_seconds") {
		t.Fatal("el fixture ganó boot_time: esta prueba ya no ejercita la ausencia que vino a cuidar")
	}
	l, _ := ParsearExposicion(texto, "/data")
	m := MuestraDesdeExposicion(l, nil, time.Now())

	if m.UptimeSeg != 0 {
		t.Errorf("uptime %d sobre un endpoint que no dice cuándo arrancó", m.UptimeSeg)
	}
	if m.TempC != nil {
		t.Errorf("temperatura %v sobre un endpoint que no publica sensores", *m.TempC)
	}
	if m.NumProcesos != 0 {
		t.Errorf("num_procesos %d sobre un endpoint que no los cuenta", m.NumProcesos)
	}
	// Y lo que SÍ publica, sí está: si esto fallara, la prueba de arriba pasaría por estar todo
	// vacío en vez de por la razón que dice.
	if m.Load1 == nil || *m.Load1 != 0.01 {
		t.Errorf("load1 quedó en %v: el endpoint sí la publica", m.Load1)
	}
	if m.NumCPU != 2 {
		t.Errorf("NumCPU quedó en %d, el endpoint declara 2 núcleos", m.NumCPU)
	}
}

// CON LA MITAD DEL PAR DEL UPTIME TAMPOCO ALCANZA, Y EL RELOJ DE ACÁ NO COMPLETA LA OTRA MITAD.
//
// Éste es el caso que el fixture real no puede ejercitar —no trae ninguna de las dos— y es donde
// el atajo produce un número en vez de un cero. Un endpoint que publica `node_boot_time_seconds`
// y no `node_time_seconds` existe, y ahí la tentación de restar contra el reloj local es máxima.
// No sirve: los relojes de dos máquinas difieren, contra una nube gestionada por minutos, y ese
// error se suma entero al uptime. Peor todavía si el remoto va adelantado: el uptime sale
// negativo y, en un uint64, gigantesco.
//
// Sabotaje que la hace fallar: cambiar la guarda por `if hayA` y restar contra ahora.Unix().
func TestConLaMitadDelParDelUptimeNoSeUsaElRelojDeAca(t *testing.T) {
	mitad := cuerpoMinimo + "node_boot_time_seconds 1.7e+09\n"
	l, ok := ParsearExposicion(mitad, "/")
	if !ok {
		t.Fatal("no se reconoció el texto")
	}
	if _, hay := l.Num(ExpArranque); !hay {
		t.Fatal("el arranque no se leyó: esta prueba no ejercita lo que dice")
	}
	if _, hay := l.Num(ExpAhora); hay {
		t.Fatal("el texto trae `ahora`: entonces no es el caso de la mitad del par")
	}
	m := MuestraDesdeExposicion(l, nil, time.Now())
	if m.UptimeSeg != 0 {
		t.Errorf("uptime %d: se completó la mitad que falta con el reloj de acá", m.UptimeSeg)
	}
}

// UN ENDPOINT SIN VITALES DE HOST SE RECHAZA ENTERO, NO SE ACEPTA EN CEROS.
//
// Un formato de exposición perfecto que sólo publica métricas de aplicación es un caso real: casi
// cualquier servicio expone `/metrics`. Aceptarlo daría una Muestra de ceros, y el panel dibujaría
// una máquina con 0 de RAM y 0 de disco — que se lee como «está rota», no como «esto no era un
// host». Es el mismo rechazo que hace la lectura por SSH cuando del otro lado no hay /proc.
//
// Sabotaje que la hace fallar: devolver ok=true siempre.
func TestUnEndpointDeAplicacionNoPasaPorUnHost(t *testing.T) {
	soloApp := `# HELP http_requests_total Cuántas
# TYPE http_requests_total counter
http_requests_total{code="200"} 1234
pg_database_size_bytes{datname="postgres"} 8.3e+06
`
	if _, ok := ParsearExposicion(soloApp, "/"); ok {
		t.Fatal("un endpoint sin vitales de host pasó por un host: la Muestra saldría en ceros")
	}

	// Y al revés: el fixture real TRAE pg_database_size_bytes y no se confunde con nada.
	l, ok := ParsearExposicion(fixtureExposicion(t), "/data")
	if !ok {
		t.Fatal("el endpoint real no pasó")
	}
	if v, hay := l.Num(ExpMemTotal); !hay || v != 1926144000 {
		t.Errorf("las métricas de aplicación ensuciaron las del host: mem_total=%v hay=%v", v, hay)
	}
}

// EL PARSEO DE UNA LÍNEA TIENE TRES TRAMPAS Y LAS TRES DAN NÚMEROS CREÍBLES.
//
// Ninguna produce un error: producen el valor equivocado, que es peor. El timestamp opcional del
// formato va DESPUÉS del valor, así que quedarse con el último campo mete una marca de tiempo en
// milisegundos donde va la memoria. Una `}` adentro de una etiqueta —`device_error=""` es un
// campo real de este endpoint y podría traerla— parte la línea en el lugar equivocado. Y buscar
// una etiqueta por prefijo confunde `mode` con cualquier otra que empiece igual.
//
// Sabotaje que la hace fallar: tomar el último campo como valor; cortar en el primer `}`; comparar
// la etiqueta con strings.HasPrefix.
func TestLasTresTrampasDeUnaLineaDeExposicion(t *testing.T) {
	casos := []struct {
		nombre string
		linea  string
		valor  string
		etiq   string
		busca  string
		quiero string
	}{
		{
			nombre: "el timestamp no es el valor",
			linea:  `node_memory_MemTotal_bytes{a="b"} 1.926144e+09 1756400000000`,
			valor:  "1.926144e+09",
			busca:  "a", quiero: "b",
		},
		{
			nombre: "una llave adentro de una etiqueta no cierra el bloque",
			linea:  `node_load1{device_error="}",cpu="0"} 0.01`,
			valor:  "0.01",
			busca:  "cpu", quiero: "0",
		},
		{
			nombre: "una comilla escapada no cierra la cadena",
			linea:  `node_load1{detalle="dice \"nada\"",mode="idle"} 0.02`,
			valor:  "0.02",
			busca:  "mode", quiero: "idle",
		},
		{
			nombre: "la etiqueta se compara entera y no por prefijo",
			linea:  `node_cpu_seconds_total{mode_extendido="user",mode="idle"} 5`,
			valor:  "5",
			busca:  "mode", quiero: "idle",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, etiquetas, valor, ok := partirLineaExposicion(c.linea)
			if !ok {
				t.Fatalf("no se pudo partir: %s", c.linea)
			}
			if valor != c.valor {
				t.Errorf("valor %q, se esperaba %q", valor, c.valor)
			}
			if got := valorDeEtiqueta(etiquetas, c.busca); got != c.quiero {
				t.Errorf("la etiqueta %s dio %q, se esperaba %q (bloque: %s)", c.busca, got, c.quiero, etiquetas)
			}
		})
	}
}

// EL PORCENTAJE DE CPU ES UNA DERIVADA, Y LA PRIMERA LECTURA NO LA TIENE.
//
// El endpoint publica 16 contadores acumulados (2 núcleos × 8 modos). Se suman todos para el
// total y sólo los de `mode="idle"` para el ocioso: es la misma forma que la primera línea de
// /proc/stat, y por eso reusa la aritmética de cpudelta.go en vez de tener una segunda copia.
//
// Sabotaje que la hace fallar: devolver 0 en la primera lectura en vez de nil; o sumar los
// contadores de idle adentro del total dos veces.
func TestElPorcentajeDeCPUNecesitaDosLecturas(t *testing.T) {
	texto := fixtureExposicion(t)
	l, _ := ParsearExposicion(texto, "/data")

	var cpu ContadorCPUExportado
	primera := MuestraDesdeExposicion(l, &cpu, time.Now())
	if primera.CPUPct != nil {
		t.Fatalf("la primera lectura trajo %v%%: no hay contra qué restar", *primera.CPUPct)
	}

	// Segunda lectura: el mismo texto con el idle avanzado 10 s sobre un total que avanza 20.
	// El sistema estuvo ocupado la mitad del intervalo.
	idle, _ := l.Num(ExpCPUIdle)
	total, _ := l.Num(ExpCPUTotal)
	l2 := LecturaExposicion{vals: map[string]float64{
		ExpMemTotal: 1, ExpCPUIdle: idle + 10, ExpCPUTotal: total + 20,
	}, cpus: map[string]bool{"0": true, "1": true}}

	segunda := MuestraDesdeExposicion(l2, &cpu, time.Now())
	if segunda.CPUPct == nil {
		t.Fatal("la segunda lectura no dio porcentaje")
	}
	if *segunda.CPUPct < 49.9 || *segunda.CPUPct > 50.1 {
		t.Errorf("el porcentaje dio %v, se esperaba 50", *segunda.CPUPct)
	}
}

// ── El viaje ────────────────────────────────────────────────────────────────────────────────

// LA CREDENCIAL VIAJA EN EL HEADER Y NO APARECE EN NINGÚN ERROR.
//
// Las dos mitades van juntas porque la segunda es la que nadie prueba. El error de net/http
// LLEVA LA URL ENTERA adentro, y esa URL puede traer un token en la query; ese texto termina en
// la respuesta de una tool, o sea en un panel, o sea en el portapapeles de alguien. Un secreto
// que se filtra por el camino del error es un secreto filtrado igual.
//
// Sabotaje que la hace fallar: devolver `err.Error()` crudo en vez de pasar por motivoDeRed.
func TestLaCredencialViajaEnElHeaderYNoSeFiltraPorElError(t *testing.T) {
	var recibido string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recibido = r.Header.Get("Authorization")
		w.Write([]byte(cuerpoMinimo))
	}))
	defer s.Close()

	const secreto = "Bearer sbp-el-token-que-no-tiene-que-salir"
	if _, err := TomarMuestraDeExposicion(DestinoExposicion{
		URL: s.URL + "/metrics?token=" + strings.TrimPrefix(secreto, "Bearer "), Autorizacion: secreto,
	}, nil, 5*time.Second); err != nil {
		t.Fatalf("el raspado falló: %v", err)
	}
	if recibido != secreto {
		t.Errorf("el endpoint recibió %q en vez de la credencial", recibido)
	}

	// Y ahora el mismo destino, muerto: el error no puede traer ni el token ni la query.
	s.Close()
	_, err := TomarMuestraDeExposicion(DestinoExposicion{
		URL: s.URL + "/metrics?token=" + strings.TrimPrefix(secreto, "Bearer "), Autorizacion: secreto,
	}, nil, 2*time.Second)
	if err == nil {
		t.Fatal("un endpoint caído no dio error")
	}
	if strings.Contains(err.Error(), "el-token-que-no-tiene-que-salir") {
		t.Errorf("el error filtró la credencial: %v", err)
	}
	if strings.Contains(err.Error(), "token=") {
		t.Errorf("el error filtró la query de la URL: %v", err)
	}
}

// UNA REDIRECCIÓN NO SE SIGUE, Y NO ES POR COMODIDAD.
//
// Un 302 hacia otro host haría que el cliente repita el pedido allá. Go quita los headers
// sensibles al cambiar de dominio, pero «quita los que considera sensibles» no es una garantía
// que uno quiera tener sobre una credencial. Y hacia una dirección interna, esto se convierte en
// un SSRF con nuestras propias credenciales: el cerebro raspando el metadata service de la nube
// porque un endpoint ajeno se lo pidió.
//
// Sabotaje que la hace fallar: sacar el CheckRedirect del cliente.
func TestUnaRedireccionNoSeSigue(t *testing.T) {
	var golpeado bool
	destino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		golpeado = true
		w.Write([]byte(cuerpoMinimo))
	}))
	defer destino.Close()
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destino.URL, http.StatusFound)
	}))
	defer origen.Close()

	_, err := TomarMuestraDeExposicion(DestinoExposicion{URL: origen.URL, Autorizacion: "Bearer x"}, nil, 5*time.Second)
	if err == nil {
		t.Fatal("la redirección se siguió en silencio")
	}
	if golpeado {
		t.Error("el pedido llegó al segundo host: la credencial pudo haber viajado con él")
	}
}

// UN CUERPO QUE PASA EL TECHO SE RECHAZA ENTERO, NO SE PARSEA A MEDIAS.
//
// Es la trampa del LimitReader: leer exactamente el techo y parsear lo que entró devuelve una
// Muestra válida armada con un texto TRUNCADO. Las familias que quedaron afuera se leerían como
// «este host no las expone» — una mentira con forma de dato bueno, y sin un error en ningún lado.
// Por eso se lee un byte de más: es la única forma de distinguir «entró justo» de «se cortó».
//
// Sabotaje que la hace fallar: limitar a ExposicionMax exacto y seguir con lo que entró.
func TestUnCuerpoDemasiadoGrandeNoSeParseaAMedias(t *testing.T) {
	const linea = "# ruido para llenar el techo\n"
	relleno := strings.Repeat(linea, (ExposicionMax/len(linea))+100)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(cuerpoMinimo))
		w.Write([]byte(relleno))
	}))
	defer s.Close()

	_, err := TomarMuestraDeExposicion(DestinoExposicion{URL: s.URL}, nil, 20*time.Second)
	if err == nil {
		t.Fatal("un cuerpo por encima del techo devolvió una Muestra: está armada con texto truncado")
	}
	if !strings.Contains(err.Error(), "a medias") {
		t.Errorf("el error no dice que se truncó: %v", err)
	}
}

// UNA CREDENCIAL RECHAZADA MANDA A MIRAR LA CREDENCIAL, NO LA RED.
//
// Un 401 y un `connection refused` son problemas distintos con arreglos distintos, y un error que
// los junta manda a alguien a revisar el firewall durante una hora cuando lo que venció fue un
// token. Es la misma regla que separa «no está instalado» de «está y falló».
//
// Sabotaje que la hace fallar: colapsar todos los estados en «contestó HTTP %d».
func TestUn401MandaAMirarLaCredencialYNoLaRed(t *testing.T) {
	for _, codigo := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "no", codigo)
		}))
		_, err := TomarMuestraDeExposicion(DestinoExposicion{URL: s.URL, Autorizacion: "Bearer viejo"}, nil, 5*time.Second)
		s.Close()
		if err == nil {
			t.Fatalf("HTTP %d no dio error", codigo)
		}
		if !strings.Contains(err.Error(), "credencial") {
			t.Errorf("HTTP %d no manda a mirar la credencial: %v", codigo, err)
		}
	}
}

// LA COMPUERTA DEL PARSER Y LA REGLA DE LOS PARES DEL DOMINIO TIENEN QUE DECIR LO MISMO.
//
// Dos guardas sobre la misma cosa que no se enteran una de la otra discuten en el mensaje de
// error, y el que lee el mensaje va a mirar donde no es. La primera versión de la compuerta pedía
// nada más `MemTotal`: un endpoint con el total y sin el disponible pasaba, armaba una Muestra, y
// Valida la rechazaba con «la muestra no es creíble» — que suena a dato corrupto cuando lo que
// pasa es que ese endpoint no alcanza para medir un host.
//
// Sabotaje que la hace fallar: pedir sólo ExpMemTotal en la compuerta.
func TestLaCompuertaYLaReglaDeLosParesNoSeContradicen(t *testing.T) {
	casos := []struct {
		nombre string
		texto  string
		pasa   bool
	}{
		{"el par entero", cuerpoMinimo, true},
		{"total sin disponible", "node_memory_MemTotal_bytes 1.926144e+09\n", false},
		{"disponible sin total", "node_memory_MemAvailable_bytes 1.2061696e+09\n", false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			l, ok := ParsearExposicion(c.texto, "/")
			if ok != c.pasa {
				t.Fatalf("la compuerta dio %v, se esperaba %v", ok, c.pasa)
			}
			if !ok {
				return
			}
			// Y lo que la compuerta deja pasar, Valida lo acepta. Si esto falla, las dos guardas
			// volvieron a discutir.
			if err := MuestraDesdeExposicion(l, nil, time.Now()).Valida(); err != nil {
				t.Errorf("pasó la compuerta y Valida la rechazó: %v", err)
			}
		})
	}

	// El disco tiene la misma regla, y su mitad ausente NO puede tumbar la muestra entera: un
	// endpoint que publica memoria y no filesystem es medible, con el disco en «no medido».
	sinDisco := cuerpoMinimo + `node_filesystem_size_bytes{mountpoint="/"} 1e+09` + "\n"
	l, ok := ParsearExposicion(sinDisco, "/")
	if !ok {
		t.Fatal("un endpoint con memoria y sin el par del disco se rechazó entero")
	}
	m := MuestraDesdeExposicion(l, nil, time.Now())
	if err := m.Valida(); err != nil {
		t.Errorf("un disco a medias produjo una muestra inválida: %v", err)
	}
	if m.DiscoTotal != 0 {
		t.Errorf("se escribió disco_total=%d sin poder calcular el usado: se dibuja como disco vacío", m.DiscoTotal)
	}
}

// UN SWAP TOTAL SIN SU LIBRE NO SE MIDE, IGUAL QUE LA MEMORIA Y EL DISCO.
//
// El bloque del disco tiene la regla escrita —«un disco total con el usado en cero se dibuja como
// un disco vacío»— y el swap era el único de los tres que la rompía: asignaba `SwapTotal` AFUERA
// del `if` que exige el libre. Un endpoint que publica `node_memory_SwapTotal_bytes` y no publica
// `SwapFree` dejaba el total puesto y el usado en cero, y como el exportador compuerta las dos
// series con `SwapTotal > 0`, salía `swap_used_bytes 0`.
//
// Ese 0 no dice «no medido»: AFIRMA «esta máquina tiene swap y no usa nada», que es la clase de
// cero que este dominio persigue desde S4 —«lo desconocido viaja como NULL, nunca como 0»— y es
// además el que más engaña, porque un swap en cero se lee como una máquina holgada.
//
// LA TABLA PRUEBA LOS TRES PARES JUNTOS y no sólo el que fallaba: si la regla vale para la memoria
// y para el disco, una prueba que mire sólo el swap deja la forma del defecto intacta para el
// cuarto par que alguien agregue.
//
// Sabotaje que la hace fallar: sacar la asignación del total de adentro del `if` del libre, en
// cualquiera de los tres pares.
func TestUnTotalSinSuLibreNoSeMide(t *testing.T) {
	for _, c := range []struct {
		par       string
		soloTotal string
		conElPar  string
		leerTotal func(Muestra) uint64
		leerUsado func(Muestra) uint64
	}{
		{
			par:       "swap",
			soloTotal: "node_memory_SwapTotal_bytes 2.147483648e+09\n",
			conElPar:  "node_memory_SwapTotal_bytes 2.147483648e+09\nnode_memory_SwapFree_bytes 1.073741824e+09\n",
			leerTotal: func(m Muestra) uint64 { return m.SwapTotal },
			leerUsado: func(m Muestra) uint64 { return m.SwapUsada },
		},
		{
			par:       "disco",
			soloTotal: "node_filesystem_size_bytes{mountpoint=\"/\"} 5.0e+11\n",
			conElPar:  "node_filesystem_size_bytes{mountpoint=\"/\"} 5.0e+11\nnode_filesystem_free_bytes{mountpoint=\"/\"} 2.0e+11\n",
			leerTotal: func(m Muestra) uint64 { return m.DiscoTotal },
			leerUsado: func(m Muestra) uint64 { return m.DiscoUsado },
		},
	} {
		t.Run(c.par, func(t *testing.T) {
			// CONTROL POSITIVO PRIMERO: con el par completo los dos campos se llenan. Sin esto, un
			// parser que dejara de reconocer estas métricas daría cero en los dos casos y la
			// aserción de abajo pasaría por la razón equivocada.
			l, ok := ParsearExposicion(cuerpoMinimo+c.conElPar, "/")
			if !ok {
				t.Fatalf("%s: el cuerpo con el par completo no parseó", c.par)
			}
			m := MuestraDesdeExposicion(l, nil, time.Now())
			if c.leerTotal(m) == 0 || c.leerUsado(m) == 0 {
				t.Fatalf("%s: con el par completo total=%d usado=%d — esta prueba no está midiendo "+
					"el parseo que cree", c.par, c.leerTotal(m), c.leerUsado(m))
			}

			// Y AHORA EL CASO: el total solo no alcanza. Los DOS quedan en cero, o sea sin serie.
			l, ok = ParsearExposicion(cuerpoMinimo+c.soloTotal, "/")
			if !ok {
				t.Fatalf("%s: el cuerpo con sólo el total no parseó", c.par)
			}
			m = MuestraDesdeExposicion(l, nil, time.Now())
			if got := c.leerTotal(m); got != 0 {
				t.Errorf("%s: sin el libre se guardó un total de %d. El exportador compuerta la "+
					"serie con total>0, así que va a emitir un usado de 0 — que AFIRMA «no usa "+
					"nada» sobre algo que nadie midió", c.par, got)
			}
			if got := c.leerUsado(m); got != 0 {
				t.Errorf("%s: se inventó un usado de %d sin tener el libre", c.par, got)
			}
		})
	}
}
