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

// R8 — EL DAEMON SIRVE LECTURAS SOBRE UNA BASE QUE NO PUEDE ESCRIBIR.
//
// Es el invariante de punta a punta del escalón, y se mide sobre el PROCESO por lo mismo que D5:
// lo que se afirma vive entre el arranque de runDaemon y lo que sale por stdout, así que un test
// sobre una función interna estaría midiendo el proxy.
//
// A diferencia de D5, acá el piso de lectura SE CONSERVA: es lo que convierte «más nueva» en
// «legible». Las dos pruebas usan la misma base más nueva y se distinguen exactamente en eso.

// baseMasNuevaPeroLegible deja una base migrada, con una nota adentro y con su piso intacto, y
// después le adelanta el user_version fuera del alcance de este binario.
func baseMasNuevaPeroLegible(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("preparar la base: %v", err)
	}
	const nota = "La nota sembrada antes de que la base quedara fuera del alcance de este binario."
	if err := eng.SaveObservation("", "prueba/escalon", nota, nil); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	eng.Close()

	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile))
	if err != nil {
		t.Fatalf("abrir cruda: %v", err)
	}
	defer db.Close()

	// El piso tiene que estar: sin él este escenario cae al modo degradado (es el de D5).
	var piso int
	if err := db.QueryRow(`SELECT read_floor FROM schema_floor WHERE id = 1`).Scan(&piso); err != nil {
		t.Fatalf("el escenario no se armó: la base no tiene piso grabado (%v)", err)
	}
	if piso > memory.EsquemaEsperado() {
		t.Fatalf("el escenario no se armó: el piso v%d está por encima de este binario (v%d)", piso, memory.EsquemaEsperado())
	}
	if _, err := db.Exec(`PRAGMA user_version = 999`); err != nil {
		t.Fatalf("adelantar user_version: %v", err)
	}
	return dir
}

// TestHelperDaemonSoloLectura no es un test: es el proceso hijo que corre runDaemon de verdad.
func TestHelperDaemonSoloLectura(t *testing.T) {
	if os.Getenv("GO_QUIERO_SER_DAEMON_RO") != "1" {
		t.Skip("helper, no es un test")
	}
	runDaemon()
	os.Exit(0)
}

func TestR8ElDaemonSirveLecturasSobreUnaBaseQueNoPuedeEscribir(t *testing.T) {
	dir := baseMasNuevaPeroLegible(t)

	entrada := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"musubi_recall","arguments":{"query":"nota sembrada base binario"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"musubi_save_observation","arguments":{"topic_key":"t/no","content":"esto no tiene que entrar"}}}`,
	}, "\n") + "\n"

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperDaemonSoloLectura")
	cmd.Env = append(os.Environ(), "GO_QUIERO_SER_DAEMON_RO=1", "MUSUBI_HOME="+dir)
	cmd.Stdin = strings.NewReader(entrada)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("el daemon murió: %v\n  stderr=%s", err, errBuf.String())
	}

	respuestas := map[float64]map[string]interface{}{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 0, 1<<20), 8<<20)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" {
			continue
		}
		var r map[string]interface{}
		if jerr := json.Unmarshal([]byte(l), &r); jerr != nil {
			t.Fatalf("stdout no es JSON-RPC: %v\n  %.200s", jerr, l)
		}
		id, _ := r["id"].(float64)
		respuestas[id] = r
	}

	// (1) Declara el escalón, y NO el modo degradado: son estados distintos y confundirlos haría
	//     que el agente se dé por muerto teniendo memoria para leer.
	res, _ := respuestas[1]["result"].(map[string]interface{})
	meta, _ := res["_meta"].(map[string]interface{})
	if meta["musubi/degraded"] != nil {
		t.Errorf("se declaró DEGRADADO teniendo una base legible: %v", meta["musubi/degraded"])
	}
	ro, _ := meta["musubi/readonly"].(map[string]interface{})
	if ro == nil {
		t.Fatalf("no declaró el escalón de sólo lectura: %v", respuestas[1])
	}

	// (2) La lectura devuelve la nota sembrada. Es la mitad que separa esta pieza de un rechazo
	//     educado: la memoria se puede consultar de verdad.
	lec := respuestas[2]
	if e, hay := lec["error"]; hay {
		t.Fatalf("la lectura falló: %v", e)
	}
	crudo, _ := json.Marshal(lec["result"])
	if !strings.Contains(string(crudo), "prueba/escalon") {
		t.Errorf("la lectura no trajo la nota sembrada:\n  %.400s", crudo)
	}

	// (3) La escritura se rechaza con su código propio.
	rpcErr, _ := respuestas[3]["error"].(map[string]interface{})
	if rpcErr == nil {
		t.Fatalf("la escritura NO fue rechazada: %v", respuestas[3])
	}
	if code, _ := rpcErr["code"].(float64); int(code) != -32005 {
		t.Errorf("código %v, esperaba -32005 (sólo lectura)", rpcErr["code"])
	}

	// (4) Y la base no se tocó: sigue en el esquema que este binario no sabe migrar.
	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile))
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer db.Close()
	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		t.Fatalf("leer user_version: %v", err)
	}
	if uv != 999 {
		t.Errorf("user_version = %d: el daemon migró una base que no podía migrar", uv)
	}
}
