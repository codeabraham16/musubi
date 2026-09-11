package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TODO ARCHIVO DE ALERTAS TIENE QUIEN LO INSTALE.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL AGUJERO DE A73, UN DIRECTORIO MÁS ALLÁ.
//
// A73 nació de esto: el cerebro exponía `/metrics` mientras NADIE lo scrapeaba — las reglas
// estaban escritas, probadas e INERTES. El registro de abiertos no lo vio porque cubre código y
// aquello era despliegue.
//
// `deploy/musubi-alerts-altura.yml` estaba exactamente igual: cuatro reglas —incluida
// `up{job="altura-db"} == 0`, la que avisa que ese scrape se cayó— que ningún guion copiaba. El
// archivo existía, se versionaba, se revisaba… y no corría en ningún lado.
//
// Y DOS GUARDAS LE DABAN PERMISO POR LEER LA PALABRA «CONDICIONAL» EN SU LÍNEA: alcanzaba con que
// el archivo dijera que su despliegue era condicional para que nadie preguntara quién evalúa esa
// condición. Acá la pregunta es otra: ¿hay un `install` que lo nombre?
// ────────────────────────────────────────────────────────────────────────────────────────────

func TestTodoArchivoDeAlertasLoInstalaAlgunGuion(t *testing.T) {
	dir := filepath.Join("..", "..", "deploy")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("no pude leer %s: %v", dir, err)
	}

	// Los guiones que pueden instalar reglas. Se juntan todos: qué guion la instala es un detalle
	// del despliegue, y atarlo a uno concreto haría fallar la guarda por una mudanza legítima.
	var guiones strings.Builder
	for _, sub := range []string{".", "docker", "prometheus"} {
		hijos, err := os.ReadDir(filepath.Join(dir, sub))
		if err != nil {
			continue
		}
		for _, h := range hijos {
			if h.IsDir() || !strings.HasSuffix(h.Name(), ".sh") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, sub, h.Name()))
			if err != nil {
				continue
			}
			guiones.Write(b)
		}
	}
	texto := guiones.String()
	if len(texto) < 10_000 {
		t.Fatalf("sólo se juntaron %d bytes de guiones de despliegue: la enumeración quedó corta y "+
			"un corpus chico hace pasar TODOS los archivos sin que nadie los haya buscado", len(texto))
	}

	revisados := 0
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, "musubi-alerts") || !strings.HasSuffix(n, ".yml") {
			continue
		}
		revisados++
		// SE EXIGE UN `install` Y NO UNA MENCIÓN: el nombre de un archivo aparece en comentarios,
		// en mensajes de error y en rutas de ejemplo. Lo que lo pone a correr es copiarlo.
		if !strings.Contains(texto, "install -m 0644 \"$REPO/deploy/"+n+"\"") {
			t.Errorf("ningún guion de deploy/ instala %s.\n"+
				"  El archivo existe, se versiona y se revisa — y no corre en ningún lado: sus reglas "+
				"están escritas, probadas e INERTES. Es el agujero de A73, que nació exactamente así.\n"+
				"  Si su despliegue es CONDICIONAL, escribí la condición y el `install` adentro del "+
				"`if`, como ya hacen flota y el backup off-host. Decir «condicional» en un comentario "+
				"no instala nada.", n)
		}
	}
	if revisados < 4 {
		t.Fatalf("sólo se revisaron %d archivos de alertas y son al menos 4: cambió dónde viven y "+
			"esta guarda está en verde sin mirar nada", revisados)
	}
}
