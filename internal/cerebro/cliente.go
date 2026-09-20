// Package cerebro arma EL cliente HTTP con el que CUALQUIER parte de musubi le habla al cerebro
// central.
//
// POR QUÉ ES UN PAQUETE Y NO UNA FUNCIÓN EN `cmd/musubi`, que es lo que era hasta hoy.
//
// El 2026-09-20 se arreglaron seis clientes de `cmd/musubi` que armaban su `http.Client` a mano y
// por eso no declaraban el ServerName. La guarda que se dejó puesta preguntaba lo correcto
// —«¿hay algún `http.Client{…}` armado a mano?»— pero acotada a `cmd/musubi/`, así que el mismo
// defecto sobrevivió intacto en otros dos paquetes y NADIE lo iba a reclamar:
//
//   - `internal/mcp/syncclient.go`: el cliente del sync saliente, o sea el que usan TODOS los
//     daemons de todas las sesiones. Sin ServerName.
//   - `internal/provision/probe.go`: dos clientes del self-check, y peor que sin ServerName —
//     tenían el esquema `http://` escrito a máquina, así que no podían ni expresar HTTPS.
//
// La lección es la de siempre en este repo, aplicada a una guarda en vez de a un arreglo: la
// guarda puesta en N−1 de N caminos. Acá el N−1 lo creó su propio alcance. Con el constructor
// viviendo en un paquete que todos pueden importar, la guarda puede preguntar lo mismo sobre el
// repo entero y la respuesta converge.
package cerebro

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"
	"time"
)

// EnvNombreTLS existe por un choque de dos cosas que las dos son ciertas (Ola 0 del plan
// empresa, 2026-09-03).
//
// El cerebro puede servir HTTPS con un certificado de `tailscale cert`, que Let's Encrypt
// emite para el NOMBRE del nodo en la malla (`musubi-server.tail89e295.ts.net`) y para
// ningún otro. Pero los agentes laten contra la IP del tailnet a propósito: con NordVPN
// activo el DNS de la malla NO resuelve los nombres MagicDNS, y eso está escrito en
// `deploy/README.md` porque costó encontrarlo.
//
// Las dos cosas juntas son un certificado que no valida: se disca una IP y el certificado
// dice un nombre. La salida NO es apagar la verificación —eso convierte el TLS en teatro y
// deja pasar a cualquiera que se meta en el medio—: es discar la IP y verificar el
// certificado contra el nombre, que es exactamente para lo que existe ServerName.
//
// Vacío ⇒ comportamiento de siempre: el nombre sale de la URL. Sólo hace falta cuando la
// URL trae una IP y el certificado trae un nombre.
const EnvNombreTLS = "MUSUBI_BRAIN_TLS_NAME"

// NombreTLS es EL ÚNICO lugar donde se lee `MUSUBI_BRAIN_TLS_NAME`.
//
// Está aparte para que la guarda de alcance pueda preguntar una sola cosa —«¿alguien más lee esta
// variable?»— en vez de perseguir cada cliente. Un segundo lector es un segundo lugar donde
// olvidarse.
func NombreTLS() string { return strings.TrimSpace(os.Getenv(EnvNombreTLS)) }

// Cliente arma EL cliente con el que se le habla al cerebro central.
//
//   - `tr` es el Transport que el llamador ya ajustó a su necesidad, o nil si le sirve el default.
//   - el ServerName se declara DESPUÉS de ese ajuste, así que no hay forma de pisarlo sin querer.
//   - `espera` en 0 significa sin timeout global, que es lo que necesita el stream del panel.
//
// SÓLO toca ServerName y MinVersion. No apaga la verificación, no cambia el pool de raíces: un
// cliente que "arregla" el TLS relajándolo es peor que no tener TLS, porque el candado del panel
// dice que está seguro.
//
// CON NOMBRE VACÍO Y SIN TRANSPORT DEVUELVE EL CLIENTE PELADO, sin Transport propio, para que el
// default del stdlib siga siendo el default: un Transport clonado que nadie necesita es una
// superficie donde mañana alguien mete un flag.
func Cliente(nombre string, espera time.Duration, tr *http.Transport) *http.Client {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" && tr == nil {
		return &http.Client{Timeout: espera}
	}
	if tr == nil {
		tr = http.DefaultTransport.(*http.Transport).Clone()
	}
	if nombre != "" {
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.ServerName = nombre
		tr.TLSClientConfig.MinVersion = tls.VersionTLS12
	}
	return &http.Client{Timeout: espera, Transport: tr}
}
