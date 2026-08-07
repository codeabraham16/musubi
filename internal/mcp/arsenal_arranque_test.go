package mcp

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// arsenal_arranque_test.go sella el contrato de la Fase B (spec «arsenal-arranque», B1): poder
// VER el arsenal del central desde cualquier proyecto.
//
// El invariante que más importa es G5. Sin central configurado, listar el arsenal tiene que
// FALLAR: devolver la lista local en su lugar se lee como un arsenal vacío, y la conclusión de
// quien mira es «no hay nada que instalar». Es la misma falla silenciosa que este track viene
// cazando desde la Fase A, pero del lado de la lectura, donde no deja rastro.

// listarArsenal decodifica la respuesta de musubi_list_skills en el DTO real.
func listarArsenal(t *testing.T, s *McpServer, args map[string]interface{}) []skillListada {
	t.Helper()
	var out []skillListada
	if err := json.Unmarshal([]byte(listar(t, s, args)), &out); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	return out
}

func porNombre(lista []skillListada, nombre string) *skillListada {
	for i := range lista {
		if lista[i].Name == nombre {
			return &lista[i]
		}
	}
	return nil
}

// TestG1ElDefaultNoCambiaNada — la tool está en producción y la Forja del cuerpo la consume.
func TestG1ElDefaultNoCambiaNada(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{skillDelArsenal("go-table-driven-tests")}}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	lista := listarArsenal(t, s, map[string]interface{}{})
	if len(lista) != 1 || lista[0].Name != "revisar-go" {
		t.Fatalf("el default debe listar SÓLO lo local, obtuve %+v", lista)
	}
	if lista[0].Origin != fuenteLocal {
		t.Errorf("origin de una entrada local = %q, esperaba %q", lista[0].Origin, fuenteLocal)
	}

	// Y sobre todo: ni una llamada al central. Que el default empiece a salir a la red la
	// vuelve lenta y falible sin que nadie lo haya pedido.
	central.mu.Lock()
	tool := central.tool
	central.mu.Unlock()
	if tool != "" {
		t.Errorf("el default salió a la red (llamó a %q); debe leer sólo el disco", tool)
	}
}

// TestG2CentralListaElArsenalNoElDisco — el proyecto está VACÍO a propósito: si la tool
// estuviera leyendo el disco y llamándolo arsenal, acá devolvería cero.
func TestG2CentralListaElArsenalNoElDisco(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{
		skillDelArsenal("go-table-driven-tests"),
		skillDelArsenal("revisar-prs"),
	}}
	s, _ := servidorConCentral(t, central)

	lista := listarArsenal(t, s, map[string]interface{}{"source": fuenteCentral})
	if len(lista) != 2 {
		t.Fatalf("esperaba las 2 del arsenal, obtuve %d: %+v", len(lista), lista)
	}
	for _, e := range lista {
		if e.Origin != fuenteCentral {
			t.Errorf("%s: origin=%q, esperaba %q", e.Name, e.Origin, fuenteCentral)
		}
		// La skill tiene que llegar COMPLETA: una entrada sin triggers ni rules no se distingue
		// de una rota, y es exactamente la trampa que ya mordió en list_skills (arsenal-visible A1).
		if len(e.Triggers) == 0 || e.Rules == "" {
			t.Errorf("%s llegó incompleta del arsenal: triggers=%v rules=%q", e.Name, e.Triggers, e.Rules)
		}
	}
}

// TestG3ElArsenalDiceSiYaLaTenes — mirando un arsenal uno se pregunta «qué me falta», no «qué existe».
func TestG3ElArsenalDiceSiYaLaTenes(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{
		skillDelArsenal("revisar-go"),            // ésta SÍ está en el proyecto
		skillDelArsenal("go-table-driven-tests"), // ésta no
	}}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	lista := listarArsenal(t, s, map[string]interface{}{"source": fuenteCentral})
	ya, falta := porNombre(lista, "revisar-go"), porNombre(lista, "go-table-driven-tests")
	if ya == nil || falta == nil {
		t.Fatalf("faltan entradas en el listado: %+v", lista)
	}
	if ya.Installed == nil || !*ya.Installed {
		t.Errorf("revisar-go está en el proyecto y debía venir installed:true, vino %v", ya.Installed)
	}
	if falta.Installed == nil || *falta.Installed {
		t.Errorf("go-table-driven-tests NO está y debía venir installed:false, vino %v", falta.Installed)
	}

	// En las entradas locales el campo se OMITE. Un `false` ahí sería directamente falso, y un
	// `true` trivial sólo agrega ruido.
	if crudo := listar(t, s, map[string]interface{}{"source": fuenteLocal}); strings.Contains(crudo, "installed") {
		t.Errorf("las entradas locales no deben traer 'installed': %s", crudo)
	}
}

// TestG4AllNoDuplicaLoQueYaTenes — `all` responde «qué tengo y qué MÁS podría tener».
func TestG4AllNoDuplicaLoQueYaTenes(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{
		skillDelArsenal("revisar-go"),
		skillDelArsenal("go-table-driven-tests"),
	}}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	lista := listarArsenal(t, s, map[string]interface{}{"source": fuenteTodas})
	if len(lista) != 2 {
		t.Fatalf("esperaba 2 entradas (1 local + 1 faltante del arsenal), obtuve %d: %+v", len(lista), lista)
	}
	if local := porNombre(lista, "revisar-go"); local == nil || local.Origin != fuenteLocal {
		t.Errorf("revisar-go debía aparecer UNA vez y como local, obtuve %+v", local)
	}
	// Control: lo que falta SÍ viene, marcado del central. Sin esto el test pasaría con un
	// `all` que simplemente ignora el arsenal — que también da 2 entradas si hay 2 locales.
	if falta := porNombre(lista, "go-table-driven-tests"); falta == nil || falta.Origin != fuenteCentral {
		t.Errorf("all debe aportar lo que falta del arsenal, obtuve %+v", falta)
	}
}

// TestG5SinCentralListarElArsenalFalla es EL INVARIANTE de esta fase.
func TestG5SinCentralListarElArsenalFalla(t *testing.T) {
	s, root := servidorConCentral(t, nil) // sin central enchufado
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	for _, fuente := range []string{fuenteCentral, fuenteTodas} {
		out, rerr := call(t, s, "musubi_list_skills", map[string]interface{}{"source": fuente})
		if rerr == nil {
			t.Fatalf("source=%s debía fallar sin central; devolvió %v, que se lee como un arsenal vacío", fuente, out)
		}
		if !strings.Contains(strings.ToLower(rerr.Message), "central") {
			t.Errorf("source=%s: el error debe nombrar al central para que se sepa qué falta: %q", fuente, rerr.Message)
		}
	}

	// Control: `local` SIGUE funcionando sin central. Sin esto, el test también pasaría con una
	// tool que quedó rota entera.
	if lista := listarArsenal(t, s, map[string]interface{}{}); len(lista) != 1 {
		t.Errorf("source=local debe funcionar sin central, obtuve %+v", lista)
	}
}

// TestG6UnSourceInvalidoNoCaeAlDefault — un typo que degrada a `local` produce la mentira de G5
// por accidente, que es peor porque nadie la busca.
func TestG6UnSourceInvalidoNoCaeAlDefault(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{skillDelArsenal("go-table-driven-tests")}}
	s, root := servidorConCentral(t, central)
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	for _, malo := range []string{"centrall", "remoto", "arsenal", "todas"} {
		out, rerr := call(t, s, "musubi_list_skills", map[string]interface{}{"source": malo})
		if rerr == nil {
			t.Errorf("source=%q debía ser error; devolvió %v", malo, out)
		}
	}

	// Control: la tolerancia de FORMA sí está (mayúsculas y espacios), para que el rechazo de
	// arriba sea por el valor y no por ser quisquilloso con el formato.
	if lista := listarArsenal(t, s, map[string]interface{}{"source": "  CENTRAL "}); len(lista) != 1 {
		t.Errorf("source con espacios y mayúsculas debe aceptarse, obtuve %+v", lista)
	}
}

// TestG7ListSkillsSigueSiendoReadOnly — lee el disco y hace una lectura remota; no escribe.
func TestG7ListSkillsSigueSiendoReadOnly(t *testing.T) {
	s := NewMcpServer(nil, "", nil)
	if !s.toolReadOnly["musubi_list_skills"] {
		t.Error("musubi_list_skills dejó de estar clasificada readOnly")
	}
}

// TestArsenalVacioNoEsLoMismoQueCentralRoto — "[]" es un arsenal vacío legítimo; "null" es un
// central que no entiende la tool (versión vieja, sin el make()). Confundirlos hace reportar
// «no hay skills para instalar» cuando el problema real es el deploy del otro lado.
func TestArsenalVacioNoEsLoMismoQueCentralRoto(t *testing.T) {
	vacio := &centralFalso{arsenal: []skillPayload{}}
	s, _ := servidorConCentral(t, vacio)
	if lista := listarArsenal(t, s, map[string]interface{}{"source": fuenteCentral}); len(lista) != 0 {
		t.Errorf("un arsenal vacío debe dar lista vacía SIN error, obtuve %+v", lista)
	}

	roto := &centralFalso{arsenal: nil} // json.Marshal de un slice nil => "null"
	s2, _ := servidorConCentral(t, roto)
	if _, rerr := call(t, s2, "musubi_list_skills", map[string]interface{}{"source": fuenteCentral}); rerr == nil {
		t.Error("un central que devuelve null debe fallar, no hacerse pasar por un arsenal vacío")
	}
}

// ---------------------------------------------------------------------------
// B2 — el arsenal aterriza en el proyecto (`musubi provision --skills`)
// ---------------------------------------------------------------------------

func rutaSkill(root, nombre string) string {
	return filepath.Join(root, config.DirName, config.SkillsDir, nombre+".yaml")
}

// TestG9ElArsenalAterrizaEnElProyecto — con las reglas completas, no un esqueleto.
func TestG9ElArsenalAterrizaEnElProyecto(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{
		skillDelArsenal("go-table-driven-tests"),
		skillDelArsenal("revisar-prs"),
	}}
	s, root := servidorConCentral(t, central)

	rep, err := s.installArsenal(false)
	if err != nil {
		t.Fatalf("installArsenal falló: %v", err)
	}
	if len(rep.Instaladas) != 2 {
		t.Fatalf("esperaba 2 instaladas, obtuve %+v", rep)
	}
	for _, nombre := range []string{"go-table-driven-tests", "revisar-prs"} {
		data, rerr := os.ReadFile(rutaSkill(root, nombre))
		if rerr != nil {
			t.Fatalf("%s no quedó escrita: %v", nombre, rerr)
		}
		texto := string(data)
		// Una skill que llega sin sus triggers no dispara nunca y se ve como si estuviera
		// instalada: la falla silenciosa que este track viene arreglando.
		if !strings.Contains(texto, "*_test.go") {
			t.Errorf("%s quedó sin sus triggers:\n%s", nombre, texto)
		}
		if !strings.Contains(texto, arsenalSource) {
			t.Errorf("%s no quedó marcada como adoptada:\n%s", nombre, texto)
		}
	}
}

// TestG10ElArsenalNoPisaLoLocal — provision es idempotente y se corre varias veces.
func TestG10ElArsenalNoPisaLoLocal(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{
		skillDelArsenal("revisar-go"), // choca con una local editada a mano
		skillDelArsenal("go-table-driven-tests"),
	}}
	s, root := servidorConCentral(t, central)
	ruta := escribirSkill(t, root, "revisar-go.yaml", skillLocal)
	antes, _ := os.ReadFile(ruta)

	rep, err := s.installArsenal(false)
	if err != nil {
		t.Fatalf("installArsenal falló: %v", err)
	}
	if len(rep.Salteadas) != 1 || rep.Salteadas[0] != "revisar-go" {
		t.Errorf("revisar-go debía reportarse salteada, obtuve %+v", rep)
	}
	despues, _ := os.ReadFile(ruta)
	if !bytes.Equal(antes, despues) {
		t.Errorf("se pisó una skill local; eso es pérdida silenciosa de trabajo:\n%s", despues)
	}

	// Control: la que faltaba SÍ se instaló. Sin esto el test pasaría con un install que no
	// hace absolutamente nada.
	if len(rep.Instaladas) != 1 || rep.Instaladas[0] != "go-table-driven-tests" {
		t.Errorf("la skill faltante debía instalarse, obtuve %+v", rep.Instaladas)
	}
}

// TestG11DryRunNoEscribe — informa qué traería, sin tocar el disco.
func TestG11DryRunNoEscribe(t *testing.T) {
	central := &centralFalso{arsenal: []skillPayload{skillDelArsenal("go-table-driven-tests")}}
	s, root := servidorConCentral(t, central)

	rep, err := s.installArsenal(true)
	if err != nil {
		t.Fatalf("installArsenal(dry-run) falló: %v", err)
	}
	if len(rep.Instaladas) != 1 {
		t.Errorf("dry-run debe INFORMAR qué instalaría, obtuve %+v", rep)
	}
	if _, serr := os.Stat(rutaSkill(root, "go-table-driven-tests")); serr == nil {
		t.Error("dry-run escribió en disco")
	}

	// Control: sin dry-run sí escribe, para que el test de arriba no pase por estar todo roto.
	if _, err := s.installArsenal(false); err != nil {
		t.Fatalf("installArsenal falló: %v", err)
	}
	if _, serr := os.Stat(rutaSkill(root, "go-table-driven-tests")); serr != nil {
		t.Errorf("sin dry-run debía escribir: %v", serr)
	}
}

// TestG12ElArsenalNoAbreUnaSegundaPuerta es EL INVARIANTE DE SEGURIDAD de la Fase B.
//
// El camino nuevo es exactamente donde se cuela una segunda puerta de escritura, con la excusa
// de que «total ya se valida al instalar de a una».
func TestG12ElArsenalNoAbreUnaSegundaPuerta(t *testing.T) {
	for _, malicioso := range []string{"../evil", "../../evil", "sub/evil", `..\evil`, "/etc/evil"} {
		t.Run(malicioso, func(t *testing.T) {
			central := &centralFalso{arsenal: []skillPayload{
				skillDelArsenal(malicioso),
				skillDelArsenal("go-table-driven-tests"),
			}}
			s, root := servidorConCentral(t, central)

			rep, err := s.installArsenal(false)
			if err != nil {
				t.Fatalf("installArsenal falló entero: %v", err)
			}
			// Lo que más importa: NADA escrito con ese nombre, en ningún lado. Se barre el
			// árbol entero en vez de mirar una ruta puntual porque cada payload malicioso
			// aterriza a distinta profundidad, y una aserción de ruta fija sólo caza a uno.
			_ = filepath.WalkDir(filepath.Dir(root), func(p string, d fs.DirEntry, werr error) error {
				if werr != nil || d.IsDir() {
					return nil //nolint:nilerr // un directorio ilegible no es una fuga
				}
				if strings.HasPrefix(d.Name(), "evil") {
					t.Errorf("FUGA: %q se escribió en %s", malicioso, p)
				}
				return nil
			})
			if len(rep.Fallidas) != 1 || rep.Fallidas[0] != malicioso {
				t.Errorf("%q debía quedar en Fallidas y no tragarse, obtuve %+v", malicioso, rep)
			}
			// Control: la sana se instala igual. Una entrada maliciosa no puede abortar el
			// resto del arsenal en silencio.
			if len(rep.Instaladas) != 1 || rep.Instaladas[0] != "go-table-driven-tests" {
				t.Errorf("la skill sana debía instalarse igual, obtuve %+v", rep.Instaladas)
			}
		})
	}
}
