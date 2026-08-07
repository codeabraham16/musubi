package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"musubi/internal/config"
	"musubi/internal/skills"
)

// skillfed_test.go sella el contrato de la federación del arsenal (spec «arsenal-federado»).
//
// El invariante de seguridad es F3: lo que vuelve del central es DATO REMOTO. Tratarlo como
// confiable «porque es nuestro central» es exactamente cómo se cuela un escape de directorio.

// centralFalso hace de cerebro central: registra lo que recibe y responde lo que se le programe.
type centralFalso struct {
	mu        sync.Mutex
	tool      string          // última tool invocada
	args      json.RawMessage // últimos argumentos recibidos
	arsenal   []skillPayload  // lo que devuelve musubi_list_skills
	errorTool bool            // si true, responde con error JSON-RPC
}

func (c *centralFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	c.mu.Lock()
	c.tool = req.Params.Name
	c.args = req.Params.Arguments
	arsenal, falla := c.arsenal, c.errorTool
	c.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if falla {
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":"x","error":{"code":-32602,"message":"rechazado por el central"}}`)
		return
	}
	texto := "ok"
	if req.Params.Name == "musubi_list_skills" {
		b, _ := json.Marshal(arsenal)
		texto = string(b)
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": "x",
		"result": map[string]any{"content": []map[string]string{{"type": "text", "text": texto}}}}
	_ = json.NewEncoder(w).Encode(resp)
}

// servidorConCentral arma un McpServer con proyecto en disco y un central falso enchufado.
func servidorConCentral(t *testing.T, central *centralFalso) (*McpServer, string) {
	t.Helper()
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	if central != nil {
		ts := httptest.NewServer(central)
		t.Cleanup(ts.Close)
		s.syncClient = newTestSyncClient(t, ts.URL)
	}
	return s, root
}

const skillLocal = `name: revisar-go
description: Como revisar codigo Go en este repo
triggers:
  - "*.go"
capabilities:
  - go
rules: |
  Correr go vet antes de proponer cualquier cambio en Go, y leer el test antes que la implementacion.
`

func skillDelArsenal(nombre string) skillPayload {
	return skillPayload{
		Name:         nombre,
		Description:  "Estructura los tests de Go como subtests de tabla",
		Triggers:     []string{"*_test.go"},
		Capabilities: []string{"go"},
		Rules:        "Escribir los tests de Go como una tabla de casos y un loop de subtests con t.Run.",
		SourceURL:    "https://go.dev/wiki/TableDrivenTests",
	}
}

// TestF1LaSkillViajaCompleta — un campo perdido en el camino deja una skill que no dispara nunca.
func TestF1LaSkillViajaCompleta(t *testing.T) {
	central := &centralFalso{}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	if _, rerr := call(t, s, "musubi_promote_skill", map[string]interface{}{"name": "revisar-go"}); rerr != nil {
		t.Fatalf("promote falló: %+v", rerr)
	}

	central.mu.Lock()
	tool, args := central.tool, string(central.args)
	central.mu.Unlock()

	if tool != "musubi_save_skill" {
		t.Errorf("el central debía recibir musubi_save_skill, recibió %q", tool)
	}
	// Las claves en minúscula: serializar skills.Skill directo emitiría {"Name":…} y el central
	// guardaría una skill con todos los campos vacíos, sin fallar.
	for _, clave := range []string{`"name"`, `"description"`, `"triggers"`, `"capabilities"`, `"rules"`} {
		if !strings.Contains(args, clave) {
			t.Errorf("falta la clave %s en lo que se envió: %s", clave, args)
		}
	}
	var got struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Triggers     []string `json:"triggers"`
		Capabilities []string `json:"capabilities"`
		Rules        string   `json:"rules"`
	}
	if err := json.Unmarshal([]byte(args), &got); err != nil {
		t.Fatalf("el central no pudo parsear los argumentos: %v\n%s", err, args)
	}
	if got.Name != "revisar-go" || got.Description == "" || got.Rules == "" {
		t.Errorf("campos vacíos tras parsear en minúscula: %+v", got)
	}
	if len(got.Triggers) != 1 || got.Triggers[0] != "*.go" {
		t.Errorf("triggers perdidos o mal: %+v — una skill sin triggers NO dispara nunca", got.Triggers)
	}
	if len(got.Capabilities) != 1 || got.Capabilities[0] != "go" {
		t.Errorf("capabilities perdidas: %+v", got.Capabilities)
	}
}

// TestF2SinCentralFallaDiciendoPorQue — un promote que "anda" sin subir nada es el peor caso.
func TestF2SinCentralFallaDiciendoPorQue(t *testing.T) {
	s, root := servidorConCentral(t, nil) // sin central enchufado
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	for _, tool := range []string{"musubi_promote_skill", "musubi_install_skill"} {
		out, rerr := call(t, s, tool, map[string]interface{}{"name": "revisar-go"})
		if rerr == nil {
			t.Fatalf("%s debía fallar sin central configurado, devolvió %v", tool, out)
		}
		if !strings.Contains(strings.ToLower(rerr.Message), "central") {
			t.Errorf("%s: el error debería nombrar al central para que se sepa qué falta: %q", tool, rerr.Message)
		}
	}
}

// TestF3InstalarNoEscapaDelDirectorioDeSkills es EL INVARIANTE DE SEGURIDAD.
func TestF3InstalarNoEscapaDelDirectorioDeSkills(t *testing.T) {
	for _, malicioso := range []string{"../evil", "../../evil", "sub/evil", `..\evil`, "/etc/evil"} {
		t.Run(malicioso, func(t *testing.T) {
			mala := skillDelArsenal(malicioso)
			central := &centralFalso{arsenal: []skillPayload{mala}}
			s, root := servidorConCentral(t, central)

			_, rerr := call(t, s, "musubi_install_skill", map[string]interface{}{"name": malicioso})
			if rerr == nil {
				t.Fatalf("instalar %q debía rechazarse", malicioso)
			}
			// Y sobre todo: NADA escrito fuera del directorio de skills.
			escapado := filepath.Join(filepath.Dir(root), "evil.yaml")
			if _, err := os.Stat(escapado); err == nil {
				t.Fatalf("FUGA: se escribió fuera del proyecto en %s", escapado)
			}
		})
	}
}

// TestF4LaSkillAdoptadaSeDistingueDeLaPropia
func TestF4LaSkillAdoptadaSeDistingueDeLaPropia(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{skillDelArsenal("go-table-driven-tests")}}
	s, root := servidorConCentral(t, central)

	if _, rerr := call(t, s, "musubi_install_skill", map[string]interface{}{"name": "go-table-driven-tests"}); rerr != nil {
		t.Fatalf("install falló: %+v", rerr)
	}
	data, err := os.ReadFile(filepath.Join(root, config.DirName, config.SkillsDir, "go-table-driven-tests.yaml"))
	if err != nil {
		t.Fatalf("la skill no quedó escrita: %v", err)
	}
	texto := string(data)
	if !strings.Contains(texto, arsenalSource) {
		t.Errorf("la skill adoptada no quedó marcada con %q; no se puede distinguir de una propia:\n%s", arsenalSource, texto)
	}
	// El rastro al origen se preserva: source_url es el enlace al catálogo del que salió.
	if !strings.Contains(texto, "go.dev/wiki/TableDrivenTests") {
		t.Errorf("se perdió el source_url, que es el rastro al origen:\n%s", texto)
	}
	// Control: una skill escrita a mano NO lleva la marca, si no la marca no distingue nada.
	escribirSkill(t, root, "propia.yaml", skillLocal)
	propia, _ := os.ReadFile(filepath.Join(root, config.DirName, config.SkillsDir, "propia.yaml"))
	if strings.Contains(string(propia), arsenalSource) {
		t.Error("una skill propia no debería llevar la marca de adoptada")
	}
}

// TestF5SinOverwriteNadaSePisa
func TestF5SinOverwriteNadaSePisa(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{skillDelArsenal("revisar-go")}}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)
	ruta := filepath.Join(root, config.DirName, config.SkillsDir, "revisar-go.yaml")
	antes, _ := os.ReadFile(ruta)

	if _, rerr := call(t, s, "musubi_install_skill", map[string]interface{}{"name": "revisar-go"}); rerr == nil {
		t.Fatal("instalar sobre una skill existente debía rechazarse sin overwrite")
	}
	despues, _ := os.ReadFile(ruta)
	if string(antes) != string(despues) {
		t.Error("el archivo local se modificó pese al rechazo")
	}

	// Control: con overwrite=true SÍ se reemplaza. Sin este caso, una implementación que
	// rechaza SIEMPRE pasaría el invariante y rompería la función.
	if _, rerr := call(t, s, "musubi_install_skill", map[string]interface{}{"name": "revisar-go", "overwrite": true}); rerr != nil {
		t.Fatalf("con overwrite=true debía instalar: %+v", rerr)
	}
	final, _ := os.ReadFile(ruta)
	if string(final) == string(antes) {
		t.Error("con overwrite=true el archivo debía reemplazarse")
	}
}

// TestF6PromoverLoQueNoExisteFalla — no se sube un esqueleto.
func TestF6PromoverLoQueNoExisteFalla(t *testing.T) {
	central := &centralFalso{}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	if _, rerr := call(t, s, "musubi_promote_skill", map[string]interface{}{"name": "no-existe"}); rerr == nil {
		t.Fatal("promover una skill inexistente debía fallar")
	}
	central.mu.Lock()
	tool := central.tool
	central.mu.Unlock()
	if tool != "" {
		t.Errorf("no debía llamarse al central para una skill inexistente, se llamó %q", tool)
	}

	// Control: la que SÍ existe se promueve, así el test no pasa por "nunca promueve nada".
	if _, rerr := call(t, s, "musubi_promote_skill", map[string]interface{}{"name": "revisar-go"}); rerr != nil {
		t.Fatalf("la skill existente debía promoverse: %+v", rerr)
	}
}

// TestF7NingunaDeLasDosEsReadOnly — las dos escriben.
func TestF7NingunaDeLasDosEsReadOnly(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	vistas := map[string]bool{}
	for i := range s.tools {
		n := s.tools[i].Name
		if n != "musubi_promote_skill" && n != "musubi_install_skill" {
			continue
		}
		vistas[n] = true
		if s.tools[i].readOnly {
			t.Errorf("%s escribe y NO debe estar marcada readOnly", n)
		}
	}
	if len(vistas) != 2 {
		t.Fatalf("faltan tools registradas: %v", vistas)
	}
}

// TestFetchSkillExigeNombreExacto — list_skills filtra por SUBCADENA; sin match exacto,
// instalar "go" bajaría cualquier pariente.
func TestFetchSkillExigeNombreExacto(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{skillDelArsenal("go-table-driven-tests")}}
	s, _ := servidorConCentral(t, central)

	if _, rerr := call(t, s, "musubi_install_skill", map[string]interface{}{"name": "go"}); rerr == nil {
		t.Fatal(`pedir "go" no debía instalar "go-table-driven-tests"`)
	}
	// Control: el nombre exacto sí instala.
	if _, rerr := call(t, s, "musubi_install_skill", map[string]interface{}{"name": "go-table-driven-tests"}); rerr != nil {
		t.Fatalf("el nombre exacto debía instalar: %+v", rerr)
	}
}

// TestUnErrorDelCentralNoSeTraga — si el central rechaza, la tool lo dice.
func TestUnErrorDelCentralNoSeTraga(t *testing.T) {
	central := &centralFalso{errorTool: true}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	if _, rerr := call(t, s, "musubi_promote_skill", map[string]interface{}{"name": "revisar-go"}); rerr == nil {
		t.Fatal("un rechazo del central debía propagarse, no tragarse")
	}
}

var _ = skills.Skill{} // el paquete se usa en la firma de PushSkill/FetchSkill
