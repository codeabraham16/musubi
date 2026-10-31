package motor

import (
	"fmt"
	"os"
	"strings"
	"time"

	"musubi/internal/config"
)

// ────────────────────────────────────────────────────────────────────────────────────────────
// LA PIEZA QUE CIERRA LA DECISIÓN DE TENER DOS MOTORES.
//
// Los dos existen y los dos están verificados, pero un paquete con dos implementaciones y
// ningún selector no es una elección: es código que nadie puede usar. Esto es el único lugar
// donde se decide cuál corre, y se decide por CONFIGURACIÓN, no por detección automática.
//
// POR QUÉ NO SE AUTODETECTA. La tentación es obvia: «si hay ANTHROPIC_API_KEY usá la API, si
// no buscá el binario». Y es justo lo que no se puede hacer, porque LOS DOS FACTURAN DISTINTO
// —`cli` gasta la suscripción, `api` gasta tokens— y el stream que sale es el mismo. Una
// heurística ahí le elige la forma de pago al usuario, y el error no se nota hasta la factura.
// ────────────────────────────────────────────────────────────────────────────────────────────

// Los nombres de proveedor que acepta la configuración. Son los que el usuario escribe en el
// yaml, así que dicen QUÉ HACE cada uno y no cómo se llama la struct de Go: nadie que edite
// un archivo de configuración sabe qué es un «Lanzador».
const (
	ProveedorCLI = "cli"
	ProveedorAPI = "api"
)

// Desde construye el motor que pide la configuración.
//
// Devuelve (nil, nil) cuando nadie pidió un motor, que es el estado de toda instalación de
// hoy. El nil NO es un error ni un motor que falla: es la ausencia, y el llamador la trata
// como «esta ruta no está disponible». Un motor que existiera pero fallara en cada turno
// sería mucho peor — contestaría 200 y un error por el stream, a un cliente que no pidió nada.
func Desde(cfg config.MotorConfig) (Motor, error) {
	// Se normaliza porque esto lo escribe una persona a mano en un yaml: "  API  " y "api"
	// son lo mismo, y rechazar el primero sería pedantería que cuesta una sesión de
	// diagnóstico.
	switch prov := strings.ToLower(strings.TrimSpace(cfg.Provider)); prov {
	case "":
		return nil, nil

	case ProveedorCLI:
		// 🔴 EL PLAZO SE ACOTA ACÁ, AL CONSTRUIR, por la misma razón que la credencial de más
		// abajo: es un error de configuración, y el único que puede corregirlo es quien editó el
		// yaml. Dejarlo pasar no daría un turno largo —daría un espejo cuyo listener no vence
		// nunca, o un tope de 290 ms—, y el síntoma no apunta al yaml por ningún lado. La doc de
		// TopeDelPlazo tiene los dos números y el porqué de chequear el ENTERO.
		if plazoFueraDeRango(cfg.TurnTimeoutSeconds) {
			return nil, fmt.Errorf(
				"motor.turn_timeout_seconds=%d se pasa del tope de %s: arriba de ahí la conversión a Duration desborda y el plazo que rige deja de ser el que dice el yaml",
				cfg.TurnTimeoutSeconds, TopeDelPlazo)
		}
		return &Lanzador{
			Binario: cfg.Binary,
			// Un 0 en el yaml NO es «timeout cero»: el Lanzador lo lee como su default de
			// diez minutos. Convertirlo acá a `0 * time.Second` conserva ese cero, que es
			// exactamente lo que hay que hacer — un timeout de cero real mataría el turno
			// antes de la primera palabra. Lo mismo con un NEGATIVO, que es el mismo error
			// tecleado distinto: el Lanzador lo manda al default igual (resuelve con `<= 0`),
			// así que acá tampoco hay nada que normalizar.
			Espera:          time.Duration(cfg.TurnTimeoutSeconds) * time.Second,
			ConservarAPIKey: cfg.KeepAPIKey,
			VersionDeSesion: cfg.SessionVersion,
		}, nil

	case ProveedorAPI:
		c := &ClienteAPI{
			Endpoint: cfg.Endpoint,
			ClaveEnv: cfg.APIKeyEnv,
			Version:  cfg.APIVersion,
			Betas:    cfg.Betas,
		}
		// 🔴 LA CREDENCIAL SE VERIFICA AL CONSTRUIR, NO EN EL PRIMER TURNO. Fail-closed: un
		// motor que arranca «bien» y se cae recién cuando alguien le pide algo convierte un
		// error de configuración en un error de runtime, y el que lo ve es el cliente
		// remoto, que no puede hacer nada al respecto. Acá lo ve quien editó el yaml.
		//
		// Se chequea la EXISTENCIA, no el valor: validar la clave exigiría un request, y
		// este constructor corre al levantar el servicio.
		if c.clave() == "" {
			return nil, fmt.Errorf(
				"motor.provider=%q pero la variable de entorno %s está vacía: sin credencial no hay turno que servir",
				ProveedorAPI, c.nombreDeLaVariable())
		}
		return c, nil

	default:
		// EL ERROR NOMBRA LOS VÁLIDOS, y es la diferencia entre una sesión de diagnóstico y
		// una corrección de diez segundos. Alguien que escribió "claude" o "anthropic" quiso
		// prender el motor; decirle sólo «proveedor desconocido» lo deja adivinando.
		return nil, fmt.Errorf("motor.provider=%q desconocido: los válidos son %q y %q (o vacío para dejarlo apagado)",
			cfg.Provider, ProveedorCLI, ProveedorAPI)
	}
}

// DescribirMotor dice en una línea qué motor quedó configurado, para el arranque y para el
// diagnóstico. Nombra la VARIABLE de la credencial, nunca su valor.
//
// Existe porque el síntoma de un motor mal configurado es un 405 silencioso, y una línea en
// el arranque es lo más barato que convierte eso en algo averiguable.
func DescribirMotor(cfg config.MotorConfig) string {
	switch prov := strings.ToLower(strings.TrimSpace(cfg.Provider)); prov {
	case "":
		return "motor: apagado (motor.provider vacío; el turno contesta 405)"
	case ProveedorCLI:
		bin := cfg.Binary
		if bin == "" {
			bin = "claude"
		}
		// NO SE ANUNCIA EL NÚMERO RESUELTO CUANDO ESTÁ FUERA DE RANGO, porque el resuelto es
		// basura envuelta y se lee como un dato: con 9223372036 esta línea diría «tope por turno
		// 2562047h47m16s» —292 años— y con 18446744074 diría «290.448384ms», mandando a buscar el
		// problema a cualquier lado menos al yaml. `Desde` ya rechazó este motor; lo único útil
		// que puede decir el arranque es POR QUÉ no arrancó.
		if plazoFueraDeRango(cfg.TurnTimeoutSeconds) {
			return fmt.Sprintf("motor: cli (binario %s, turn_timeout_seconds=%d FUERA DE RANGO —el tope es %s—: el motor no arranca)",
				bin, cfg.TurnTimeoutSeconds, TopeDelPlazo)
		}
		espera := plazoDelTurno(time.Duration(cfg.TurnTimeoutSeconds) * time.Second)
		return fmt.Sprintf("motor: cli (binario %s, tope por turno %s, factura la suscripción)", bin, espera)
	case ProveedorAPI:
		variable := cfg.APIKeyEnv
		if variable == "" {
			variable = "ANTHROPIC_API_KEY"
		}
		endpoint := cfg.Endpoint
		if endpoint == "" {
			endpoint = EndpointPorDefecto
		}
		// Se dice si la variable TIENE algo, no qué tiene: es lo único que hace falta para
		// diagnosticar y lo único que se puede imprimir sin filtrar el secreto.
		cargada := "VACÍA"
		if os.Getenv(variable) != "" {
			cargada = "cargada"
		}
		return fmt.Sprintf("motor: api (%s, credencial en %s: %s, factura tokens)", endpoint, variable, cargada)
	default:
		return fmt.Sprintf("motor: provider %q desconocido (el turno contesta 405)", cfg.Provider)
	}
}
