package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ahorro_agente.go — el ajuste de Claude Code con que Musubi ahorra tokens sin quitarle poder al
// agente, puesto por `musubi agente instalar`.
//
// DE DÓNDE SALE. Medido el 2026-09-26 sobre 14 días de transcripts: el 68 % del gasto era RELEER la
// conversación. Cada pedido manda el contexto entero, y con el modelo de 1M la sesión crecía hasta
// cerca del millón antes de resumirse (mediana: 434k tokens por pedido). Simulado sobre esos mismos
// pedidos, resumir al llegar a 250k gasta un 46 % menos (400k: 38 %). Arrancar una sesión nueva al
// volver de una pausa larga ahorraría el 54 %, pero Claude Code no deja que nadie la abra solo; 250k
// es lo que más se le acerca sin quitarle poder al agente: sigue siendo más contexto que el de una
// sesión normal de Claude, que es de 200k. Lo que el resumen deja atrás no se pierde: Musubi trae la
// memoria relevante en cada turno.
//
// QUÉ SE PONE: `autoCompactWindow`, la ventana con que Claude Code decide cuándo compactar (medido en
// el binario 2.1.283: `/context` pasa de «/ 1m» a «/ 400k»). Con un modelo de 200k no cambia nada:
// Claude Code la recorta a la del modelo.
//
// SÓLO SE TOCA LO QUE MUSUBI PUSO, igual que los permisos. Un valor que la persona ya tenía se
// respeta y no se anota. El que pone Musubi queda anotado en el plugin (marcaDeAhorro): reinstalar
// lo actualiza, `quitar` lo saca, y si la persona lo cambia a mano deja de ser de Musubi.

const (
	// marcaDeAhorro es el archivo, dentro del plugin, donde queda qué ventana puso Musubi y en qué
	// settings.
	marcaDeAhorro = ".musubi-ahorro.json"
	// claveVentanaDeCompactacion es la clave del settings.json de Claude Code.
	claveVentanaDeCompactacion = "autoCompactWindow"
	// ventanaDeCompactacionPorDefecto es la que pone la instalación. Más abajo el ahorro casi no crece
	// (200k: 48 %, 150k: 48 %) y las compactaciones se disparan (54 y 90 en cuatro días, contra 38).
	ventanaDeCompactacionPorDefecto = 250_000
)

// ahorroAnotado es lo que queda escrito en marcaDeAhorro.
type ahorroAnotado struct {
	Settings string `json:"settings"`
	Ventana  int    `json:"ventana"`
}

// ventanaEn lee la ventana de un settings ya leído: si hay una y, si la hay, si es un número entero.
// Otra cosa (un texto, "auto") es de la persona y no se interpreta.
func ventanaEn(raiz map[string]json.RawMessage) (valor int, hay, esNumero bool) {
	crudo, ok := raiz[claveVentanaDeCompactacion]
	if !ok {
		return 0, false, false
	}
	if json.Unmarshal(crudo, &valor) != nil {
		return 0, true, false
	}
	return valor, true, true
}

// instalarAhorro deja en `settings` la ventana de compactación `ventana` (0: ninguna de Musubi) y
// anota en el plugin si la puso Musubi. Si la persona tiene una propia, no la toca y la devuelve.
func instalarAhorro(dirPlugin, settings string, ventana int) (ahorroAnotado, string, error) {
	previo, _ := leerAhorroAnotado(dirPlugin)
	if previo.Settings != "" && previo.Settings != settings {
		if err := sacarVentana(previo.Settings, previo.Ventana); err != nil {
			return ahorroAnotado{}, "", fmt.Errorf("sacar la ventana de %s: %w", previo.Settings, err)
		}
		previo = ahorroAnotado{}
	}
	raiz, perms, err := leerSettings(settings)
	if err != nil {
		return ahorroAnotado{}, "", err
	}
	actual, hay, esNumero := ventanaEn(raiz)
	esDeMusubi := hay && esNumero && previo.Ventana != 0 && actual == previo.Ventana
	if hay && !esDeMusubi {
		// La puso la persona, o cambió a mano la de Musubi: se respeta y deja de estar anotada.
		return ahorroAnotado{}, string(raiz[claveVentanaDeCompactacion]), borrarMarcaDeAhorro(dirPlugin)
	}
	if ventana <= 0 {
		if esDeMusubi {
			delete(raiz, claveVentanaDeCompactacion)
			if err := escribirSettings(settings, raiz, perms); err != nil {
				return ahorroAnotado{}, "", err
			}
		}
		return ahorroAnotado{}, "", borrarMarcaDeAhorro(dirPlugin)
	}
	b, err := json.Marshal(ventana)
	if err != nil {
		return ahorroAnotado{}, "", err
	}
	raiz[claveVentanaDeCompactacion] = b
	if err := escribirSettings(settings, raiz, perms); err != nil {
		return ahorroAnotado{}, "", err
	}
	nuevo := ahorroAnotado{Settings: settings, Ventana: ventana}
	return nuevo, "", escribirJSON(filepath.Join(dirPlugin, marcaDeAhorro), nuevo)
}

// quitarAhorro saca de su settings la ventana que puso Musubi, y borra la anotación.
func quitarAhorro(dirPlugin string) error {
	previo, ok := leerAhorroAnotado(dirPlugin)
	if !ok {
		return nil
	}
	if err := sacarVentana(previo.Settings, previo.Ventana); err != nil {
		return err
	}
	return borrarMarcaDeAhorro(dirPlugin)
}

// sacarVentana saca de un settings la ventana que puso Musubi, si sigue siendo ésa: si la persona la
// cambió, ya es suya.
func sacarVentana(settings string, ventana int) error {
	raiz, perms, err := leerSettings(settings)
	if err != nil {
		return err
	}
	if actual, hay, esNumero := ventanaEn(raiz); !hay || !esNumero || actual != ventana {
		return nil
	}
	delete(raiz, claveVentanaDeCompactacion)
	return escribirSettings(settings, raiz, perms)
}

func leerAhorroAnotado(dirPlugin string) (ahorroAnotado, bool) {
	var a ahorroAnotado
	crudo, err := os.ReadFile(filepath.Join(dirPlugin, marcaDeAhorro))
	if err != nil || json.Unmarshal(crudo, &a) != nil || a.Settings == "" || a.Ventana <= 0 {
		return ahorroAnotado{}, false
	}
	return a, true
}

func borrarMarcaDeAhorro(dirPlugin string) error {
	if err := os.Remove(filepath.Join(dirPlugin, marcaDeAhorro)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// estadoDelAhorro dice qué ventana de compactación rige en `settings` y de quién es.
func estadoDelAhorro(dirPlugin, settings string) string {
	if a, ok := leerAhorroAnotado(dirPlugin); ok {
		settings = a.Settings
	}
	raiz, _, err := leerSettings(settings)
	if err != nil {
		return fmt.Sprintf("Ahorro: no pude leer %s: %v", settings, err)
	}
	actual, hay, esNumero := ventanaEn(raiz)
	if !hay {
		return "Ahorro: sin ventana de compactación. Con el modelo de 1M la conversación crece hasta cerca del millón antes de resumirse; `musubi agente instalar` la pone."
	}
	if a, ok := leerAhorroAnotado(dirPlugin); ok && esNumero && actual == a.Ventana {
		return fmt.Sprintf("Ahorro: la conversación se resume al llegar a %s tokens (autoCompactWindow puesto por Musubi en %s).", enMiles(actual), settings)
	}
	return fmt.Sprintf("Ahorro: ventana de compactación propia (%s) en %s; Musubi no la toca.", string(raiz[claveVentanaDeCompactacion]), settings)
}

// enMiles escribe 400000 como «400k».
func enMiles(n int) string {
	if n%1000 == 0 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprint(n)
}
