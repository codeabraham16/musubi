package mcp

// exposicion_config.go es DÓNDE se declara que una máquina de Tier B se mide raspando un endpoint
// en formato de exposición, y CON QUÉ credencial.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ UN ARCHIVO Y NO COLUMNAS EN LA TABLA
//
// Un destino de éstos necesita tres cosas —la URL, la credencial y qué sistema de archivos
// mirar— y ninguna de las tres es del DISPOSITIVO: son de CÓMO LO ALCANZA ESTE CEREBRO. La misma
// base vista desde otro Musubi tendría otra credencial. Meterlas en `devices` haría que se
// federen y se respalden junto con el inventario, que es exactamente lo que no se quiere de una
// credencial.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA CREDENCIAL NO ESTÁ ACÁ. ACÁ ESTÁ EL NOMBRE DE DÓNDE MIRARLA.
//
// Es el idioma que este repo ya usa (`embedding.api_key_env: OPENAI_API_KEY` en config.yaml) y
// no es ceremonia: este archivo se lee, se versiona por error, se copia a un ticket y se pega en
// un chat. El secreto vive en el entorno del cerebro —`/etc/musubi/musubi.env`, modo 600, que ya
// es donde viven los otros— y acá viaja únicamente su NOMBRE.

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"musubi/internal/fleet"
)

// entradaExposicion es lo que se declara por dispositivo.
type entradaExposicion struct {
	URL string `yaml:"url"`
	// AuthEnv es el NOMBRE de la variable de entorno con el valor del header Authorization.
	// Vacío = endpoint sin credencial, que es legítimo en una red cerrada.
	AuthEnv string `yaml:"auth_env"`
	// Montaje es qué sistema de archivos mirar. Vacío = la raíz. En una base gestionada la raíz
	// es el sistema operativo y el volumen que se llena es otro: mirar la raíz ahí es mirar el
	// disco equivocado y no enterarse nunca.
	Montaje string `yaml:"montaje"`
}

type registroExposicion struct {
	Dispositivos map[string]entradaExposicion `yaml:"dispositivos"`
}

// rutaExposicion resuelve el archivo, con la MISMA regla que principals.yaml: relativo al
// workspace y jamás al directorio de trabajo del proceso. En un servicio de systemd sin
// WorkingDirectory el cwd es `/`, y ese error se degrada en silencio.
func (s *McpServer) rutaExposicion() string {
	return filepath.Join(s.projectPath, ".musubi", "flota-exposicion.yaml")
}

// destinoDeExposicion devuelve cómo raspar a este dispositivo, o hay=false si no se declaró.
//
// SE LEE EL ARCHIVO EN CADA SONDEO, sin caché. Es un archivo de unas líneas y los sondeos son
// cada minutos: el costo es ninguno, y a cambio no existe el modo de fallo de «cambié la
// credencial y el cerebro sigue usando la vieja porque la tiene en memoria». Una caché acá
// compraría microsegundos y pagaría con una clase entera de bugs de invalidación.
func (s *McpServer) destinoDeExposicion(nombre string) (fleet.DestinoExposicion, bool, error) {
	b, err := os.ReadFile(s.rutaExposicion())
	if err != nil {
		// QUE NO EXISTA NO ES UN ERROR: es lo normal en cualquier despliegue que no tenga
		// máquinas de este tipo. Se distingue de «existe y no se puede leer», que sí lo es —
		// un archivo con permisos mal puestos no puede parecer un archivo ausente.
		if os.IsNotExist(err) {
			return fleet.DestinoExposicion{}, false, nil
		}
		return fleet.DestinoExposicion{}, false, fmt.Errorf("no se pudo leer %s: %w", s.rutaExposicion(), err)
	}
	var reg registroExposicion
	if err := yaml.Unmarshal(b, &reg); err != nil {
		// UN YAML ROTO ES UN ERROR Y NO UN «no hay nada declarado». Tratarlo como ausencia haría
		// que una coma de más degrade el sondeo a SSH sin decir nada, y el síntoma sería «esta
		// máquina dejó de medirse» con el archivo a la vista, correcto salvo por una línea.
		return fleet.DestinoExposicion{}, false, fmt.Errorf("%s no es YAML válido: %w", s.rutaExposicion(), err)
	}
	e, hay := reg.Dispositivos[nombre]
	if !hay {
		return fleet.DestinoExposicion{}, false, nil
	}
	d, err := resolverExposicion(nombre, e, os.Getenv)
	if err != nil {
		return fleet.DestinoExposicion{}, false, err
	}
	return d, true, nil
}

// resolverExposicion valida la entrada y resuelve la credencial. `entorno` es parámetro para que
// las pruebas no tengan que ensuciar el entorno del proceso.
func resolverExposicion(nombre string, e entradaExposicion, entorno func(string) string) (fleet.DestinoExposicion, error) {
	crudo := strings.TrimSpace(e.URL)
	if crudo == "" {
		return fleet.DestinoExposicion{}, fmt.Errorf("%q está declarado en la exposición y no tiene `url`", nombre)
	}
	u, err := url.Parse(crudo)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fleet.DestinoExposicion{}, fmt.Errorf("la `url` de %q no es http(s) con host", nombre)
	}
	// LA URL NO PUEDE TRAER LA CREDENCIAL ADENTRO. `https://usuario:clave@host/metrics` funciona
	// y convierte este archivo en un almacén de secretos por la puerta de atrás — que es
	// exactamente lo que el diseño de `auth_env` evita. Se rechaza en vez de aceptarse en
	// silencio, porque un secreto que ya está en un archivo versionado no se puede des-filtrar.
	if u.User != nil {
		return fleet.DestinoExposicion{}, fmt.Errorf("la `url` de %q lleva usuario y clave adentro: la credencial va en `auth_env`, no en la URL", nombre)
	}

	d := fleet.DestinoExposicion{URL: crudo, Montaje: strings.TrimSpace(e.Montaje)}
	if v := strings.TrimSpace(e.AuthEnv); v != "" {
		// UNA VARIABLE DECLARADA Y AUSENTE ES UN ERROR, NO UN ENDPOINT SIN CREDENCIAL. Seguir sin
		// el header daría un 401 del otro lado, y ese 401 manda a mirar el token —que está
		// perfecto— en vez del entorno del cerebro, que es donde está el problema.
		secreto := strings.TrimSpace(entorno(v))
		if secreto == "" {
			return fleet.DestinoExposicion{}, fmt.Errorf("%q declara `auth_env: %s` y esa variable no está en el entorno del cerebro", nombre, v)
		}
		d.Autorizacion = secreto
	}
	return d, nil
}
