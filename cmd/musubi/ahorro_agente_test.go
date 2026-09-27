package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// leerVentana devuelve el autoCompactWindow crudo de un settings.json ("" si no hay) y el resto de
// sus claves.
func leerVentana(t *testing.T, ruta string) (string, map[string]json.RawMessage) {
	t.Helper()
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	var raiz map[string]json.RawMessage
	if err := json.Unmarshal(crudo, &raiz); err != nil {
		t.Fatalf("el settings quedó ilegible: %v\n%s", err, crudo)
	}
	return string(raiz[claveVentanaDeCompactacion]), raiz
}

func escribirSettingsDePrueba(t *testing.T, ruta, contenido string) {
	t.Helper()
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

// LA VENTANA DE COMPACTACIÓN LA PONE MUSUBI, Y LA DE LA PERSONA NO SE TOCA.
//
// Resumir la conversación al llegar a 400k en vez de cerca del millón es el ahorro grande (38 % en la
// simulación sobre 14 días de pedidos reales), y lo pone el instalador en el settings del usuario,
// donde convive con lo de la persona. Lo que no puede pasar: pisar una ventana que la persona eligió,
// llevársela al quitar Musubi, olvidar cuál puso Musubi (y no poder cambiarla ni sacarla después), o
// dejarla en un settings viejo cuando la instalación se muda.
//
// Sabotaje que la hace fallar: tomar como de Musubi cualquier ventana numérica.
// arnes: archivo="cmd/musubi/ahorro_agente.go"
// arnes: de="\tesDeMusubi := hay && esNumero && previo.Ventana != 0 && actual == previo.Ventana\n"
// arnes: a="\tesDeMusubi := hay && esNumero && actual >= 0\n"
//
// Sabotaje que la hace fallar: que quitar se lleve una ventana que la persona cambió a mano.
// arnes: archivo="cmd/musubi/ahorro_agente.go"
// arnes: de="!hay || !esNumero || actual != ventana {\n"
// arnes: a="!hay || !esNumero || actual != ventana && false {\n"
//
// Sabotaje que la hace fallar: no anotar la ventana que puso Musubi.
// arnes: archivo="cmd/musubi/ahorro_agente.go"
// arnes: de="\treturn nuevo, \"\", escribirJSON(filepath.Join(dirPlugin, marcaDeAhorro), nuevo)\n"
// arnes: a="\treturn nuevo, \"\", nil\n"
//
// Sabotaje que la hace fallar: dejar la ventana en el settings viejo al mudarse.
// arnes: archivo="cmd/musubi/ahorro_agente.go"
// arnes: de="\tif previo.Settings != \"\" && previo.Settings != settings {\n"
// arnes: a="\tif false && previo.Settings != \"\" && previo.Settings != settings {\n"
//
// Sabotaje que la hace fallar: que --compactar-en 0 no saque la ventana de Musubi.
// arnes: archivo="cmd/musubi/ahorro_agente.go"
// arnes: de="\t\tif esDeMusubi {\n"
// arnes: a="\t\tif false && esDeMusubi {\n"
func TestLaVentanaDeCompactacionEsDeMusubiSoloSiLaPusoMusubi(t *testing.T) {
	dir := t.TempDir()
	plugin := filepath.Join(dir, "plugin")
	if err := os.MkdirAll(plugin, 0o755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(dir, "settings.json")
	escribirSettingsDePrueba(t, settings, `{"model":"opus[1m]","effortLevel":"xhigh","permissions":{"allow":["Bash(ls:*)"]}}`)

	// Primera instalación: pone la ventana, la anota y no toca lo demás.
	if a, propia, err := instalarAhorro(plugin, settings, 400_000); err != nil || propia != "" || a.Ventana != 400_000 {
		t.Fatalf("instalar: anotado=%+v propia=%q err=%v", a, propia, err)
	}
	v, raiz := leerVentana(t, settings)
	if v != "400000" {
		t.Fatalf("la ventana quedó en %q, quería 400000", v)
	}
	if string(raiz["model"]) != `"opus[1m]"` || string(raiz["effortLevel"]) != `"xhigh"` || !strings.Contains(string(raiz["permissions"]), "Bash(ls:*)") {
		t.Errorf("instalar tocó lo de la persona: %v", raiz)
	}
	if !strings.Contains(estadoDelAhorro(plugin, settings), "400k") {
		t.Errorf("estado no dice que Musubi resume a los 400k: %s", estadoDelAhorro(plugin, settings))
	}

	// Reinstalar con otra ventana cambia la de Musubi.
	if _, _, err := instalarAhorro(plugin, settings, 300_000); err != nil {
		t.Fatal(err)
	}
	if v, _ := leerVentana(t, settings); v != "300000" {
		t.Fatalf("reinstalar con 300000 dejó %q: Musubi no reconoció su propia ventana", v)
	}

	// La persona la cambia a mano: desde ahí es suya y quitar no se la lleva.
	escribirSettingsDePrueba(t, settings, `{"model":"opus[1m]","autoCompactWindow":500000}`)
	if err := quitarAhorro(plugin); err != nil {
		t.Fatal(err)
	}
	if v, _ := leerVentana(t, settings); v != "500000" {
		t.Fatalf("quitar se llevó la ventana que la persona puso a mano (quedó %q)", v)
	}

	// Una ventana propia de antes de instalar tampoco se pisa.
	escribirSettingsDePrueba(t, settings, `{"autoCompactWindow":250000}`)
	a, propia, err := instalarAhorro(plugin, settings, 400_000)
	if err != nil || propia != "250000" || a.Ventana != 0 {
		t.Fatalf("con una ventana propia: anotado=%+v propia=%q err=%v", a, propia, err)
	}
	if v, _ := leerVentana(t, settings); v != "250000" {
		t.Fatalf("instalar pisó la ventana de la persona (quedó %q)", v)
	}
	if !strings.Contains(estadoDelAhorro(plugin, settings), "propia") {
		t.Errorf("estado no dice que la ventana es de la persona: %s", estadoDelAhorro(plugin, settings))
	}

	// --compactar-en 0 saca la de Musubi y deja lo demás.
	escribirSettingsDePrueba(t, settings, `{"theme":"dark"}`)
	if _, _, err := instalarAhorro(plugin, settings, 400_000); err != nil {
		t.Fatal(err)
	}
	if _, _, err := instalarAhorro(plugin, settings, 0); err != nil {
		t.Fatal(err)
	}
	v, raiz = leerVentana(t, settings)
	if v != "" || string(raiz["theme"]) != `"dark"` {
		t.Fatalf("--compactar-en 0: ventana %q, theme %s", v, raiz["theme"])
	}

	// Mudarse de settings saca la ventana del viejo.
	if _, _, err := instalarAhorro(plugin, settings, 400_000); err != nil {
		t.Fatal(err)
	}
	otro := filepath.Join(dir, "otro.json")
	if _, _, err := instalarAhorro(plugin, otro, 400_000); err != nil {
		t.Fatal(err)
	}
	if v, _ := leerVentana(t, settings); v != "" {
		t.Errorf("la ventana quedó en el settings viejo (%q) al mudarse a %s", v, otro)
	}
	if v, _ := leerVentana(t, otro); v != "400000" {
		t.Errorf("el settings nuevo quedó con %q", v)
	}
}

// UN SETTINGS QUE NO SE ENTIENDE NO SE TOCA, tampoco para poner el ahorro.
func TestElAhorroNoPisaUnSettingsIlegible(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	roto := `{"model": "opus", ` // cortado
	escribirSettingsDePrueba(t, settings, roto)
	if _, _, err := instalarAhorro(dir, settings, 400_000); err == nil {
		t.Fatal("instalar el ahorro sobre un settings roto no dio error")
	}
	if crudo, _ := os.ReadFile(settings); string(crudo) != roto {
		t.Errorf("el settings roto se reescribió:\n%s", crudo)
	}
}

// `musubi agente instalar` PONE EL AHORRO EN EL SETTINGS DEL USUARIO, Y `quitar` LO SACA.
//
// Se mide sobre el proceso: el flag y el settings por defecto viven en runAgente.
//
// Sabotaje que la hace fallar: que instalar no ponga la ventana por defecto.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t\ta, propia, err := instalarAhorro(dir, settings, *compactarEn)\n"
// arnes: a="\t\ta, propia, err := instalarAhorro(dir, settings, *compactarEn*0)\n"
//
// Sabotaje que la hace fallar: que quitar deje la ventana puesta.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t\tif err := quitarAhorro(dir); err != nil {\n"
// arnes: a="\t\tif err := error(nil); err != nil {\n"
func TestElInstaladorPoneYSacaElAhorro(t *testing.T) {
	home := t.TempDir()
	plugin := filepath.Join(home, ".claude", "skills", "musubi")
	settings := filepath.Join(home, ".claude", "settings.json")
	sinConfig := []string{"CLAUDE_CONFIG_DIR="}

	correrMusubiCon(t, home, home, "", sinConfig, "agente", "instalar", "--dir", plugin)
	if v, _ := leerVentana(t, settings); v != "400000" {
		t.Fatalf("instalar dejó la ventana en %q, quería 400000", v)
	}
	out := correrMusubiCon(t, home, home, "", sinConfig, "agente", "estado", "--dir", plugin)
	if !strings.Contains(out, "Ahorro:") || !strings.Contains(out, "400k") {
		t.Errorf("estado no dice cuándo se resume la conversación:\n%s", out)
	}

	correrMusubiCon(t, home, home, "", sinConfig, "agente", "quitar", "--dir", plugin)
	if v, _ := leerVentana(t, settings); v != "" {
		t.Errorf("quitar dejó la ventana de Musubi: %q", v)
	}
}
