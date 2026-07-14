package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

func runCerebro(args []string) {
	fs := flag.NewFlagSet("cerebro", flag.ExitOnError)
	url := fs.String("url", "", "URL del cerebro central (default: $MUSUBI_CENTRAL_URL)")
	tokenEnv := fs.String("token-env", "MANDO_MUSUBI_TOKEN", "variable de entorno con el token")
	timeout := fs.Int("timeout", 60, "timeout por request, en segundos")
	_ = fs.Parse(args)

	base := strings.TrimSpace(*url)
	if base == "" {
		base = strings.TrimSpace(os.Getenv("MUSUBI_CENTRAL_URL"))
	}
	if base == "" {
		fmt.Fprintln(os.Stderr, "musubi cerebro: falta la URL del cerebro (--url o $MUSUBI_CENTRAL_URL)")
		os.Exit(1)
	}
	token := strings.TrimSpace(os.Getenv(*tokenEnv))
	if token == "" {
		// Fail-closed: sin credencial, el cerebro rechazaría TODO y el canal sería una sucesión de
		// 401 silenciosos. Mejor no arrancar y decir exactamente qué falta.
		fmt.Fprintf(os.Stderr, "musubi cerebro: la variable %s está vacía; sin token el cerebro rechaza todo\n", *tokenEnv)
		os.Exit(1)
	}
	endpoint := strings.TrimRight(base, "/") + "/mcp"

	client := &http.Client{Timeout: time.Duration(*timeout) * time.Second}
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
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(req, &probe); err != nil {
			writeLine(out, errorRPC(nil, -32700, "musubi cerebro: JSON ilegible en stdin: "+err.Error()))
			continue
		}
		esNotificacion := len(probe.ID) == 0

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

func writeLine(w *bufio.Writer, b []byte) {
	w.Write(b)
	w.WriteByte('\n')
	w.Flush() // sin flush, el cliente espera una respuesta que quedó en el buffer: deadlock.
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
