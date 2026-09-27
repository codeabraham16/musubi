package mcp

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/buildid"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// A133 — EL DESFASE DEL RELOJ DE CADA MÁQUINA. El agente manda en cada latido la hora de su reloj
// al enviarlo (`enviado_ms`); el cerebro le resta la suya al recibirlo. Estas pruebas entran por
// el handler real del latido siempre que lo que custodian pasa por ahí.

// maquinaQueLate levanta un cerebro con HTTP real y una máquina enrolada con las capacidades
// dadas. `envolver` puede reemplazar la base ANTES de levantar el HTTP (nil = la base tal cual):
// cambiarla después sería escribir un campo que el handler lee desde otra goroutine.
func maquinaQueLate(t *testing.T, caps []string, envolver func(memory.StorageBackend) memory.StorageBackend) (s *McpServer, url, tok, id string) {
	t.Helper()
	s = newTestServer(t, embedding.NoopProvider{})
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": caps, "project": "casa", "os": "linux", "arch": "amd64",
	})
	if e != nil {
		t.Fatalf("fleet_enroll: %+v", e)
	}
	tok, _ = jsonOf(t, res)["token"].(string)
	d, ok, err := s.engine.DevicePorNombre("casa", "pc-gio")
	if tok == "" || err != nil || !ok {
		t.Fatalf("no quedó enrolada: token=%v ok=%v err=%v", tok != "", ok, err)
	}
	if envolver != nil {
		s.engine = envolver(s.engine)
	}
	return s, servidorHTTP(t, s).URL, tok, d.ID
}

// latirConCuerpo hace latir a la máquina por el handler real y devuelve la respuesta parseada.
func latirConCuerpo(t *testing.T, url, tok string, c fleet.CuerpoLatido) fleet.RespuestaLatido {
	t.Helper()
	cuerpo, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	code, body := postCon(t, url+fleetHeartbeatPath, tok, string(cuerpo))
	if code != http.StatusOK {
		t.Fatalf("el latido falló: %d %s", code, body)
	}
	var r fleet.RespuestaLatido
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("parsear la respuesta: %v (%s)", err, body)
	}
	return r
}

// latirConHora late con lo que manda un agente de capver 4: versión, contrato y la hora de su
// reloj. `enviadoMs` en 0 es un latido SIN hora, como el de un agente anterior.
func latirConHora(t *testing.T, url, tok string, enviadoMs int64) fleet.RespuestaLatido {
	t.Helper()
	return latirConCuerpo(t, url, tok, fleet.CuerpoLatido{
		Version: "0.141.0-prueba", Capver: buildid.Capver, EnviadoMs: enviadoMs,
	})
}

// EL DESFASE TIENE SIGNO, Y EL SIGNO ES LA MITAD DEL DIAGNÓSTICO.
//
// Un reloj ADELANTADO y uno ATRASADO se arreglan en lugares distintos: la pila del BIOS agotada
// atrasa, un NTP apuntado a un servidor equivocado puede correr para cualquier lado. La alerta
// compara con `abs()`, pero la serie tiene que conservar el signo para que quien la lea sepa cuál
// de los dos tiene enfrente.
//
// Sabotaje: restar al revés.
// arnes: archivo="internal/mcp/reloj.go"
// arnes: de="\t\tdesfase: time.UnixMilli(enviadoMs).Sub(llegada),"
// arnes: a="\t\tdesfase: llegada.Sub(time.UnixMilli(enviadoMs)),"
// arnes: colision_ok="TestUnRelojAbsurdoSeMideSinDesbordar"
//
// Sabotaje: devolver la magnitud en vez del desfase.
// arnes: archivo="internal/mcp/reloj.go"
// arnes: de="\treturn m.desfase, true"
// arnes: a="\treturn m.desfase.Abs(), true"
func TestElDesfaseConservaElSigno(t *testing.T) {
	s, url, tok, id := maquinaQueLate(t, []string{"metrics", "exec"}, nil)
	for _, c := range []struct {
		nombre      string
		corrimiento time.Duration
	}{
		{"adelantado", 90 * time.Second},
		{"atrasado", -45 * time.Second},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			latirConHora(t, url, tok, time.Now().Add(c.corrimiento).UnixMilli())
			desfase, hay := s.relojDe(id, time.Now())
			if !hay {
				t.Fatal("el latido trajo la hora y no quedó ninguna medición")
			}
			// El viaje hasta el handler corre el desfase unos milisegundos hacia abajo, nunca hacia
			// arriba: el margen es para eso, y es chico al lado del corrimiento.
			if desfase > c.corrimiento || desfase < c.corrimiento-5*time.Second {
				t.Fatalf("un reloj corrido %v se midió como %v", c.corrimiento, desfase)
			}
		})
	}
}

// UN RELOJ ABSURDO SE MIDE, Y NO DA LA VUELTA.
//
// Es exactamente la máquina que esto existe para ver, y `enviado_ms` puede traer cualquier entero.
// Restar los milisegundos a mano desborda el int64 de nanosegundos de `time.Duration` pasados los
// ±292 años y el resultado sale con cualquier signo: un reloj en el año 10000 podía leerse
// ATRASADO. `time.Time.Sub` satura en el extremo en vez de desbordar.
//
// SE COMPARA CONTRA EL VALOR SATURADO EXACTO, y no contra «más de cien años»: un desborde da un
// número arbitrario, y ése cae del lado correcto de cualquier umbral una de cada tantas corridas.
//
// Sabotaje: restar en milisegundos.
// arnes: archivo="internal/mcp/reloj.go"
// arnes: de="\t\tdesfase: time.UnixMilli(enviadoMs).Sub(llegada),"
// arnes: a="\t\tdesfase: time.Duration(enviadoMs-llegada.UnixMilli()) * time.Millisecond,"
// arnes: colision_ok="TestElDesfaseConservaElSigno"
func TestUnRelojAbsurdoSeMideSinDesbordar(t *testing.T) {
	s, url, tok, id := maquinaQueLate(t, []string{"metrics", "exec"}, nil)
	for _, c := range []struct {
		nombre    string
		enviadoMs int64
		esperado  time.Duration
	}{
		{"en el año 10000", time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), math.MaxInt64},
		{"el entero más grande", math.MaxInt64, math.MaxInt64},
		{"el entero más chico", math.MinInt64, math.MinInt64},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			latirConHora(t, url, tok, c.enviadoMs)
			desfase, hay := s.relojDe(id, time.Now())
			if !hay {
				t.Fatal("el reloj absurdo no quedó medido: justo el que más importa ver")
			}
			if desfase != c.esperado {
				t.Fatalf("enviado_ms=%d se midió como %v (%d ns), esperaba el extremo saturado %d ns",
					c.enviadoMs, desfase, int64(desfase), int64(c.esperado))
			}
		})
	}
}

// UN LATIDO SIN HORA BORRA LA MEDICIÓN ANTERIOR.
//
// Pasa de verdad: a una máquina le vuelven a poner un agente anterior al capver 4 y sigue
// latiendo, sin hora; o el cuerpo llega ilegible. Si la medición anterior se quedara, el
// exportador seguiría publicando un desfase que nadie volvió a medir, con la misma cara que uno
// fresco, hasta que venza.
//
// Sabotaje: no borrar.
// arnes: archivo="internal/mcp/reloj.go"
// arnes: de="\t\ts.relojes.Delete(deviceID)"
// arnes: a="\t\t_ = deviceID"
func TestUnLatidoSinHoraBorraLaMedicion(t *testing.T) {
	for _, c := range []struct {
		nombre string
		cuerpo string
	}{
		{"un agente anterior al capver 4", `{"version":"0.137.0-prueba","capver":3}`},
		{"un cuerpo ilegible", `{esto no es json`},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			s, url, tok, id := maquinaQueLate(t, []string{"metrics", "exec"}, nil)
			latirConHora(t, url, tok, time.Now().Add(30*time.Second).UnixMilli())
			if _, hay := s.relojDe(id, time.Now()); !hay {
				t.Fatal("control: el latido con hora no dejó medición")
			}

			if code, body := postCon(t, url+fleetHeartbeatPath, tok, c.cuerpo); code != http.StatusOK {
				t.Fatalf("control: el latido sin hora se rechazó (%d %s), así que no mide lo que dice", code, body)
			}
			if desfase, hay := s.relojDe(id, time.Now()); hay {
				t.Fatalf("después de un latido sin hora se sigue publicando un desfase de %v que nadie volvió a medir", desfase)
			}
		})
	}
}

// LA MEDICIÓN VENCE CON EL UMBRAL DE «EN LÍNEA».
//
// Una máquina que dejó de latir figura caída a los 90 s, y desde ahí nadie volvió a mirar su reloj.
// Publicar el último desfase conocido diría «este reloj se corrió tanto» sobre algo que nadie
// confirmó.
//
// Sabotaje: sacar la vigencia.
// arnes: archivo="internal/mcp/reloj.go"
// arnes: de="\tif !ok || ahora.Sub(m.cuando) > relojVigencia {"
// arnes: a="\tif !ok {"
//
// Sabotaje: que la medición dure más que la máquina en línea.
// arnes: archivo="internal/mcp/reloj.go"
// arnes: de="const relojVigencia = umbralEnLineaDefault\n"
// arnes: a="const relojVigencia = 10 * umbralEnLineaDefault\n"
func TestLaMedicionDelRelojVence(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// SE MIDE CONTRA EL UMBRAL QUE DA POR CAÍDA A UNA MÁQUINA CON AGENTE, y no contra
	// relojVigencia: contra su propia constante, la prueba pasaba con cualquier valor.
	vigencia := umbralEnLineaPara(fleet.Device{Tier: fleet.TierAgente}, s.sondaIntervalo)
	llegada := time.Now()
	s.registrarReloj("pc-gio", llegada.Add(time.Minute).UnixMilli(), llegada)

	if _, hay := s.relojDe("pc-gio", llegada.Add(vigencia)); !hay {
		t.Fatal("control: la medición no vale ni en el borde de su vigencia")
	}
	if desfase, hay := s.relojDe("pc-gio", llegada.Add(vigencia+time.Second)); hay {
		t.Fatalf("una medición vencida se sigue publicando (%v): nadie volvió a mirar ese reloj", desfase)
	}
}

// baseLenta demora la primera lectura del handler —resolver el token— y la transacción del
// latido, como un disco ocupado o un checkpoint del WAL. Con la demora sólo en la transacción, una
// llegada tomada en cualquier punto del medio —después del token, del cuerpo o de las escrituras
// del autorreporte— pasaba en verde.
type baseLenta struct {
	memory.StorageBackend
	demora time.Duration
}

func (b baseLenta) DevicePorTokenConRotacion(token string, ahora time.Time) (fleet.Device, bool, bool, error) {
	time.Sleep(b.demora)
	return b.StorageBackend.DevicePorTokenConRotacion(token, ahora)
}

func (b baseLenta) LatirYTomarComandos(id string, ahora time.Time, muestra string, tope int) (bool, []fleet.Comando, error) {
	time.Sleep(b.demora)
	return b.StorageBackend.LatirYTomarComandos(id, ahora, muestra, tope)
}

// LA HORA DE LLEGADA SE TOMA AL ENTRAR, NO DESPUÉS DE LA BASE.
//
// El handler escribe la base antes de anotar el reloj, y tiene que ser así: sólo un latido
// aceptado mide. Pero si la hora de llegada se tomara ahí, cada milisegundo que tarde la base se
// leería como un reloj ATRASADO en la máquina, y un cerebro lento haría disparar la alerta sobre
// máquinas en hora. Y la base ya se toca antes, al resolver el token: por eso la demora está en
// esa primera lectura y en la última.
//
// El reloj de la «máquina» es el mismo que el del cerebro, así que el desfase real es cero más el
// viaje por loopback.
//
// Sabotaje: anotar el reloj con la hora de después de la base.
// arnes: archivo="internal/mcp/fleet_http.go"
// arnes: de="\t\ts.registrarReloj(d.ID, enviadoMs, llegada)\n\n\t\tresp := fleet.RespuestaLatido{"
// arnes: a="\t\ts.registrarReloj(d.ID, enviadoMs, time.Now())\n\t\t_ = llegada\n\n\t\tresp := fleet.RespuestaLatido{"
//
// Sabotaje: tomar la llegada después de resolver el token, que es la primera espera de la base.
// arnes: archivo="internal/mcp/fleet_http.go"
// arnes: de="\t\td, conElNuevo, ok, err := s.engine.DevicePorTokenConRotacion(token, time.Now())\n"
// arnes: a="\t\td, conElNuevo, ok, err := s.engine.DevicePorTokenConRotacion(token, time.Now())\n\t\tllegada = time.Now()\n"
func TestLaLlegadaSeTomaAntesDeLaBase(t *testing.T) {
	const demora = 500 * time.Millisecond
	s, url, tok, id := maquinaQueLate(t, []string{"metrics", "exec"}, func(b memory.StorageBackend) memory.StorageBackend {
		return baseLenta{StorageBackend: b, demora: demora}
	})

	latirConHora(t, url, tok, time.Now().UnixMilli())
	desfase, hay := s.relojDe(id, time.Now())
	if !hay {
		t.Fatal("el latido trajo la hora y no quedó ninguna medición")
	}
	if desfase < -demora/2 {
		t.Fatalf("un reloj en hora se midió %v atrasado: es la demora de la base (%v) leída como desfase de la máquina", -desfase, demora)
	}
}

// baseQueRevoca rechaza la transacción del latido mientras `revocada` esté puesta, como cuando un
// admin da de baja la máquina entre la resolución del token y la escritura.
type baseQueRevoca struct {
	memory.StorageBackend
	revocada *atomic.Bool
}

func (b baseQueRevoca) LatirYTomarComandos(id string, ahora time.Time, muestra string, tope int) (bool, []fleet.Comando, error) {
	if b.revocada.Load() {
		return false, nil, nil
	}
	return b.StorageBackend.LatirYTomarComandos(id, ahora, muestra, tope)
}

// UN LATIDO QUE LA BASE RECHAZA NO TOCA LA MEDICIÓN.
//
// Entre resolver el token y escribir la base, un admin puede revocar la máquina: la transacción
// devuelve `actualizado == false` y el latido se contesta 401. Lo que ese latido traía no se
// anota, igual que el resto de lo que traía.
//
// Sabotaje: anotar el reloj sin mirar si la base aceptó el latido.
// arnes: archivo="internal/mcp/fleet_http.go"
// arnes: de="\t\t\td.ID, time.Now(), muestraJSON, maxComandosPorLatido)\n"
// arnes: a="\t\t\td.ID, time.Now(), muestraJSON, maxComandosPorLatido)\n\t\ts.registrarReloj(d.ID, enviadoMs, llegada)\n"
func TestUnLatidoRechazadoNoTocaLaMedicion(t *testing.T) {
	var revocada atomic.Bool
	s, url, tok, id := maquinaQueLate(t, []string{"metrics", "exec"}, func(b memory.StorageBackend) memory.StorageBackend {
		return baseQueRevoca{StorageBackend: b, revocada: &revocada}
	})

	latirConHora(t, url, tok, time.Now().Add(time.Minute).UnixMilli())
	antes, hay := s.relojDe(id, time.Now())
	if !hay {
		t.Fatal("control: el latido aceptado no dejó medición")
	}

	revocada.Store(true)
	cuerpo, _ := json.Marshal(fleet.CuerpoLatido{
		Version: "0.141.0-prueba", Capver: buildid.Capver, EnviadoMs: time.Now().Add(-time.Hour).UnixMilli(),
	})
	if code, body := postCon(t, url+fleetHeartbeatPath, tok, string(cuerpo)); code != http.StatusUnauthorized {
		t.Fatalf("control: el latido revocado no se rechazó (%d %s), así que no mide lo que dice", code, body)
	}
	if despues, hay := s.relojDe(id, time.Now()); !hay || despues != antes {
		t.Fatalf("un latido rechazado cambió la medición: era %v y quedó %v (medida=%v)", antes, despues, hay)
	}
}

// LA HORA SE MIDE AUNQUE LA MUESTRA SE DESCARTE.
//
// El reloj es del latido, no de la telemetría. Una máquina sin la capacidad `metrics`, una que no
// manda muestra —un OS sin colector— o una cuya muestra no se puede leer siguen latiendo, y son
// justamente las que menos se ven desde el cerebro. Si la hora saliera por el mismo camino que la
// muestra, cualquiera de sus salidas tempranas se la llevaría puesta, sin error y sin aviso.
//
// Sabotaje: devolver la hora sólo cuando la muestra se guardó.
// arnes: archivo="internal/mcp/fleet_http.go"
// arnes: de="\treturn json, notaMuestra, notaServicios, notaProtocolo, cuerpo.EnviadoMs"
// arnes: a="\tif notaMuestra != \"guardada\" {\n\t\treturn json, notaMuestra, notaServicios, notaProtocolo, 0\n\t}\n\treturn json, notaMuestra, notaServicios, notaProtocolo, cuerpo.EnviadoMs"
func TestLaHoraSeMideAunqueLaMuestraSeDescarte(t *testing.T) {
	sana, err := muestraDePrueba().Serializar()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		nombre  string
		caps    []string
		muestra json.RawMessage
	}{
		{"sin muestra", []string{"metrics", "exec"}, nil},
		{"sin la capacidad metrics", []string{"exec"}, json.RawMessage(sana)},
		{"con una muestra ilegible", []string{"metrics", "exec"}, json.RawMessage(`[1,2,3]`)},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			s, url, tok, id := maquinaQueLate(t, c.caps, nil)
			r := latirConCuerpo(t, url, tok, fleet.CuerpoLatido{
				Version: "0.141.0-prueba", Capver: buildid.Capver, Muestra: c.muestra,
				EnviadoMs: time.Now().Add(20 * time.Second).UnixMilli(),
			})
			if r.Muestra == "guardada" {
				t.Fatal("control: la muestra se guardó, así que esta prueba no recorre la salida temprana que dice")
			}
			if _, hay := s.relojDe(id, time.Now()); !hay {
				t.Fatalf("la muestra se descartó (%q) y se llevó puesta la hora: esta máquina no tiene desfase publicado", r.Muestra)
			}
		})
	}
}

// LA SERIE DEL RELOJ EXISTE SÓLO CON UNA MEDICIÓN, Y CON SU SIGNO.
//
// Un 0 en la serie afirma «este reloj está en hora». Para una máquina con un agente anterior al
// capver 4 nadie lo midió, y el cero la haría pasar por sana justo porque no sabe decirlo.
//
// Sabotaje: emitir 0 cuando no hay medición.
// arnes: archivo="internal/mcp/fleet_prometheus.go"
// arnes: de="\t\tdesfase, hay := relojDe(d.ID, ahora)\n\t\tif !hay {\n\t\t\tcontinue\n\t\t}"
// arnes: a="\t\tdesfase, hay := relojDe(d.ID, ahora)\n\t\tif !hay {\n\t\t\tdesfase = 0\n\t\t}"
func TestLaSerieDelRelojEstaSoloCuandoHayMedicion(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	medida := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	maquinaConMuestra(t, s, "casa", "sin-hora", *muestraDePrueba(), ahora)
	// Llegada en un milisegundo exacto: así el desfase es exacto y el render se compara entero.
	llegada := time.UnixMilli(ahora.UnixMilli())
	s.registrarReloj(medida.ID, llegada.Add(-45*time.Second).UnixMilli(), llegada)

	var b strings.Builder
	renderFlota(&b, s.engine, ptrPrincipal(principalDePrometheus()), ahora, s.sondaIntervalo, versionDePrueba, nil, s.relojDe, serviciosPorProyectoDefault, aprobacionesPorProyectoDefault)
	out := b.String()
	if !strings.Contains(out, nombreRelojDesfase+`{project="casa",device="pc-gio"} -45.000`+"\n") {
		t.Fatalf("la máquina medida no publica su desfase con signo:\n%s", out)
	}
	if strings.Contains(out, nombreRelojDesfase+`{project="casa",device="sin-hora"}`) {
		t.Fatalf("se publicó el desfase de una máquina que nadie midió: un 0 acá dice «en hora»\n%s", out)
	}
}

// EL DESFASE LLEGA AL /metrics DE VERDAD.
//
// Las pruebas del render le pasan la medición a mano; ésta entra por el GET que hace Prometheus,
// que es donde un cable roto en http.go dejaría la serie sin publicar con todo lo demás en verde.
//
// Sabotaje: que http.go no le pase la medición al exportador.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="s.vidaDeRedDe, s.relojDe,"
// arnes: a="s.vidaDeRedDe, nil,"
// arnes: colision_ok="TestLaPerillaGobiernaElMetricsDeVerdadYNoSoloAlRenderFlota"
func TestElDesfaseLlegaAlMetricsDeVerdad(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	d := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	llegada := time.UnixMilli(ahora.UnixMilli())
	s.registrarReloj(d.ID, llegada.Add(90*time.Second).UnixMilli(), llegada)

	if out := metricsDeVerdad(t, s); !strings.Contains(out, nombreRelojDesfase+`{project="casa",device="pc-gio"} 90.000`+"\n") {
		t.Fatalf("el /metrics de verdad no publica el desfase medido:\n%s", out)
	}
}

// EL LISTADO MUESTRA EL DESFASE, Y SÓLO SI SE MIDIÓ.
//
// Es donde se mira una máquina puntual. Sin el campo, el desfase existiría sólo como serie, y
// una alerta que dispara mandaría a buscar un número que la tool de la flota no muestra.
//
// Sabotaje: sacar el campo del listado.
// arnes: archivo="internal/mcp/methods_fleet.go"
// arnes: de="\t\t\t\tfila[\"reloj_desfase_s\"] = math.Round(desfase.Seconds()*1000) / 1000"
// arnes: a="\t\t\t\t_ = math.Round(desfase.Seconds())"
func TestElListadoMuestraElDesfaseDelReloj(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	medida := maquinaConMuestra(t, s, "casa", "pc-gio", *muestraDePrueba(), ahora)
	maquinaConMuestra(t, s, "casa", "sin-hora", *muestraDePrueba(), ahora)
	llegada := time.UnixMilli(ahora.UnixMilli())
	s.registrarReloj(medida.ID, llegada.Add(-45*time.Second).UnixMilli(), llegada)

	vistas := 0
	for _, fila := range listarFlota(t, s, "casa") {
		switch fila["name"] {
		case "pc-gio":
			vistas++
			if v, ok := fila["reloj_desfase_s"].(float64); !ok || v != -45 {
				t.Errorf("reloj_desfase_s = %v (%T), esperaba -45", fila["reloj_desfase_s"], fila["reloj_desfase_s"])
			}
		case "sin-hora":
			vistas++
			if v, hay := fila["reloj_desfase_s"]; hay {
				t.Errorf("una máquina que nadie midió trae reloj_desfase_s = %v: un 0 acá dice «en hora»", v)
			}
		}
	}
	if vistas != 2 {
		t.Fatalf("control: el listado trajo %d de las 2 máquinas", vistas)
	}
}

// LA ALERTA DEL RELOJ DISPARA PARA LOS DOS LADOS, Y SE CALLA CON LA MÁQUINA CAÍDA O EN
// MANTENIMIENTO.
//
// Las otras guardas de las alertas miran NOMBRES y FORMAS, no lo que la `expr` hace con el valor.
// Y TestNingunaAlertaDeMuestraDisparaSobreUnaMaquinaCaida sólo revisa las series
// `musubi_fleet_device_*` y `musubi_fleet_service_*`: la del reloj no es ninguna de las dos, así
// que sin esta prueba la regla se podía vaciar en verde. Medido en la revisión: sin el `abs`, con
// otro umbral, con otro plazo o con la serie mal escrita, las pruebas que leen reglas seguían verdes.
//
// Sabotaje que la pone roja: comparar el desfase con su signo, que calla a un reloj atrasado.
// arnes: archivo="deploy/musubi-alerts-flota.yml"
// arnes: de="abs(musubi_fleet_clock_offset_"
// arnes: a="(musubi_fleet_clock_offset_"
//
// Sabotaje que la pone roja: leer una serie que el cerebro no emite, que la deja muda para siempre.
// arnes: archivo="deploy/musubi-alerts-flota.yml"
// arnes: de="offset_seconds) > 30"
// arnes: a="offset_second) > 30"
//
// Sabotaje que la pone roja: invertir la guarda de la máquina caída, que la calla con la máquina viva.
// arnes: archivo="deploy/musubi-alerts-flota.yml"
// arnes: de="> 30\n          unless on(project, device) (musubi_fleet_device_up == 0)"
// arnes: a="> 30\n          unless on(project, device) (musubi_fleet_device_up == 1)"
//
// Sabotaje que la pone roja: invertir la de mantenimiento, que la calla fuera de toda ventana.
// arnes: archivo="deploy/musubi-alerts-flota.yml"
// arnes: de="(musubi_fleet_device_maintenance == 1)\n        for: 15m\n        labels: { severity: warning }\n        annotations:\n          summary: \"El reloj"
// arnes: a="(musubi_fleet_device_maintenance == 0)\n        for: 15m\n        labels: { severity: warning }\n        annotations:\n          summary: \"El reloj"
//
// Sabotaje que la pone roja: sacarle el plazo, que la hace sonar en cada despertar. Pisa el ancla
// del sabotaje anterior sólo porque ése usa el `for:` de contexto para ser único: cada uno cae por
// una aserción distinta.
// arnes: archivo="deploy/musubi-alerts-flota.yml"
// arnes: de="        for: 15m\n        labels: { severity: warning }\n        annotations:\n          summary: \"El reloj"
// arnes: a="        for: 0s\n        labels: { severity: warning }\n        annotations:\n          summary: \"El reloj"
// arnes: colision_ok="TestLaAlertaDelRelojDisparaParaLosDosLados"
func TestLaAlertaDelRelojDisparaParaLosDosLados(t *testing.T) {
	escenario := func(desfase, up, mantenimiento float64) map[string]valorEnVentana {
		return map[string]valorEnVentana{
			nombreRelojDesfase:                {ahora: desfase},
			"musubi_fleet_device_up":          {ahora: up},
			"musubi_fleet_device_maintenance": {ahora: mantenimiento},
		}
	}
	casos := []struct {
		que     string
		series  map[string]valorEnVentana
		dispara bool
	}{
		{"el reloj está 45 s adelantado", escenario(45, 1, 0), true},
		{"el reloj está 45 s atrasado", escenario(-45, 1, 0), true},
		{"el reloj está 20 s adelantado", escenario(20, 1, 0), false},
		{"el reloj está 20 s atrasado", escenario(-20, 1, 0), false},
		{"la máquina está caída", escenario(45, 0, 0), false},
		{"la máquina está en mantenimiento", escenario(45, 1, 1), false},
	}

	var alerta alertaCruda
	for _, a := range alertasCrudasDelRepo(t) {
		if a.Archivo == "musubi-alerts-flota.yml" && a.Nombre == "RelojDesfasado" {
			alerta = a
		}
	}
	if alerta.Nombre == "" {
		t.Fatal("no existe la alerta RelojDesfasado en musubi-alerts-flota.yml")
	}
	arbol, err := parsearProm(alerta.Expr)
	if err != nil {
		t.Fatalf("no pude parsear %q: %v", alerta.Expr, err)
	}
	for _, c := range casos {
		// El escenario trae sólo las series que el cerebro emite: una regla que lee otra no se
		// puede evaluar, y eso es rojo, porque en producción tampoco sonaría nunca.
		_, dispara, err := evaluarEnEscenario(arbol, c.series)
		if err != nil {
			t.Errorf("no pude evaluar RelojDesfasado cuando %s: %v\n  `%s`", c.que, err, alerta.Expr)
			continue
		}
		if dispara != c.dispara {
			if c.dispara {
				t.Errorf("RelojDesfasado NO DISPARA cuando %s: `%s`. Es justo lo que avisa, y la alerta queda muda sin un solo error.", c.que, alerta.Expr)
			} else {
				t.Errorf("RelojDesfasado dispara cuando %s: `%s`. Una alerta que suena cuando no corresponde entrena a ignorar el canal.", c.que, alerta.Expr)
			}
		}
	}

	// EL PLAZO FILTRA EL SALTO DE UN MOMENTO: una máquina que vuelve de suspender late antes de
	// sincronizar su reloj. Sin plazo sonaría en cada despertar; con horas, un reloj corrido se
	// vería tarde. Los 15 min son la decisión de la spec, y cambiarlos pasa por acá.
	plazo, err := duracionProm(alerta.Plazo)
	switch {
	case !alerta.TienePlazo:
		t.Errorf("RelojDesfasado no tiene `for:`, y la decisión es 15m: sin plazo suena en cada despertar de una máquina suspendida")
	case err != nil:
		t.Errorf("RelojDesfasado: `for: %s` ilegible: %v", alerta.Plazo, err)
	case plazo < 15*time.Minute:
		t.Errorf("RelojDesfasado tiene `for: %s`, y la decisión es 15m: más corto suena en cada despertar de una máquina suspendida", alerta.Plazo)
	case plazo > 15*time.Minute:
		t.Errorf("RelojDesfasado tiene `for: %s`, y la decisión es 15m: más largo avisa tarde de un reloj que ya rompe los códigos TOTP", alerta.Plazo)
	}
}
