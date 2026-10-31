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

func TestLosDosProveedoresCumplenLaInterfaz(t *testing.T) {
	// La fábrica es el único lugar donde los dos motores se usan como el MISMO tipo. Las
	// aserciones `var _ Motor` de cada archivo cubren la firma, pero no que la fábrica los
	// devuelva por la interfaz: eso se ve recién acá.
	t.Setenv("ANTHROPIC_API_KEY", "sk-de-prueba")
	for _, prov := range []string{ProveedorCLI, ProveedorAPI} {
		m, err := Desde(config.MotorConfig{Provider: prov})
		if err != nil {
			t.Fatalf("provider=%q: %v", prov, err)
		}
		var _ Motor = m
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
