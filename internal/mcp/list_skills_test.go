package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// list_skills_test.go sella el contrato de musubi_list_skills (spec «arsenal-visible», F5.1).
//
// El invariante fundamental es A1 —las claves del JSON— porque su violacion NO da error: un
// cliente que parsea en minuscula recibiria un array del largo correcto con todos los campos
// vacios, y el panel del cuerpo pasaria de "vacio" a "N filas en blanco". Un bug que parece
// funcionar es peor que el que este PR viene a arreglar.

// escribirSkill deja una skill en .musubi/skills/ del proyecto y devuelve su ruta.
func escribirSkill(t *testing.T, root, archivo, contenido string) string {
	t.Helper()
	dir := filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("no se pudo crear el directorio de skills: %v", err)
	}
	path := filepath.Join(dir, archivo)
	if err := os.WriteFile(path, []byte(contenido), 0644); err != nil {
		t.Fatalf("no se pudo escribir %s: %v", archivo, err)
	}
	return path
}

// listar invoca la tool y devuelve el texto crudo de la respuesta (sin parsear, para que los
// tests de forma del JSON puedan mirar las claves reales).
func listar(t *testing.T, s *McpServer, args map[string]interface{}) string {
	t.Helper()
	out, rerr := call(t, s, "musubi_list_skills", args)
	if rerr != nil {
		t.Fatalf("musubi_list_skills fallo: %+v", rerr)
	}
	resp, ok := out.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %#v", out)
	}
	return resp.Content[0].Text
}

const skillDeEjemplo = `name: revisar-go
description: Como revisar codigo Go en este repo
triggers:
  - "*.go"
capabilities:
  - go
source: catalogo-v1
source_url: https://ejemplo.invalid/revisar-go
rules: |
  Correr go vet antes de proponer cualquier cambio en Go.
`

// TestA1LasClavesSonLasQueElCuerpoParsea es el invariante fundamental: si las claves cambian de
// caja, el consumidor no falla — muestra filas vacias.
func TestA1LasClavesSonLasQueElCuerpoParsea(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	escribirSkill(t, root, "revisar-go.yaml", skillDeEjemplo)

	txt := listar(t, s, nil)

	// 1) Las claves exactas, en minuscula, tal como las desserializa internal/brain del cuerpo.
	for _, clave := range []string{
		`"name"`, `"description"`, `"triggers"`, `"capabilities"`,
		`"source"`, `"source_url"`, `"rules"`,
	} {
		if !strings.Contains(txt, clave) {
			t.Errorf("falta la clave %s en la respuesta: %s", clave, txt)
		}
	}
	// 2) Y NINGUNA con el nombre de campo de Go, que es lo que saldria al serializar
	//    skills.Skill directo (solo tiene tags YAML).
	for _, prohibida := range []string{`"Name"`, `"Description"`, `"SourceURL"`, `"Rules"`} {
		if strings.Contains(txt, prohibida) {
			t.Errorf("clave %s con el nombre de campo de Go: se serializo skills.Skill directo en vez del DTO\nrespuesta: %s", prohibida, txt)
		}
	}

	// 3) La prueba de fondo: un cliente que parsea en minuscula recibe los VALORES, no vacios.
	var got []struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Triggers     []string `json:"triggers"`
		Capabilities []string `json:"capabilities"`
		Source       string   `json:"source"`
		SourceURL    string   `json:"source_url"`
		Rules        string   `json:"rules"`
	}
	if err := json.Unmarshal([]byte(txt), &got); err != nil {
		t.Fatalf("un cliente no pudo parsear la respuesta: %v\n%s", err, txt)
	}
	if len(got) != 1 {
		t.Fatalf("esperaba 1 skill, obtuve %d: %s", len(got), txt)
	}
	g := got[0]
	if g.Name != "revisar-go" {
		t.Errorf("name vacio o incorrecto: %q", g.Name)
	}
	if g.Description == "" || g.Source != "catalogo-v1" || g.SourceURL == "" || g.Rules == "" {
		t.Errorf("campos vacios tras parsear en minuscula: %+v", g)
	}
	if len(g.Triggers) != 1 || g.Triggers[0] != "*.go" {
		t.Errorf("triggers mal: %+v", g.Triggers)
	}
	if len(g.Capabilities) != 1 || g.Capabilities[0] != "go" {
		t.Errorf("capabilities mal: %+v", g.Capabilities)
	}
}

// TestA2SinSkillsDevuelveArrayVacioNoNull — `var` en vez de `make` produce `null`, y el contrato
// de una tool que lista es devolver una lista.
func TestA2SinSkillsDevuelveArrayVacioNoNull(t *testing.T) {
	// Sin directorio de skills siquiera.
	s := newTestServerWithPath(t, t.TempDir())
	if txt := strings.TrimSpace(listar(t, s, nil)); txt != "[]" {
		t.Errorf("sin directorio esperaba `[]`, obtuve %q", txt)
	}

	// Con directorio pero vacio.
	root := t.TempDir()
	s2 := newTestServerWithPath(t, root)
	if err := os.MkdirAll(filepath.Join(root, config.DirName, config.SkillsDir), 0755); err != nil {
		t.Fatal(err)
	}
	if txt := strings.TrimSpace(listar(t, s2, nil)); txt != "[]" {
		t.Errorf("con directorio vacio esperaba `[]`, obtuve %q", txt)
	}

	// Y con un query que no matchea nada (el filtro no debe devolver null tampoco).
	root3 := t.TempDir()
	s3 := newTestServerWithPath(t, root3)
	escribirSkill(t, root3, "revisar-go.yaml", skillDeEjemplo)
	if txt := strings.TrimSpace(listar(t, s3, map[string]interface{}{"query": "nada-que-ver"})); txt != "[]" {
		t.Errorf("con query sin matches esperaba `[]`, obtuve %q", txt)
	}
}

// TestA3QueryFiltraPorNombreYDescripcionSinCaja
func TestA3QueryFiltraPorNombreYDescripcionSinCaja(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	escribirSkill(t, root, "revisar-go.yaml", skillDeEjemplo)
	escribirSkill(t, root, "desplegar.yaml", "name: desplegar\ndescription: Pasos de DESPLIEGUE al server\ntriggers: [\"*.sh\"]\nrules: reglas de despliegue\n")

	nombres := func(args map[string]interface{}) []string {
		var got []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(listar(t, s, args)), &got); err != nil {
			t.Fatalf("respuesta ilegible: %v", err)
		}
		out := make([]string, 0, len(got))
		for _, g := range got {
			out = append(out, g.Name)
		}
		return out
	}

	// Control: sin query vienen las dos. Sin este control, un filtro roto que no devuelve
	// nunca nada haria pasar los casos de abajo por el motivo equivocado.
	if n := nombres(nil); len(n) != 2 {
		t.Fatalf("control: esperaba 2 skills sin query, obtuve %v", n)
	}
	// Por nombre.
	if n := nombres(map[string]interface{}{"query": "revisar"}); len(n) != 1 || n[0] != "revisar-go" {
		t.Errorf("query por nombre: %v", n)
	}
	// Por DESCRIPCION, y con la caja cambiada respecto del archivo ("DESPLIEGUE").
	if n := nombres(map[string]interface{}{"query": "despliegue"}); len(n) != 1 || n[0] != "desplegar" {
		t.Errorf("query por descripcion sin distinguir mayusculas: %v", n)
	}
	// Y al reves: query en mayuscula contra un nombre en minuscula.
	if n := nombres(map[string]interface{}{"query": "REVISAR-GO"}); len(n) != 1 || n[0] != "revisar-go" {
		t.Errorf("query en mayusculas: %v", n)
	}
}

// TestA4LimitRecortaYCeroNoRecorta
func TestA4LimitRecortaYCeroNoRecorta(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	for _, n := range []string{"uno", "dos", "tres"} {
		escribirSkill(t, root, n+".yaml", "name: "+n+"\ndescription: skill "+n+"\ntriggers: [\"*.go\"]\nrules: reglas de "+n+"\n")
	}

	cuantas := func(args map[string]interface{}) int {
		var got []json.RawMessage
		if err := json.Unmarshal([]byte(listar(t, s, args)), &got); err != nil {
			t.Fatalf("respuesta ilegible: %v", err)
		}
		return len(got)
	}

	if n := cuantas(nil); n != 3 {
		t.Fatalf("control: esperaba 3 sin limit, obtuve %d", n)
	}
	if n := cuantas(map[string]interface{}{"limit": 2}); n != 2 {
		t.Errorf("limit=2 deberia recortar a 2, obtuve %d", n)
	}
	if n := cuantas(map[string]interface{}{"limit": 0}); n != 3 {
		t.Errorf("limit=0 NO deberia recortar, obtuve %d", n)
	}
	if n := cuantas(map[string]interface{}{"limit": -1}); n != 3 {
		t.Errorf("limit negativo NO deberia recortar, obtuve %d", n)
	}
	if n := cuantas(map[string]interface{}{"limit": 99}); n != 3 {
		t.Errorf("limit mayor al total deberia devolver todo, obtuve %d", n)
	}
}

// TestA6UnYamlRotoNoTumbaLaLista — el arsenal no se cae entero por un archivo mal escrito.
func TestA6UnYamlRotoNoTumbaLaLista(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	escribirSkill(t, root, "revisar-go.yaml", skillDeEjemplo)
	escribirSkill(t, root, "rota.yaml", "esto: [no es: yaml valido\n  - {{{\n")
	escribirSkill(t, root, "sin-nombre.yaml", "description: le falta el name\nrules: algo\n")
	escribirSkill(t, root, "ignorado.txt", "ni siquiera es yaml")

	var got []struct {
		Name string `json:"name"`
	}
	txt := listar(t, s, nil)
	if err := json.Unmarshal([]byte(txt), &got); err != nil {
		t.Fatalf("la lista no sobrevivio a un YAML roto: %v\n%s", err, txt)
	}
	if len(got) != 1 || got[0].Name != "revisar-go" {
		t.Errorf("esperaba solo la skill valida, obtuve %+v", got)
	}
}

// TestA5ListSkillsEsReadOnly — la clasificacion del Track 19 la hace TestEveryReadOnlyToolClassified;
// aca se sella que la tool esta registrada como readOnly, que es la premisa de esa clasificacion.
func TestA5ListSkillsEsReadOnly(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	for i := range s.tools {
		if s.tools[i].Name == "musubi_list_skills" {
			if !s.tools[i].readOnly {
				t.Error("musubi_list_skills deberia estar registrada como readOnly")
			}
			return
		}
	}
	t.Fatal("musubi_list_skills no esta registrada")
}
