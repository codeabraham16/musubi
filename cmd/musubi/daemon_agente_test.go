package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/guiones"
)

// EL DAEMON QUE CORRE DE VERDAD LE HABLA AL AGENTE.
//
// Las pruebas de internal/mcp fijan qué hace el servidor CON la option; ésta fija que `musubi
// daemon` la pase. Se mide sobre el proceso por lo mismo que D5: el cableado vive en runDaemon, y un
// test sobre un helper mediría el proxy. Sin la option, todo lo de agente.go existe y nadie lo usa:
// el binario compila, las pruebas del paquete pasan y el agente sigue sin saber que Musubi existe.

// TestHelperDaemonSano no es un test: es el proceso hijo que corre runDaemon de verdad.
func TestHelperDaemonSano(t *testing.T) {
	if os.Getenv("GO_QUIERO_SER_DAEMON_SANO") != "1" {
		t.Skip("helper, no es un test")
	}
	runDaemon()
	os.Exit(0)
}

// Sabotaje que la hace fallar: sacarle la option al daemon.
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="mcp.WithInstruccionesParaElAgente(), "
// arnes: a=""
func TestElDaemonLeHablaAlAgente(t *testing.T) {
	dir := t.TempDir()
	// Sin chequeo de versión: el daemon lo lanza en una goroutine contra GitHub, y una prueba no
	// sale a la red. Negativo y no 0, porque 0 la config lo lee como «usá el default».
	cfg := config.Default()
	cfg.Update.CheckIntervalHours = -1
	contenido, err := cfg.Marshal()
	if err != nil {
		t.Fatalf("serializar la config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.DirName, config.ConfigFile), contenido, 0o644); err != nil {
		t.Fatal(err)
	}

	entrada := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"

	cmd := guiones.Herramienta(t, os.Args[0], "-test.run=^TestHelperDaemonSano$")
	cmd.Env = append(os.Environ(), "GO_QUIERO_SER_DAEMON_SANO=1", "MUSUBI_HOME="+dir)
	cmd.Stdin = strings.NewReader(entrada)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("el daemon no terminó bien: %v\n  stdout=%.300q\n  stderr=%s", err, out, errBuf.String())
	}

	respuestas := map[float64]map[string]interface{}{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 0, 1<<20), 8<<20) // tools/list es grande; el buffer por defecto lo corta
	for sc.Scan() {
		linea := strings.TrimSpace(sc.Text())
		if linea == "" {
			continue
		}
		var r map[string]interface{}
		if jerr := json.Unmarshal([]byte(linea), &r); jerr != nil {
			t.Fatalf("el daemon escribió algo que no es JSON-RPC por stdout: %v\n  %.200s", jerr, linea)
		}
		id, _ := r["id"].(float64)
		respuestas[id] = r
	}

	init, _ := respuestas[1]["result"].(map[string]interface{})
	if init == nil {
		t.Fatalf("no hubo respuesta al initialize: %v", respuestas[1])
	}
	texto, _ := init["instructions"].(string)
	if !strings.Contains(texto, "musubi_recall") {
		t.Errorf("el daemon no le mandó instrucciones al agente: instructions=%q", texto)
	}

	lista, _ := respuestas[2]["result"].(map[string]interface{})
	tools, _ := lista["tools"].([]interface{})
	if len(tools) == 0 {
		t.Fatalf("no hubo catálogo: %v", respuestas[2])
	}
	cargadas := 0
	for _, x := range tools {
		tl, _ := x.(map[string]interface{})
		meta, _ := tl["_meta"].(map[string]interface{})
		if v, _ := meta["anthropic/alwaysLoad"].(bool); v {
			cargadas++
		}
	}
	if cargadas == 0 {
		t.Errorf("el daemon sirvió las %d tools diferidas: ninguna llega cargada", len(tools))
	}
}
