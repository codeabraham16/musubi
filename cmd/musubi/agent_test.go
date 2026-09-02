package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// B4/D5 — la CREDENCIAL viaja en el header y el cuerpo es chico y sin identidad.
//
// El cuerpo dejó de ser vacío cuando el agente empezó a autorreportar su versión y su dirección
// (qué build corre y por dónde se la alcanza). Eso NO afloja el invariante: lo que el device no
// puede decir es QUIÉN ES —eso sale del token— y sigue sin haber dónde ponerlo. Lo fija
// TestElCuerpoNoLlevaIdentidadNunca.
func TestElLatidoLlevaElTokenEnElHeaderYUnCuerpoChico(t *testing.T) {
	var vistoAuth string
	var largoCuerpo int64 = -1
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vistoAuth = r.Header.Get("Authorization")
		largoCuerpo = r.ContentLength
		if r.Method != http.MethodPost {
			t.Errorf("el agente usó %s, esperaba POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// El inventario se anula a propósito: esta prueba mide el AUTORREPORTE, y dejarla mirando lo
	// que corra en la máquina que ejecuta la suite la haría pasar o fallar según el host. Que el
	// inventario no viaje en cada latido lo custodia TestElInventarioNoViajaEnCadaLatido.
	anteriorEnum := enumerarServicios
	enumerarServicios = func() ([]fleet.ReporteServicio, error) { return nil, nil }
	t.Cleanup(func() { enumerarServicios = anteriorEnum })

	res := latir(ts.URL+"/fleet/heartbeat", "tok-abc", nil)
	if !res.ok {
		t.Fatalf("el latido falló: %+v", res)
	}
	if vistoAuth != "Bearer tok-abc" {
		t.Errorf("Authorization = %q, esperaba el bearer del dispositivo", vistoAuth)
	}
	// Sin muestra Y SIN INVENTARIO NUEVO, el cuerpo es sólo el autorreporte: decenas de bytes.
	//
	// La condición «sin inventario nuevo» es de S12 y hace la guarda MÁS estricta, no menos. El
	// primer latido lleva el inventario a propósito —si no, la máquina nunca reportaría lo que
	// corre—, así que lo que se custodia es el ESTADO ESTABLE: el segundo latido, con el mismo
	// inventario, tiene que volver a ser chico. Sin eso, colgar el inventario de cada latido
	// manda 7 KB cada diez segundos por máquina, que fue exactamente lo que esta prueba atajó.
	if largoCuerpo > 512 {
		t.Errorf("el latido SIN muestra mandó %d bytes: debería ser sólo el autorreporte", largoCuerpo)
	}
}

// TestElInventarioNoViajaEnCadaLatido — la contraparte de la guarda de arriba.
//
// El inventario se manda cuando CAMBIÓ, y además cada `intervaloInventarioCompleto` para que
// `last_report` no envejezca. Entre medio, el latido no lo lleva. Que no viaje NO borra nada: la
// poda del cerebro sólo corre cuando llega una lista.
//
// Sabotaje que la hace fallar: devolver siempre `lista` en serviciosDelLatido, sin mirar la huella.
func TestElInventarioNoViajaEnCadaLatido(t *testing.T) {
	anterior := enumerarServicios
	enumerarServicios = func() ([]fleet.ReporteServicio, error) {
		return []fleet.ReporteServicio{{
			Nombre: "postgres", Clase: "systemd",
			Salud: fleet.SaludServicio{Tomada: time.Now(), Estado: fleet.EstadoCorriendo},
		}}, nil
	}
	ultimoInventario.Lock()
	ultimoInventario.huella, ultimoInventario.enviado = "", time.Time{}
	ultimoInventario.Unlock()
	t.Cleanup(func() {
		enumerarServicios = anterior
		ultimoInventario.Lock()
		ultimoInventario.huella, ultimoInventario.enviado = "", time.Time{}
		ultimoInventario.Unlock()
	})

	if primero := serviciosDelLatido(); len(primero) != 1 {
		t.Fatalf("el PRIMER latido no llevó el inventario (%d servicios): la máquina nunca reportaría lo que corre", len(primero))
	}
	if segundo := serviciosDelLatido(); segundo != nil {
		t.Errorf("el segundo latido volvió a mandar el inventario sin que cambiara nada (%d servicios): son 7 KB cada diez segundos por máquina", len(segundo))
	}

	// Y cuando SÍ cambia, viaja de nuevo — o un servicio que se cae tardaría 5 minutos en verse.
	enumerarServicios = func() ([]fleet.ReporteServicio, error) {
		return []fleet.ReporteServicio{{
			Nombre: "postgres", Clase: "systemd",
			Salud: fleet.SaludServicio{Tomada: time.Now(), Estado: fleet.EstadoFallado},
		}}, nil
	}
	if cambiado := serviciosDelLatido(); len(cambiado) != 1 {
		t.Error("el inventario cambió de estado y NO viajó: un servicio caído tardaría hasta 5 minutos en verse")
	}
}

// B5 — un 401 se clasifica como REVOCADO (no se reintenta), no como fallo transitorio.
// Sabotaje: tratar el 401 como un error más → el agente golpea el lockout del cerebro para
// siempre en vez de detenerse.
func TestUn401SeClasificaComoRevocadoYNoComoFalloTransitorio(t *testing.T) {
	casos := []struct {
		status         int
		quieroOK       bool
		quieroRevocado bool
	}{
		{http.StatusOK, true, false},
		{http.StatusUnauthorized, false, true},
		{http.StatusServiceUnavailable, false, false},  // cerebro caído: reintentar
		{http.StatusInternalServerError, false, false}, // ídem
		{http.StatusTooManyRequests, false, false},     // lockout: reintentar con backoff
	}
	for _, c := range casos {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
		}))
		res := latir(ts.URL, "tok", nil)
		ts.Close()
		if res.ok != c.quieroOK || res.revocado != c.quieroRevocado {
			t.Errorf("status %d: ok=%v revocado=%v, esperaba ok=%v revocado=%v",
				c.status, res.ok, res.revocado, c.quieroOK, c.quieroRevocado)
		}
	}
}

// B7 — el cerebro inalcanzable NO mata al agente: se clasifica como reintentable.
// Sabotaje: devolver revocado=true ante un error de red → una caída de red da de baja
// permanentemente a toda la flota hasta que alguien la levante a mano, máquina por máquina.
func TestElCerebroInalcanzableEsReintentableNoRevocado(t *testing.T) {
	// Un servidor que ya cerró: la conexión se rechaza.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL
	ts.Close()

	res := latir(url, "tok", nil)
	if res.ok {
		t.Fatal("un cerebro caído no debería dar un latido exitoso")
	}
	if res.revocado {
		t.Fatal("un error de RED se clasificó como revocado: una caída de red daría de baja a la flota entera")
	}
	if res.motivo == "" {
		t.Error("el fallo no explica por qué")
	}
}

// B7 — un cerebro que acepta la conexión y no responde NUNCA no puede colgar el bucle.
// Sabotaje: quitarle el Timeout a clienteLatido → el agente queda esperando para siempre y la
// máquina figura viva sin volver a latir jamás.
func TestElClienteDelLatidoTieneTimeout(t *testing.T) {
	if clienteLatido.Timeout <= 0 {
		t.Fatal("clienteLatido no tiene timeout: un cerebro que no responde cuelga el bucle para siempre")
	}
	if clienteLatido.Timeout >= intervaloLatidoDefault {
		t.Errorf("el timeout (%s) no es menor que el intervalo (%s): los latidos se apilarían",
			clienteLatido.Timeout, intervaloLatidoDefault)
	}
}

// El backoff crece pero no sin fin: tras una noche de cerebro caído, la máquina tiene que
// reaparecer en minutos, no en horas.
func TestElBackoffTieneTecho(t *testing.T) {
	espera := esperaMinima
	for i := 0; i < 100; i++ {
		if espera *= 2; espera > esperaMaxima {
			espera = esperaMaxima
		}
	}
	if espera != esperaMaxima {
		t.Errorf("tras 100 fallos la espera es %s, esperaba el techo %s", espera, esperaMaxima)
	}
	if esperaMaxima > 10*time.Minute {
		t.Errorf("el techo del backoff es %s: demasiado para que una máquina reaparezca tras una caída", esperaMaxima)
	}
}

// El intervalo del agente y el umbral del cerebro están atados por el factor 3. Si alguien mueve
// uno sin mirar el otro, esta prueba lo dice — el número vive en internal/mcp y no se puede
// importar desde acá, así que se replica con su justificación a la vista.
func TestElIntervaloDelAgenteEsUnTercioDelUmbralDelCerebro(t *testing.T) {
	const umbralDelCerebro = 90 * time.Second // internal/mcp/methods_fleet.go · umbralEnLineaDefault
	if umbralDelCerebro/intervaloLatidoDefault != 3 {
		t.Errorf("el umbral del cerebro (%s) dejó de ser 3× el intervalo del agente (%s): "+
			"con menos margen, un hipo de red pinta la flota de rojo",
			umbralDelCerebro, intervaloLatidoDefault)
	}
}

// El bucle se detiene solo cuando lo revocan, sin que nadie lo mate.
func TestElBucleSeDetieneAlSerRevocado(t *testing.T) {
	var latidos atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if latidos.Add(1) >= 2 {
			w.WriteHeader(http.StatusUnauthorized) // el admin revocó entre latidos
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	listo := make(chan struct{})
	go func() {
		bucleDeLatidos(ts.URL, "tok", 10*time.Millisecond, nil)
		close(listo)
	}()

	select {
	case <-listo:
		if n := latidos.Load(); n != 2 {
			t.Errorf("latió %d veces, esperaba detenerse en el segundo (el revocado)", n)
		}
	// 30s Y NO 5: NO SE AFLOJA EL INVARIANTE, SE AFLOJA LA RED. Lo que esta prueba ata es
	// que el bucle SE DETENGA ante un 401; el reloj es sólo el guardia contra un cuelgue, y
	// un kill-switch roto no vuelve NUNCA, así que sigue fallando igual a los 30s.
	//
	// Medido el 2026-09-02 en una PC Windows real: `latir()` llama a `serviciosDelLatido()`
	// en CADA latido y, con 79 servicios, cada uno cuesta ~2,3s. Dos latidos = 4,6s contra un
	// presupuesto de 5, así que la prueba PASA cuando corre sola y se cae en 5,00s bajo la
	// carga del paquete. Era un flake de reloj, no un fallo del kill-switch.
	case <-time.After(30 * time.Second):
		t.Fatal("el agente siguió latiendo tras ser revocado: el kill-switch no se entiende desde la máquina")
	}
}

// La ayuda dice explícitamente que el token del dispositivo no abre la memoria. Es la propiedad
// de seguridad del track y quien instala el agente tiene que poder leerla sin abrir el código.
func TestLaAyudaDiceQueElTokenNoAbreLaMemoria(t *testing.T) {
	salida := capturarSalida(t, ayudaAgent)
	if !strings.Contains(salida, "/mcp") || !strings.Contains(strings.ToLower(salida), "memoria") {
		t.Errorf("la ayuda no aclara que el token del dispositivo no da acceso a la memoria:\n%s", salida)
	}
}

// capturarSalida corre f con stdout redirigido y devuelve lo que imprimió.
func capturarSalida(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out)
}

// D5 — el cuerpo del latido con telemetría lleva SÓLO la muestra. Ni hostname, ni device_id,
// ni proyecto: la identidad la decide el token del lado del cerebro.
//
// Sabotaje que la hace fallar: agregar cualquier campo de identidad al JSON que arma latir().
func TestElCuerpoNoLlevaIdentidadNunca(t *testing.T) {
	var visto string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		visto = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cpu := 12.5
	m := &fleet.Muestra{Tomada: time.Now().UTC(), CPUPct: &cpu, NumCPU: 4, MemTotal: 100, MemUsada: 10}
	if res := latir(ts.URL, "tok", m); !res.ok {
		t.Fatalf("el latido con muestra falló: %+v", res)
	}

	var cuerpo map[string]any
	if err := json.Unmarshal([]byte(visto), &cuerpo); err != nil {
		t.Fatalf("el cuerpo no es JSON: %v (%s)", err, visto)
	}
	// LISTA BLANCA, no conteo. El cuerpo puede crecer —así creció con el autorreporte— pero
	// cada clave nueva tiene que pasar por acá, que es donde se piensa si es identidad o no.
	// `rustdesk_id` se sumó tras pensarlo, que es para lo que sirve esta lista.
	//
	// NO es identidad en el sentido que importa: el cerebro lo guarda en la fila del TOKEN, así
	// que una máquina no puede usarlo para que algo se le atribuya a otra. Es un identificador
	// PÚBLICO del cliente de pantalla —sin la contraseña de sesión no sirve para entrar—, y es
	// el equivalente de `version` y `direccion`: lo que la máquina sabe de sí misma.
	//
	// El techo real, y conviene tenerlo escrito: una máquina comprometida puede reportar un id
	// AJENO y hacer que un operador se conecte a la máquina equivocada. No le da acceso a nada
	// (la contraseña se aplicó en la máquina que mintió, no en la otra), pero puede desorientar.
	// Cerrarlo del todo exigiría que el cerebro verifique el id contra el relay, y eso es un
	// slice propio.
	//
	// `servicios` (S12) entra por la misma puerta y con la misma pregunta contestada: un
	// fleet.ReporteServicio no tiene NINGÚN campo de identidad —ni device, ni project, ni id— así
	// que lo único que ese bloque puede tocar es el inventario de la máquina del token. El agente
	// TODAVÍA no lo manda (enumerar systemd, el SCM y Docker es un slice propio, cabo A42), pero
	// la decisión se declara acá, que es donde se piensa, y la ejercita
	// TestUnCuerpoConServiciosSigueSinLlevarIdentidad.
	// `puede_preguntar` y `motivo_no_preguntar` entran a la lista blanca tras el examen que el
	// mensaje de abajo exige: NINGUNA de las dos dice QUIÉN ES esta máquina. La primera es una
	// capacidad medida —«hay dónde dibujar un diálogo acá»— y la segunda dice por qué no la hay.
	// Como `version` y `direccion`, son lo que la máquina sabe DE SÍ MISMA y el cerebro no puede
	// averiguar solo; la única fila que pueden tocar sigue siendo la del token presentado.
	permitidas := map[string]bool{"muestra": true, "version": true, "direccion": true,
		"rustdesk_id": true, "servicios": true, "puede_preguntar": true, "motivo_no_preguntar": true}
	for k := range cuerpo {
		if !permitidas[k] {
			t.Errorf("el cuerpo trae una clave no declarada: %q. Si es legítima, sumala a la lista "+
				"blanca DESPUÉS de convencerte de que no es identidad.\n%s", k, visto)
		}
	}
	if _, hay := cuerpo["muestra"]; !hay {
		t.Fatalf("el cuerpo no trae `muestra`: %s", visto)
	}
	// Nada que huela a identidad, ni siquiera adentro de la muestra.
	for _, prohibido := range []string{"device_id", "device", "hostname", "project", "token", `"name"`} {
		if strings.Contains(visto, prohibido) {
			t.Errorf("el cuerpo menciona %q: la identidad no puede viajar en el cuerpo\n%s", prohibido, visto)
		}
	}
}

// D5 · S12 — EL BLOQUE DE SERVICIOS TAMPOCO LLEVA IDENTIDAD, Y ES EL REGISTRO EN CASTELLANO LO
// QUE LO SALVA.
//
// El barrido de arriba busca la subcadena `"name"` en el JSON ENTERO. Un ReporteServicio con el
// campo llamado `name` —el nombre obvio, el que sale solo— haría fallar el latido de toda la
// flota en cuanto el enumerador aterrice, y el que lo escriba no va a estar mirando este archivo.
// Así que el cuerpo se arma acá tal como viajaría y se le corre el MISMO barrido, hoy, antes.
//
// Sabotaje que la hace fallar: renombrar el tag de fleet.ReporteServicio.Nombre de `nombre` a
// `name` (o agregarle un `device_id`).
func TestUnCuerpoConServiciosSigueSinLlevarIdentidad(t *testing.T) {
	pid := 4242
	carga := map[string]any{
		"version": version,
		"servicios": []fleet.ReporteServicio{{
			Nombre: "postgresql.service", Clase: "systemd",
			Salud: fleet.SaludServicio{
				Tomada: time.Now().UTC(), Estado: fleet.EstadoCorriendo, PID: &pid,
				Detalle: "Result=success",
			},
		}},
	}
	crudo, err := json.Marshal(carga)
	if err != nil {
		t.Fatalf("el cuerpo no serializa: %v", err)
	}
	visto := string(crudo)

	var cuerpo map[string]any
	if err := json.Unmarshal(crudo, &cuerpo); err != nil {
		t.Fatalf("el cuerpo no es JSON: %v (%s)", err, visto)
	}
	// `puede_preguntar` y `motivo_no_preguntar` entran a la lista blanca tras el examen que el
	// mensaje de abajo exige: NINGUNA de las dos dice QUIÉN ES esta máquina. La primera es una
	// capacidad medida —«hay dónde dibujar un diálogo acá»— y la segunda dice por qué no la hay.
	// Como `version` y `direccion`, son lo que la máquina sabe DE SÍ MISMA y el cerebro no puede
	// averiguar solo; la única fila que pueden tocar sigue siendo la del token presentado.
	permitidas := map[string]bool{"muestra": true, "version": true, "direccion": true,
		"rustdesk_id": true, "servicios": true, "puede_preguntar": true, "motivo_no_preguntar": true}
	for k := range cuerpo {
		if !permitidas[k] {
			t.Errorf("el cuerpo con servicios trae una clave no declarada: %q\n%s", k, visto)
		}
	}
	// El MISMO barrido que el latido de arriba, sobre el JSON entero.
	for _, prohibido := range []string{"device_id", "device", "hostname", "project", "token", `"name"`} {
		if strings.Contains(visto, prohibido) {
			t.Errorf("el bloque de servicios menciona %q: la identidad no puede viajar en el cuerpo\n%s", prohibido, visto)
		}
	}
}

// D4 — en un OS sin colector, tomarMuestra devuelve nil y el agente late IGUAL.
// Sabotaje: hacer que tomarMuestra devuelva una Muestra vacía ante el error → todos los Windows
// aparecerían al 0 % de CPU, que se cree y no se arregla.
func TestSinColectorElAgenteLateIgualYNoMandaCeros(t *testing.T) {
	if m := tomarMuestra(colectorRoto{}); m != nil {
		t.Fatalf("un colector que falla produjo una muestra: %+v", m)
	}
	if m := tomarMuestra(nil); m != nil {
		t.Fatal("un colector nil produjo una muestra")
	}
	// Y el latido sale igual: estar viva es información útil aunque no se pueda medir cómo está.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	if res := latir(ts.URL, "tok", tomarMuestra(colectorRoto{})); !res.ok {
		t.Fatalf("sin colector, el agente dejó de latir: %+v", res)
	}
}

type colectorRoto struct{}

func (colectorRoto) Tomar() (fleet.Muestra, error) { return fleet.Muestra{}, fleet.ErrSinColector }

// CABO CERRADO — la dirección que reporta el agente prefiere el TAILNET (100.64.0.0/10).
//
// La IP de la LAN de una oficina no le sirve al cerebro para alcanzar nada, y hay una por cada
// red a la que el equipo se conecte; la del tailnet es estable y ruteable desde donde vive el
// cerebro. Si no hay tailnet se cae a la primera IPv4 no-loopback, que es mejor que nada.
//
// Es informativa: el cerebro NO la usa para autenticar. Si la usara, un device podría mentir.
func TestLaDireccionReportadaNoEsLoopback(t *testing.T) {
	d := direccionPropia()
	if d == "" {
		t.Skip("esta máquina no tiene ninguna IPv4 no-loopback")
	}
	if strings.HasPrefix(d, "127.") {
		t.Errorf("se reportó una dirección de loopback (%q): no sirve para alcanzar la máquina", d)
	}
	t.Logf("dirección reportada: %s", d)
}

// EL AGENTE HABLA POR DOS RUTAS, Y LAS DERIVA DE LA BASE.
//
// Lo encontró la prueba end-to-end, no los unitarios: éstos apuntaban a un httptest que responde
// a CUALQUIER ruta, así que pasar la URL del latido como base —y construir
// `/fleet/heartbeat/fleet/result`— pasaba desapercibido. Contra un cerebro real fue un 404 y el
// resultado de cada comando se perdía.
//
// Sabotaje que la hace fallar: concatenar la ruta a una base que ya la tiene.
func TestLasDosRutasSeDerivanDeLaBase(t *testing.T) {
	casos := []string{
		"http://127.0.0.1:7717",
		"http://127.0.0.1:7717/",
		// Un operador que setea MUSUBI_BRAIN_URL con la ruta ya pegada no debería quedarse sin
		// exec por eso.
		"http://127.0.0.1:7717/fleet/heartbeat",
		"http://127.0.0.1:7717/mcp",
	}
	for _, base := range casos {
		if got := rutaLatido(base); got != "http://127.0.0.1:7717/fleet/heartbeat" {
			t.Errorf("rutaLatido(%q) = %q", base, got)
		}
		if got := rutaResultado(base); got != "http://127.0.0.1:7717/fleet/result" {
			t.Errorf("rutaResultado(%q) = %q", base, got)
		}
	}
}

// Y el camino completo contra un servidor que EXIGE las rutas correctas, que es lo que el
// httptest permisivo no verificaba.
func TestElAgenteUsaLaRutaCorrectaParaCadaCosa(t *testing.T) {
	vistas := map[string]bool{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vistas[r.URL.Path] = true
		switch r.URL.Path {
		case "/fleet/heartbeat", "/fleet/result":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	if res := latir(ts.URL, "tok", nil); !res.ok {
		t.Fatalf("el latido no llegó a su ruta: %+v", res)
	}
	if err := reportar(ts.URL, "tok", resultadoDeComando{ComandoID: "x"}); err != nil {
		t.Fatalf("el reporte no llegó a su ruta: %v", err)
	}
	for _, r := range []string{"/fleet/heartbeat", "/fleet/result"} {
		if !vistas[r] {
			t.Errorf("nunca se llamó a %s (rutas vistas: %v)", r, vistas)
		}
	}
}
