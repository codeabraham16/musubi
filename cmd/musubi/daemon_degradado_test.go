package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"

	_ "modernc.org/sqlite"
)

// D5 — EL DAEMON NO SE MUERE MUDO CUANDO LA MEMORIA NO ABRE.
//
// Este es el invariante de verdad, y por eso se mide sobre el PROCESO y no sobre una función: el
// defecto vivía entre el os.Exit(1) y el cliente MCP, así que un test que llamara a un helper
// interno estaría midiendo el proxy y no la cosa. Acá se corre runDaemon en un subproceso real,
// con su stdin y su stdout, contra una base que este binario no puede abrir.
//
// La línea base, medida el 2026-09-07 antes del cambio: exit=1 y CERO BYTES por stdout.

// baseIncompatible arma un workspace con una base marcada más nueva de lo que este binario sabe
// leer. Se migra primero con el motor real —así es una base de musubi de verdad y no un archivo
// vacío— y recién después se le adelanta el user_version.
func baseIncompatible(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("preparar la base: %v", err)
	}
	eng.Close()

	ruta := filepath.Join(dir, config.DirName, config.DBFile)
	db, err := sql.Open("sqlite", ruta)
	if err != nil {
		t.Fatalf("abrir la base para adelantarle el esquema: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA user_version = 999`); err != nil {
		t.Fatalf("adelantar user_version: %v", err)
	}
	// Y SE LE BORRA EL PISO DE LECTURA, porque si no este escenario ya NO es el degradado.
	// Desde que las bases graban su piso, una base más nueva cuyo piso este binario alcanza se
	// abre en SÓLO LECTURA, que es otro estado. El caso degradado es el de una base SIN evidencia
	// —migrada por un binario anterior a esa pieza—, y así se arma.
	if _, err := db.Exec(`DELETE FROM schema_floor`); err != nil {
		t.Fatalf("borrar el piso de lectura: %v", err)
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("releer user_version: %v", err)
	}
	// Sin esta relectura, un PRAGMA que no tomó dejaría la base en el esquema de siempre y el
	// daemon arrancaría sano: el test pasaría sin haber ejercitado nada.
	if v != 999 {
		t.Fatalf("la base quedó en user_version=%d: el escenario no se armó", v)
	}
	return dir
}

// TestHelperDaemonDegradado no es un test: es el proceso hijo que corre runDaemon de verdad.
func TestHelperDaemonDegradado(t *testing.T) {
	if os.Getenv("GO_QUIERO_SER_DAEMON") != "1" {
		t.Skip("helper, no es un test")
	}
	runDaemon()
	os.Exit(0)
}

func TestD5ElDaemonSirveProtocoloAunqueLaMemoriaNoAbra(t *testing.T) {
	dir := baseIncompatible(t)

	entrada := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"musubi_recall","arguments":{"query_text":"x"}}}`,
	}, "\n") + "\n"

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperDaemonDegradado")
	cmd.Env = append(os.Environ(), "GO_QUIERO_SER_DAEMON=1", "MUSUBI_HOME="+dir)
	cmd.Stdin = strings.NewReader(entrada)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("el daemon murió en vez de atender: %v\n  stdout=%q\n  stderr=%s", err, out, errBuf.String())
	}
	if len(out) == 0 {
		t.Fatal("el daemon no escribió NADA por stdout: el cliente MCP no puede distinguirlo de un musubi no instalado")
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

	// (1) El handshake llegó y declara la degradación con su causa.
	init, ok := respuestas[1]
	if !ok {
		t.Fatal("no hubo respuesta al initialize")
	}
	res, _ := init["result"].(map[string]interface{})
	meta, _ := res["_meta"].(map[string]interface{})
	deg, _ := meta["musubi/degraded"].(map[string]interface{})
	if deg == nil {
		t.Fatalf("el handshake no declaró degradación: %v", init)
	}
	razon, _ := deg["reason"].(string)
	if !strings.Contains(razon, "v999") {
		t.Errorf("la causa declarada no menciona el esquema de la base: %q", razon)
	}

	// (2) El catálogo sigue estando: las tools existen, lo que falta es con qué trabajar.
	lista, ok := respuestas[2]
	if !ok {
		t.Fatal("no hubo respuesta al tools/list")
	}
	resLista, _ := lista["result"].(map[string]interface{})
	tools, _ := resLista["tools"].([]interface{})
	if len(tools) == 0 {
		t.Error("el daemon degradado sirvió un catálogo vacío")
	}

	// (3) La llamada se rechaza diciendo por qué, no con un error interno ni con un resultado vacío.
	call, ok := respuestas[3]
	if !ok {
		t.Fatal("no hubo respuesta al tools/call")
	}
	rpcErr, _ := call["error"].(map[string]interface{})
	if rpcErr == nil {
		t.Fatalf("el daemon degradado contestó OK a una tool: %v", call)
	}
	msg, _ := rpcErr["message"].(string)
	if !strings.Contains(msg, "v999") || !strings.Contains(strings.ToUpper(msg), "DEGRADADO") {
		t.Errorf("el error de la tool no explica el estado del servidor: %q", msg)
	}
}
