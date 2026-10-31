package motor

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
)

// Sabotaje C1: sin proveedor devuelve un motor igual (prende lo que nadie pidio)
// Se pisa con C4 porque C4 necesita arrastrar estas dos líneas para que su `switch` sea único, no
// porque mida lo mismo: C4 mide la NORMALIZACIÓN del provider.
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\tcase \"\":\n\t\treturn nil, nil"
// arnes: a="\tcase \"\":\n\t\treturn &Lanzador{}, nil"
// arnes: colision_ok="TestElProveedorSeLeeSinImportarMayusculasNiEspacios"
func TestSinProveedorNoHayMotorYTampocoHayError(t *testing.T) {
	// 🔴 EL ESTADO DE TODA INSTALACIÓN DE HOY. El nil no es un error ni un motor que falla:
	// es la ausencia, y el llamador la lee como «esta ruta no está disponible». Si esto
	// devolviera un error, levantar el servicio fallaría en cada máquina que no configuró
	// un motor — o sea en todas.
	m, err := Desde(config.MotorConfig{})
	if err != nil {
		t.Fatalf("sin proveedor: err = %v, quiero nil", err)
	}
	if m != nil {
		t.Errorf("sin proveedor: motor = %T, quiero nil", m)
	}
}

// Sabotaje C2: un provider mal escrito se apaga en silencio en vez de gritar
// Se pisa con C3, que sabotea el `fmt.Errorf` de adentro, y son dos cosas distintas: ésta mide que
// falle RUIDOSAMENTE, C3 que el error sea ÚTIL (que nombre los válidos).
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\tdefault:\n\t\t// EL ERROR NOMBRA LOS VÁLIDOS, y es la diferencia entre una sesión de diagnóstico y\n\t\t// una corrección de diez segundos. Alguien que escribió \"claude\" o \"anthropic\" quiso\n\t\t// prender el motor; decirle sólo «proveedor desconocido» lo deja adivinando.\n\t\treturn nil, fmt.Errorf(\"motor.provider=%q desconocido: los válidos son %q y %q (o vacío para dejarlo apagado)\",\n\t\t\tcfg.Provider, ProveedorCLI, ProveedorAPI)"
// arnes: a="\tdefault:\n\t\treturn nil, nil"
// arnes: colision_ok="TestUnProveedorMalEscritoEsUnErrorQueNombraLosValidos"
//
// Sabotaje C3: el error del provider desconocido no nombra los validos
// Se pisa con C2, que sabotea el `default:` entero: ver su ancla. Ésta mide sólo el TEXTO.
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\treturn nil, fmt.Errorf(\"motor.provider=%q desconocido: los válidos son %q y %q (o vacío para dejarlo apagado)\",\n\t\t\tcfg.Provider, ProveedorCLI, ProveedorAPI)"
// arnes: a="\t\treturn nil, fmt.Errorf(\"proveedor %q desconocido\", cfg.Provider)"
// arnes: colision_ok="TestUnProveedorMalEscritoEsUnErrorQueNombraLosValidos"
func TestUnProveedorMalEscritoEsUnErrorQueNombraLosValidos(t *testing.T) {
	// Alguien que escribió "claude" QUISO prender el motor. Apagarlo en silencio le deja un
	// 405 sin ninguna pista, y el yaml «ya tiene un motor» — es la forma más barata de perder
	// una tarde.
	for _, prov := range []string{"claude", "anthropic", "lanzador", "cli-api"} {
		_, err := Desde(config.MotorConfig{Provider: prov})
		if err == nil {
			t.Errorf("provider=%q: pasó sin error", prov)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, prov) {
			t.Errorf("provider=%q: el error no repite lo que se escribió: %s", prov, msg)
		}
		// Esto es lo que convierte una sesión de diagnóstico en una corrección de diez
		// segundos, y es lo que un «proveedor desconocido» pelado no hace.
		if !strings.Contains(msg, `"cli"`) || !strings.Contains(msg, `"api"`) {
			t.Errorf("provider=%q: el error no nombra los válidos: %s", prov, msg)
		}
	}
}

// Sabotaje C4: el provider no se normaliza (un espacio lo rompe)
// Arrastra las dos líneas del `case ""` sólo para que el `switch` sea único, no porque mida eso.
// NO lleva `colision_ok` y la asimetría es el dato: aplicar este sabotaje CONSERVA intactas las
// dos líneas del `case ""`, así que no rompe el `de` de C1 y nadie lo denuncia. La deuda es de la
// otra dirección —C1 sí borra el `return nil, nil` que este `de` necesita— y por eso la respuesta
// vive en el ancla de C1. Declararla también acá fue un «ya lo miré» sobre algo que nadie miró,
// y la guarda del censo lo cazó como rancio.
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\tswitch prov := strings.ToLower(strings.TrimSpace(cfg.Provider)); prov {\n\tcase \"\":\n\t\treturn nil, nil"
// arnes: a="\tswitch prov := cfg.Provider; prov {\n\tcase \"\":\n\t\treturn nil, nil"
func TestElProveedorSeLeeSinImportarMayusculasNiEspacios(t *testing.T) {
	// Esto lo escribe una persona a mano en un yaml. Rechazar "  CLI " sería pedantería que
	// cuesta una sesión de diagnóstico por un espacio que nadie ve.
	for _, prov := range []string{"cli", "CLI", "  Cli  ", "\tcli\n"} {
		m, err := Desde(config.MotorConfig{Provider: prov})
		if err != nil {
			t.Errorf("provider=%q: %v", prov, err)
			continue
		}
		if _, ok := m.(*Lanzador); !ok {
			t.Errorf("provider=%q: motor = %T, quiero *Lanzador", prov, m)
		}
	}
}

// Sabotaje C5: KeepAPIKey no llega al Lanzador (factura por API sin que se note)
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\t\tConservarAPIKey: cfg.KeepAPIKey,"
// arnes: a=""
func TestElProveedorCLIRecibeLoQueDiceElYaml(t *testing.T) {
	m, err := Desde(config.MotorConfig{
		Provider:           "cli",
		Binary:             "/opt/claude",
		TurnTimeoutSeconds: 120,
		KeepAPIKey:         true,
		SessionVersion:     "9.9.9",
	})
	if err != nil {
		t.Fatal(err)
	}
	l, ok := m.(*Lanzador)
	if !ok {
		t.Fatalf("motor = %T, quiero *Lanzador", m)
	}
	if l.Binario != "/opt/claude" {
		t.Errorf("Binario = %q", l.Binario)
	}
	if l.Espera != 120*time.Second {
		t.Errorf("Espera = %s, quiero 2m", l.Espera)
	}
	// KeepAPIKey no es cosmético: el sentido de `cli` es facturar con la suscripción, y una
	// API key que se filtra al hijo la hace facturar por API sin que se note desde el
	// cliente. Que el campo llegue tiene que estar medido.
	if !l.ConservarAPIKey {
		t.Error("ConservarAPIKey = false, y el yaml pedía true")
	}
	if l.VersionDeSesion != "9.9.9" {
		t.Errorf("VersionDeSesion = %q", l.VersionDeSesion)
	}
}

// Sabotaje C6: el tope en cero se convierte a un numero propio
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\t\tEspera:          time.Duration(cfg.TurnTimeoutSeconds) * time.Second,"
// arnes: a="\t\t\tEspera:          time.Duration(cfg.TurnTimeoutSeconds+30) * time.Second,"
func TestUnTopeDeTurnoEnCeroQuedaEnCeroParaQueGaneElDefault(t *testing.T) {
	// 🔴 LA TRAMPA DE LA CONVERSIÓN. `turn_timeout_seconds` ausente es 0, y el Lanzador lee
	// el 0 como su default de diez minutos. Si la fábrica «ayudara» poniendo un número
	// propio, el default dejaría de estar en un solo lugar; y si convirtiera mal, un turno
	// con thinking largo moriría antes de la primera palabra con un timeout que nadie pidió.
	m, err := Desde(config.MotorConfig{Provider: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	l := m.(*Lanzador)
	if l.Espera != 0 {
		t.Errorf("Espera = %s, quiero 0 para que el Lanzador aplique DemoraPorDefecto (%s)", l.Espera, DemoraPorDefecto)
	}
}

// Sabotaje C7: la credencial no se verifica al construir (falla en el primer turno)
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\tif c.clave() == \"\" {\n\t\t\treturn nil, fmt.Errorf(\n\t\t\t\t\"motor.provider=%q pero la variable de entorno %s está vacía: sin credencial no hay turno que servir\",\n\t\t\t\tProveedorAPI, c.nombreDeLaVariable())\n\t\t}"
// arnes: a=""
func TestElProveedorAPISinCredencialFallaAlConstruir(t *testing.T) {
	// 🔴 FAIL-CLOSED, Y EL MOMENTO ES EL INVARIANTE. Un motor que arranca «bien» y se cae
	// recién cuando alguien le pide un turno convierte un error de configuración en un error
	// de runtime, y el que lo ve es el cliente remoto, que no puede hacer nada. Acá lo ve
	// quien editó el yaml, al levantar el servicio.
	t.Setenv("MOTOR_CLAVE_QUE_NO_EXISTE", "")
	_, err := Desde(config.MotorConfig{Provider: "api", APIKeyEnv: "MOTOR_CLAVE_QUE_NO_EXISTE"})
	if err == nil {
		t.Fatal("el motor api se construyó sin credencial")
	}
	// El error nombra la VARIABLE, nunca el valor: es lo único que hace falta para
	// diagnosticar y lo único que se puede imprimir sin filtrar el secreto.
	if !strings.Contains(err.Error(), "MOTOR_CLAVE_QUE_NO_EXISTE") {
		t.Errorf("el error no nombra la variable: %s", err)
	}
}

func TestElProveedorAPIConCredencialRecibeLoQueDiceElYaml(t *testing.T) {
	t.Setenv("MOTOR_CLAVE_DE_PRUEBA", "sk-de-prueba")
	m, err := Desde(config.MotorConfig{
		Provider:   "api",
		Endpoint:   "http://127.0.0.1:9/v1/messages",
		APIKeyEnv:  "MOTOR_CLAVE_DE_PRUEBA",
		APIVersion: "2099-01-01",
		Betas:      []string{"una-beta"},
	})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := m.(*ClienteAPI)
	if !ok {
		t.Fatalf("motor = %T, quiero *ClienteAPI", m)
	}
	if c.Endpoint != "http://127.0.0.1:9/v1/messages" || c.ClaveEnv != "MOTOR_CLAVE_DE_PRUEBA" {
		t.Errorf("ClienteAPI = %+v", c)
	}
	if c.Version != "2099-01-01" || len(c.Betas) != 1 || c.Betas[0] != "una-beta" {
		t.Errorf("Version = %q, Betas = %v", c.Version, c.Betas)
	}
	// La clave NO se copia al struct: se lee del entorno en cada turno. Guardarla en un
	// campo la pondría en cualquier volcado del struct, y no hace falta.
	if c.Clave != "" {
		t.Errorf("Clave = %q, y no debería copiarse al struct", c.Clave)
	}
}

func TestElProveedorAPIUsaLaVariableDeAnthropicCuandoNoSeDeclaraOtra(t *testing.T) {
	// El default tiene que estar en UN lugar (el método del ClienteAPI), no repetido en la
	// fábrica: dos copias del nombre de una variable de entorno se separan sin que nadie lo
	// note, y el síntoma es un motor que no encuentra una credencial que sí está puesta.
	t.Setenv("ANTHROPIC_API_KEY", "sk-del-entorno")
	m, err := Desde(config.MotorConfig{Provider: "api"})
	if err != nil {
		t.Fatalf("con ANTHROPIC_API_KEY puesta: %v", err)
	}
	if _, ok := m.(*ClienteAPI); !ok {
		t.Errorf("motor = %T, quiero *ClienteAPI", m)
	}
}

// Sabotaje C8: DescribirMotor imprime el VALOR de la credencial
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\tcargada := \"VACÍA\"\n\t\tif os.Getenv(variable) != \"\" {\n\t\t\tcargada = \"cargada\"\n\t\t}"
// arnes: a="\t\tcargada := os.Getenv(variable)"
func TestLaDescripcionDelMotorNuncaImprimeElSecreto(t *testing.T) {
	// El síntoma de un motor mal configurado es un 405 silencioso, así que esta línea existe
	// para hacerlo averiguable. Y por eso mismo es un lugar donde un secreto se filtraría al
	// log de arranque: dice si la variable TIENE algo, no qué tiene.
	t.Setenv("MOTOR_CLAVE_DE_PRUEBA", "sk-secreto-no-imprimir")
	d := DescribirMotor(config.MotorConfig{Provider: "api", APIKeyEnv: "MOTOR_CLAVE_DE_PRUEBA"})
	if strings.Contains(d, "sk-secreto-no-imprimir") {
		t.Fatalf("la descripción FILTRÓ el secreto: %s", d)
	}
	if !strings.Contains(d, "MOTOR_CLAVE_DE_PRUEBA") || !strings.Contains(d, "cargada") {
		t.Errorf("la descripción no dice qué variable ni si está cargada: %s", d)
	}

	// Y el caso que importa para diagnosticar: la variable vacía se declara VACÍA.
	t.Setenv("MOTOR_CLAVE_VACIA", "")
	if d := DescribirMotor(config.MotorConfig{Provider: "api", APIKeyEnv: "MOTOR_CLAVE_VACIA"}); !strings.Contains(d, "VACÍA") {
		t.Errorf("con la variable vacía la descripción no lo dice: %s", d)
	}
}

// Sabotaje C9: la descripcion del motor apagado no explica el 405
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\treturn \"motor: apagado (motor.provider vacío; el turno contesta 405)\""
// arnes: a="\t\treturn \"motor: apagado\""
func TestLaDescripcionDelMotorApagadoExplicaEl405(t *testing.T) {
	// Es el estado por defecto, así que es el que más se va a leer. Tiene que decir las dos
	// cosas: que está apagado y cuál es el síntoma que se ve del otro lado.
	d := DescribirMotor(config.MotorConfig{})
	if !strings.Contains(d, "apagado") || !strings.Contains(d, "405") {
		t.Errorf("la descripción del motor apagado = %q", d)
	}

	// Y un provider mal escrito tiene que distinguirse de un apagado, porque la corrección
	// es distinta: uno hay que prenderlo, el otro hay que escribirlo bien.
	mal := DescribirMotor(config.MotorConfig{Provider: "claude"})
	if !strings.Contains(mal, "desconocido") || !strings.Contains(mal, "claude") {
		t.Errorf("la descripción de un provider mal escrito = %q", mal)
	}
}

func TestLaFabricaEntregaLosDosProveedoresSinError(t *testing.T) {
	// ESTE TEST NO MIDE QUE LOS MOTORES CUMPLAN LA INTERFAZ, aunque así se llamaba. `Desde`
	// DEVUELVE `(Motor, error)`, así que eso lo garantiza el compilador por la firma y ningún
	// test puede agregarle nada. Acá había un `var _ Motor = m` puesto para custodiarlo que no
	// custodiaba nada —`m` YA era de tipo `Motor`—, y el comentario de arriba afirmaba justo lo
	// contrario: que las aserciones de cada archivo «no cubren que la fábrica los devuelva por
	// la interfaz». Sí lo cubren, en tiempo de compilación. Lo delató `golangci-lint` con
	// QF1011 el 2026-10-31, y lo que se borró es una guarda vacua, no una aserción útil.
	//
	// Lo que SÍ queda medido, y no es gratis: que para los dos providers válidos la fábrica
	// devuelva sin error y con un motor NO NIL. Un `(nil, nil)` compila, respeta la firma y
	// revienta recién en el primer turno, que es donde más caro sale.
	t.Setenv("ANTHROPIC_API_KEY", "sk-de-prueba")
	for _, prov := range []string{ProveedorCLI, ProveedorAPI} {
		m, err := Desde(config.MotorConfig{Provider: prov})
		if err != nil {
			t.Fatalf("provider=%q: %v", prov, err)
		}
		if m == nil {
			t.Errorf("provider=%q: motor nil sin error", prov)
		}
	}
}

// EL DIAGNÓSTICO DICE EL TOPE QUE RIGE, NO EL QUE ESTÁ ESCRITO EN EL YAML. Esta línea existe
// porque el síntoma de un motor mal configurado es un 405 silencioso, así que es la única
// pista que alguien va a leer en el arranque. Anunciar un número que el turno no va a usar la
// convierte en una pista FALSA: con `turn_timeout_seconds: -1` el Lanzador cae al defecto, y
// una descripción que dice «-1s» manda a buscar el problema al lugar equivocado.
//
// Sabotaje M40: la fabrica no resuelve el plazo y anuncia el crudo del yaml
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\tespera := plazoDelTurno(time.Duration(cfg.TurnTimeoutSeconds) * time.Second)"
// arnes: a="\t\tespera := time.Duration(cfg.TurnTimeoutSeconds) * time.Second"
func TestElDiagnosticoInformaElTopeQueRigeYNoElDelYaml(t *testing.T) {
	d := DescribirMotor(config.MotorConfig{Provider: "cli", TurnTimeoutSeconds: -1})
	if strings.Contains(d, "-1s") {
		t.Fatalf("la descripción anuncia el tope del yaml y no el que rige: %s", d)
	}
	if !strings.Contains(d, DemoraPorDefecto.String()) {
		t.Errorf("con un tope negativo la descripción no nombra el defecto (%s): %s", DemoraPorDefecto, d)
	}
}

// EL DIAGNÓSTICO NOMBRA EL TOPE POSITIVO QUE SE CONFIGURÓ, que es la otra mitad de lo que el
// test de arriba promete en su nombre. Con el tope negativo alcanza para ver que no se filtra el
// crudo del yaml, pero NO para distinguir «informa el tope que rige» de «informa siempre el
// defecto»: medido el 2026-10-29, un cuerpo que ignorara la configuración y escribiera el
// defecto a secas pasaba el custodio del negativo con el paquete entero en verde, y un
// `turn_timeout_seconds: 120` se anunciaba como 10m0s. La pista del arranque es lo único que un
// operador tiene para saber qué plazo rige, así que mentir ahí es peor que no imprimir nada.
//
// Sabotaje M45: el diagnostico pisa el plazo configurado y anuncia siempre el defecto
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\treturn fmt.Sprintf(\"motor: cli (binario %s, tope por turno %s, factura la suscripción)\", bin, espera)"
// arnes: a="\t\tespera = DemoraPorDefecto\n\t\treturn fmt.Sprintf(\"motor: cli (binario %s, tope por turno %s, factura la suscripción)\", bin, espera)"
func TestElDiagnosticoNombraElTopePositivoConfigurado(t *testing.T) {
	d := DescribirMotor(config.MotorConfig{Provider: "cli", TurnTimeoutSeconds: 120})
	if !strings.Contains(d, "2m0s") {
		t.Errorf("con turn_timeout_seconds: 120 la descripción no nombra 2m0s: %s", d)
	}
	if strings.Contains(d, DemoraPorDefecto.String()) {
		t.Errorf("con un tope configurado la descripción anuncia el defecto (%s): %s", DemoraPorDefecto, d)
	}
}

// Y NO FILTRA EL CRUDO DEL YAML EN NINGÚN FORMATO. El test del tope negativo busca la subcadena
// "-1s", que es el `time.Duration.String()` del valor: eso ata la guarda al FORMATO, así que el
// mismo valor impreso como "-1" o como "-1000ms" pasaría limpio. La aserción tiene que decir «no
// anuncia el valor del yaml», no «no contiene esos tres caracteres».
//
// Sabotaje M46: el diagnostico agrega el crudo del yaml al binario, en otro formato que -1s
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\treturn fmt.Sprintf(\"motor: cli (binario %s, tope por turno %s, factura la suscripción)\", bin, espera)"
// arnes: a="\t\tbin += fmt.Sprintf(\" [yaml=%d]\", cfg.TurnTimeoutSeconds)\n\t\treturn fmt.Sprintf(\"motor: cli (binario %s, tope por turno %s, factura la suscripción)\", bin, espera)"
func TestElDiagnosticoNoFiltraElCrudoDelYamlEnNingunFormato(t *testing.T) {
	d := DescribirMotor(config.MotorConfig{Provider: "cli", TurnTimeoutSeconds: -1})
	if strings.Contains(d, "-1") {
		t.Errorf("la descripción anuncia el -1 del yaml en algún formato: %s", d)
	}
}

// ── EL BORDE DE ARRIBA: UN NÚMERO QUE ENVUELVE Y SIGUE PARECIENDO UN PLAZO ────────────────────
//
// El borde de abajo —el cero y el negativo— lo cierra `plazoDelTurno` y lo miden los tests de más
// arriba. El de arriba no lo cerraba nadie, y la doc de TopeDelPlazo tiene los dos modos con sus
// números medidos. Lo que miden estos tres es que la guarda EXISTA (M47), que ALCANCE (M48) y que
// el arranque no mienta cuando dispara (M49).

// Sabotaje M47: la guarda del borde de arriba no mira nada y el desborde pasa
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="\treturn seg > int(TopeDelPlazo/time.Second)"
// arnes: a="\treturn false"
func TestUnPlazoQueDesbordaNoConstruyeMotor(t *testing.T) {
	// 🔴 LOS DOS VALORES ENTRAN POR `plazoDelTurno` COMO SI FUERAN PLAZOS LEGÍTIMOS —a los dos
	// `d > 0` les da true—, y es la razón entera de que la guarda viva en la fábrica y mire el
	// ENTERO: aguas abajo el desborde ya pasó y es indistinguible de un plazo que alguien quiso.
	casos := []struct {
		nombre  string
		seg     int
		sintoma string
	}{
		{
			nombre:  "la suma del idle desborda y el listener del espejo no vence nunca",
			seg:     9223372036,
			sintoma: "espera+margenDelIdle queda negativo, y net/http no pone deadline (server.go:2113 pide d > 0)",
		},
		{
			nombre:  "la multiplicacion envuelve y deja un tope de 290ms",
			seg:     18446744074,
			sintoma: "time.Duration(seg)*time.Second da +290.448384ms, positivo y plausible",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			// Se mide primero que el caso SIGA SIENDO el caso. Si un día `plazoDelTurno`
			// aprendiera a cazar esto, el test pasaría por el motivo equivocado y nadie se
			// enteraría de que mide otra cosa.
			if d := time.Duration(c.seg) * time.Second; plazoDelTurno(d) != d {
				t.Fatalf("el caso dejó de ser el caso: plazoDelTurno ya resuelve %d (%s)", c.seg, c.sintoma)
			}
			m, err := Desde(config.MotorConfig{Provider: "cli", TurnTimeoutSeconds: c.seg})
			if err == nil {
				t.Fatalf("Desde aceptó turn_timeout_seconds=%d: %s", c.seg, c.sintoma)
			}
			if m != nil {
				t.Errorf("Desde devolvió un motor Y un error con turn_timeout_seconds=%d", c.seg)
			}
			// EL ERROR NOMBRA EL CAMPO Y EL TOPE, por lo mismo que el de provider nombra los
			// válidos: sin eso, quien editó el yaml queda adivinando qué línea corregir.
			if !strings.Contains(err.Error(), "turn_timeout_seconds") {
				t.Errorf("el error no nombra el campo del yaml: %v", err)
			}
			if !strings.Contains(err.Error(), TopeDelPlazo.String()) {
				t.Errorf("el error no nombra el tope (%s): %v", TopeDelPlazo, err)
			}
		})
	}
}

// 🔴 EL TOPE TIENE QUE ALCANZAR, NO SÓLO EXISTIR, y es la lección que margenDelIdle ya pagó una
// vez: una aserción escrita en términos de la constante que vigila se mueve con ella y deja de
// medir. Subir TopeDelPlazo hasta un valor que vuelve a desbordar es exactamente el cambio que
// alguien apurado haría para «no limitarle el turno al usuario», y con una guarda que sólo pide
// que la constante exista, ese cambio entra en verde.
//
// ⚠️ Y SU SABOTAJE TAMBIÉN DEJA ROJO AL CUSTODIO DE M47, sin que eso sea una colisión declarable:
// `colision_ok` contesta el aviso TEXTUAL —aplicar un sabotaje rompe el ancla `de` de otro—, y acá
// las dos anclas son líneas distintas. Lo que se pisa es el COMPORTAMIENTO: con el tope en
// 9223372036 segundos, `plazoFueraDeRango(9223372036)` pasa a ser falso y el primer caso de M47
// deja de ser rechazado. Son dos guardas con motivos distintos sobre la misma constante, y vale
// saberlo al leer un rojo doble: el de M47 sería un daño colateral, no una segunda medición.
//
// Sabotaje M48: el tope sube a un valor que vuelve a desbordar la suma del idle
// arnes: archivo="internal/motor/lanzador.go"
// arnes: de="const TopeDelPlazo = 24 * time.Hour"
// arnes: a="const TopeDelPlazo = 9223372036 * time.Second"
func TestElTopeDelPlazoAlcanzaYNoSoloExiste(t *testing.T) {
	// (a) LO QUE EL ESPEJO HACE CON ÉL. `IdleTimeout: espera + margenDelIdle` es la suma que
	// desbordaba; con el máximo que la fábrica acepta todavía tiene que CRECER.
	//
	// ⚠️ LA SUMA PASA POR UNA VARIABLE, Y NO ES ESTILO. Escrita `TopeDelPlazo+margenDelIdle` es
	// una EXPRESIÓN CONSTANTE, y entonces esta aserción NO PUEDE FALLAR NUNCA: cualquier tope que
	// la volviera verdadera lo rechaza el compilador —«constant 9223372096000000000 … overflows
	// int64»—, así que el paquete no compila y la prueba no llega a correr. Medido el 2026-10-31
	// con el sabotaje M48 puesto: `arnes -overlay` devolvía «sin veredicto», que es exactamente lo
	// que acá no cuenta como medición. Por la variable la suma es de RUNTIME y desborda en
	// silencio, que además es lo que de verdad hace `espejo.go`: ahí `espera` sale de
	// `plazoDelTurno` y es un Duration de runtime, no una constante.
	tope := TopeDelPlazo
	if tope+margenDelIdle <= tope {
		t.Errorf("TopeDelPlazo (%d ns) + margenDelIdle desborda: el IdleTimeout del espejo queda "+
			"negativo y net/http deja de poner deadline", int64(TopeDelPlazo))
	}
	// (b) UN PISO DE CORDURA CON SU NÚMERO PROPIO, escrito acá y no derivado del tope. Sin esto,
	// (a) se satisface con cualquier valor astronómico que por casualidad no desborde y el tope
	// deja de acotar nada en la práctica: un turno de meses es un proceso colgado con un cliente
	// esperando del otro lado.
	if TopeDelPlazo > 7*24*time.Hour {
		t.Errorf("TopeDelPlazo = %s, y arriba de una semana ya no acota nada que sea un turno", TopeDelPlazo)
	}
	// (c) Y EL BORDE DE ABAJO NO SE CRUZÓ CON EL DE ARRIBA: un tope por debajo del defecto dejaría
	// fuera de rango a todo turno que no configura nada, que es el caso normal.
	if TopeDelPlazo < DemoraPorDefecto {
		t.Errorf("TopeDelPlazo (%s) < DemoraPorDefecto (%s): el default quedaría fuera de rango",
			TopeDelPlazo, DemoraPorDefecto)
	}
}

// EL ARRANQUE DICE POR QUÉ NO ARRANCÓ, y nombrar el valor del yaml acá NO contradice al test de
// más arriba que prohíbe filtrarlo. Lo que ese test protege es que no se anuncie como el tope QUE
// RIGE —con `-1` el Lanzador cae al defecto, y un «-1s» manda a buscar el problema al lugar
// equivocado—; acá no rige ningún tope porque `Desde` rechazó el motor, y el número es lo único
// con lo que alguien encuentra la línea en el yaml. La diferencia es «éste es tu plazo» contra
// «esto escribiste y no se puede usar».
//
// Sabotaje M49: el diagnostico de fuera de rango anuncia el plazo resuelto, que es basura envuelta
// arnes: archivo="internal/motor/factory.go"
// arnes: de="\t\t\treturn fmt.Sprintf(\"motor: cli (binario %s, turn_timeout_seconds=%d FUERA DE RANGO —el tope es %s—: el motor no arranca)\",\n\t\t\t\tbin, cfg.TurnTimeoutSeconds, TopeDelPlazo)"
// arnes: a="\t\t\treturn fmt.Sprintf(\"motor: cli (binario %s, tope por turno %s, factura la suscripción)\", bin, plazoDelTurno(time.Duration(cfg.TurnTimeoutSeconds)*time.Second))"
func TestElDiagnosticoDeUnPlazoFueraDeRangoNoAnunciaElResuelto(t *testing.T) {
	const seg = 9223372036
	d := DescribirMotor(config.MotorConfig{Provider: "cli", TurnTimeoutSeconds: seg})

	// LO QUE NO PUEDE DECIR: el plazo resuelto, que con este valor son 292 años y se lee como un
	// dato. Se CALCULA en vez de escribirse, para que la aserción siga apuntando al número de
	// verdad si alguien cambia el caso de arriba.
	basura := plazoDelTurno(time.Duration(seg) * time.Second).String()
	if strings.Contains(d, basura) {
		t.Errorf("el diagnóstico anuncia el plazo envuelto (%s) como si rigiera: %s", basura, d)
	}
	// LO QUE SÍ TIENE QUE DECIR: el valor del yaml, que es con lo que se encuentra la línea...
	if !strings.Contains(d, "9223372036") {
		t.Errorf("el diagnóstico no nombra el valor que sobra, así que no se puede ubicar en el yaml: %s", d)
	}
	// ...y la consecuencia, que es que no hay motor.
	if !strings.Contains(d, "no arranca") {
		t.Errorf("el diagnóstico no dice que el motor no arranca: %s", d)
	}
}
