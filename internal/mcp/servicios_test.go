package mcp

// Pruebas del slice S12 del track «Control de flota»: la entidad SERVICIO — qué corre adentro de
// cada máquina— vista por las tools y por la puerta del dispositivo.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// principalDeFlota arma una credencial acotada a un proyecto con las concesiones que se le pidan.
func principalDeFlota(nombre, proyecto string, grants map[fleet.Cap][]string) *Principal {
	return &Principal{Name: nombre, Role: RoleWriter, ProjectID: proyecto, Fleet: grants}
}

// cuerpoDeServicios arma el JSON de un latido con un bloque de servicios.
func cuerpoDeServicios(reportes ...fleet.ReporteServicio) string {
	b, _ := json.Marshal(map[string]any{"servicios": reportes})
	return string(b)
}

func saludViva(estado fleet.EstadoServicio) fleet.SaludServicio {
	pid := 1234
	return fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: estado, PID: &pid}
}

// DECLARAR UN SERVICIO EN LA MÁQUINA DE OTRO TENANT SE RECHAZA CON EL MISMO TEXTO QUE SI NO
// EXISTIERA.
//
// Es el pedido explícito del slice: el mensaje no puede revelar si el device existe. Distinguirlos
// convertiría esta tool en un ORÁCULO de qué máquinas tiene el vecino — reconocimiento puro, y
// gratis, porque no hace falta ni ver la respuesta de otra tool.
//
// La compuerta real es writeOriginFor + DevicePorNombre DENTRO de ese proyecto: la máquina ajena
// simplemente no aparece. El texto igual se compara palabra por palabra, porque es fácil
// «mejorar» el mensaje y contar de más.
//
// Sabotaje que la hace fallar: resolver el device con DevicePorID (o sin el proyecto), o devolver
// «esa máquina es del proyecto %q» cuando existe y es ajena.
func TestDeclararUnServicioEnLaMaquinaDeOtroTenantNoRevelaQueExiste(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "web", "servidor-de-web") // la máquina de la víctima
	enrolarDePrueba(t, s, "casa", "nas")            // una máquina propia, para el control

	// Un admin ACOTADO a `casa`: read=own y write=own declarados, así que writeOriginFor lo sella
	// en su tenant. Es el caso realista —el admin del proyecto de un cliente— y es el único que
	// prueba algo: un admin FEDERADO (write=any) declara donde quiere a propósito, igual que
	// revoca donde quiere, y con él este test pasaría probando nada.
	atacante := &Principal{Name: "mallory", Role: RoleAdmin, ProjectID: "casa", Read: ReadOwn, Write: WriteOwn}

	_, errAjena := callAsPrincipal(t, s, atacante, "musubi_fleet_service_declare",
		map[string]any{"device": "servidor-de-web", "nombre": "postgres", "project": "web"})
	_, errInexistente := callAsPrincipal(t, s, atacante, "musubi_fleet_service_declare",
		map[string]any{"device": "no-existe-en-ningun-lado", "nombre": "postgres"})

	if errAjena == nil {
		t.Fatal("se declaró un servicio en la máquina de otro tenant")
	}
	if errInexistente == nil {
		t.Fatal("se declaró un servicio en una máquina inexistente")
	}
	// Se comparan las PLANTILLAS, no los textos: el nombre que el atacante escribió vuelve en el
	// mensaje (y tiene que volver, o no sabría cuál se equivocó). Lo que no puede diferir es todo
	// lo demás — ni una palabra, ni un código distinto.
	plantilla := func(e *RpcError, device string) string {
		return fmt.Sprintf("%d|%s", e.Code, strings.ReplaceAll(e.Message, device, "«el device»"))
	}
	deAjena := plantilla(errAjena, "servidor-de-web")
	deInexistente := plantilla(errInexistente, "no-existe-en-ningun-lado")
	if deAjena != deInexistente {
		t.Errorf("el rechazo distingue «ajena» de «inexistente» y se vuelve un oráculo:\n  ajena:       %s\n  inexistente: %s",
			deAjena, deInexistente)
	}
	if strings.Contains(errAjena.Message, "web") && !strings.Contains(errAjena.Message, "casa") {
		t.Errorf("el mensaje nombra el proyecto ajeno: %s", errAjena.Message)
	}
	// Y sobre su PROPIA máquina sí puede.
	if _, e := callAsPrincipal(t, s, atacante, "musubi_fleet_service_declare",
		map[string]any{"device": "nas", "nombre": "postgres"}); e != nil {
		t.Fatalf("no pudo declarar en su propia máquina: %+v", e)
	}
}

// UN SERVICIO DECLARADO Y TODAVÍA SIN MEDIR SE INFORMA `desconocido`, NUNCA `detenido`, Y CON
// `ultimo_reporte` EN null.
//
// Son los dos estados que el slice separa: «no sé» y «está caído». Si la fila dijera `detenido`,
// declarar un servicio a mano lo pintaría de rojo en el acto y alguien saldría a arreglar algo que
// nunca se rompió. Y un `ultimo_reporte` en cero se dibujaría como una fecha del año 1.
//
// Sabotaje: hacer que filaDeServicio devuelva EstadoDetenido cuando Salud es nil; o serializar
// UltimoReporte cero como fecha en vez de null.
func TestUnServicioSinSaludSeInformaDesconocidoYNoDetenido(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "nas")
	if _, e := call(t, s, "musubi_fleet_service_declare",
		map[string]any{"device": "nas", "nombre": "bot-telegram", "clase": "systemd", "project": "casa"}); e != nil {
		t.Fatalf("declare: %+v", e)
	}

	res, e := call(t, s, "musubi_fleet_services", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatalf("services: %+v", e)
	}
	filas, _ := jsonOf(t, res)["services"].([]any)
	if len(filas) != 1 {
		t.Fatalf("services devolvió %d filas", len(filas))
	}
	fila := filas[0].(map[string]any)
	if fila["estado"] != string(fleet.EstadoDesconocido) {
		t.Errorf("un servicio sin medir informa estado=%v, esperaba %q", fila["estado"], fleet.EstadoDesconocido)
	}
	if fila["estado"] == string(fleet.EstadoDetenido) {
		t.Error("un servicio sin medir se informó DETENIDO: «no sé» y «está caído» son cosas distintas")
	}
	if v, hay := fila["ultimo_reporte"]; !hay || v != nil {
		t.Errorf("ultimo_reporte = %v, esperaba null: «declarado y todavía sin medir» no tiene fecha", v)
	}
	if v := fila["antiguedad_s"]; v != nil {
		t.Errorf("antiguedad_s = %v, esperaba null", v)
	}
	if fila["fresco"] != false {
		t.Errorf("un servicio que nunca reportó se informó fresco")
	}
	// Y lo desconocido viaja como null, no como cero.
	for _, campo := range []string{"pid", "reinicios", "desde"} {
		if v := fila[campo]; v != nil {
			t.Errorf("%s = %v, esperaba null: un cero inventado es indistinguible de uno medido", campo, v)
		}
	}
}

// LA COMPUERTA DE LA TOOL ES POR MÁQUINA, NO POR PROYECTO.
//
// Sin `metrics` SOBRE ESA MÁQUINA no se ven sus servicios, aunque sean del propio proyecto. Es la
// misma regla que musubi_fleet_metrics: ver el inventario de máquinas y ver qué corre adentro son
// permisos distintos.
//
// Sabotaje: filtrar sólo por proyecto y no llamar a PuedeSobreDevice — el principal sin
// concesiones ve los dos servicios.
func TestUnPrincipalSinMetricsNoVeLosServiciosDeEsaMaquina(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "nas")
	enrolarDePrueba(t, s, "casa", "pc-gio")
	for _, m := range []struct{ dev, svc string }{{"nas", "postgres"}, {"pc-gio", "docker"}} {
		if _, e := call(t, s, "musubi_fleet_service_declare",
			map[string]any{"device": m.dev, "nombre": m.svc, "project": "casa"}); e != nil {
			t.Fatalf("declare %s: %+v", m.svc, e)
		}
	}

	// Sólo `metrics` sobre `nas`. `pc-gio` no está en la concesión.
	acotado := principalDeFlota("bob", "casa", map[fleet.Cap][]string{fleet.CapMetrics: {"nas"}})
	res, e := callAsPrincipal(t, s, acotado, "musubi_fleet_services", map[string]any{})
	if e != nil {
		t.Fatalf("services: %+v", e)
	}
	out := jsonOf(t, res)
	filas, _ := out["services"].([]any)
	if len(filas) != 1 {
		t.Fatalf("vio %d servicios, esperaba sólo el de `nas`: %v", len(filas), out)
	}
	if filas[0].(map[string]any)["nombre"] != "postgres" {
		t.Errorf("vio el servicio equivocado: %v", filas[0])
	}
	// Y se DICE cuántos quedaron afuera, en agregado y sin nombrarlos: negar no revela existencia.
	if n, _ := out["sin_permiso"].(float64); n != 1 {
		t.Errorf("sin_permiso = %v, esperaba 1: una lista corta sin explicación se lee como «no hay más»", out["sin_permiso"])
	}
	if strings.Contains(textOf(t, res), "docker") {
		t.Error("la respuesta nombra el servicio que no puede ver")
	}
	// Sin NINGUNA concesión de flota: lista vacía. La compuerta no se deriva del rol.
	sinNada := principalDeFlota("carol", "casa", nil)
	res2, e := callAsPrincipal(t, s, sinNada, "musubi_fleet_services", map[string]any{})
	if e != nil {
		t.Fatalf("services sin concesiones: %+v", e)
	}
	if filas, _ := jsonOf(t, res2)["services"].([]any); len(filas) != 0 {
		t.Errorf("un principal sin concesiones de flota vio %d servicios", len(filas))
	}
}

// EL LATIDO TRAE EL INVENTARIO Y NO GANA IDENTIDAD.
//
// El bloque `servicios` entra por la puerta del dispositivo, el device sale del TOKEN, y el
// ReporteServicio no tiene por dónde declarar de quién es.
//
// Sabotaje: sacar el `s.guardarServiciosDelLatido` del camino del latido → el agente reporta y no
// pasa nada, sin un solo error.
func TestElLatidoRegistraLosServiciosDeSuPropiaMaquina(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	cuerpo := cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "postgresql.service", Clase: "systemd", Salud: saludViva(fleet.EstadoCorriendo)},
		fleet.ReporteServicio{Nombre: "nginx.service", Clase: "systemd", Salud: saludViva(fleet.EstadoFallado)},
	)
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpo); code != http.StatusOK {
		t.Fatalf("el latido con servicios devolvió %d: %s", code, body)
	}

	res, e := call(t, s, "musubi_fleet_services", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatalf("services: %+v", e)
	}
	filas, _ := jsonOf(t, res)["services"].([]any)
	if len(filas) != 2 {
		t.Fatalf("se registraron %d servicios de 2: %s", len(filas), textOf(t, res))
	}
	por := map[string]map[string]any{}
	for _, f := range filas {
		m := f.(map[string]any)
		por[m["nombre"].(string)] = m
	}
	if por["postgresql.service"]["estado"] != string(fleet.EstadoCorriendo) {
		t.Errorf("postgres quedó en %v", por["postgresql.service"]["estado"])
	}
	if por["nginx.service"]["estado"] != string(fleet.EstadoFallado) {
		t.Errorf("nginx quedó en %v", por["nginx.service"]["estado"])
	}
	if por["postgresql.service"]["device"] != "pc-gio" {
		t.Errorf("el servicio se atribuyó a %v en vez de a la máquina del token", por["postgresql.service"]["device"])
	}
	if por["postgresql.service"]["fresco"] != true {
		t.Error("un servicio recién reportado no se informó fresco")
	}
}

// UN LATIDO CON DEMASIADOS SERVICIOS DESCARTA EL BLOQUE Y SIGUE VALIENDO.
//
// Es D7 con otra ropa: estar viva y saber enumerarse son cosas distintas. Un 400 haría que una
// máquina con el enumerador roto DESAPAREZCA del inventario — precisamente cuando más querés
// verla.
//
// Y el bloque se descarta ENTERO en vez de truncarse: un inventario a medias haría que la poda por
// ausencia dé de baja los servicios que quedaron afuera del corte.
//
// Sabotaje: devolver 400 cuando el bloque se pasa del techo; o truncar a los primeros 64 en vez
// de descartar.
func TestUnLatidoConDemasiadosServiciosDescartaElBloqueYSigueValiendo(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	muchos := make([]fleet.ReporteServicio, 0, fleet.ServiciosPorLatido+10)
	for i := 0; i < fleet.ServiciosPorLatido+10; i++ {
		muchos = append(muchos, fleet.ReporteServicio{
			Nombre: fmt.Sprintf("servicio-%03d", i), Salud: saludViva(fleet.EstadoCorriendo)})
	}
	code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoDeServicios(muchos...))
	if code != http.StatusOK {
		t.Fatalf("el latido con demasiados servicios devolvió %d: %s", code, body)
	}
	if !strings.Contains(body, `"ok":true`) {
		t.Errorf("el latido no valió: %s", body)
	}
	// La máquina SIGUE VIVA en el inventario…
	inv, e := call(t, s, "musubi_fleet_list", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(textOf(t, inv), `"online":true`) {
		t.Error("la máquina no quedó en línea tras un bloque de servicios sobrado")
	}
	// …y NO se guardó ningún servicio.
	res, e := call(t, s, "musubi_fleet_services", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatal(e)
	}
	if filas, _ := jsonOf(t, res)["services"].([]any); len(filas) != 0 {
		t.Fatalf("se guardaron %d servicios de un bloque que se pasó del techo", len(filas))
	}
}

// UNA MÁQUINA SIN `metrics` LATE IGUAL Y SU INVENTARIO SE DESCARTA EN LA PUERTA.
//
// D8 llegando a los servicios: la capacidad no es decorativa. Sin esto, conceder capacidades sería
// un gesto sin efecto y el inventario diría una cosa mientras la base guarda otra.
//
// SE MIRA EL ALMACÉN Y NO LA TOOL, y ésa es toda la diferencia entre esta prueba y una decorativa.
// La primera versión preguntaba por `musubi_fleet_services` y quedaba VERDE con la compuerta de la
// puerta borrada —verificado corriéndolo—: la fila se escribía igual, y la tool la filtraba después
// por su propia compuerta de LECTURA. O sea que probaba la defensa equivocada, y la escritura sin
// permiso quedaba pasando desapercibida. Se pregunta por `ServiciosDeDevice`, que no compuertea.
//
// Sabotaje que la hace fallar: sacar el `if !d.Permite(fleet.CapMetrics)` de
// guardarServiciosDelLatido.
func TestUnaMaquinaSinMetricsNoRegistraServiciosPeroLateIgual(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// Se enrola SIN `metrics`: sólo `exec`.
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "muda", "tier": "A", "caps": []string{"exec"}, "project": "casa", "os": "linux"})
	if e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	tok, _ := jsonOf(t, res)["token"].(string)
	ts := servidorHTTPDeFlota(t, s)

	code, body := postCon(t, ts.URL+fleetHeartbeatPath, tok,
		cuerpoDeServicios(fleet.ReporteServicio{Nombre: "postgres", Salud: saludViva(fleet.EstadoCorriendo)}))
	if code != http.StatusOK {
		t.Fatalf("la máquina sin `metrics` no pudo latir: %d %s", code, body)
	}
	// La máquina SIGUE VIVA: estar viva y poder ser medida son cosas distintas.
	d, existe, err := s.engine.DevicePorNombre("casa", "muda")
	if err != nil || !existe {
		t.Fatalf("la máquina desapareció: %v", err)
	}
	if d.LastSeen.IsZero() {
		t.Error("el latido de una máquina sin `metrics` no se registró")
	}
	// Y NO se escribió ni una fila de servicio. Se pregunta al ALMACÉN, que no compuertea.
	guardados, err := s.engine.ServiciosDeDevice(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(guardados) != 0 {
		t.Fatalf("una máquina sin `metrics` escribió %d servicios en la base: %+v", len(guardados), guardados)
	}
}

// LA PODA POR AUSENCIA CORRE DESDE EL LATIDO, Y UN LATIDO SIN BLOQUE NO VACÍA EL INVENTARIO.
//
// Las dos mitades juntas, porque el riesgo está en la segunda: un agente viejo —o uno cuyo
// enumerador falló— manda el latido sin `servicios`, y eso NO puede significar «no corre nada».
//
// Sabotaje: llamar a PodarServiciosAusentes con la lista vacía cuando no vino el bloque (o sacarle
// al almacén el early-return de `len(vivos) == 0`).
func TestLaPodaPorAusenciaCorreDesdeElLatidoYUnLatidoMudoNoVaciaNada(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)

	// Tres servicios.
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "a", Salud: saludViva(fleet.EstadoCorriendo)},
		fleet.ReporteServicio{Nombre: "b", Salud: saludViva(fleet.EstadoCorriendo)},
		fleet.ReporteServicio{Nombre: "c", Salud: saludViva(fleet.EstadoCorriendo)},
	)); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	if n := cuantosServicios(t, s); n != 3 {
		t.Fatalf("se registraron %d de 3", n)
	}

	// El siguiente latido ya no trae `c`: se poda.
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "a", Salud: saludViva(fleet.EstadoCorriendo)},
		fleet.ReporteServicio{Nombre: "b", Salud: saludViva(fleet.EstadoCorriendo)},
	)); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	if n := cuantosServicios(t, s); n != 2 {
		t.Fatalf("la poda dejó %d servicios, esperaba 2", n)
	}

	// Y un latido SIN bloque de servicios no vacía nada: «no reportó ninguno» es también lo que se
	// ve cuando el agente arrancó a medias.
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, `{"version":"0.1.0"}`); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	if n := cuantosServicios(t, s); n != 2 {
		t.Fatalf("un latido mudo dejó %d servicios: vació el inventario por no traer el bloque", n)
	}
}

// EL LATIDO NO SE LLEVA PUESTO LO QUE SE DECLARÓ A MANO. Es el escenario reportado, entero.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTA PRUEBA VALE EL VIAJE COMPLETO POR HTTP
//
// La guarda vive en el almacén (`AND declared = 0`) y ahí ya tiene su prueba. Ésta la ejercita
// por donde el defecto iba a aparecer de verdad: alguien declara un bot con la tool, la máquina
// late enumerando SUS units —que nunca van a incluir al bot, porque para eso existe la tool— y la
// poda corre. Entre la tool y el UPDATE hay tres capas y cualquiera de las tres puede volver a
// abrir el agujero (armar `vivos` en otro lado, podar antes de guardar, «simplificar» el flag).
//
// Con A42 abierto el agente todavía no enumera, así que esto no explota HOY: explota entero, en
// toda la flota a la vez, el día que se despache el slice de enumeración. Ésa es exactamente la
// clase de bomba que una prueba de integración tiene que desactivar antes.
//
// Sabotaje que la hace fallar (VERIFICADO): sacarle el `AND declared = 0` al UPDATE de
// PodarServiciosAusentes. `bot-telegram` desaparece del listado y redeclararlo... lo revive, así
// que la segunda mitad de esta prueba también cubre la salida que antes no existía.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestElLatidoNoPodaLoQueSeDeclaroAMano(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t) // enrola `pc-gio` en el proyecto `casa`

	if _, e := call(t, s, "musubi_fleet_service_declare",
		map[string]any{"device": "pc-gio", "nombre": "bot-telegram", "clase": "docker", "project": "casa"}); e != nil {
		t.Fatalf("declarar el bot: %+v", e)
	}

	// La máquina late enumerando lo suyo. El bot no está —ningún enumerador de systemd lo ve— y
	// eso NO puede significar que dejó de existir.
	code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "sshd.service", Salud: saludViva(fleet.EstadoCorriendo)},
	))
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}

	nombres := map[string]bool{}
	res, e := call(t, s, "musubi_fleet_services", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatalf("services: %+v", e)
	}
	filas, _ := jsonOf(t, res)["services"].([]any)
	for _, f := range filas {
		fila, _ := f.(map[string]any)
		nombre, _ := fila["nombre"].(string)
		nombres[nombre] = true
		// La columna `declarado` viaja en la fila: sin ella, las dos filas se ven iguales y
		// esperan cosas distintas de la poda.
		if _, hay := fila["declarado"]; !hay {
			t.Errorf("la fila de %q no dice si la declaró una persona o la enumeró la máquina", nombre)
		}
	}
	if !nombres["bot-telegram"] {
		t.Fatalf("el latido dio de baja `bot-telegram`, que declaró una persona y ninguna máquina enumera: "+
			"la respuesta del latido fue %s", strings.TrimSpace(body))
	}
	if !nombres["sshd.service"] {
		t.Error("se perdió `sshd.service`, que sí vino en el latido")
	}

	// ── Y LA SALIDA, POR EL ÚNICO CAMINO POR EL QUE SE LLEGA DE VERDAD ──────────────────────
	//
	// Un servicio que la máquina enumeraba y dejó de enumerar SÍ se poda (está bien: eso es lo que
	// la poda sabe). Si después alguien decide que esa cosa existe igual —se corre a mano, es un
	// bot, el enumerador no la ve— y la declara, antes se llevaba un «ya existe un servicio con
	// ese nombre en esa máquina»: la fila revocada seguía ocupando el único (project, device,
	// name) y el listado por defecto no la mostraba, así que el mensaje era falso desde donde
	// mira quien opera y encima no decía qué hacer. Ahora la declaración la trae de vuelta, y de
	// ahí en más queda protegida como cualquier declarada.
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "sshd.service", Salud: saludViva(fleet.EstadoCorriendo)},
		fleet.ReporteServicio{Nombre: "viejo.service", Salud: saludViva(fleet.EstadoCorriendo)},
	)); code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}
	// El latido siguiente ya no lo trae: se poda, porque a esta fila la trajo la máquina.
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "sshd.service", Salud: saludViva(fleet.EstadoCorriendo)},
	)); code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}
	if nombresDeServiciosDeCasa(t, s)["viejo.service"] {
		t.Fatal("no se podó `viejo.service`, que la máquina dejó de enumerar: sin eso, la segunda mitad de esta prueba no prueba nada")
	}

	if _, e := call(t, s, "musubi_fleet_service_declare",
		map[string]any{"device": "pc-gio", "nombre": "viejo.service", "clase": "docker", "project": "casa"}); e != nil {
		t.Fatalf("declarar a mano un servicio que la poda se había llevado: %+v — la fila revocada sigue "+
			"ocupando el nombre, así que sin revivirla quien opera no tiene NINGUNA salida", e)
	}
	if !nombresDeServiciosDeCasa(t, s)["viejo.service"] {
		t.Fatal("`viejo.service` no volvió al inventario después de declararlo")
	}
	// Y ya declarado, el latido que no lo menciona no se lo lleva más.
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "sshd.service", Salud: saludViva(fleet.EstadoCorriendo)},
	)); code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}
	if !nombresDeServiciosDeCasa(t, s)["viejo.service"] {
		t.Error("el latido volvió a podar `viejo.service` después de que una persona lo declarara: revivirlo " +
			"sin marcarlo declarado deja al operador en el mismo lugar, dando vueltas")
	}
}

// nombresDeServiciosDeCasa lista por la MISMA tool que usa una persona: lo que no se ve ahí, para
// quien opera no existe — que es justo el punto del defecto que estas pruebas cierran.
func nombresDeServiciosDeCasa(t *testing.T, s *McpServer) map[string]bool {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_services", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatalf("services: %+v", e)
	}
	out := map[string]bool{}
	filas, _ := jsonOf(t, res)["services"].([]any)
	for _, f := range filas {
		fila, _ := f.(map[string]any)
		nombre, _ := fila["nombre"].(string)
		out[nombre] = true
	}
	return out
}

// UN SERVICIO CON NOTICIAS VIEJAS DEJA DE ESTAR `fresco` SIN CAMBIAR DE `estado`.
//
// Son dos ejes distintos y hay que poder verlos a la vez: `estado` es lo ÚLTIMO que se supo, y
// `fresco` es si eso sigue valiendo. Colapsarlos —pintar `desconocido` cuando el dato envejece—
// perdería la única pista de qué estaba pasando cuando se dejó de saber.
//
// Sabotaje: hacer que filaDeServicio devuelva `desconocido` cuando el reporte es viejo; o que
// `fresco` sea siempre true mientras haya salud.
func TestUnServicioConNoticiasViejasDejaDeEstarFrescoSinCambiarDeEstado(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "nas")
	d, _, err := s.engine.DevicePorNombre("casa", "nas")
	if err != nil {
		t.Fatal(err)
	}
	// Un reporte de hace dos días, escrito por el almacén con su propio reloj.
	viejo := time.Now().Add(-48 * time.Hour)
	if _, _, err := s.engine.ReportarServicios(d.ID, viejo, []fleet.ReporteServicio{
		{Nombre: "postgres", Salud: fleet.SaludServicio{Tomada: viejo, Estado: fleet.EstadoCorriendo}}}); err != nil {
		t.Fatal(err)
	}

	res, e := call(t, s, "musubi_fleet_services", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatal(e)
	}
	filas, _ := jsonOf(t, res)["services"].([]any)
	if len(filas) != 1 {
		t.Fatalf("filas: %s", textOf(t, res))
	}
	fila := filas[0].(map[string]any)
	if fila["fresco"] != false {
		t.Error("un reporte de hace dos días se informó FRESCO: «corriendo» y «sin noticias» son cosas distintas")
	}
	if fila["estado"] != string(fleet.EstadoCorriendo) {
		t.Errorf("el estado cambió a %v al envejecer: se perdió la pista de qué pasaba cuando se dejó de saber", fila["estado"])
	}
	if edad, _ := fila["antiguedad_s"].(float64); edad < 47*3600 {
		t.Errorf("antiguedad_s = %v: no se ve que el dato es viejo", fila["antiguedad_s"])
	}
}

// UN FILTRO POR UNA MÁQUINA QUE NO EXISTE (O QUE NO PODÉS VER) RESPONDE VACÍO, NO UN ERROR.
//
// Si «no existe» diera un error distinto de «no la ves», el parámetro `device` sería un oráculo de
// qué máquinas hay en el proyecto de al lado.
//
// Sabotaje: devolver rpcErrorf("no hay una máquina llamada %q") cuando el filtro no resuelve.
func TestFiltrarPorUnaMaquinaQueNoVesRespondeVacioYNoUnError(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "web", "servidor-de-web")
	enrolarDePrueba(t, s, "casa", "nas")

	espia := principalDeFlota("mallory", "casa", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}})
	res, e := callAsPrincipal(t, s, espia, "musubi_fleet_services", map[string]any{"device": "servidor-de-web"})
	if e != nil {
		t.Fatalf("el filtro por una máquina ajena dio error en vez de vacío: %+v", e)
	}
	filas, _ := jsonOf(t, res)["services"].([]any)
	if len(filas) != 0 {
		t.Fatalf("vio %d servicios de la máquina ajena", len(filas))
	}
	res2, e := callAsPrincipal(t, s, espia, "musubi_fleet_services", map[string]any{"device": "esta-no-existe"})
	if e != nil {
		t.Fatalf("el filtro por una máquina inexistente dio error: %+v", e)
	}
	if textOf(t, res) != textOf(t, res2) {
		t.Errorf("«ajena» y «inexistente» responden distinto y el filtro se vuelve un oráculo:\n  ajena:       %s\n  inexistente: %s",
			textOf(t, res), textOf(t, res2))
	}
}

// DECLARAR UN SERVICIO ES ADMIN. Un writer con `metrics` sobre todo puede MIRAR y no puede
// ESCRIBIR en el inventario del plano de control.
//
// Sabotaje: sacar el `if !p.isAdmin()` de toolFleetServiceDeclare.
func TestDeclararUnServicioExigeAdmin(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "nas")
	writer := principalDeFlota("bob", "casa", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}})

	_, e := callAsPrincipal(t, s, writer, "musubi_fleet_service_declare",
		map[string]any{"device": "nas", "nombre": "postgres"})
	if e == nil {
		t.Fatal("un writer declaró un servicio: la escritura del inventario es admin")
	}
	if e.Code != codeUnauthorized {
		t.Errorf("el rechazo salió con código %d, esperaba %d", e.Code, codeUnauthorized)
	}
	// Y MIRAR sí puede.
	if _, e := callAsPrincipal(t, s, writer, "musubi_fleet_services", map[string]any{}); e != nil {
		t.Fatalf("el mismo writer no pudo listar los servicios: %+v", e)
	}
}

// EL AGENTE SE ENTERA DE QUÉ PASÓ CON SU INVENTARIO, PASE LO QUE PASE.
//
// Un bloque descartado en silencio se ve, DESDE LA MÁQUINA, idéntico a uno que nunca se mandó — y
// quien puede arreglarlo es justamente el que no lee los logs del cerebro. Es la misma decisión
// que ya toma la nota de la muestra, y por el mismo motivo.
//
// Los tres casos que importan son los tres que dan CERO filas: sin `metrics`, con el bloque
// sobrado, y el que sí guardó. Si el primero y el segundo no dijeran nada, se verían iguales entre
// sí Y iguales a no haber mandado nada.
//
// Sabotaje que la hace fallar: que guardarServiciosDelLatido devuelva "" en la rama de la
// capacidad que falta (o en la del techo) — el latido responde 200 y el agente no tiene forma de
// saber que su inventario no llegó.
func TestElAgenteSeEnteraDeQuePasoConSuInventario(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTPDeFlota(t, s)

	// 1 · una máquina CON `metrics`: se guarda y se dice cuántos.
	conMetrics := enrolarDePrueba(t, s, "casa", "pc-gio")
	_, body := postCon(t, ts.URL+fleetHeartbeatPath, conMetrics, cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "postgres", Salud: saludViva(fleet.EstadoCorriendo)}))
	if !strings.Contains(body, "guardados") || !strings.Contains(body, `"servicios"`) {
		t.Errorf("el agente no se enteró de que su inventario se guardó: %s", body)
	}

	// 2 · una máquina SIN `metrics`: se descarta y se dice POR QUÉ, nombrando la capacidad.
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "muda", "tier": "A", "caps": []string{"exec"}, "project": "casa", "os": "linux"})
	if e != nil {
		t.Fatal(e)
	}
	sinMetrics, _ := jsonOf(t, res)["token"].(string)
	_, body = postCon(t, ts.URL+fleetHeartbeatPath, sinMetrics, cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "postgres", Salud: saludViva(fleet.EstadoCorriendo)}))
	if !strings.Contains(body, "descartados") || !strings.Contains(body, "metrics") {
		t.Errorf("una máquina sin `metrics` no se entera de por qué su inventario no llegó: %s", body)
	}

	// 3 · el bloque sobrado: se dice el techo, que es lo accionable.
	muchos := make([]fleet.ReporteServicio, 0, fleet.ServiciosPorLatido+1)
	for i := 0; i <= fleet.ServiciosPorLatido; i++ {
		muchos = append(muchos, fleet.ReporteServicio{
			Nombre: fmt.Sprintf("s-%03d", i), Salud: saludViva(fleet.EstadoCorriendo)})
	}
	_, body = postCon(t, ts.URL+fleetHeartbeatPath, conMetrics, cuerpoDeServicios(muchos...))
	if !strings.Contains(body, "descartados") || !strings.Contains(body, "techo") {
		t.Errorf("un bloque sobrado no dice qué hacer: %s", body)
	}

	// 4 · un latido SIN bloque no inventa una nota: no hay nada que contar.
	_, body = postCon(t, ts.URL+fleetHeartbeatPath, conMetrics, `{"version":"0.1.0"}`)
	if strings.Contains(body, `"servicios"`) {
		t.Errorf("un latido sin inventario trajo una nota igual: %s", body)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────────────────────

// servidorHTTPDeFlota levanta el HTTP del cerebro sobre un server ya armado. Es la mitad de
// servidorConFlota que hace falta cuando el dispositivo se enrola con OTRAS capacidades que las
// del helper de S2 — que da `metrics` y `exec` a todo el mundo y por eso no sirve para probar la
// compuerta.
func servidorHTTPDeFlota(t *testing.T, s *McpServer) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{
		reqTimeout: 10 * time.Second, token: "token-de-una-persona", loopbackOnly: true,
	}))
	t.Cleanup(ts.Close)
	return ts
}

func cuantosServicios(t *testing.T, s *McpServer) int {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_services", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatalf("services: %+v", e)
	}
	filas, _ := jsonOf(t, res)["services"].([]any)
	return len(filas)
}

// ── A51 · El historial de una máquina revocada ──────────────────────────────────────────────

// maquinaRevocadaConServicios enrola una máquina, le reporta dos servicios y la revoca. Devuelve
// el servidor listo para preguntarle por el historial.
func maquinaRevocadaConServicios(t *testing.T) *McpServer {
	t.Helper()
	s, ts, tokenDevice, _ := servidorConFlota(t)
	cuerpo := cuerpoDeServicios(
		fleet.ReporteServicio{Nombre: "postgresql.service", Clase: "systemd", Salud: saludViva(fleet.EstadoCorriendo)},
		fleet.ReporteServicio{Nombre: "nginx.service", Clase: "systemd", Salud: saludViva(fleet.EstadoFallado)},
	)
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, cuerpo); code != http.StatusOK {
		t.Fatalf("el latido con servicios devolvió %d: %s", code, body)
	}
	if _, e := call(t, s, "musubi_fleet_revoke", map[string]any{"name": "pc-gio", "project": "casa"}); e != nil {
		t.Fatalf("fleet_revoke: %+v", e)
	}
	return s
}

func nombresDeServicios(t *testing.T, res interface{}) []string {
	t.Helper()
	out := jsonOf(t, res)
	crudas, _ := out["services"].([]any)
	var nombres []string
	for _, f := range crudas {
		nombres = append(nombres, f.(map[string]any)["nombre"].(string))
	}
	return nombres
}

// `incluir_revocados: true` prometía en su propia descripción los servicios «de máquinas
// revocadas», y ésa era la mitad falsa: el kill-switch de la revocación tumbaba el device ANTES de
// mirar la concesión, así que las filas —que la migración 36 conserva A PROPÓSITO para la
// auditoría— no salían nunca y no había forma de verlas.
//
// Una auditoría que nadie puede leer no es una auditoría.
//
// Sabotaje que la hace fallar: volver a `PuedeSobreDevice` en la rama de `IncluirRevocados`.
func TestElHistorialDeUnaMaquinaRevocadaSePuedeAuditar(t *testing.T) {
	s := maquinaRevocadaConServicios(t)
	auditor := principalDeFlota("auditor", "casa", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}})

	// Sin pedirlo, la máquina revocada no aparece: el default no cambia.
	res, e := callAsPrincipal(t, s, auditor, "musubi_fleet_services", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatalf("services: %+v", e)
	}
	if n := nombresDeServicios(t, res); len(n) != 0 {
		t.Errorf("sin incluir_revocados salieron %v: el default tiene que seguir mostrando sólo lo vivo", n)
	}

	res, e = callAsPrincipal(t, s, auditor, "musubi_fleet_services",
		map[string]any{"project": "casa", "incluir_revocados": true})
	if e != nil {
		t.Fatalf("services con incluir_revocados: %+v", e)
	}
	nombres := nombresDeServicios(t, res)
	if len(nombres) != 2 {
		t.Fatalf("el historial trajo %v; se esperaban los dos servicios de la máquina revocada", nombres)
	}

	// LA FILA DICE QUE LA MÁQUINA ESTÁ REVOCADA, y no sólo el servicio. Son dos bajas distintas:
	// `revocado` es «este servicio se dejó de usar» y `device_revocado` es «esta máquina salió de
	// la flota con todo adentro». Confundirlas hace leer un retiro deliberado donde hubo una baja.
	fila := jsonOf(t, res)["services"].([]any)[0].(map[string]any)
	if fila["device_revocado"] != true {
		t.Errorf("la fila no dice que la MÁQUINA está revocada: %#v", fila["device_revocado"])
	}
	if _, hay := fila["revocado"]; !hay {
		t.Error("desapareció `revocado`, que es la baja del SERVICIO: las dos tienen que verse")
	}
}

// LA MITAD QUE IMPORTA. Levantar el kill-switch para auditar no puede convertirse en una puerta
// lateral: la concesión `metrics` sigue haciendo falta, y la tenencia también. Quien no podía ver
// los servicios de esa máquina mientras vivía tampoco los ve después.
//
// Sin esta prueba, el arreglo de A51 sería indistinguible de «con incluir_revocados se ve todo».
//
// Sabotaje que la hace fallar: que PuedeVerHistorialDeDevice devuelva true directamente, o que
// saltee alcanzaElProyecto / tieneGrant en vez de delegar en PuedeSobreDevice.
func TestAuditarUnaMaquinaRevocadaSigueExigiendoLaConcesionYLaTenencia(t *testing.T) {
	s := maquinaRevocadaConServicios(t)

	// (a) Del mismo proyecto y SIN concesión `metrics`: no ve nada, y se cuenta.
	sinConcesion := principalDeFlota("mirón", "casa", map[fleet.Cap][]string{})
	res, e := callAsPrincipal(t, s, sinConcesion, "musubi_fleet_services",
		map[string]any{"project": "casa", "incluir_revocados": true})
	if e != nil {
		t.Fatalf("services: %+v", e)
	}
	if n := nombresDeServicios(t, res); len(n) != 0 {
		t.Errorf("una credencial SIN concesión `metrics` auditó una máquina revocada: %v", n)
	}
	if sp := jsonOf(t, res)["sin_permiso"]; sp != float64(2) {
		t.Errorf("sin_permiso = %v; las dos filas tapadas tienen que contarse, no desaparecer", sp)
	}

	// (b) De OTRO proyecto y CON la concesión: la tenencia se aplica antes que el grant, así que
	// nombrar la máquina ajena en principals.yaml no la alcanza. Ni siquiera llega a `sin_permiso`
	// —el proyecto ajeno no está en su barrido— y eso es lo correcto: no es un oráculo.
	ajeno := principalDeFlota("vecino", "otra-casa", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}})
	res, e = callAsPrincipal(t, s, ajeno, "musubi_fleet_services",
		map[string]any{"project": "casa", "incluir_revocados": true})
	if e == nil {
		if n := nombresDeServicios(t, res); len(n) != 0 {
			t.Errorf("una credencial de otro tenant auditó la máquina revocada del vecino: %v", n)
		}
	}

	// (c) LA TENENCIA, DIRECTO CONTRA LA COMPUERTA. El caso (b) de arriba pasa por la tool, y ahí
	// el barrido por proyecto filtra al vecino ANTES de llegar a PuedeVerHistorialDeDevice — así
	// que (b) no ejercita la tenencia de la compuerta, y sacarle `alcanzaElProyecto` a la variante
	// de auditoría lo dejaba en verde. Medido: el sabotaje de la tenencia sólo lo atrapaba la
	// guarda del TIER, que es otro invariante. Acá se llama a la función a mano.
	d, ok, err := s.engine.DevicePorNombre("casa", "pc-gio")
	if err != nil || !ok {
		t.Fatalf("no se pudo releer la máquina revocada: %v", err)
	}
	if PuedeVerHistorialDeDevice(ajeno, d, fleet.CapMetrics) {
		t.Error("la variante de auditoría salteó la tenencia: nombrar la máquina de otro tenant en " +
			"principals.yaml no puede alcanzarla, ni viva ni revocada")
	}
	// Y el control positivo, para que la línea de arriba no pase por estar rota de otra forma.
	propio := principalDeFlota("auditor", "casa", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}})
	if !PuedeVerHistorialDeDevice(propio, d, fleet.CapMetrics) {
		t.Error("el auditor del proyecto propio tampoco puede: la compuerta está cerrada de más")
	}
}

// LA REVOCACIÓN SIGUE SIENDO ABSOLUTA PARA TODO LO QUE TOQUE LA MÁQUINA. El arreglo de A51 abre
// una puerta de LECTURA de lo ya escrito, y es fácil que se filtre a las otras: exec, pantalla y
// shell pasan por PuedeSobreDevice, donde el kill-switch manda (C6).
//
// Sabotaje que la hace fallar: mover el `d.Revoked = false` adentro de PuedeSobreDevice.
func TestAuditarNoAflojaElKillSwitchParaOperar(t *testing.T) {
	s := maquinaRevocadaConServicios(t)
	d, ok, err := s.engine.DevicePorNombre("casa", "pc-gio")
	if err != nil || !ok {
		t.Fatalf("no se pudo releer la máquina revocada: %v", err)
	}
	if !d.Revoked {
		t.Fatal("la máquina no quedó revocada: el escenario de la prueba no es el que dice")
	}
	poderoso := principalDeFlota("root", "casa", map[fleet.Cap][]string{
		fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}, fleet.CapScreen: {"*"}, fleet.CapShell: {"*"},
	})
	for _, c := range []fleet.Cap{fleet.CapExec, fleet.CapScreen, fleet.CapScreenView, fleet.CapShell, fleet.CapMetrics} {
		if PuedeSobreDevice(poderoso, d, c) {
			t.Errorf("PuedeSobreDevice concedió %q sobre una máquina REVOCADA: el kill-switch se aflojó", c)
		}
	}
	// Y el historial sólo se abre para leer: la variante de auditoría concede `metrics`…
	if !PuedeVerHistorialDeDevice(poderoso, d, fleet.CapMetrics) {
		t.Error("la variante de auditoría no deja leer el historial de una máquina revocada")
	}
	// …y NO abre nada más por su cuenta: lo que concede sigue saliendo de la concesión Y DEL TIER.
	// Se prueba contra un tier que NO admite la capacidad —un Tier C no da `shell`— porque el
	// techo del aparato es el otro invariante que un `d.Revoked = false` mal ubicado se llevaría
	// puesto. No se usa t.Skip: una prueba omitida se lee igual que una que no existe.
	movil := d
	movil.Tier = fleet.TierMovil
	if fleet.TierAdmite(movil.Tier, fleet.CapShell) {
		t.Fatal("el fixture eligió un tier que SÍ admite shell: la línea de abajo no probaría nada")
	}
	if PuedeVerHistorialDeDevice(poderoso, movil, fleet.CapShell) {
		t.Error("la variante de auditoría concedió `shell` en un tier que no lo admite: aflojó más que la revocación")
	}
}
