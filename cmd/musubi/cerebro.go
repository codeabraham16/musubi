package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"musubi/internal/config"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// cerebro.go implementa `musubi cerebro`: un servidor MCP por STDIO que no tiene memoria propia —
// REENVÍA cada llamada al cerebro central por HTTP, con la credencial puesta acá.
//
// POR QUÉ EXISTE (y no simplemente un server "type: http" en el .mcp.json):
//
//  1. El cliente MCP-sobre-HTTP de Claude Code hoy NO envía los `headers` que declarás en el
//     .mcp.json (bug anthropics/claude-code#48514), así que la credencial nunca llega y el server
//     rechaza. Peor: intenta OAuth por DESCUBRIMIENTO en vez de por un 401 (bug #46879), y termina
//     en un "SDK auth failed" que no dice nada. Acá el header lo ponemos NOSOTROS: no hay nada que
//     el cliente pueda omitir.
//  2. stdio no tiene OAuth ni negociación de sesión: es un pipe. Menos superficie, menos que falle.
//  3. El proceso que habla con el cerebro pasa a ser `musubi` (no el ejecutable del editor), que es
//     el que ya está excluido en el split-tunnel de la VPN. La conectividad deja de depender de qué
//     binario invocó el cliente.
//
// LO QUE **NO** HACE, y es deliberado: no toca la memoria LOCAL del repo. Este canal CONSULTA el
// cerebro en vivo. Ver todo ≠ replicar todo: si el daemon local bajara la memoria de los demás
// proyectos, el recall de este repo competiría para siempre con ruido de producción ajena.
// Son dos planos: el daemon local (acotado, rápido, offline) y este canal (federado, en vivo).

// defaultDialTimeoutSeg es cuánto se espera para ESTABLECER la conexión con el cerebro central.
//
// ★ EL NÚMERO SALE DE UN INCIDENTE MEDIDO, no de elegirlo lindo. Con el central caído por un corte
// de luz, a otra máquina del equipo NO le arrancaba Claude Code entero. La causa, cronometrada:
//
//	initialize                 21 s   <- timeout de conexión del SISTEMA OPERATIVO
//	notifications/initialized  21 s   <- nadie espera su respuesta, y cuesta igual
//	tools/list                 21 s
//	                         ------
//	                           63 s   contra los 60 s que el host da para inicializar un MCP server
//
// Ninguna request sola llegaba al timeout de 60 s: el que las sumaba era el arranque. Y el precio
// no era perder el canal federado —eso sería aceptable— sino que la SESIÓN ENTERA no levantara. Un
// servidor MCP caído tiene que degradar a «ese servidor no está», nunca tumbar al agente.
//
// Con 5 s, el mismo arranque cuesta ~15 s: entra cómodo aunque el host apure el presupuesto. El
// timeout de REQUEST sigue en 60 s, que es otra cosa: cubre a un central vivo pero lento.
const defaultDialTimeoutSeg = 5

// clienteCerebro arma el cliente HTTP con los DOS timeouts separados, que es el punto: uno acota
// cuánto se espera para que el otro extremo CONTESTE (request) y el otro cuánto para saber si
// siquiera ESTÁ (dial). Dejar el segundo en manos del sistema operativo es lo que produjo el
// incidente: en Windows son ~21 s por intento y nadie los eligió.
func clienteCerebro(timeoutSeg, dialSeg int) *http.Client {
	if dialSeg <= 0 {
		dialSeg = defaultDialTimeoutSeg
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{
		Timeout:   time.Duration(dialSeg) * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	// El nombre TLS se declara acá y no en el Transport de arriba: este canal disca la IP del
	// tailnet (con NordVPN el MagicDNS no resuelve) y el certificado no lleva SAN de IP (A129).
	return clienteHaciaElCerebro(nombreTLSDelCerebro(), time.Duration(timeoutSeg)*time.Second, tr)
}

func runCerebro(args []string) {
	fs := flag.NewFlagSet("cerebro", flag.ExitOnError)
	url := fs.String("url", "", "URL del cerebro central (default: $MUSUBI_CENTRAL_URL)")
	tokenEnv := fs.String("token-env", "MANDO_MUSUBI_TOKEN", "variable de entorno con el token")
	timeout := fs.Int("timeout", 60, "timeout por request, en segundos")
	dialTimeout := fs.Int("dial-timeout", defaultDialTimeoutSeg, "timeout para ESTABLECER la conexión, en segundos")
	alcance := fs.Bool("alcance", false, "sondea /readyz y sale: no abre el canal MCP ni necesita token")
	_ = fs.Parse(args)

	base := strings.TrimSpace(*url)
	if base == "" {
		base = strings.TrimSpace(os.Getenv("MUSUBI_CENTRAL_URL"))
	}
	if base == "" {
		fmt.Fprintln(os.Stderr, "musubi cerebro: falta la URL del cerebro (--url o $MUSUBI_CENTRAL_URL)")
		os.Exit(1)
	}
	// `--alcance` SALE ANTES DEL TOKEN, y no es un atajo: `/readyz` no pide credencial, así que
	// exigirla acá convertiría «no llego al cerebro» en «te falta una variable de entorno» — que es
	// justo la confusión que este comando existe para deshacer. Se puede correr desde una máquina
	// recién enrolada, antes de tener token.
	if *alcance {
		os.Exit(informarAlcance(os.Stdout, base, nombreTLSDelCerebro(), esperaDelSondeo))
	}

	token, terr := config.SecretoDeEnv(*tokenEnv)
	if terr != nil {
		fmt.Fprintf(os.Stderr, "musubi cerebro: %v\n", terr)
		os.Exit(1)
	}
	if token == "" {
		// Fail-closed: sin credencial, el cerebro rechazaría TODO y el canal sería una sucesión de
		// 401 silenciosos. Mejor no arrancar y decir exactamente qué falta.
		fmt.Fprintf(os.Stderr, "musubi cerebro: la variable %s está vacía; sin token el cerebro rechaza todo\n", *tokenEnv)
		os.Exit(1)
	}
	endpoint := strings.TrimRight(base, "/") + "/mcp"

	client := clienteCerebro(*timeout, *dialTimeout)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	// El framing MCP por stdio es un JSON por línea. Las líneas de un tools/list del cerebro pueden
	// ser grandes (35 tools con sus schemas), así que el buffer del scanner se agranda a 8 MB: con el
	// default de 64 KB, un tools/list se cortaría a la mitad y el canal moriría en el handshake.
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 1<<20), 8<<20)

	for in.Scan() {
		// El BOM UTF-8 al principio del stream rompe el JSON. Lo mete cualquier productor que
		// escriba en UTF-8 "con firma" (PowerShell, por caso). Se saca antes de parsear: cuesta una
		// línea y evita que el canal muera en el handshake por una marca invisible.
		line := bytes.TrimSpace(bytes.TrimPrefix(in.Bytes(), []byte("\xef\xbb\xbf")))
		if len(line) == 0 {
			continue
		}
		// Copia: el buffer del scanner se reusa en la próxima línea.
		req := append([]byte(nil), line...)

		// Una NOTIFICACIÓN (sin "id") no lleva respuesta: se reenvía y no se escribe nada de vuelta.
		// Si respondiéramos igual, el cliente vería una respuesta huérfana y cerraría la sesión.
		//
		// OJO CON LA DISTINCIÓN: "no parsea" NO es "es una notificación". Tratarlos igual hace que
		// una línea corrupta DESAPAREZCA EN SILENCIO y el cliente espere para siempre una respuesta
		// que nunca va a llegar. Un JSON ilegible se responde con un error de parseo, no se traga.
		var probe struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(req, &probe); err != nil {
			writeLine(out, errorRPC(nil, -32700, "musubi cerebro: JSON ilegible en stdin: "+err.Error()))
			continue
		}
		esNotificacion := len(probe.ID) == 0

		// EL HANDSHAKE MCP SE TERMINA ACÁ, SIN DISCAR EL CENTRAL. Antes se reenviaba, y un dial en
		// frío que pasara de los 5 s dejaba al cliente MCP marcando este server `failed` para TODA
		// la sesión aunque el central estuviera vivo (medido 2026-10-28). Se puede responder local
		// porque el central es STATELESS: su /mcp despacha cada POST por separado y NO exige un
		// initialize previo (internal/mcp/http.go), así que tools/list y tools/call siguen
		// funcionando sin haberle reenviado este saludo.
		// Se respeta la distinción request/notificación que fija el comentario de arriba: el
		// initialize es SIEMPRE un request (lleva id) y la notificación de cierre NUNCA lo lleva. Un
		// mensaje malformado que invierta eso NO toma este atajo —cae al forward de siempre—, para no
		// contradecir esa regla por un caso que un cliente conforme jamás manda.
		switch {
		case probe.Method == "initialize" && !esNotificacion:
			writeLine(out, saludoLocal(probe.ID))
			continue
		case probe.Method == "notifications/initialized" && esNotificacion:
			// Cierre del handshake: una notificación no lleva respuesta y el central no la
			// necesita. Se traga en vez de reenviarla, que sería un dial en frío extra al pedo.
			continue
		}

		resp, err := forward(client, endpoint, token, req)
		if err != nil {
			if esNotificacion {
				continue
			}
			// El error viaja como un error JSON-RPC con el MISMO id, para que el cliente lo asocie a
			// su pedido en vez de quedarse esperando para siempre.
			writeLine(out, errorRPC(probe.ID, -32603, "musubi cerebro: "+err.Error()))
			continue
		}
		if esNotificacion {
			continue
		}
		writeLine(out, resp)
	}
	if err := in.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "musubi cerebro: error leyendo stdin: %v\n", err)
	}
}

// forward manda el JSON-RPC crudo al cerebro con la credencial y devuelve la respuesta cruda.
func forward(client *http.Client, endpoint, token string, payload []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// LA RAZÓN DE SER DE ESTE COMANDO: el header lo ponemos acá. No hay cliente que pueda omitirlo.
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("el cerebro devolvió HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// writeLine escribe una respuesta y la EMPUJA: sin flush, el cliente espera para siempre una
// respuesta que quedó en el buffer. Si stdout se rompe, el canal está muerto — se dice por stderr
// en vez de seguir escribiendo al vacío.
func writeLine(w *bufio.Writer, b []byte) {
	if _, err := w.Write(b); err != nil {
		fmt.Fprintf(os.Stderr, "musubi cerebro: no se pudo escribir la respuesta: %v\n", err)
		return
	}
	if err := w.WriteByte('\n'); err != nil {
		fmt.Fprintf(os.Stderr, "musubi cerebro: no se pudo cerrar la línea: %v\n", err)
		return
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "musubi cerebro: no se pudo enviar la respuesta: %v\n", err)
	}
}

// saludoLocal arma la respuesta al `initialize` SIN discar el central. Replica el shape que
// devuelve el server central (internal/mcp, handleInitialize) para que el cliente lo acepte igual:
// protocolVersion fijo (el que el central ya negocia hoy), capabilities.tools y serverInfo. El `id`
// se ecoa tal cual. NO trae `_meta` (identity/readonly): es diagnóstico, no lo exige el handshake, y
// traerlo obligaría a consultar al central — justo el dial que este saludo existe para evitar.
func saludoLocal(id json.RawMessage) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "musubi-cerebro", "version": version},
		},
	})
	return b
}

func errorRPC(id json.RawMessage, code int, msg string) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": msg},
	})
	return b
}
