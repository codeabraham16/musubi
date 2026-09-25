package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// repoConArchivoIgnorado arma lo que tenía davantis-1 el 2026-09-24: main limpio y en origin/main,
// y al lado un archivo indexable que el .gitignore deja afuera. `git status` no lo lista, así que la
// guarda del árbol sucio lo deja pasar; y el índice lo lee igual, porque lee el disco.
//
// EnMain (commiteado) llama a Borrador (ignorado): la arista EnMain→Borrador sale de un archivo del
// commit y apunta a uno que el commit no tiene. Y a.go importa fmt: el nodo pkg:fmt es de los que no
// tienen archivo.
func repoConArchivoIgnorado(t *testing.T) (dir, enMain string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	sinVariablesDeGit(t)
	dir = t.TempDir()
	gitDePrueba(t, dir, "2026-09-24T22:22:54Z", "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, ".gitignore"), "borrador.go\n")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n\nimport \"fmt\"\n\nfunc EnMain() { Borrador(); fmt.Println() }\n")
	gitDePrueba(t, dir, "2026-09-24T22:22:54Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-24T22:22:54Z", "commit", "-q", "-m", "base")
	enMain = gitDePrueba(t, dir, "", "rev-parse", "HEAD")
	gitDePrueba(t, dir, "", "update-ref", "refs/remotes/origin/main", enMain)
	writeFile(t, filepath.Join(dir, "borrador.go"), "package a\n\nfunc Borrador() {}\n")
	return dir, enMain
}

// EL MAPA PUBLICADO DESCRIBE EL COMMIT, NO EL DISCO. Un archivo que git ignora se indexa en local
// —el grafo local sirve para el trabajo en curso— pero no sube al central con la etiqueta de un
// commit que no lo tiene: ni sus nodos, ni las aristas que lo tocan, ni su gist. Es el caso medido:
// boceto-g.js, gitignoreado, le sumaba 1.222 nodos al mapa del central etiquetado eb2cdb7.
//
// Sabotaje que la pone roja: no recortar los nodos.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="enElCommit(n.Path) {"
// arnes: a="enElCommit(n.Path) || true {"
//
// Sabotaje que la pone roja: dejar las aristas que apuntan a un nodo que no viajó.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="return !salieron[e.FromKey] && !salieron[e.ToKey]"
// arnes: a="return !salieron[e.FromKey] || !salieron[e.ToKey]"
//
// Sabotaje que la pone roja: publicar los gists sin recortar.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="func(g memory.CodeMemory) bool { return enElCommit(g.Path) }"
// arnes: a="func(g memory.CodeMemory) bool { return enElCommit(g.Path) || true }"
//
// Sabotaje que la pone roja: dejar el recorte definido y desconectado del push.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="foto, err = s.filtrarFotoAlCommit(foto, pub)"
// arnes: a="_, _ = s.filtrarFotoAlCommit(foto, pub)"
//
// Los nodos SIN archivo —los paquetes importados, 188 de los 13.329 de musubi en el central— sí
// viajan: no son de ningún archivo, y NormalizeCodePath de una ruta vacía da «.», que ningún commit
// lista. Sin la excepción, el recorte los sacaba a todos, y con ellos las aristas de import.
//
// Sabotaje que la pone roja: recortar también los nodos sin archivo.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="if n.Path == \"\" || enElCommit"
// arnes: a="if enElCommit"
func TestElMapaPublicadoDescribeElCommitYNoElDisco(t *testing.T) {
	dir, enMain := repoConArchivoIgnorado(t)
	central, url := centralDeVerdad(t)
	s := servidorSobreElArbol(t, dir)
	s.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})
	for _, g := range []memory.CodeMemory{
		{Path: "a.go", Gist: "EnMain, commiteado"},
		{Path: "borrador.go", Gist: "Borrador, ignorado por git"},
	} {
		if err := s.engine.SaveCodeMemory(g); err != nil {
			t.Fatal(err)
		}
	}
	res := mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})

	ctx := s.scopedCtx(context.Background())
	own := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: "musubi"})

	// El grafo LOCAL, leído DESPUÉS del push, sigue teniendo el archivo ignorado y la arista hacia él:
	// el recorte es de lo que se publica. Y sin eso, además, la prueba no probaría nada.
	localNodos, err := s.engine.AllGraphNodesCtx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ignorados := map[string]bool{}
	sinArchivo := ""
	for _, n := range localNodos {
		if n.Path == "borrador.go" {
			ignorados[n.Key] = true
		}
		if n.Key == "pkg:fmt" && n.Path == "" {
			sinArchivo = n.Key
		}
	}
	localAristas, err := s.engine.AllGraphEdgesCtx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	haciaLoIgnorado := 0
	for _, e := range localAristas {
		if ignorados[e.ToKey] && !ignorados[e.FromKey] {
			haciaLoIgnorado++
		}
	}
	if len(ignorados) == 0 || haciaLoIgnorado == 0 || sinArchivo == "" {
		t.Fatalf("andamio: el grafo local tenía que traer borrador.go (%d nodos), una arista desde a.go hacia él (%d) y el nodo pkg:fmt sin archivo (%q)",
			len(ignorados), haciaLoIgnorado, sinArchivo)
	}

	if pub, _ := central.engine.PublicacionDelGrafoDe("musubi"); pub.Head != enMain {
		t.Fatalf("precondición: el central tenía que publicar %s, tiene %+v\nrespuesta: %s", enMain[:7], pub, textOf(t, res))
	}
	if !nombresEnElCentral(t, central)["EnMain"] {
		t.Fatal("precondición: lo commiteado (EnMain) tenía que llegar al central")
	}

	nodos, err := central.engine.AllGraphNodesCtx(own)
	if err != nil {
		t.Fatal(err)
	}
	llegoSinArchivo := false
	for _, n := range nodos {
		if n.Path == "borrador.go" {
			t.Errorf("el central publicó %q, de un archivo que git ignora, con la etiqueta %s", n.Key, enMain[:7])
		}
		llegoSinArchivo = llegoSinArchivo || n.Key == sinArchivo
	}
	if !llegoSinArchivo {
		t.Errorf("el recorte sacó %q, un nodo sin archivo (un paquete importado): ésos no son de ningún archivo y viajan siempre", sinArchivo)
	}
	aristas, err := central.engine.AllGraphEdgesCtx(own)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range aristas {
		if ignorados[e.FromKey] || ignorados[e.ToKey] {
			t.Errorf("el central publicó la arista %s→%s, que toca un nodo que no viajó", e.FromKey, e.ToKey)
		}
	}
	gists, err := central.engine.AllCodeMemoryCtx(own)
	if err != nil {
		t.Fatal(err)
	}
	var rutas []string
	for _, g := range gists {
		rutas = append(rutas, g.Path)
	}
	if strings.Join(rutas, ",") != "a.go" {
		t.Errorf("los gists del central tenían que ser sólo los del commit [a.go], son %v", rutas)
	}
}

// UN PROYECTO QUE ES UN SUBDIRECTORIO DEL REPO SE RECORTA CON SUS PROPIAS RUTAS. El grafo guarda
// las rutas relativas al proyecto, y `git -C <proyecto> ls-tree` las lista relativas a ese mismo
// directorio. Si la lista saliera relativa a la raíz del repo, ninguna ruta del grafo coincidiría:
// el recorte sacaría TODO, y como un recorte que no deja ni un archivo no se publica, el proyecto
// dejaría de llegar al central (sin esa guarda, el reemplazo vacío le borraría el mapa).
//
// Sabotaje que la pone roja: listar el commit con rutas desde la raíz del repo.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="\"ls-tree\", \"-r\""
// arnes: a="\"ls-tree\", \"--full-name\", \"-r\""
func TestUnProyectoEnUnSubdirectorioDelRepoSeRecortaConSusRutas(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	sinVariablesDeGit(t)
	repo := t.TempDir()
	dir := filepath.Join(repo, "servicio")
	gitDePrueba(t, repo, "2026-09-24T22:22:54Z", "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(repo, ".gitignore"), "borrador.go\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/servicio\n")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n\nfunc EnMain() {}\n")
	gitDePrueba(t, repo, "2026-09-24T22:22:54Z", "add", ".")
	gitDePrueba(t, repo, "2026-09-24T22:22:54Z", "commit", "-q", "-m", "base")
	enMain := gitDePrueba(t, repo, "", "rev-parse", "HEAD")
	gitDePrueba(t, repo, "", "update-ref", "refs/remotes/origin/main", enMain)
	writeFile(t, filepath.Join(dir, "borrador.go"), "package a\n\nfunc Borrador() {}\n")

	central, url := centralDeVerdad(t)
	s := servidorSobreElArbol(t, dir)
	s.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})
	res := mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})

	// Que no se publique ES el defecto, no un andamio roto: es lo que pasa si el recorte no encuentra
	// las rutas del proyecto en el commit.
	if pub, _ := central.engine.PublicacionDelGrafoDe("musubi"); pub.Head != enMain {
		t.Fatalf("un proyecto en un subdirectorio del repo no llegó al central con la etiqueta %s (tiene %+v): el recorte no encontró sus rutas en el commit\nrespuesta: %s", enMain[:7], pub, textOf(t, res))
	}
	en := nombresEnElCentral(t, central)
	if !en["EnMain"] {
		t.Errorf("el recorte sacó lo commiteado de un proyecto en un subdirectorio del repo: el central quedó con %v", en)
	}
	if en["Borrador"] {
		t.Errorf("el archivo ignorado del subdirectorio llegó al central: %v", en)
	}
}

// UN RECORTE QUE SACA TODO NO SE PUBLICA. Un proyecto sin `.git` propio, adentro de un repo padre que
// lo ignora entero: git le contesta con el sello, la rama y el status del PADRE —ninguno ve lo
// ignorado—, así que se publica con la etiqueta de un commit que no tiene ni un archivo del proyecto.
// El recorte deja la foto en cero, y el push es de REEMPLAZO: con el mapa vacío, el central borraba
// el de ese proyecto y la tool contestaba federated:true. Lo encontró la revisión, con una sonda.
//
// Sabotaje que la pone roja: publicar el recorte aunque no haya dejado ni un archivo.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="antes > 0 && despues == 0 {"
// arnes: a="antes > 0 && despues == 0 && false {"
func TestUnRecorteQueSacaTodoNoVaciaElMapaDelCentral(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	sinVariablesDeGit(t)
	repo := t.TempDir()
	dir := filepath.Join(repo, "servicio")
	gitDePrueba(t, repo, "2026-09-24T22:22:54Z", "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(repo, ".gitignore"), "servicio/\n")
	writeFile(t, filepath.Join(repo, "raiz.go"), "package raiz\n\nfunc EnLaRaiz() {}\n")
	gitDePrueba(t, repo, "2026-09-24T22:22:54Z", "add", ".")
	gitDePrueba(t, repo, "2026-09-24T22:22:54Z", "commit", "-q", "-m", "base")
	gitDePrueba(t, repo, "", "update-ref", "refs/remotes/origin/main", gitDePrueba(t, repo, "", "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/servicio\n")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n\nfunc Servicio() {}\n")

	central := nuevoCentralQueCuentaPushes(t)
	s := servidorFederado(t, dir, central)
	res := mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})

	if archivosEnElGrafo(t, s) == 0 {
		t.Fatalf("andamio: el índice local tenía que traer a.go: %s", textOf(t, res))
	}
	if sello, _, _ := s.engine.GetMeta(memory.MetaCodegraphHead); sello == "" {
		t.Fatalf("andamio: el índice tenía que quedar sellado con el commit del repo padre: %s", textOf(t, res))
	}
	if n := central.pushes.Load(); n != 0 {
		t.Fatalf("el recorte dejó la foto sin un solo archivo y salió igual al central (%d pushes): el reemplazo le borra el mapa\nrespuesta: %s", n, textOf(t, res))
	}
	var cuerpo map[string]interface{}
	_ = json.Unmarshal([]byte(textOf(t, res)), &cuerpo)
	if cuerpo["federated"] != false || !strings.Contains(fmt.Sprint(cuerpo["federated_motivo"]), "no contiene ninguno") {
		t.Errorf("la tool tiene que decir federated=false CON el motivo: %v", cuerpo)
	}
}

// SI GIT NO PUEDE LISTAR EL COMMIT, NO SE PUBLICA. Mandar la foto entera sería publicar el disco con
// la etiqueta del commit; mandarla vacía, borrar el mapa del central. Un repo sin commits es la forma
// determinista de que `git ls-tree HEAD` falle en un árbol que SÍ es un repo. Y la tool lo dice.
//
// Sabotaje que la pone roja: seguir de largo cuando git falla.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="if rc != 0 {"
// arnes: a="if rc != 0 && false {"
//
// Sabotaje que la pone roja: callar el motivo (un federated:false mudo).
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="\t\ts.motivoDelPush = err.Error()\n\t\tlogx.Error(\"federación del grafo: no se pudo armar"
// arnes: a="\t\tlogx.Error(\"federación del grafo: no se pudo armar"
//
// Y vale igual para un proyecto que es un SUBDIRECTORIO del repo: el `.git` está en un padre, y
// hayRepoGit tiene que subir a buscarlo como lo busca git. Si mirara sólo el directorio del proyecto,
// lo daría por «no es un repo» y publicaría la foto entera sin recortar.
//
// Sabotaje que la pone roja: que hayRepoGit no suba a los padres.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="d = padre"
// arnes: a="return false"
func TestSiGitNoPuedeListarElCommitNoSePublica(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	sinVariablesDeGit(t)
	for _, caso := range []struct{ nombre, sub string }{
		{"en la raíz del repo", ""},
		{"en un subdirectorio del repo", "servicio"},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			var repo, dir string
			if caso.sub == "" {
				repo = proyectoGoSinIndexar(t)
				dir = repo
			} else {
				repo = t.TempDir()
				dir = filepath.Join(repo, caso.sub)
				writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/servicio\n")
				writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() {}\n")
			}
			gitDePrueba(t, repo, "", "init", "-q", "-b", "main")
			noSePublicaSiGitNoLista(t, dir)
		})
	}
}

// noSePublicaSiGitNoLista indexa dir —un repo sin commits— contra un central que cuenta los push.
func noSePublicaSiGitNoLista(t *testing.T, dir string) {
	t.Helper()
	central := nuevoCentralQueCuentaPushes(t)
	s := servidorFederado(t, dir, central)

	res := mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	if archivosEnElGrafo(t, s) == 0 {
		t.Fatalf("andamio: el índice local tenía que correr igual: %s", textOf(t, res))
	}
	if n := central.pushes.Load(); n != 0 {
		t.Fatalf("git no pudo listar el commit y el grafo salió igual al central (%d pushes)", n)
	}
	var cuerpo map[string]interface{}
	_ = json.Unmarshal([]byte(textOf(t, res)), &cuerpo)
	if cuerpo["federated"] != false || !strings.Contains(fmt.Sprint(cuerpo["federated_motivo"]), "git no pudo listar") {
		t.Errorf("la tool tiene que decir federated=false CON el motivo: %v", cuerpo)
	}
	if est, _ := s.engine.EstadoDelPushDelGrafo(); !est.EmpujadoEn.IsZero() {
		t.Errorf("un push que no salió quedó registrado como exitoso: %+v", est)
	}
}

// EL RECORTE ES CONTRA EL COMMIT DE LA ETIQUETA, NO CONTRA EL HEAD DE AHORA. origenDelGrafo fija el
// commit y el recorte corre después: si entre los dos alguien cambia de rama, la foto de A no puede
// recortarse con la lista de B y subir con la etiqueta de A. Acá A tiene x.go, B lo borra y agrega
// z.go, y el árbol queda en B: contra A, x.go se queda y z.go sale.
//
// Sabotaje que la pone roja: listar el HEAD de ahora en vez del commit de la etiqueta.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="\"--name-only\", commit)"
// arnes: a="\"--name-only\", \"HEAD\")"
//
// Y una ruta ABSOLUTA dentro del proyecto (un gist guardado así) cuenta como su relativa: el commit
// lista rutas relativas, y sin normalizar, ese gist salía aunque el archivo esté en el commit.
//
// Sabotaje que la pone roja: comparar la ruta tal cual, sin normalizarla contra el proyecto.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="rastreados[memory.NormalizeCodePath(s.projectPath, ruta)]"
// arnes: a="rastreados[ruta]"
func TestElRecorteEsContraElCommitDeLaEtiquetaYNoContraElHEAD(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	sinVariablesDeGit(t)
	dir := t.TempDir()
	gitDePrueba(t, dir, "2026-09-24T22:22:54Z", "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(dir, "x.go"), "package a\n\nfunc X() {}\n")
	gitDePrueba(t, dir, "2026-09-24T22:22:54Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-24T22:22:54Z", "commit", "-q", "-m", "A")
	commitA := gitDePrueba(t, dir, "", "rev-parse", "HEAD")
	gitDePrueba(t, dir, "", "rm", "-q", "x.go")
	writeFile(t, filepath.Join(dir, "z.go"), "package a\n\nfunc Z() {}\n")
	gitDePrueba(t, dir, "2026-09-25T10:00:00Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-25T10:00:00Z", "commit", "-q", "-m", "B")

	s := &McpServer{projectPath: dir}
	foto := memory.FotoDelGrafo{
		Nodes: []memory.GraphNode{
			{Key: "x.go#func:X", Kind: "func", Name: "X", Path: "x.go"},
			{Key: "z.go#func:Z", Kind: "func", Name: "Z", Path: "z.go"},
		},
		Gists: []memory.CodeMemory{{Path: filepath.Join(dir, "x.go"), Gist: "X, guardado con la ruta absoluta"}},
	}
	recortada, err := s.filtrarFotoAlCommit(foto, memory.PublicacionDelGrafo{Head: commitA})
	if err != nil {
		t.Fatalf("el recorte contra %s tenía que salir: %v", commitA[:7], err)
	}
	var claves []string
	for _, n := range recortada.Nodes {
		claves = append(claves, n.Key)
	}
	if strings.Join(claves, ",") != "x.go#func:X" {
		t.Errorf("recortada contra %s (tiene x.go, no z.go) con el árbol en otro commit, quedaron %v", commitA[:7], claves)
	}
	if len(recortada.Gists) != 1 {
		t.Errorf("el gist de x.go, guardado con su ruta absoluta dentro del proyecto, tenía que quedarse: quedaron %+v", recortada.Gists)
	}
}

// UN PROYECTO QUE NO ES UN REPO GIT PUBLICA COMO SIEMPRE. Sin commit no hay etiqueta que mentir ni
// lista contra la cual recortar, y git contesta con el mismo código de salida que cuando falla: sin
// mirar si hay un `.git`, estos proyectos dejarían de federar su grafo.
//
// Sabotaje que la pone roja: tratar todo directorio como un repo git.
// arnes: archivo="internal/mcp/codegraph_foto_al_commit.go"
// arnes: de="if !hayRepoGit(s.projectPath) {"
// arnes: a="if !hayRepoGit(s.projectPath) && false {"
func TestUnProyectoSinGitPublicaSuGrafoEntero(t *testing.T) {
	sinVariablesDeGit(t)
	dir := proyectoGoSinIndexar(t)
	if hayRepoGit(dir) {
		t.Skipf("el directorio temporal %s cuelga de un repo git: la prueba necesita uno que no", dir)
	}
	central, url := centralDeVerdad(t)
	s := servidorSobreElArbol(t, dir)
	s.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})

	res := mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	if en := nombresEnElCentral(t, central); !en["Alpha"] || !en["beta"] {
		t.Fatalf("el grafo de un proyecto sin git tenía que llegar entero al central, llegó %v\nrespuesta: %s", en, textOf(t, res))
	}
}
