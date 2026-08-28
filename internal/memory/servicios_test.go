package memory

import (
	"errors"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// saludDePrueba arma una salud creíble para no repetirla en cada caso.
func saludDePrueba(estado fleet.EstadoServicio) fleet.SaludServicio {
	pid := 4242
	rein := 0
	return fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: estado, PID: &pid, Reinicios: &rein}
}

// EL project_id DE UN SERVICIO SALE DEL DEVICE, NO DEL PEDIDO.
//
// Sin foreign keys, un servicio atribuido a un proyecto distinto del de su máquina es
// perfectamente representable, y esa desalineación es una fuga de tenant con la forma exacta de
// A6: la fila aparecería listando el proyecto del atacante.
//
// Sabotaje que la hace fallar: en AltaServicio, copiar `s.ProjectID` del argumento en vez de
// resolver el device y tomar `d.ProjectID`.
func TestElProyectoDelServicioSaleDelDeviceYNoDelPedido(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")

	s, err := e.AltaServicio(fleet.Servicio{
		Nombre: "postgres", DeviceID: d.ID, Clase: "systemd",
		ProjectID: "el-proyecto-del-atacante", // lo DECLARA, y no le sirve de nada
	})
	if err != nil {
		t.Fatalf("AltaServicio: %v", err)
	}
	if s.ProjectID != "casa" {
		t.Fatalf("el servicio quedó en el proyecto %q: el pedido pisó al device", s.ProjectID)
	}
	// Y no aparece listando el proyecto que declaró.
	if got, _ := e.ListarServicios("el-proyecto-del-atacante", "", false); len(got) != 0 {
		t.Fatalf("el servicio aparece en el proyecto declarado: %+v", got)
	}
	if got, _ := e.ListarServicios("casa", "", false); len(got) != 1 {
		t.Fatalf("el servicio no aparece en el proyecto del device: %+v", got)
	}
}

// UNA MÁQUINA REVOCADA NO ADMITE SERVICIOS NUEVOS, Y EL RECHAZO NO CUENTA DE MÁS.
//
// El mensaje es el MISMO para «no existe» y para «existe pero está revocada»: distinguirlos
// convierte el alta en un ORÁCULO de qué máquinas existieron alguna vez, que es la misma razón por
// la que motivoRechazo es un solo texto en la puerta del dispositivo. (La compuerta por TENANT
// vive una capa más arriba, en la tool, y tiene su propia prueba allá.)
//
// Sabotaje: devolver «esa máquina está revocada» cuando el device existe y está de baja.
func TestElAltaSobreUnaMaquinaRevocadaNoRevelaQueExistio(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	if _, err := e.RevocarDevice("casa", "nas"); err != nil {
		t.Fatal(err)
	}

	_, errRevocada := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: d.ID})
	_, errInexistente := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: "no-existe"})
	if errRevocada == nil {
		t.Fatal("se dio de alta un servicio en una máquina revocada")
	}
	if errInexistente == nil {
		t.Fatal("se dio de alta un servicio en una máquina inexistente")
	}
	if errRevocada.Error() != errInexistente.Error() {
		t.Errorf("el rechazo distingue «revocada» de «inexistente» y se vuelve un oráculo:\n  revocada: %v\n  inexistente: %v", errRevocada, errInexistente)
	}
	// Y el mensaje le dice al operador QUÉ hacer, no sólo qué pasó.
	if !strings.Contains(errRevocada.Error(), "musubi_fleet_enroll") {
		t.Errorf("el error no dice cómo salir del paso: %v", errRevocada)
	}
}

// UNA MÁQUINA SÓLO PUEDE TOCAR SUS PROPIOS SERVICIOS.
//
// Sin el `AND device_id = ?` del UPDATE, cualquier máquina de la flota puede reportar que el
// postgres de producción está caído. Es un error de SEGURIDAD, no de datos.
//
// Sabotaje: sacar el `AND device_id = ?` del UPDATE de ReportarServicios → el reporte de A pisa
// la salud del servicio de B.
func TestUnaMaquinaNoPuedeReportarLosServiciosDeOtra(t *testing.T) {
	e := newTestEngine(t)
	a, _ := altaDePrueba(t, e, "casa", "maquina-a")
	b, _ := altaDePrueba(t, e, "casa", "maquina-b")

	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: b.ID, Clase: "systemd"}); err != nil {
		t.Fatal(err)
	}
	// La máquina A reporta que el `postgres` está FALLADO. El único `postgres` del proyecto es
	// el de B.
	nuevos, act, err := e.ReportarServicios(a.ID, time.Now(), []fleet.ReporteServicio{
		{Nombre: "postgres", Salud: saludDePrueba(fleet.EstadoFallado)}})
	if err != nil {
		t.Fatalf("ReportarServicios: %v", err)
	}
	if act != 0 {
		t.Fatalf("la máquina A actualizó %d servicios de B", act)
	}
	// Lo que sí puede es crear el SUYO propio con el mismo nombre, y son dos filas distintas.
	if nuevos != 1 {
		t.Fatalf("la máquina A no registró su propio servicio: nuevos=%d", nuevos)
	}
	deB, _ := e.ServiciosDeDevice(b.ID)
	if len(deB) != 1 {
		t.Fatalf("los servicios de B: %+v", deB)
	}
	if deB[0].Salud != nil {
		t.Fatalf("el reporte de A tocó la salud del servicio de B: %+v", *deB[0].Salud)
	}
	if deB[0].EstadoActual() != fleet.EstadoDesconocido {
		t.Errorf("el servicio de B quedó en %q", deB[0].EstadoActual())
	}
}

// DOS MÁQUINAS PUEDEN TENER CADA UNA SU postgres; UNA MÁQUINA NO PUEDE TENER DOS.
//
// El nombre de un servicio sólo es único DENTRO de su máquina. Con el único por (project_id,
// name), el segundo host no podría registrar el suyo — y el síntoma sería «el alta falla en la
// máquina nueva», que nadie asocia con un índice.
//
// Sabotaje: declarar el índice único como (project_id, name) en la migración 36.
func TestDosMaquinasPuedenTenerCadaUnaSuPostgres(t *testing.T) {
	e := newTestEngine(t)
	a, _ := altaDePrueba(t, e, "casa", "maquina-a")
	b, _ := altaDePrueba(t, e, "casa", "maquina-b")

	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: a.ID}); err != nil {
		t.Fatalf("el postgres de A: %v", err)
	}
	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: b.ID}); err != nil {
		t.Fatalf("el postgres de B se rechazó: %v — el nombre sólo es único DENTRO de una máquina", err)
	}
	// Y la MISMA máquina no puede tenerlo dos veces.
	_, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: a.ID})
	if !errors.Is(err, fleet.ErrServicioDuplicado) {
		t.Fatalf("un duplicado en la misma máquina dio %v, esperaba ErrServicioDuplicado", err)
	}
}

// LA ÚLTIMA SALUD NO SE BORRA SOLA, Y EL SERVICIO QUE HOY NO SE PUDO MEDIR AVANZA IGUAL.
//
// Es la asimetría D7 un nivel más abajo: «el servicio existe» y «la máquina supo medirlo» son
// cosas distintas. Un reporte con el nombre bueno y la salud ilegible deja `last_health` con lo
// último bueno Y avanza `last_report` — perder las dos a la vez sería castigar al servicio por un
// `systemctl show` que falló por permisos.
//
// Sabotaje que la hace fallar (VERIFICADO, y la primera versión de esta prueba NO lo cazaba
// porque el reporte inválido se descartaba antes de llegar al UPDATE): escribir `last_health = ?`
// directo en vez del CASE → el servicio pierde su última medición buena en cuanto la máquina
// tropieza una sola vez.
func TestLaUltimaSaludViveEnLaFilaYNoSeBorraSola(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: d.ID}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.ReportarServicios(d.ID, time.Now(), []fleet.ReporteServicio{
		{Nombre: "postgres", Salud: saludDePrueba(fleet.EstadoCorriendo)}}); err != nil {
		t.Fatal(err)
	}
	antes, _ := e.ServiciosDeDevice(d.ID)
	if len(antes) != 1 || antes[0].Salud == nil {
		t.Fatalf("no se guardó la salud: %+v", antes)
	}

	// Un reporte con el NOMBRE bueno y la SALUD ilegible (sin `tomada`).
	mudo := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	if _, _, err := e.ReportarServicios(d.ID, mudo, []fleet.ReporteServicio{
		{Nombre: "postgres", Salud: fleet.SaludServicio{Estado: fleet.EstadoCorriendo}}}); err != nil {
		t.Fatal(err)
	}
	despues, _ := e.ServiciosDeDevice(d.ID)
	if len(despues) != 1 {
		t.Fatalf("el servicio desapareció: %+v", despues)
	}
	if despues[0].Salud == nil {
		t.Fatal("un reporte sin salud válida BORRÓ la última medición buena")
	}
	if despues[0].Salud.Estado != fleet.EstadoCorriendo {
		t.Errorf("la salud conservada es %q", despues[0].Salud.Estado)
	}
	// Y el reporte SÍ avanzó: el servicio sigue dando señales aunque no sepa medirse.
	if !despues[0].UltimoReporte.Equal(mudo) {
		t.Errorf("last_report quedó en %v y esperaba %v: el servicio dio señales y no se anotó",
			despues[0].UltimoReporte, mudo)
	}
}

// UNA SALUD ILEGIBLE NO ROMPE EL LISTADO. Perder una medición vieja es barato; quedarse sin
// inventario porque un campo no parsea es el fallo caro.
//
// Sabotaje: devolver el error de SaludDesdeTexto desde escanearServicio.
func TestUnaSaludIlegibleNoRompeElListado(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	s, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: d.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE services SET last_health = '{{{' WHERE id = ?`, s.ID); err != nil {
		t.Fatal(err)
	}
	got, err := e.ListarServicios("casa", "", false)
	if err != nil {
		t.Fatalf("una salud ilegible tumbó el listado entero: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("el servicio desapareció del listado: %+v", got)
	}
	if got[0].Salud != nil {
		t.Error("una salud ilegible se devolvió como si fuera legible")
	}
	if got[0].EstadoActual() != fleet.EstadoDesconocido {
		t.Errorf("el servicio con salud ilegible informa %q, esperaba desconocido", got[0].EstadoActual())
	}
}

// EL LISTADO AÍSLA POR TENANT Y NO DEVUELVE LAS HUÉRFANAS.
//
// La fila con project_id vacío se inserta A MANO para que la guarda tenga algo que tapar: sin
// ella el test pasaría con la guarda borrada, probando nada.
//
// Sabotaje: sacar la guarda de `projectID == ""` de ListarServicios; o filtrar con un JOIN a
// devices en vez de por el project_id denormalizado.
func TestListarServiciosAislaPorProyectoYNoDevuelveLasHuerfanas(t *testing.T) {
	e := newTestEngine(t)
	casa, _ := altaDePrueba(t, e, "casa", "nas")
	web, _ := altaDePrueba(t, e, "web", "servidor")
	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: casa.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "nginx", DeviceID: web.ID}); err != nil {
		t.Fatal(err)
	}
	// La huérfana: ninguna tool la crea, pero un backfill o una reparación a mano sí.
	if _, err := e.db.Exec(
		`INSERT INTO services (id, name, project_id, device_id, kind, registered_at, last_health, revoked)
		 VALUES ('huerfano', 'HUERFANO', '', 'nadie', '', '', '', 0)`); err != nil {
		t.Fatal(err)
	}

	deCasa, _ := e.ListarServicios("casa", "", false)
	if len(deCasa) != 1 || deCasa[0].Nombre != "postgres" {
		t.Fatalf("el listado de casa: %+v", deCasa)
	}
	sinProyecto, err := e.ListarServicios("", "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sinProyecto {
		if s.Nombre == "HUERFANO" {
			t.Fatal("listar sin proyecto devolvió la fila huérfana: justo la que no pertenece a nadie")
		}
	}
	if len(sinProyecto) != 0 {
		t.Fatalf("listar sin proyecto devolvió %d filas", len(sinProyecto))
	}
}

// REVOCAR UNA MÁQUINA REVOCA SUS SERVICIOS, en la misma transacción.
//
// Sin esto, los servicios de una máquina dada de baja siguen apareciendo en el listado del
// proyecto como si nada — y el hueco pasa desapercibido justo hasta un incidente, que es cuando
// alguien revoca y después mira.
//
// Sabotaje: dejar RevocarDevice como estaba (un UPDATE sobre `devices` y punto).
func TestRevocarUnaMaquinaSacaSusServiciosDelListado(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	otra, _ := altaDePrueba(t, e, "casa", "pc-gio")
	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: d.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "nginx", DeviceID: otra.ID}); err != nil {
		t.Fatal(err)
	}

	ok, err := e.RevocarDevice("casa", "nas")
	if err != nil || !ok {
		t.Fatalf("RevocarDevice: ok=%v err=%v", ok, err)
	}
	vivos, _ := e.ListarServicios("casa", "", false)
	for _, s := range vivos {
		if s.Nombre == "postgres" {
			t.Fatal("el servicio de la máquina revocada sigue en el listado del proyecto")
		}
	}
	if len(vivos) != 1 || vivos[0].Nombre != "nginx" {
		t.Fatalf("se llevó puestos los servicios de otras máquinas: %+v", vivos)
	}
	// La fila QUEDA, como la del device: la auditoría necesita saber qué corría ahí.
	conBajas, _ := e.ListarServicios("casa", "", true)
	if len(conBajas) != 2 {
		t.Fatalf("la fila del servicio revocado se borró en vez de quedar: %+v", conBajas)
	}
	// Y revocar una máquina inexistente sigue devolviendo (false, nil), no un error.
	if ok, err := e.RevocarDevice("casa", "no-existe"); ok || err != nil {
		t.Errorf("revocar una máquina inexistente: ok=%v err=%v", ok, err)
	}
}

// LA PODA POR AUSENCIA CON LISTA VACÍA NO BORRA NADA.
//
// «Este device no reportó ningún servicio» es también lo que se ve cuando el agente arrancó a
// medias o cuando systemd no contestó. Vaciar el inventario por eso es irreversible.
//
// LOS TRES SERVICIOS LOS CREA UN REPORTE Y NO EL ALTA A MANO, y el cambio no es cosmético: lo
// declarado a mano NO lo poda nadie (migración 37, ver servicios_declarados_test.go), así que
// armando el escenario con AltaServicio esta prueba pasaría a verificar la guarda equivocada — el
// segundo tramo, «con una lista no vacía sí poda», no podaría nunca y no se notaría.
//
// Sabotaje: quitar el early-return de `len(vivos) == 0` de PodarServiciosAusentes.
func TestPodarServiciosAusentesConListaVaciaNoBorraNada(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	reportar := func(dev string, nombres ...string) {
		t.Helper()
		var rs []fleet.ReporteServicio
		for _, n := range nombres {
			rs = append(rs, fleet.ReporteServicio{Nombre: n, Salud: saludDePrueba(fleet.EstadoCorriendo)})
		}
		if _, _, err := e.ReportarServicios(dev, time.Now().UTC(), rs); err != nil {
			t.Fatal(err)
		}
	}
	reportar(d.ID, "postgres", "nginx", "redis")
	n, err := e.PodarServiciosAusentes(d.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("una lista vacía podó %d servicios", n)
	}
	if vivos, _ := e.ServiciosDeDevice(d.ID); len(vivos) != 3 {
		t.Fatalf("quedaron %d servicios de 3", len(vivos))
	}
	// Con una lista NO vacía sí poda, y sólo lo ausente.
	if n, err := e.PodarServiciosAusentes(d.ID, []string{"postgres", "nginx"}); err != nil || n != 1 {
		t.Fatalf("la poda real: n=%d err=%v", n, err)
	}
	vivos, _ := e.ServiciosDeDevice(d.ID)
	if len(vivos) != 2 {
		t.Fatalf("quedaron %d servicios de 2: %+v", len(vivos), vivos)
	}
	// Y la poda NO cruza máquinas: sólo toca las de `deviceID`.
	otra, _ := altaDePrueba(t, e, "casa", "pc-gio")
	reportar(otra.ID, "solo-de-la-otra")
	if _, err := e.PodarServiciosAusentes(d.ID, []string{"postgres"}); err != nil {
		t.Fatal(err)
	}
	if deOtra, _ := e.ServiciosDeDevice(otra.ID); len(deOtra) != 1 {
		t.Fatal("la poda de una máquina se llevó puestos los servicios de otra")
	}
}

// UN SERVICIO REPORTADO POR PRIMERA VEZ SE CREA; EL MISMO REPORTE OTRA VEZ SE ACTUALIZA.
//
// Es el upsert por (device_id, name). Si cada latido creara una fila nueva, el inventario de una
// máquina crecería una fila cada 30 s — y el índice único lo frenaría con un error que el latido
// no debería ver nunca.
//
// Sabotaje: reemplazar el UPDATE-y-si-no-INSERT por un INSERT a secas.
func TestUnServicioReportadoSeCreaUnaVezYDespuesSeActualiza(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")

	rep := []fleet.ReporteServicio{{Nombre: "postgres", Clase: "systemd", Salud: saludDePrueba(fleet.EstadoCorriendo)}}
	if n, a, err := e.ReportarServicios(d.ID, time.Now(), rep); err != nil || n != 1 || a != 0 {
		t.Fatalf("primer reporte: nuevos=%d actualizados=%d err=%v", n, a, err)
	}
	rep[0].Salud = saludDePrueba(fleet.EstadoFallado)
	if n, a, err := e.ReportarServicios(d.ID, time.Now(), rep); err != nil || n != 0 || a != 1 {
		t.Fatalf("segundo reporte: nuevos=%d actualizados=%d err=%v", n, a, err)
	}
	vivos, _ := e.ServiciosDeDevice(d.ID)
	if len(vivos) != 1 {
		t.Fatalf("el upsert creó %d filas para el mismo servicio", len(vivos))
	}
	if vivos[0].EstadoActual() != fleet.EstadoFallado {
		t.Errorf("el segundo reporte no actualizó el estado: %q", vivos[0].EstadoActual())
	}
	if vivos[0].Clase != "systemd" {
		t.Errorf("la clase reportada no se guardó: %q", vivos[0].Clase)
	}
}

// UN NOMBRE INVÁLIDO SE SALTEA; UNA SALUD INVÁLIDA REGISTRA EL SERVICIO COMO `desconocido`.
//
// Los dos errores no son el mismo error, y tratarlos igual pierde información del lado caro: sin
// NOMBRE no hay servicio del que hablar, pero sin SALUD sí lo hay — la máquina lo nombró. Se
// guarda como `desconocido`, que es exactamente lo que es, en vez de desaparecer. Pasa de verdad:
// `systemctl list-units` da los nombres y `systemctl show` puede fallar por permisos en la misma
// corrida.
//
// Y ninguno de los dos tumba a los demás: un error devuelto acá haría que una unit con un nombre
// raro borre de la pantalla el inventario entero de esa máquina.
//
// Sabotaje: devolver error desde ReportarServicios cuando un reporte no valida.
func TestUnReporteInvalidoNoSeLlevaPuestosALosDemas(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	nuevos, _, err := e.ReportarServicios(d.ID, time.Now(), []fleet.ReporteServicio{
		{Nombre: "bueno-1", Salud: saludDePrueba(fleet.EstadoCorriendo)},
		{Nombre: "", Salud: saludDePrueba(fleet.EstadoCorriendo)},                               // sin nombre: se saltea
		{Nombre: "sin-fecha", Salud: fleet.SaludServicio{Estado: fleet.EstadoDetenido}},         // salud ilegible
		{Nombre: "estado-raro", Salud: fleet.SaludServicio{Estado: "sano", Tomada: time.Now()}}, // salud ilegible
		{Nombre: "bueno-2", Salud: saludDePrueba(fleet.EstadoDetenido)},
	})
	if err != nil {
		t.Fatalf("un reporte inválido devolvió error: %v", err)
	}
	if nuevos != 4 {
		t.Fatalf("se registraron %d servicios: esperaba los 2 buenos + los 2 de salud ilegible", nuevos)
	}
	vivos, _ := e.ServiciosDeDevice(d.ID)
	por := map[string]fleet.Servicio{}
	for _, s := range vivos {
		por[s.Nombre] = s
	}
	if _, hay := por[""]; hay {
		t.Error("se registró un servicio SIN NOMBRE: no hay servicio del que hablar")
	}
	if len(vivos) != 4 {
		t.Fatalf("quedaron %d filas: %+v", len(vivos), vivos)
	}
	// Los de salud ilegible existen y están DESCONOCIDOS, nunca detenidos.
	for _, n := range []string{"sin-fecha", "estado-raro"} {
		if por[n].EstadoActual() != fleet.EstadoDesconocido {
			t.Errorf("%s quedó en %q, esperaba desconocido", n, por[n].EstadoActual())
		}
		if por[n].Salud != nil {
			t.Errorf("%s guardó una salud que no se pudo interpretar: %+v", n, *por[n].Salud)
		}
	}
	if por["bueno-2"].EstadoActual() != fleet.EstadoDetenido {
		t.Errorf("bueno-2 quedó en %q", por["bueno-2"].EstadoActual())
	}
}

// UN SERVICIO REVOCADO NO RESUCITA POR UN REPORTE, Y TAMPOCO SIGUE RECIBIENDO TELEMETRÍA.
//
// Son las dos mitades del `AND revoked = 0`, y la segunda es la que de verdad lo custodia: sin esa
// cláusula el UPDATE ENCUENTRA la fila revocada y le pisa `last_report` y `last_health`. NO la
// resucita —`revoked` sigue en 1 y el listado no la devuelve— así que el síntoma no se ve por
// ningún lado: queda una fila dada de baja acumulando mediciones frescas, y el día que alguien la
// reactive va a leer un estado que nadie miró en meses como si fuera de ahora.
//
// Sabotaje que la hace fallar (VERIFICADO — la primera versión de esta prueba miraba sólo el
// listado y NO lo cazaba; por eso está escrita así): sacar el `AND revoked = 0` del UPDATE.
func TestUnServicioRevocadoNoResucitaNiSigueRecibiendoTelemetria(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	sv, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: d.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.RevocarServiciosDeDevice(d.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.ReportarServicios(d.ID, time.Now(), []fleet.ReporteServicio{
		{Nombre: "postgres", Salud: saludDePrueba(fleet.EstadoCorriendo)}}); err != nil {
		t.Fatal(err)
	}
	if vivos, _ := e.ServiciosDeDevice(d.ID); len(vivos) != 0 {
		t.Fatalf("un servicio revocado revivió por un reporte: %+v", vivos)
	}
	// No se duplicó la fila por el camino…
	todas, _ := e.ListarServicios("casa", "", true)
	if len(todas) != 1 {
		t.Fatalf("quedaron %d filas para un solo servicio: %+v", len(todas), todas)
	}
	// …y la fila revocada NO recibió la medición. Esto es lo que el `AND revoked = 0` sostiene.
	var reporte, salud string
	if err := e.db.QueryRow(
		`SELECT COALESCE(last_report,''), last_health FROM services WHERE id = ?`, sv.ID).Scan(&reporte, &salud); err != nil {
		t.Fatal(err)
	}
	if reporte != "" || salud != "" {
		t.Errorf("la fila revocada siguió recibiendo telemetría: last_report=%q last_health=%q", reporte, salud)
	}
}

// EL NOMBRE Y EL DETALLE QUE MANDA LA MÁQUINA SE ACOTAN AL GUARDARLOS.
//
// Lo que entra por la puerta del dispositivo viene de la superficie más expuesta de la flota: un
// nombre de 4 KiB ensuciaría una columna que después se dibuja en una tabla.
//
// Sabotaje: sacar el RecortarReporte del lazo de ReportarServicios.
func TestLoQueReportaLaMaquinaSeAcotaAlGuardarlo(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	salud := saludDePrueba(fleet.EstadoCorriendo)
	salud.Detalle = strings.Repeat("x", fleet.DetalleServicioMax+500)
	if _, _, err := e.ReportarServicios(d.ID, time.Now(), []fleet.ReporteServicio{
		{Nombre: strings.Repeat("u", fleet.NombreServicioMax+500), Salud: salud}}); err != nil {
		t.Fatal(err)
	}
	vivos, _ := e.ServiciosDeDevice(d.ID)
	if len(vivos) != 1 {
		t.Fatalf("no se guardó el servicio: %+v", vivos)
	}
	if n := len([]rune(vivos[0].Nombre)); n != fleet.NombreServicioMax {
		t.Errorf("el nombre guardado tiene %d runas, esperaba %d", n, fleet.NombreServicioMax)
	}
	if n := len([]rune(vivos[0].Salud.Detalle)); n != fleet.DetalleServicioMax {
		t.Errorf("el detalle guardado tiene %d runas, esperaba %d", n, fleet.DetalleServicioMax)
	}
}

// EL INVENTARIO NO ES UN TRINQUETE: LO QUE LA PODA SE LLEVÓ VUELVE CUANDO LA MÁQUINA LO REPORTA.
//
// Ésta es la mitad que faltaba del par, y su ausencia costó 18 contenedores en el servidor real.
// La poda por ausencia daba de baja lo que un latido no traía, y NADA lo volvía a traer: el
// UPDATE del reporte llevaba `AND revoked = 0`, así que la fila revocada no se actualizaba, y el
// INSERT chocaba con el índice único y se descartaba en silencio. La máquina reportaba sus 18
// contenedores en cada latido, para siempre, sin efecto y sin error en ningún lado.
//
// Podar por ausencia y no despodar por presencia es una asimetría, no una precaución: si la fila
// está acá porque la máquina la reporta, que la reporte de nuevo es exactamente la condición que
// la creó. Lo que NO vuelve solo es lo que puso una persona — eso lo custodia el par de abajo, y
// las dos mitades tienen que estar porque la forma más fácil de romper cada una es la otra.
//
// Sabotaje que la hace fallar: devolver el UPDATE a `WHERE name = ? AND device_id = ? AND
// revoked = 0` y sacarle el `revoked = 0` del SET.
func TestLoQuePodoLaAusenciaVuelveConLaPresencia(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")

	reportar := func(clase string, nombres ...string) {
		t.Helper()
		var rs []fleet.ReporteServicio
		for _, n := range nombres {
			rs = append(rs, fleet.ReporteServicio{Nombre: n, Clase: clase,
				Salud: saludDePrueba(fleet.EstadoCorriendo)})
		}
		if _, _, err := e.ReportarServicios(d.ID, time.Now().UTC(), rs); err != nil {
			t.Fatal(err)
		}
	}

	// La máquina reporta su unit y sus dos contenedores.
	reportar("systemd", "sshd")
	reportar("podman", "vaultwarden", "musubi-prometheus")

	// Una corrida en la que `podman ps` falla: el inventario llega con la unit sola y la poda se
	// lleva los dos contenedores. Esto es lo que pasó de verdad.
	if n, err := e.PodarServiciosAusentes(d.ID, []string{"sshd"}); err != nil || n != 2 {
		t.Fatalf("la poda del caso: n=%d err=%v", n, err)
	}
	if vivos, _ := e.ServiciosDeDevice(d.ID); len(vivos) != 1 {
		t.Fatalf("después de la poda quedaron %d servicios, se esperaba 1", len(vivos))
	}

	// El latido siguiente ya ve podman de nuevo. Los dos tienen que volver.
	reportar("podman", "vaultwarden", "musubi-prometheus")
	vivos, err := e.ServiciosDeDevice(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(vivos) != 3 {
		t.Fatalf("volvieron %d servicios de 3: el inventario sólo sabe achicarse: %+v", len(vivos), vivos)
	}

	// Y vuelven ACTUALIZADOS, no como cáscaras: la clase es la que la máquina acaba de mandar.
	// En producción el síntoma iba con éste — las 18 filas tenían la clase en blanco, y sin la
	// resurrección el reporte que la traía bien tampoco podía escribirla.
	por := map[string]fleet.Servicio{}
	for _, s := range vivos {
		por[s.Nombre] = s
	}
	for _, n := range []string{"vaultwarden", "musubi-prometheus"} {
		if por[n].Clase != "podman" {
			t.Errorf("%s volvió con la clase %q en vez de podman", n, por[n].Clase)
		}
		if por[n].Salud == nil || por[n].EstadoActual() != fleet.EstadoCorriendo {
			t.Errorf("%s volvió sin la salud del reporte que lo resucitó: %+v", n, por[n])
		}
	}

	// Ni una fila de más: la resurrección es un UPDATE de la que ya estaba, no una segunda.
	todas, _ := e.ListarServicios("casa", "", true)
	if len(todas) != 3 {
		t.Fatalf("quedaron %d filas para 3 servicios: la resurrección duplicó", len(todas))
	}
}

// LA RESURRECCIÓN LLEGA HASTA DONDE EMPIEZA LA DECISIÓN DE UNA PERSONA.
//
// Es el borde exacto del arreglo de arriba, y sin esta prueba la forma más cómoda de hacerlo
// pasar —sacar el WHERE del UPDATE y listo— quedaría verde. Un servicio dado de alta a mano
// (`declared = 1`) que alguien revocó NO puede volver porque la máquina lo siga viendo: vuelve
// por `fleet_service_declare`, que es alguien decidiéndolo.
//
// Sabotaje que la hace fallar: cambiar el WHERE del UPDATE por `AND (revoked = 0 OR 1 = 1)`, o
// sea sacarle el `declared = 0` a la condición de resurrección.
func TestElServicioDeclaradoAManoNoResucitaPorqueLaMaquinaLoSigaViendo(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "nas")
	sv, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: d.ID, Clase: "systemd"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.RevocarServiciosDeDevice(d.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.ReportarServicios(d.ID, time.Now().UTC(), []fleet.ReporteServicio{
		{Nombre: "postgres", Clase: "systemd", Salud: saludDePrueba(fleet.EstadoCorriendo)}}); err != nil {
		t.Fatal(err)
	}
	if vivos, _ := e.ServiciosDeDevice(d.ID); len(vivos) != 0 {
		t.Fatalf("el servicio declarado a mano volvió por un reporte: %+v", vivos)
	}
	// Y tampoco recibió la medición por la puerta de atrás: una fila de baja que acumula
	// telemetría fresca es la que engaña al que la reactive meses después.
	var reporte, salud string
	if err := e.db.QueryRow(
		`SELECT COALESCE(last_report,''), last_health FROM services WHERE id = ?`, sv.ID).Scan(&reporte, &salud); err != nil {
		t.Fatal(err)
	}
	if reporte != "" || salud != "" {
		t.Errorf("la fila declarada y revocada siguió recibiendo telemetría: last_report=%q last_health=%q", reporte, salud)
	}
}
