package main

import (
	"strings"
	"testing"
)

// entradaDeEdicion arma el stdin del hook para una edición sobre un archivo.
func entradaDeEdicion(path, sesion string) string {
	return `{"tool_name":"Edit","tool_input":{"file_path":"` + path + `"},"session_id":"` + sesion + `"}`
}

// EL SILENCIO ANTES DE EDITAR UN ARCHIVO QUE EL GRAFO NO PUEDE CUBRIR ERA UNA AFIRMACIÓN DE
// SEGURIDAD QUE NADIE HABÍA DECIDIDO HACER.
//
// impactMessage ya se cuidaba mucho del grafo VIEJO: si el archivo está indexado y ningún símbolo
// tiene callers, no dice «no arrastra a nadie» sino «el grafo no sabe». Pero cuando el archivo NO
// PUEDE estar en el grafo —un .sql, un .rs, un .php— devolvía "" y el hook entero se callaba, que
// se lee igual que «no hay nada que decir». El caso raro tratado con cuidado, el común sin tratar.
//
// Sabotaje: hacer que avisoSinGrafo devuelva "" siempre → vuelve el mudo y esta prueba se pone roja.
func TestAntesDeEditarUnArchivoSinGrafoElHookLoDICE(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "migraciones/alertas.sql", "create table alertas (id int);\n")
	store := &fakeCodeStore{}

	out := precheckOutput(store, root, strings.NewReader(entradaDeEdicion(root+"/migraciones/alertas.sql", "s1")))
	if out == "" {
		t.Fatal("el hook se quedó MUDO antes de editar un .sql: el silencio ocupa el lugar de «fijate quién depende de esto» y se lee como que no arrastra a nadie")
	}
	if !strings.Contains(out, "NO PUEDE estar en el grafo") {
		t.Errorf("el aviso tiene que decir que el archivo no puede estar en el grafo, no que falta indexarlo; obtuve: %s", out)
	}
	// La frase que hace el trabajo: convierte el silencio futuro en «no sé» y no en «no hay riesgo».
	if !strings.Contains(out, "nunca como") {
		t.Errorf("el aviso tiene que decir explícitamente cómo leer el silencio de los demás archivos; obtuve: %s", out)
	}
}

// Y CUANDO EL LENGUAJE SÍ SE INDEXA, EL AVISO ES OTRO PORQUE LA ACCIÓN ES OTRA.
//
// Un `.go` que no está en el grafo es un índice que falta y tiene arreglo; un `.sql` no. Mandar a
// correr codegraph_index sobre algo que el binario no puede indexar es mandar a perder el tiempo,
// que es el defecto que el hint de methods_codegraph ya había cerrado del otro lado.
func TestSiElLenguajeSeIndexaElAvisoMandaAIndexarYNoAResignarse(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/p.go", "package p\nfunc Uno(){}\n")
	store := &fakeCodeStore{}

	out := precheckOutput(store, root, strings.NewReader(entradaDeEdicion(root+"/src/p.go", "s1")))
	if !strings.Contains(out, "codegraph_index") {
		t.Errorf("para un .go sin indexar el aviso tiene que ofrecer el índice; obtuve: %s", out)
	}
	if strings.Contains(out, "NO PUEDE estar en el grafo") {
		t.Errorf("un .go SÍ puede estar en el grafo: el aviso no debe decir lo contrario; obtuve: %s", out)
	}
}

// EL PRESUPUESTO DE RUIDO ES LA MITAD DEL DISEÑO, Y SIN ESTA PRUEBA NO EXISTE.
//
// Este hook corre antes de CADA edición. Un aviso por archivo convierte un proyecto de SQL o de
// React en una pared de texto repetido, y una advertencia que aparece siempre se deja de leer —que
// es exactamente cómo se pierde la que sí importa—. El dato es del binario y del proyecto, no del
// archivo: decirlo una vez por sesión alcanza.
//
// Sabotaje: sacarle la guarda del ledger a avisoSinGrafo → el aviso se repite y esto se pone rojo.
func TestElAvisoSinGrafoSeDiceUnaVezPorSesionYNoEnCadaEdicion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.sql", "create table a (id int);\n")
	writeFile(t, root, "b.sql", "create table b (id int);\n")
	store := &fakeCodeStore{}

	primera := precheckOutput(store, root, strings.NewReader(entradaDeEdicion(root+"/a.sql", "s1")))
	if primera == "" {
		t.Fatal("la primera edición de la sesión tiene que avisar")
	}
	// Otro archivo, misma sesión: ya se dijo.
	if segunda := precheckOutput(store, root, strings.NewReader(entradaDeEdicion(root+"/b.sql", "s1"))); segunda != "" {
		t.Errorf("el aviso se repitió en la misma sesión sobre otro archivo: eso es la pared de texto que este diseño evita.\n  obtuve: %s", segunda)
	}
	// Y el MISMO archivo tampoco lo repite.
	if otraVez := precheckOutput(store, root, strings.NewReader(entradaDeEdicion(root+"/a.sql", "s1"))); otraVez != "" {
		t.Errorf("el aviso se repitió sobre el mismo archivo en la misma sesión; obtuve: %s", otraVez)
	}
}

// EN LA LECTURA SE SIGUE CALLANDO, Y ES UNA DECISIÓN.
//
// Al leer, el silencio no afirma nada peligroso: el archivo se va a leer igual, y el gist y la
// telemetría hablan por su cuenta. El aviso existe porque antes de EDITAR el silencio ocupa el
// lugar de «fijate quién depende de esto». Extenderlo a la lectura sería gastar el presupuesto de
// ruido donde no compra nada.
func TestAlLEERUnArchivoSinGrafoElHookSigueCallado(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.sql", "create table a (id int);\n")
	store := &fakeCodeStore{}

	in := `{"tool_name":"Read","tool_input":{"file_path":"` + root + `/a.sql"},"session_id":"s1"}`
	if out := precheckOutput(store, root, strings.NewReader(in)); out != "" {
		t.Errorf("la LECTURA de un archivo sin grafo no debe gastar el presupuesto de ruido; obtuve: %s", out)
	}
}
