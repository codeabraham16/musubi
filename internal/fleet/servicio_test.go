package fleet

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// «DESCONOCIDO» NO ES «DETENIDO», y el lugar donde eso se decide es UNO SOLO.
//
// Es el invariante que le da sentido al slice: una máquina que no pudo enumerar sus servicios no
// está afirmando que el postgres esté caído. Si EstadoActual devolviera `detenido` ante la
// ausencia de salud, un agente que arrancó a medias pintaría de rojo toda la flota y alguien
// saldría a arreglar algo que nunca se rompió.
//
// Sabotaje que la hace fallar: en EstadoActual, devolver EstadoDetenido cuando Salud es nil.
func TestUnServicioSinSaludEstaDesconocidoYNoDetenido(t *testing.T) {
	s := Servicio{Nombre: "postgres", Salud: nil}
	if got := s.EstadoActual(); got != EstadoDesconocido {
		t.Fatalf("un servicio sin salud informa %q, esperaba %q: «no sé» y «está caído» son cosas distintas", got, EstadoDesconocido)
	}
	if got := s.EstadoActual(); got == EstadoDetenido {
		t.Fatal("un servicio sin medir se informó como DETENIDO: eso despierta a alguien de madrugada por nada")
	}
	// Y con salud, se informa lo que la salud dice y no otra cosa.
	s.Salud = &SaludServicio{Estado: EstadoFallado, Tomada: time.Now()}
	if got := s.EstadoActual(); got != EstadoFallado {
		t.Fatalf("con salud se informó %q en vez de %q", got, EstadoFallado)
	}
}

// «DECLARADO Y TODAVÍA SIN MUESTRAS» ES UN ESTADO LEGÍTIMO Y DISTINTO DE «CAÍDO».
//
// Un servicio que alguien dio de alta a mano y que nadie midió todavía tiene que poder existir:
// UltimoReporte en cero, Salud nil, y ni un error en el camino. Si SaludDesdeTexto("") devolviera
// error, dar de alta un servicio y listarlo antes del primer latido rompería.
//
// Sabotaje: hacer que SaludDesdeTexto("") devuelva un error en vez de (nil, nil).
func TestUnServicioDeclaradoYSinMedirEsUnEstadoLegitimo(t *testing.T) {
	salud, err := SaludDesdeTexto("")
	if err != nil {
		t.Fatalf("una salud vacía dio error: %v — «todavía nadie lo midió» es el estado inicial, no una falla", err)
	}
	if salud != nil {
		t.Fatalf("una salud vacía devolvió %+v, esperaba nil", *salud)
	}
	s := Servicio{Nombre: "bot-telegram", Salud: salud}
	if s.Fresco(time.Now(), time.Minute) {
		t.Error("un servicio que nunca reportó se informó FRESCO")
	}
	if s.EstadoActual() != EstadoDesconocido {
		t.Errorf("un servicio declarado y sin medir informa %q", s.EstadoActual())
	}
}

// EL FRESCOR SE DERIVA, NO SE GUARDA — y con un reloj cero no se inventa vida.
//
// Gemela de TestEnLineaConRelojCeroNoInventaVida: `cero.Sub(cero)` es 0 y entra dentro de
// cualquier umbral, así que sin la guarda de UltimoReporte un servicio que nunca reportó se
// informaría fresco ante un reloj sin inicializar.
//
// Sabotaje: sacar `s.UltimoReporte.IsZero()` de Fresco.
func TestFrescoConRelojCeroNoInventaVida(t *testing.T) {
	nunca := Servicio{Nombre: "x"}
	if nunca.Fresco(time.Time{}, time.Minute) {
		t.Error("un servicio que nunca reportó, con un reloj cero, se informó fresco")
	}
	ahora := time.Now()
	reciente := Servicio{Nombre: "x", UltimoReporte: ahora.Add(-10 * time.Second)}
	if !reciente.Fresco(ahora, time.Minute) {
		t.Error("un reporte de hace 10 s con umbral de 1 min no se informó fresco")
	}
	viejo := Servicio{Nombre: "x", UltimoReporte: ahora.Add(-48 * time.Hour)}
	if viejo.Fresco(ahora, time.Minute) {
		t.Error("un reporte de hace dos días se informó fresco: «corriendo» y «sin noticias» son cosas distintas")
	}
	revocado := Servicio{Nombre: "x", UltimoReporte: ahora, Revocado: true}
	if revocado.Fresco(ahora, time.Minute) {
		t.Error("un servicio revocado se informó fresco")
	}
}

// UNA SALUD SIN `tomada` NO SE ACEPTA, y un pid 0 tampoco.
//
// Sin `tomada` no hay forma de distinguir un reporte de hace un minuto de uno de hace una semana,
// que es exactamente la pregunta que el panel hace. Y un pid 0 copiado crudo es el mismo cero
// mentiroso de siempre: un servicio detenido manda null.
//
// Sabotaje: sacar la comprobación de Tomada.IsZero() de Valida.
func TestUnaSaludSinFechaOConPidCeroSeRechaza(t *testing.T) {
	cero := 0
	casos := []struct {
		nombre string
		salud  SaludServicio
		quiero string
	}{
		{"sin tomada", SaludServicio{Estado: EstadoCorriendo}, "tomada"},
		{"pid cero", SaludServicio{Estado: EstadoCorriendo, Tomada: time.Now(), PID: &cero}, "pid"},
		{"estado inventado", SaludServicio{Estado: "sano", Tomada: time.Now()}, "estado"},
	}
	for _, c := range casos {
		err := c.salud.Valida()
		if err == nil {
			t.Errorf("%s: se aceptó una salud que no se puede dibujar honestamente", c.nombre)
			continue
		}
		if !strings.Contains(err.Error(), c.quiero) {
			t.Errorf("%s: el error no nombra %q: %v", c.nombre, c.quiero, err)
		}
	}
	// Y la salud completa y sensata pasa, incluidos los opcionales en nil.
	buena := SaludServicio{Estado: EstadoDesconocido, Tomada: time.Now()}
	if err := buena.Valida(); err != nil {
		t.Errorf("una salud `desconocido` sin pid ni reinicios se rechazó: %v", err)
	}
}

// EL REPORTE SE RECORTA, NO SE RECHAZA — y se recorta por RUNAS.
//
// Una unit con un nombre largo es una unit con un nombre largo, no un ataque: perder el servicio
// entero por eso sería peor que mostrarlo cortado. Y el corte va por runas porque el `Result=` de
// systemd trae acentos: cortar a la mitad de un carácter multibyte deja basura en una celda que
// después se dibuja.
//
// Sabotaje: recortar con s[:max] (bytes) en vez de por runas — el detalle queda con un byte
// suelto y deja de ser UTF-8 válido.
func TestElReporteSeRecortaPorRunasYNoSeRechaza(t *testing.T) {
	r := RecortarReporte(ReporteServicio{
		Nombre: strings.Repeat("ñ", NombreServicioMax+50),
		Clase:  "  SystemD ",
		Salud:  SaludServicio{Estado: EstadoCorriendo, Tomada: time.Now(), Detalle: strings.Repeat("á", DetalleServicioMax+50)},
	})
	if n := len([]rune(r.Nombre)); n != NombreServicioMax {
		t.Errorf("el nombre quedó en %d runas, esperaba %d", n, NombreServicioMax)
	}
	if n := len([]rune(r.Salud.Detalle)); n != DetalleServicioMax {
		t.Errorf("el detalle quedó en %d runas, esperaba %d", n, DetalleServicioMax)
	}
	// Lo recortado sigue siendo UTF-8 válido: ni un byte suelto.
	if !json.Valid(mustJSON(t, r)) {
		t.Error("el reporte recortado no serializa a JSON válido: se cortó a la mitad de una runa")
	}
	if r.Clase != "systemd" {
		t.Errorf("la clase no se normalizó: %q", r.Clase)
	}
	// Una clase que este binario no conoce se trata como AUSENTE, no como rechazo.
	if got := RecortarReporte(ReporteServicio{Nombre: "x", Clase: "kubernetes"}).Clase; got != "" {
		t.Errorf("una clase desconocida quedó como %q en vez de vaciarse", got)
	}
}

// EL REPORTE DE UNA MÁQUINA NO TIENE POR DÓNDE DECLARAR DE QUIÉN ES.
//
// Es B4/D5 llegando hasta este struct: la identidad sale del token y la tenencia de la fila del
// device. Si ReporteServicio tuviera un `device_id` o un `project`, la garantía pasaría a
// depender de que nadie lo lea — o sea, de disciplina.
//
// Sabotaje: agregarle a ReporteServicio un campo `DeviceID string \`json:"device_id"\“ — el
// barrido de claves de acá lo caza, y el de cmd/musubi/agent_test.go lo caza de nuevo.
func TestUnReporteDeServicioNoTienePorDondePasarIdentidad(t *testing.T) {
	crudo := mustJSON(t, ReporteServicio{Nombre: "postgres", Salud: SaludServicio{Estado: EstadoCorriendo, Tomada: time.Now()}})
	var claves map[string]any
	if err := json.Unmarshal(crudo, &claves); err != nil {
		t.Fatalf("el reporte no es JSON: %v", err)
	}
	permitidas := map[string]bool{"nombre": true, "clase": true, "salud": true}
	for k := range claves {
		if !permitidas[k] {
			t.Errorf("el reporte trae la clave %q: si es legítima, sumala acá DESPUÉS de convencerte de que no es identidad", k)
		}
	}
	for _, prohibido := range []string{"device_id", "project", "token", `"name"`, "hostname"} {
		if strings.Contains(string(crudo), prohibido) {
			t.Errorf("el reporte menciona %q: la identidad no puede viajar en el cuerpo\n%s", prohibido, crudo)
		}
	}
}

// EL ALTA ES FAIL-CLOSED: sin proyecto, sin device o con una clase inventada, no hay servicio.
//
// El proyecto merece un párrafo: en un servicio NO lo declara nadie, se copia del device. Llegar
// acá sin él significa que el device no se resolvió — y una fila sin project_id se ve desde todos
// los tenants, que es la falla A6 con otra ropa.
//
// Sabotaje: sacar la comprobación de ProjectID vacío de ValidarAltaServicio.
func TestElAltaDeUnServicioEsFailClosed(t *testing.T) {
	base := Servicio{Nombre: "postgres", ProjectID: "casa", DeviceID: "dev-1", Clase: "systemd"}
	if err := ValidarAltaServicio(base); err != nil {
		t.Fatalf("un alta completa se rechazó: %v", err)
	}
	casos := map[string]Servicio{
		"sin nombre":         {ProjectID: "casa", DeviceID: "dev-1"},
		"sin proyecto":       {Nombre: "postgres", DeviceID: "dev-1"},
		"sin device":         {Nombre: "postgres", ProjectID: "casa"},
		"clase inventada":    {Nombre: "postgres", ProjectID: "casa", DeviceID: "dev-1", Clase: "kubernetes"},
		"nombre con saltos":  {Nombre: "postgres\nfalso", ProjectID: "casa", DeviceID: "dev-1"},
		"nombre kilométrico": {Nombre: strings.Repeat("x", NombreServicioMax+1), ProjectID: "casa", DeviceID: "dev-1"},
	}
	for nombre, s := range casos {
		if err := ValidarAltaServicio(s); err == nil {
			t.Errorf("%s: se dio de alta igual", nombre)
		}
	}
	// La clase VACÍA sí es válida: «no se declaró» es distinto de «se inventó una».
	sinClase := base
	sinClase.Clase = ""
	if err := ValidarAltaServicio(sinClase); err != nil {
		t.Errorf("una clase vacía se rechazó: %v — un Tier B declarado a mano no siempre sabe cuál es", err)
	}
}

// EL SERVICIO NO TIENE CAMPO DE ESTADO GUARDADO, y esta prueba de FORMA custodia esa ausencia.
//
// Es la gemela en el dominio de la prueba de esquema sobre la tabla. Un booleano `sano` en la
// struct es el primer paso para que alguien lo persista, y un booleano persistido se queda en
// true para siempre cuando la cosa muere de golpe.
//
// Sabotaje: agregarle a Servicio un campo `Sano bool` (o `Activo`, `Up`, `Online`, `Healthy`).
func TestElServicioNoGuardaUnEstadoDerivable(t *testing.T) {
	crudo := mustJSON(t, Servicio{Nombre: "x"})
	for _, prohibido := range []string{"sano", "activo", "up", "online", "healthy", "status"} {
		if strings.Contains(strings.ToLower(string(crudo)), `"`+prohibido+`"`) {
			t.Errorf("fleet.Servicio expone un campo %q: el estado se DERIVA al leer (EstadoActual/Fresco), no se guarda", prohibido)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("no se pudo serializar: %v", err)
	}
	return b
}

// TestLasClasesCubrenLoQueLosEnumeradoresEMITEN — la guarda del defecto que se vio desplegando.
//
// El agente enumera contenedores de podman y servicios de launchd, y el cerebro le vaciaba la
// clase en silencio porque no estaban en el enum. No daba error: dejaba 18 filas correctas con
// una columna en blanco, indistinguibles de las que de verdad no saben decir quién las corre.
//
// Esta prueba ata el enum a lo que los enumeradores producen. Si alguien agrega un enumerador
// nuevo y olvida la clase, se pone roja acá en vez de perderse el dato en producción.
//
// Sabotaje que la hace fallar: sacar "podman" de clasesConocidas.
func TestLasClasesCubrenLoQueLosEnumeradoresEmiten(t *testing.T) {
	// Cada una la emite un enumerador de cmd/musubi: systemd y podman/docker en Linux,
	// windows en Windows, launchd en macOS.
	for _, c := range []string{"systemd", "podman", "docker", "windows", "launchd"} {
		if !ClaseValida(c) {
			t.Errorf("la clase %q la emite un enumerador y el cerebro la descarta: la fila se guarda igual, con la columna en blanco y sin un solo error", c)
		}
	}
	// Y sigue siendo un ENUM: texto libre agruparía mal el día que alguien escriba «Systemd».
	for _, c := range []string{"cualquier-cosa", "Systemd ", "supervisord"} {
		if ClaseValida(c) && strings.TrimSpace(strings.ToLower(c)) != "systemd" {
			t.Errorf("la clase %q se aceptó: dejó de ser un enum acotado", c)
		}
	}
}

// TestElUmbralDeFrescuraAguantaElRitmoDelInventario — la guarda del defecto que se vio en la
// primera fila que devolvió producción.
//
// El agente reenvía el inventario cada `InventarioCada`. El cerebro marca un servicio como no
// fresco pasado `UmbralInventario`. Si el umbral no le gana holgadamente al piso, TODO servicio
// se lee viejo para siempre — y un `fresco: false` permanente no es una alarma, es ruido de fondo
// que enseña a ignorar la columna. Fue exactamente lo que pasó: 90 s de umbral contra 5 min de
// piso, y las 54 filas de musubi-server salieron todas con `fresco: false`.
//
// Sabotaje que la hace fallar: poner UmbralInventario = InventarioCada.
func TestElUmbralDeFrescuraAguantaElRitmoDelInventario(t *testing.T) {
	if UmbralInventario <= InventarioCada {
		t.Fatalf("UmbralInventario (%s) no le gana al piso de reenvío (%s): todo servicio se leería viejo para siempre",
			UmbralInventario, InventarioCada)
	}
	// Tiene que aguantar UN reenvío perdido. Un latido que no llegó o un reinicio del agente no
	// pueden marcar la flota entera como vieja.
	if UmbralInventario < 2*InventarioCada {
		t.Errorf("UmbralInventario (%s) no aguanta un reenvío perdido (haría falta %s): un solo latido caído marcaría todo viejo",
			UmbralInventario, 2*InventarioCada)
	}

	// Y el caso concreto: un servicio que reportó hace un intervalo SIGUE fresco.
	sv := Servicio{UltimoReporte: time.Now().Add(-InventarioCada - 30*time.Second)}
	if !sv.Fresco(time.Now(), UmbralInventario) {
		t.Error("un servicio que reportó hace poco más de un intervalo ya se lee viejo")
	}
	// Pero uno abandonado, no.
	viejo := Servicio{UltimoReporte: time.Now().Add(-3 * UmbralInventario)}
	if viejo.Fresco(time.Now(), UmbralInventario) {
		t.Error("un servicio que no reporta hace media hora se sigue leyendo fresco: el `fresco` dejó de separar «corriendo» de «lo último que supimos»")
	}
}
