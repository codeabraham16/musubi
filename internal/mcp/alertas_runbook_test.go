package mcp

// alertas_runbook_test.go mata una clase de drift que no rompe ninguna compilación y que sólo se
// descubre en el peor momento: UNA ALERTA QUE MANDA A UN RUNBOOK QUE NO EXISTE.
//
// El aviso llega a las 4 de la mañana con un `runbook: deploy/RUNBOOK.md#loquesea`, alguien abre
// el archivo, no encuentra la sección, y el minuto que iba a ahorrar la anotación se convierte en
// diez de buscar. Es exactamente el tipo de cosa que se rompe al renombrar una alerta y que
// ninguna prueba de Go tocaría — así que se prueba desde acá, que es donde vive la suite.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestCadaRunbookDeUnaAlertaApuntaAUnaSeccionQueExiste recorre las reglas y verifica el ancla.
//
// Sabotaje que la hace fallar: renombrar una sección del RUNBOOK sin tocar la regla que la cita.
func TestCadaRunbookDeUnaAlertaApuntaAUnaSeccionQueExiste(t *testing.T) {
	reglas := []byte(reglasDeAlerta(t))
	runbook, err := os.ReadFile("../../deploy/RUNBOOK.md")
	if err != nil {
		t.Fatalf("no se pudo leer el runbook: %v", err)
	}

	// Las anclas de GitHub: el título en minúsculas, sin lo que no sea alfanumérico ni espacio,
	// y con los espacios como guiones.
	anclas := map[string]bool{}
	reTitulo := regexp.MustCompile(`(?m)^#{2,3}\s+(.+?)\s*$`)
	limpiar := regexp.MustCompile(`[^a-z0-9\s-]`)
	for _, m := range reTitulo.FindAllStringSubmatch(string(runbook), -1) {
		a := limpiar.ReplaceAllString(strings.ToLower(m[1]), "")
		anclas[strings.ReplaceAll(strings.TrimSpace(a), " ", "-")] = true
	}
	if len(anclas) < 5 {
		t.Fatalf("sólo se detectaron %d secciones en el runbook; el parser de títulos se rompió y la prueba no probaría nada", len(anclas))
	}

	reRunbook := regexp.MustCompile(`runbook:\s*"?deploy/RUNBOOK\.md#([a-z0-9-]+)`)
	citas := reRunbook.FindAllStringSubmatch(string(reglas), -1)
	if len(citas) == 0 {
		t.Fatal("ninguna regla cita el runbook: o se borraron todas las anotaciones, o el patrón cambió")
	}
	for _, m := range citas {
		if !anclas[m[1]] {
			t.Errorf("una alerta manda a deploy/RUNBOOK.md#%s y esa sección no existe: el operador va a llegar a un archivo que no le dice nada", m[1])
		}
	}
}

// reglasDeAlerta junta los CUATRO archivos de reglas.
//
// Se repartieron a propósito, y cada uno se instala bajo su propia condición: las de flota sólo
// si el cerebro expone `musubi_fleet_*`, la del backup off-host sólo si hay `BACKUP_REMOTE`.
// Cargar una regla cuya precondición no se cumple no es inocuo — `absent()` y el centinela `-1`
// disparan desde el día uno, y una alarma que no se apaga arreglando algo enseña a ignorar el
// canal entero. Ver deploy/docker/preparar.sh.
//
// Pero las GUARDAS de este archivo valen para todas por igual: un nombre repetido o una severidad
// ausente rompen lo mismo estén donde estén. Por eso se leen juntas acá, y por eso la lista está
// escrita a mano: si aparece un cuarto archivo de reglas y nadie lo agrega, esta función lo
// ignora en silencio — así que la cuenta mínima de abajo es la que lo delata.
func reglasDeAlerta(t *testing.T) string {
	t.Helper()
	var todo strings.Builder
	for _, f := range []string{
		"../../deploy/musubi-alerts.yml",
		"../../deploy/musubi-alerts-flota.yml",
		"../../deploy/musubi-alerts-backup-offhost.yml",
		"../../deploy/musubi-alerts-altura.yml",
	} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("falta un archivo de reglas (%s): %v", f, err)
		}
		todo.Write(b)
		todo.WriteString("\n")
	}
	return todo.String()
}

// TestCadaAlertaDeFlotaTieneNombreYSeveridadUnicos evita dos errores mudos distintos.
//
// Un `alert:` repetido hace que Prometheus cargue las dos y las agrupe como si fueran la misma —
// una tapa a la otra. Y una alerta sin `severity` no matchea ninguna ruta de Alertmanager salvo
// la default: se envía por el canal equivocado, o con la cadencia equivocada, sin que nada falle.
//
// Sabotaje que la hace fallar: duplicar el nombre de una alerta, o quitarle su severity.
func TestLasAlertasTienenNombreUnicoYSeveridadDeclarada(t *testing.T) {
	texto := reglasDeAlerta(t)

	nombres := regexp.MustCompile(`(?m)^\s*-\s+alert:\s*(\S+)`).FindAllStringSubmatch(texto, -1)
	if len(nombres) < 10 {
		t.Fatalf("se detectaron %d alertas; el patrón se rompió", len(nombres))
	}
	vistos := map[string]bool{}
	for _, m := range nombres {
		if vistos[m[1]] {
			t.Errorf("la alerta %q está declarada dos veces: Prometheus las agrupa y una tapa a la otra", m[1])
		}
		vistos[m[1]] = true
	}

	// Cada bloque de alerta tiene que declarar su severidad. Se parte por `- alert:` y se mira
	// cada trozo, que es más simple que un parser de YAML y no le agrega una dependencia al repo.
	bloques := strings.Split(texto, "- alert:")[1:]
	for i, bloque := range bloques {
		nombre := strings.Fields(bloque)[0]
		if !strings.Contains(bloque, "severity:") {
			t.Errorf("la alerta %q (bloque %d) no declara `severity`: no va a matchear ninguna ruta de Alertmanager y va a salir por el canal equivocado", nombre, i)
		}
	}
}

// El dead-man's switch tiene que existir Y estar SIEMPRE en firing.
//
// Es la única regla del archivo cuyo valor está en no dispararse nunca como problema: si alguien
// la "arregla" poniéndole una condición, deja de ser un latido y la cadena de alertas vuelve a
// tener el punto ciego que S10 vino a cerrar (I18).
func TestElDeadMansSwitchSigueSiendoIncondicional(t *testing.T) {
	// A propósito lee SÓLO musubi-alerts.yml, no los tres. El latido es la única regla que se
	// instala SIEMPRE, sin precondición: si alguien lo moviera a un archivo condicional, habría
	// despliegues sin latido — y la falta de latido es lo único que delata que la cadena murió.
	b, err := os.ReadFile("../../deploy/musubi-alerts.yml")
	if err != nil {
		t.Fatal(err)
	}
	texto := string(b)
	i := strings.Index(texto, "MusubiSiempreViva")
	if i < 0 {
		t.Fatal("desapareció el dead-man's switch: sin él, que Prometheus o Alertmanager se caigan se ve exactamente igual que «todo bien»")
	}
	bloque := texto[i:]
	if j := strings.Index(bloque[1:], "- alert:"); j > 0 {
		bloque = bloque[:j]
	}
	if !strings.Contains(bloque, "expr: vector(1)") {
		t.Error("el dead-man's switch dejó de ser incondicional: una condición lo convierte en una alerta más, y el punto ciego vuelve")
	}
	if !strings.Contains(bloque, "for:") {
		return // sin `for` es lo correcto: un latido no espera
	}
	t.Error("el dead-man's switch tiene un `for:`: un latido que espera antes de latir no es un latido")
}

// Y la contraparte del switch: Alertmanager tiene que RUTEARLO FUERA DEL CANAL NORMAL.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE SE EXIGE ES LA RUTA PROPIA, NO UN RECEPTOR CONCRETO
//
// La primera versión pedía literalmente `receiver: 'watchdog'`, y eso resultó ser más estricto de
// lo que el invariante necesita. Hay DOS destinos legítimos y la diferencia es de despliegue, no
// de diseño:
//
//	'watchdog'  hay un dead-man's switch externo escuchando el latido
//	'null'      no lo hay, y se declara: la alerta se ve en Alertmanager y no le llega a nadie
//
// Lo que NO puede pasar —y es todo el invariante— es que el latido caiga al receptor por DEFECTO.
// `MusubiSiempreViva` está SIEMPRE en firing a propósito, así que sin ruta propia notifica por el
// canal real cada `repeat_interval`, para siempre. Medido: pasó al comentar la ruta, y hubo que
// revertirlo en el acto. Una alarma que no se apaga enseña a ignorar el canal entero.
//
// Por eso la prueba mira que la ruta EXISTA y que su receptor NO sea 'default', en vez de fijar
// un nombre: fijarlo obligaba a elegir entre tener el guarda o poder declarar que no hay watchdog.
//
// Sabotaje que la hace fallar: borrar la ruta de MusubiSiempreViva, o apuntarla a 'default'.
func TestElDeadMansSwitchTieneSuPropiaRutaYReceptor(t *testing.T) {
	b, err := os.ReadFile("../../deploy/prometheus/alertmanager.yml")
	if err != nil {
		t.Fatalf("no hay configuración de Alertmanager: las alertas se evalúan y no le llegan a nadie (A4): %v", err)
	}
	texto := string(b)
	i := strings.Index(texto, `alertname = "MusubiSiempreViva"`)
	if i < 0 {
		t.Fatal("el latido no tiene ruta propia en Alertmanager: cae al receptor por defecto y " +
			"notifica por el canal real cada repeat_interval, para siempre")
	}
	// El receptor de ESA ruta es la línea `receiver:` que sigue al matcher.
	resto := texto[i:]
	j := strings.Index(resto, "receiver:")
	if j < 0 {
		t.Fatal("la ruta del latido no declara receptor")
	}
	linea := resto[j:]
	if k := strings.IndexByte(linea, '\n'); k > 0 {
		linea = linea[:k]
	}
	if strings.Contains(linea, "'default'") {
		t.Error("el latido va al receptor por DEFECTO: está siempre en firing, así que sería " +
			"ruido en el canal real cada repeat_interval, para siempre")
	}
	if !strings.Contains(linea, "'watchdog'") && !strings.Contains(linea, "'null'") {
		t.Errorf("el latido va a %q, que no es ni el watchdog externo ni el receptor nulo: "+
			"los dos son legítimos y cualquier otro hay que justificarlo", strings.TrimSpace(linea))
	}
	// Los dos receptores tienen que EXISTIR, se use el que se use: cambiar de uno a otro es
	// cambiar una palabra, y eso sólo vale si el otro está declarado.
	for _, r := range []string{"- name: 'watchdog'", "- name: 'null'"} {
		if !strings.Contains(texto, r) {
			t.Error(fmt.Sprintf("falta el receptor %q: pasar de tener dead-man's switch a no "+
				"tenerlo tiene que ser cambiar una palabra, no editar la lista de receptores", r))
		}
	}
	// El secreto no puede vivir en un archivo que se commitea.
	for _, prohibido := range []string{"bot_token:", "api_key:", "password:"} {
		if strings.Contains(texto, prohibido) {
			t.Errorf("alertmanager.yml tiene un %s en claro: los secretos entran por archivo (bot_token_file / url_file), nunca por la config versionada", prohibido)
		}
	}
}
