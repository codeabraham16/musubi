package fleet

// politica_servicio_test.go custodia A44: las políticas que miran un SERVICIO en vez de una
// métrica del host.
//
// Es el plano de ACTUAR, así que sus modos de fallo no son «no se ve algo»: son «el cerebro
// ejecutó un comando que no correspondía», o «no lo ejecutó cuando sí».

import (
	"strings"
	"testing"
	"time"
)

func politicaDeServicio(nombre, servicio string, c Condicion) Politica {
	return Politica{
		Nombre: nombre, Principal: "curador", Cuando: c, Supera: 5,
		Sobre: []string{"*"}, Servicio: servicio,
		Hacer: []string{"systemctl", "restart", servicio}, Cooldown: 10 * time.Minute,
	}
}

// DOS SERVICIOS DE LA MISMA MÁQUINA NO COMPARTEN ENFRIAMIENTO.
//
// Es el bloqueo que A44 tenía anotado, y por qué la política no se podía escribir antes. El
// cooldown se llevaba por (política × máquina): con eso, reiniciar `nginx` DEJARÍA MUDA a la
// política de `postgres` durante todo el enfriamiento, y el segundo servicio se quedaría caído
// sin que nada actúe — justo por haber actuado sobre el primero.
//
// Peor que no tener la política: da la sensación de que algo vigila.
//
// Sabotaje que la hace fallar: sacar el servicio de ClaveDeCooldown.
func TestDosServiciosDeLaMismaMaquinaNoCompartenEnfriamiento(t *testing.T) {
	nginx := politicaDeServicio("revivir-nginx", "nginx", CondServicioCaido)
	postgres := politicaDeServicio("revivir-postgres", "postgres", CondServicioCaido)

	if nginx.ClaveDeCooldown("d1") == postgres.ClaveDeCooldown("d1") {
		t.Fatal("dos políticas sobre servicios distintos de la misma máquina comparten clave")
	}
	// Y la MISMA política sobre dos máquinas tampoco se cruza: eso ya andaba y no se puede
	// romper al agregar el servicio.
	if nginx.ClaveDeCooldown("d1") == nginx.ClaveDeCooldown("d2") {
		t.Error("la misma política sobre dos máquinas comparte clave: se perdió el aislamiento por máquina")
	}
	// Una política de HOST conserva su clave sin cola: el estado guardado de las que ya existen
	// sigue valiendo, así que agregar esto no rearma los cooldowns de toda la flota.
	host := Politica{Nombre: "purgar-disco", Cuando: CondDiscoPct}
	if got := host.ClaveDeCooldown("d1"); got != "purgar-disco\x00d1\x00" {
		t.Errorf("la clave de una política de host cambió de forma: %q", got)
	}
}

// UNA POLÍTICA DE SERVICIO EXIGE NOMBRAR EL SERVICIO, Y UNA DE HOST LO PROHÍBE.
//
// La primera mitad es de seguridad y está explicada en el dominio: no se admite «el que se haya
// caído» porque el nombre lo REPORTA LA MÁQUINA, y sustituirlo dentro de un argv haría que un
// dato no confiable termine siendo un argumento de un comando que el cerebro ejecuta — con la
// allowlist validada antes de saber qué se va a ejecutar de verdad.
//
// La segunda mitad no es tiquismiquis: una política de disco con un servicio declarado se lee
// como «vigilá el disco de ese servicio», que no es lo que hace y no existe. Aceptarlo dejaría a
// alguien creyendo que vigila algo distinto de lo que vigila.
//
// Sabotaje que la hace fallar: aceptar `servicio` vacío en una condición de servicio, o aceptarlo
// no vacío en una de host.
func TestLaCondicionDecideSiElServicioEsObligatorioOProhibido(t *testing.T) {
	sinServicio := politicaDeServicio("revivir", "", CondServicioCaido)
	sinServicio.Hacer = []string{"systemctl", "restart", "nginx"}
	err := sinServicio.Validar()
	if err == nil {
		t.Error("se aceptó una política de servicio sin nombrar el servicio: eso obliga a sustituirlo " +
			"en el comando, y ahí la allowlist deja de acotar nada")
	} else if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("el error no explica el riesgo real: %v", err)
	}

	conServicio := politicaDeServicio("revivir-nginx", "nginx", CondServicioCaido)
	if err := conServicio.Validar(); err != nil {
		t.Errorf("una política de servicio bien formada se rechazó: %v", err)
	}

	// Y al revés: una de host con `servicio` se rechaza.
	hostConServicio := Politica{
		Nombre: "purgar", Principal: "curador", Cuando: CondDiscoPct, Supera: 90,
		Sobre: []string{"*"}, Servicio: "nginx", Hacer: []string{"journalctl", "--vacuum-size=200M"},
	}
	if err := hostConServicio.Validar(); err == nil {
		t.Error("una política de disco aceptó un `servicio`: se lee como que vigila el disco de ese " +
			"servicio, que no es lo que hace")
	}
}

// EL NOMBRE DEL SERVICIO SE VALIDA, PORQUE TERMINA EN UN ARGV.
//
// Aunque no se sustituya en el comando, un nombre absurdo en la política produce una regla que no
// puede cumplirse nunca y que nadie va a notar: no dispara, y «no disparó» se lee como «no hizo
// falta».
//
// Sabotaje que la hace fallar: sacar la llamada a NombreDeServicioValido.
func TestElNombreDelServicioDeLaPoliticaSeValida(t *testing.T) {
	for _, malo := range []string{strings.Repeat("x", NombreServicioMax+10), "con espacio\ty tab", "\x00nulo"} {
		p := politicaDeServicio("revivir", malo, CondServicioCaido)
		p.Hacer = []string{"systemctl", "restart", "nginx"}
		if err := p.Validar(); err == nil {
			t.Errorf("se aceptó el nombre de servicio %q: la política nunca se cumpliría y nadie lo notaría", malo)
		}
	}
}

// `desconocido` NO ES `caído`, Y ESA ASIMETRÍA LLEGA HASTA EL PLANO DE ACTUAR.
//
// Es la regla que sostiene toda la tool de servicios, y acá es donde de verdad cuesta romperla:
// una máquina que no pudo ENUMERAR sus servicios no está diciendo que el postgres esté caído.
// Reiniciar algo por no haber podido mirarlo es exactamente el automatismo que nadie quiere —
// y el caso es real: un `systemctl` que falla por permisos, un agente recién arrancado.
//
// Sabotaje que la hace fallar: hacer que EstadoServicio desconocido cuente como caído.
func TestUnServicioDesconocidoNoDisparaUnaPolitica(t *testing.T) {
	casos := []struct {
		estado EstadoServicio
		cuenta bool
	}{
		{EstadoFallado, true},
		{EstadoDetenido, true},
		{EstadoCorriendo, false},
		{EstadoDesconocido, false},
		{EstadoServicio(""), false},
	}
	for _, c := range casos {
		t.Run(string(c.estado), func(t *testing.T) {
			if got := EstadoCuentaComoCaido(c.estado); got != c.cuenta {
				t.Errorf("%q cuenta como caído = %v, se esperaba %v", c.estado, got, c.cuenta)
			}
		})
	}
}

// NO SE ACTÚA SOBRE UN INVENTARIO VIEJO, Y ACÁ ES PEOR QUE CON UNA MUESTRA.
//
// I13 dice que una política no actúa sobre telemetría rancia: si la máquina dejó de reportar, el
// disco pudo haberse vaciado hace veinte minutos y actuar sería reaccionar a algo que ya no pasa.
//
// Con servicios el riesgo es MAYOR, y por un motivo que este mismo sistema creó: el agente deja de
// mandar el inventario cuando una fuente falla —a propósito, porque media lista da de baja lo que
// no trae—. O sea que «sin noticias» es un estado que producimos nosotros. Actuar sobre eso sería
// reiniciar servicios porque el agente no pudo enumerarlos.
//
// Sabotaje que la hace fallar: ignorar `fresco` en DisparaSobreServicio.
func TestUnaPoliticaNoActuaSobreUnInventarioViejo(t *testing.T) {
	pol := politicaDeServicio("revivir-nginx", "nginx", CondServicioCaido)
	caido := Servicio{Nombre: "nginx", Salud: &SaludServicio{Estado: EstadoFallado}}

	if v, dispara := pol.DisparaSobreServicio(caido, true); !dispara || v == nil {
		t.Fatal("un servicio caído y fresco no disparó")
	}
	if _, dispara := pol.DisparaSobreServicio(caido, false); dispara {
		t.Error("disparó sobre un inventario VIEJO: el agente deja de mandarlo cuando una fuente " +
			"falla, así que eso sería reiniciar servicios porque no se los pudo enumerar")
	}
	// Y un servicio REVOCADO tampoco dispara: alguien lo sacó del inventario, aunque su última
	// salud diga que se cayó. Actuar ahí es actuar sobre lo que se decidió que no importe.
	revocado := caido
	revocado.Revocado = true
	if _, dispara := pol.DisparaSobreServicio(revocado, true); dispara {
		t.Error("disparó sobre un servicio dado de baja")
	}
}

// AUSENTE NO ES CERO EN EL CONTADOR DE REINICIOS, Y EL SILENCIO SERÍA TOTAL.
//
// El SCM de Windows no sabe decir cuántas veces se reinició un servicio, y `podman ps` tampoco en
// runtimes viejos. Tratar ese nil como 0 haría que una política de reinicios NUNCA dispare en esas
// máquinas — y no habría error, ni serie, ni línea de log. Sería una política instalada, visible
// en el panel, que no vigila nada.
//
// Sabotaje que la hace fallar: usar 0 cuando Reinicios es nil.
func TestUnContadorDeReiniciosAusenteNoCuentaComoCero(t *testing.T) {
	pol := politicaDeServicio("nginx-a-los-tumbos", "nginx", CondServicioReinicios)
	pol.Supera = 5

	n := 9
	conDato := Servicio{Nombre: "nginx", Salud: &SaludServicio{Estado: EstadoCorriendo, Reinicios: &n}}
	if v, dispara := pol.DisparaSobreServicio(conDato, true); !dispara || v == nil || *v != 9 {
		t.Errorf("9 reinicios sobre un umbral de 5 no disparó: dispara=%v", dispara)
	}

	// EL VALOR VUELVE nil, Y ESO ES LO QUE DISTINGUE «no sé» DE «cero». Sin esa distinción, la
	// primera versión de esta prueba pasaba con el nil tratado como 0: para cualquier umbral
	// válido el resultado observable era idéntico, así que la prueba afirmaba guardar algo que el
	// código no hacía.
	sinDato := Servicio{Nombre: "nginx", Salud: &SaludServicio{Estado: EstadoCorriendo}}
	v, dispara := pol.DisparaSobreServicio(sinDato, true)
	if dispara {
		t.Error("disparó sin saber cuántos reinicios hubo")
	}
	if v != nil {
		t.Errorf("devolvió el valor %v sobre una plataforma que no sabe contarlos: «no sé» y «cero» "+
			"tienen que verse distinto para el panel, la métrica y el log del disparo", *v)
	}
	// Y no dispara por debajo del umbral, que es lo obvio y hay que sostener.
	pocos := 2
	bajo := Servicio{Nombre: "nginx", Salud: &SaludServicio{Estado: EstadoCorriendo, Reinicios: &pocos}}
	if _, dispara := pol.DisparaSobreServicio(bajo, true); dispara {
		t.Error("disparó con 2 reinicios sobre un umbral de 5")
	}

	// UN SERVICIO QUE ANDA PUEDE DISPARAR ESTA CONDICIÓN, y ése es todo su motivo de existir:
	// algo que su supervisor levanta cada treinta segundos está corriendo en cada mirada y no
	// está sano. `servicio_caido` no lo ve nunca.
	if _, dispara := pol.DisparaSobreServicio(conDato, true); !dispara {
		t.Error("un servicio corriendo con 9 reinicios no disparó: es justo el caso que " +
			"`servicio_caido` no puede ver")
	}
}

// UNA POLÍTICA DE HOST NO SE EVALÚA POR EL CAMINO DE SERVICIOS, NI AL REVÉS.
//
// Son dos evaluadores y cruzarlos daría un disparo silencioso: una política de disco pasada por
// DisparaSobreServicio devolvería «no dispara» siempre, y nadie lo notaría — la política estaría
// instalada y muda.
//
// Sabotaje que la hace fallar: agregar `case CondServicioCaido:` al switch de `Dispara` (el de
// métricas del host) mapeándolo a cualquier campo de la muestra.
//
// SOBRE LA GUARDA DE `EsDeServicio`: sacarla NO hace fallar esta prueba, y conviene decirlo en vez
// de anotar un sabotaje que no ocurre. La protección real son los DOS switches, que son
// exhaustivos: una condición de host cae al `default` del evaluador de servicios y viceversa. La
// guarda es cinturón y tirantes —hace explícita la intención— y esta prueba custodia lo que de
// verdad protege: que ninguna condición aparezca en los dos switches.
func TestLosDosEvaluadoresNoSeCruzan(t *testing.T) {
	host := Politica{Nombre: "purgar", Cuando: CondDiscoPct, Supera: 90}
	sv := Servicio{Nombre: "nginx", Salud: &SaludServicio{Estado: EstadoFallado}}
	if _, dispara := host.DisparaSobreServicio(sv, true); dispara {
		t.Error("una política de disco disparó por el camino de servicios")
	}
	// LA MUESTRA LLEVA VALORES ALTOS EN TODOS LOS CAMPOS A PROPÓSITO. Con una muestra a medio
	// llenar, meter una condición de servicio en el switch del host devuelve nil y no dispara —
	// así que la prueba pasaría por el vacío de la muestra y no por la separación de los dos
	// caminos. Es el mismo error que tuvo su primera versión.
	cpu := 99.0
	load := 40.0
	m := &Muestra{
		DiscoTotal: 100, DiscoUsado: 99, DiscoDisponible: 1,
		MemTotal: 100, MemUsada: 99,
		CPUPct: &cpu, Load5: &load, NumCPU: 1, TempC: &cpu,
	}
	deServicio := politicaDeServicio("revivir", "nginx", CondServicioCaido)
	if _, dispara := deServicio.Dispara(m); dispara {
		t.Error("una política de servicio disparó por el camino de métricas del host: alguna " +
			"condición de servicio se coló en el switch equivocado")
	}
	reinicios := politicaDeServicio("tumbos", "nginx", CondServicioReinicios)
	if _, dispara := reinicios.Dispara(m); dispara {
		t.Error("servicio_reinicios disparó por el camino de métricas del host")
	}
}
