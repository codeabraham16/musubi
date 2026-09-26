package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"musubi/internal/mcp"
	"musubi/internal/provision"
)

// permisos_agente.go — los permisos de Claude Code que Musubi necesita para trabajar sin pedirle
// confirmación a la persona, puestos por `musubi agente instalar`.
//
// POR QUÉ LOS PONE EL INSTALADOR. Un plugin no puede darse permisos: Claude Code le acepta sólo los
// ajustes `agent` y `subagentStatusLine` (medido en el binario 2.1.223). Sin reglas, en modo normal
// cada llamada a Musubi y cada `git log` del subagente de tareas piden confirmación, y en modo auto
// decide el clasificador, que el 2026-09-26 bloqueó escrituras de memoria legítimas. La persona lo
// pidió así: que vengan con la instalación, por defecto.
//
// QUÉ SE PERMITE Y QUÉ NO:
//
//   - los servidores de Musubi enteros, con los tres nombres con que se conectan (el del proyecto,
//     el del relé al cerebro y el del plugin);
//   - los comandos de git de sólo lectura con que el subagente de tareas verifica una nota;
//   - y en «preguntar» las tools que actúan FUERA de la memoria —sobre otra máquina de la flota o
//     sobre credenciales— (mcp.ToolsQuePreguntan). Es la regla de oro de la sala de mando como
//     permiso: un cambio en una máquina ajena se hace con el sí de la persona, en el momento. Con
//     --flota-sin-preguntar esas también quedan permitidas: es decisión de la persona, no un default.
//
// SÓLO SE TOCA LO QUE MUSUBI AGREGÓ. Las reglas que agrega quedan anotadas en el plugin
// (marcaDePermisos); reinstalar reemplaza ésas, `quitar` saca ésas, y una regla que la persona ya
// tenía —aunque sea idéntica— no se anota y no se toca nunca.

// marcaDePermisos es el archivo, dentro del plugin, donde queda qué reglas agregó Musubi y en qué
// settings.
const marcaDePermisos = ".musubi-permisos.json"

// comandosGitDeLectura son los comandos con que el subagente de tareas verifica una nota contra la
// historia. Los nombra su protocolo (subagente_tareas.go) y reciben su regla acá: de la misma lista.
var comandosGitDeLectura = []string{"git log", "git show", "git grep", "git branch --contains"}

// servidoresDeMusubi son los nombres con que Musubi se conecta a Claude Code: el del proyecto
// cableado a mano (clave "musubi" del .mcp.json, la misma que mira elProyectoYaConectaMusubi), el
// del relé al cerebro que escribe `musubi provision`, y el del plugin (plugin_<plugin>_<servidor>).
func servidoresDeMusubi(plugin string) []string {
	return []string{nombrePlugin, provision.ServidorCerebro, "plugin_" + plugin + "_" + nombrePlugin}
}

// reglasDelAgente devuelve las reglas que Musubi necesita: las que se permiten y las que se preguntan.
func reglasDelAgente(plugin string, flotaSinPreguntar bool) (allow, ask []string) {
	servidores := servidoresDeMusubi(plugin)
	for _, srv := range servidores {
		allow = append(allow, "mcp__"+srv)
	}
	for _, c := range comandosGitDeLectura {
		allow = append(allow, "Bash("+c+":*)")
	}
	if flotaSinPreguntar {
		return allow, nil
	}
	for _, tool := range mcp.ToolsQuePreguntan() {
		for _, srv := range servidores {
			ask = append(ask, "mcp__"+srv+"__"+tool)
		}
	}
	return allow, ask
}

// permisosAnotados es lo que queda escrito en marcaDePermisos.
type permisosAnotados struct {
	Settings string   `json:"settings"`
	Allow    []string `json:"allow"`
	Ask      []string `json:"ask"`
}

// settingsPorDefecto es el settings.json del usuario, en la misma carpeta de configuración que el
// plugin (dirConfigDeClaude).
func settingsPorDefecto() (string, error) {
	base, err := dirConfigDeClaude()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "settings.json"), nil
}

// instalarPermisos deja en `settings` las reglas que Musubi necesita y anota en el plugin cuáles
// agregó. Reinstalar reemplaza lo que agregó la vez anterior (y, si antes se anotaron en otro
// settings, las saca de allá). Devuelve cuántas reglas quedaron puestas por Musubi.
func instalarPermisos(dirPlugin, settings string, flotaSinPreguntar bool) (permisosAnotados, error) {
	previo, _ := leerPermisosAnotados(dirPlugin)
	if previo.Settings != "" && previo.Settings != settings {
		if err := sacarReglas(previo.Settings, previo.Allow, previo.Ask); err != nil {
			return permisosAnotados{}, fmt.Errorf("sacar los permisos de %s: %w", previo.Settings, err)
		}
		previo = permisosAnotados{}
	}
	allow, ask := reglasDelAgente(nombrePlugin, flotaSinPreguntar)

	raiz, perms, err := leerSettings(settings)
	if err != nil {
		return permisosAnotados{}, err
	}
	nuevo := permisosAnotados{Settings: settings}
	nuevo.Allow, perms["allow"], err = ponerReglas(perms["allow"], allow, previo.Allow)
	if err != nil {
		return permisosAnotados{}, err
	}
	nuevo.Ask, perms["ask"], err = ponerReglas(perms["ask"], ask, previo.Ask)
	if err != nil {
		return permisosAnotados{}, err
	}
	if err := escribirSettings(settings, raiz, perms); err != nil {
		return permisosAnotados{}, err
	}
	return nuevo, escribirJSON(filepath.Join(dirPlugin, marcaDePermisos), nuevo)
}

// quitarPermisos saca de su settings las reglas que Musubi agregó, y borra la anotación.
func quitarPermisos(dirPlugin string) error {
	previo, ok := leerPermisosAnotados(dirPlugin)
	if !ok {
		return nil
	}
	if err := sacarReglas(previo.Settings, previo.Allow, previo.Ask); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dirPlugin, marcaDePermisos))
}

func leerPermisosAnotados(dirPlugin string) (permisosAnotados, bool) {
	var p permisosAnotados
	crudo, err := os.ReadFile(filepath.Join(dirPlugin, marcaDePermisos))
	if err != nil || json.Unmarshal(crudo, &p) != nil || p.Settings == "" {
		return permisosAnotados{}, false
	}
	return p, true
}

// ponerReglas deja en la lista `actual` las reglas `quiero`: saca las que Musubi había agregado
// antes (`anotadas`) y ya no quiere, y agrega las que faltan. Devuelve cuáles quedan anotadas como
// de Musubi: las que agregó ahora más las anotadas que siguen. Una regla que ya estaba y Musubi no
// había anotado es de la persona: no se anota, no se duplica y no se toca.
func ponerReglas(actual json.RawMessage, quiero, anotadas []string) ([]string, json.RawMessage, error) {
	var lista []string
	if len(actual) > 0 {
		if err := json.Unmarshal(actual, &lista); err != nil {
			return nil, nil, fmt.Errorf("la lista de permisos no es una lista de textos: %w", err)
		}
	}
	quieroSet := map[string]bool{}
	for _, r := range quiero {
		quieroSet[r] = true
	}
	eraNuestra := map[string]bool{}
	for _, r := range anotadas {
		eraNuestra[r] = true
	}
	var queda, nuestras []string
	presente := map[string]bool{}
	for _, r := range lista {
		if eraNuestra[r] && !quieroSet[r] {
			continue // la agregó Musubi y ya no la quiere
		}
		// Lo de la persona queda como está, en su orden y con sus repeticiones si las tiene.
		queda = append(queda, r)
		if eraNuestra[r] && !presente[r] {
			nuestras = append(nuestras, r)
		}
		presente[r] = true
	}
	for _, r := range quiero {
		if presente[r] {
			continue
		}
		presente[r] = true
		queda = append(queda, r)
		nuestras = append(nuestras, r)
	}
	sort.Strings(nuestras)
	if len(queda) == 0 {
		return nuestras, nil, nil
	}
	b, err := json.Marshal(queda)
	return nuestras, b, err
}

// sacarReglas saca de un settings las reglas dadas (las que Musubi había agregado).
func sacarReglas(settings string, allow, ask []string) error {
	raiz, perms, err := leerSettings(settings)
	if err != nil {
		return err
	}
	for clave, sacar := range map[string][]string{"allow": allow, "ask": ask} {
		if _, perms[clave], err = ponerReglas(perms[clave], nil, sacar); err != nil {
			return err
		}
	}
	return escribirSettings(settings, raiz, perms)
}

// leerSettings lee un settings.json (vacío si no existe) y su objeto `permissions`.
//
// UN SETTINGS QUE NO SE ENTIENDE NO SE TOCA: se devuelve el error. Reescribirlo desde cero borraría
// todo lo que la persona tenía ahí, y un settings roto apaga en Claude Code todo lo que ese archivo
// configura.
func leerSettings(ruta string) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	raiz := map[string]json.RawMessage{}
	perms := map[string]json.RawMessage{}
	crudo, err := os.ReadFile(ruta)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	if len(crudo) > 0 {
		if err := json.Unmarshal(crudo, &raiz); err != nil {
			return nil, nil, fmt.Errorf("%s no es JSON válido (no lo toco): %w", ruta, err)
		}
		if p, ok := raiz["permissions"]; ok {
			if err := json.Unmarshal(p, &perms); err != nil {
				return nil, nil, fmt.Errorf("%s: `permissions` no es un objeto (no lo toco): %w", ruta, err)
			}
		}
	}
	return raiz, perms, nil
}

// escribirSettings vuelve a armar el settings con `permissions` actualizado, sin las listas que
// quedaron vacías.
func escribirSettings(ruta string, raiz, perms map[string]json.RawMessage) error {
	for clave, v := range perms {
		if v == nil {
			delete(perms, clave)
		}
	}
	if len(perms) == 0 {
		delete(raiz, "permissions")
	} else {
		b, err := json.Marshal(perms)
		if err != nil {
			return err
		}
		raiz["permissions"] = b
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(raiz, "", "  ")
	if err != nil {
		return err
	}
	return escribirArchivoAtomico(ruta, append(out, '\n'), 0o644)
}
