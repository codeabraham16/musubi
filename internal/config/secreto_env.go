package config

import (
	"fmt"
	"os"
	"strings"
)

// SufijoArchivoDeSecreto es el sufijo del patrón `<VAR>_FILE`: la convención de docker/podman y
// systemd para pasar un secreto por archivo en vez de por el entorno, que es la forma que se filtra
// sola (cualquier cosa que liste el entorno la imprime).
const SufijoArchivoDeSecreto = "_FILE"

// SecretoDeEnv resuelve el secreto nombrado por la variable `nombre`, con el respaldo estándar
// `<nombre>_FILE` (un archivo, idealmente modo 600, cuyo contenido ES el secreto).
//
// POR QUÉ EXISTE, y no es un adorno: `deploy/musubi-tool.sh` documenta `MUSUBI_TOKEN_FILE` como «lo
// recomendado», pero eso valía SÓLO para los scripts de shell — ningún .go lo leía. El 2026-09-05
// alguien siguió la recomendación, cambió `MUSUBI_TOKEN` por `MUSUBI_TOKEN_FILE` en su shell, los
// scripts siguieron andando (así que la señal fue «arreglado») y el daemon quedó SIN CREDENCIAL,
// fallando su drain cada 30 s contra el central durante horas. Una recomendación que sólo entiende
// la mitad del sistema es peor que no darla. Ver el cabo A89.
//
// Precedencia: la variable directa gana sobre el archivo, igual que en `musubi-tool.sh`, porque
// algo puesto a mano en el entorno es una decisión más explícita que un archivo que quedó ahí.
//
// Devuelve error SÓLO si el archivo fue nombrado y no se pudo leer: eso es una configuración rota y
// merece ruido, no un secreto vacío que después falla como un 401 sin explicación.
func SecretoDeEnv(nombre string) (string, error) {
	if nombre == "" {
		return "", nil
	}
	if v := strings.TrimSpace(os.Getenv(nombre)); v != "" {
		return v, nil
	}
	ruta := strings.TrimSpace(os.Getenv(nombre + SufijoArchivoDeSecreto))
	if ruta == "" {
		return "", nil
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return "", fmt.Errorf("%s%s apunta a %q y no se pudo leer: %w", nombre, SufijoArchivoDeSecreto, ruta, err)
	}
	// Un archivo escrito con `echo` termina en \n y el token NO lo incluye; sin este Trim el bearer
	// viaja con un salto de línea y el central contesta 401 sin decir por qué.
	secreto := strings.TrimSpace(string(datos))

	// ────────────────────────────────────────────────────────────────────────────────────────
	// UN ARCHIVO DE VARIAS LÍNEAS SE RECHAZA ACÁ, CON SU MOTIVO, EN VEZ DE MORIR LEJOS Y MUDO
	//
	// El Trim de arriba saca el `\n` del final y nada más. Con DOS líneas no vacías, lo que
	// vuelve es un secreto con un salto de línea ADENTRO — y eso no da 401: `net/http` se niega a
	// mandar el pedido. Medido el 2026-09-05 con un archivo de dos tokens:
	//
	//     SecretoDeEnv -> "tokenNUEVO\ntokenVIEJO"
	//     net/http     -> invalid header field value for "Authorization"
	//
	// El error no nombra el archivo, ni la variable, ni el salto de línea. Apunta a la biblioteca
	// HTTP, o sea al lugar donde NO está la causa — y este mismo par de variables ya costó cuatro
	// intentos de diagnóstico el 2026-08-31 con todo lo demás verificado correcto (A89).
	//
	// POR QUÉ SE RECHAZA Y NO SE ADIVINA LA PRIMERA LÍNEA: acá no hay un formato multi-token
	// definido. El que SÍ existe —una lista de tokens, el más nuevo primero, para que una rotación
	// tenga fallback— es del token de DISPOSITIVO y lo lee `cmd/musubi/agent_token.go`, que es otra
	// variable (`MUSUBI_DEVICE_TOKEN_FILE`). Quedarse con la primera línea acá inventaría ese
	// formato para un camino que no lo tiene, y elegiría en silencio entre dos credenciales cuando
	// lo honesto es decir que no se sabe cuál quiso poner.
	if strings.ContainsAny(secreto, "\n\r") {
		lineas := 0
		for _, l := range strings.Split(strings.ReplaceAll(secreto, "\r\n", "\n"), "\n") {
			if strings.TrimSpace(l) != "" {
				lineas++
			}
		}
		return "", fmt.Errorf("%s%s apunta a %q y ese archivo tiene %d líneas no vacías: este camino "+
			"espera UN secreto y no una lista. Un valor con un salto de línea adentro hace que net/http "+
			"se niegue a mandar el pedido con «invalid header field value», que no nombra ni el archivo "+
			"ni la variable. Dejá una sola línea. (El formato de varios tokens existe SÓLO para "+
			"MUSUBI_DEVICE_TOKEN_FILE, el del agente.)",
			nombre, SufijoArchivoDeSecreto, ruta, lineas)
	}
	return secreto, nil
}

// NombresDeSecreto devuelve las dos formas de nombrar el mismo secreto, para poder decirlas en un
// mensaje de error sin que quien lo lea tenga que adivinar la segunda.
func NombresDeSecreto(nombre string) string {
	if nombre == "" {
		return ""
	}
	return fmt.Sprintf("$%s (o $%s%s con la ruta de un archivo)", nombre, nombre, SufijoArchivoDeSecreto)
}
