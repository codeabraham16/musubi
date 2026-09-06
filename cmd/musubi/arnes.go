package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"musubi/internal/memory"
)

// arnes.go implementa `musubi arnes`: la fase que comprueba si el arnés de revisión SE ENCENDIÓ.
//
// POR QUÉ ESTE COMANDO EXISTE. Todo lo demás de este track mejora un arnés que hoy corre CERO
// veces. El modo de falla dominante de este repo está medido y tiene nombre: se construye y no se
// enciende. Sería absurdo cerrar un plan sobre encendido sin la pieza que comprueba el encendido —
// y sin ella el plan entero es una apuesta.
//
// LO QUE MÁS IMPORTA ACÁ NO ES MEDIR, ES DISTINGUIR. Un cero puede significar dos cosas opuestas y
// cada una pide lo contrario de la otra:
//
//   - el bloque NUNCA SE INYECTÓ  ⇒ el problema es el mecanismo, y hay que arreglarlo;
//   - se inyectó y NADIE LO SIGUIÓ ⇒ el problema es la hipótesis, y hay que dejar de agregar piezas.
//
// Confundirlas es exactamente cómo se termina agregando la séptima pieza a algo que nadie iba a
// usar. Por eso el veredicto se calcula de las dos señales juntas, nunca de una.

// EL CRITERIO DE ÉXITO SE ESCRIBE ANTES DE MEDIR.
//
// Está acá, en el código, y no en la cabeza de nadie: si a las dos semanas musubi_debate sigue en
// cero CON el gate disparando, el problema no era el catálogo ni el texto de la skill. Escribirlo
// ahora es lo que evita la tentación de explicar el cero a posteriori — que es lo que siempre pasa
// cuando el criterio se decide después de ver el número.
const criterioDeExito = "CRITERIO ESCRITO ANTES DE MEDIR: si a las dos semanas el gate disparó y musubi_debate sigue en CERO, " +
	"el problema no es el catálogo ni el texto de la skill. Hay que DEJAR DE AGREGARLE PIEZAS y revisar la hipótesis de fondo."

// ventanaPorDefecto son los días que mira el comando. Dos semanas es lo que el plan fijó como
// horizonte para juzgar el encendido.
const ventanaPorDefecto = 14

// MedicionArnes son las cuatro señales, ya recolectadas.
type MedicionArnes struct {
	Dias           int    `json:"dias"`
	TokensGate     int    `json:"tokens_review_gate"`
	LlamadasDebate int    `json:"llamadas_musubi_debate"`
	Abiertos       int    `json:"debates_abiertos"`
	Cerrados       int    `json:"debates_cerrados"`
	Vueltas        []int  `json:"vueltas_por_cadena"`
	Veredicto      string `json:"veredicto"`
	Diagnostico    string `json:"diagnostico"`
	Criterio       string `json:"criterio"`
}

// reVuelta reconoce la convención de topic de F4: «... · vuelta k/K · ...».
var reVuelta = regexp.MustCompile(`vuelta\s+(\d+)\s*/\s*(\d+)`)

// vueltasDeTopics extrae el número de vuelta de cada topic que siga la convención. Devuelve una
// lista ordenada. Los topics que no la siguen se ignoran: son debates de antes de F4, o de otra
// cosa, y contarlos como «vuelta 1» inflaría la medición con lo que no se está midiendo.
func vueltasDeTopics(topics []string) []int {
	var out []int
	for _, t := range topics {
		if m := reVuelta.FindStringSubmatch(t); m != nil {
			if k, err := strconv.Atoi(m[1]); err == nil {
				out = append(out, k)
			}
		}
	}
	sort.Ints(out)
	return out
}

// veredictoArnes traduce las señales a un veredicto y su diagnóstico. Es PURA a propósito: es la
// única parte con juicio, y tiene que poder testearse con números fijos.
//
// Los cuatro estados no son grados de lo mismo: cada uno pide una acción distinta.
func veredictoArnes(m MedicionArnes) (veredicto, diagnostico string) {
	switch {
	case m.TokensGate == 0 && m.LlamadasDebate == 0:
		return "APAGADO", "el bloque del gate no se inyectó NI UNA VEZ (0 tokens en la superficie review_gate). " +
			"Antes de tocar la skill: comprobá que el hook UserPromptSubmit corra el binario nuevo, y que " +
			"MUSUBI_REVIEW_GATE no esté en 0. El problema es de cableado, no de texto."
	case m.TokensGate == 0 && m.LlamadasDebate > 0:
		return "SE USA SIN EL GATE", "hubo debates sin que el gate se inyectara: alguien invoca la revisión por su " +
			"cuenta. Es una buena noticia sobre la práctica y una mala sobre el mecanismo — el gate sigue sin correr."
	case m.LlamadasDebate == 0:
		return "IGNORADO", "el gate SÍ se inyectó (" + strconv.Itoa(m.TokensGate) + " tokens) y musubi_debate sigue en CERO. " +
			"Esto ya no se arregla con más mecanismo: el bloque llegó y no se siguió. " + criterioDeExito
	case m.Abiertos > 0 && m.Cerrados == 0:
		return "A MEDIO CAMINO", "se abren debates y no se recuenta ninguno. Un debate abierto para siempre no es un " +
			"debate en curso: es un panel que alguien lanzó y nadie cerró con action=tally."
	default:
		return "ENCENDIDO", "el gate se inyecta y los debates se abren y cierran. Lo que queda por calibrar es el K de " +
			"las vueltas, que hoy es un juicio y no una medición."
	}
}

// runArnes implementa el comando.
func runArnes(args []string) {
	dias := ventanaPorDefecto
	comoJSON := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			comoJSON = true
		case "--dias":
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
					dias = n
				}
			}
		}
	}

	root := workspaceDir()
	eng, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude abrir la memoria:", err)
		os.Exit(1)
	}
	defer eng.Close()
	ctx := context.Background()

	m := MedicionArnes{Dias: dias, Criterio: criterioDeExito}

	// 1) ¿El gate disparó? Cero tokens en la superficie = el bloque nunca se inyectó.
	if l, lerr := eng.LedgerStatus(); lerr == nil {
		m.TokensGate = l.Surfaces["review_gate"]
	}
	// 2) ¿Alguien lo obedeció?
	if filas, uerr := eng.ToolUsage(ctx, dias); uerr == nil {
		for _, f := range filas {
			if f.Tool == "musubi_debate" {
				m.LlamadasDebate = f.Calls
			}
		}
	}
	// 3) y 4) ¿Los debates cierran, y cuántas vueltas toma una corrección?
	if st, derr := eng.EstadoDebatesCtx(ctx); derr == nil {
		m.Abiertos, m.Cerrados = st.Abiertos, st.Cerrados
		m.Vueltas = vueltasDeTopics(st.Topics)
	}

	m.Veredicto, m.Diagnostico = veredictoArnes(m)

	if comoJSON {
		b, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(b))
		return
	}
	fmt.Printf("Arnés de revisión — ventana de %d día(s)\n\n", m.Dias)
	fmt.Printf("  1. ¿el gate disparó?          %s\n", conCero(m.TokensGate, "tokens en la superficie review_gate"))
	fmt.Printf("  2. ¿alguien lo obedeció?      %s\n", conCero(m.LlamadasDebate, "llamadas a musubi_debate"))
	fmt.Printf("  3. ¿los debates cierran?      %d cerrado(s), %d abierto(s)\n", m.Cerrados, m.Abiertos)
	fmt.Printf("  4. ¿cuántas vueltas toma?     %s\n\n", resumenVueltas(m.Vueltas))
	fmt.Printf("  %s: %s\n", m.Veredicto, m.Diagnostico)
}

// conCero muestra un número y, si es cero, lo dice con todas las letras: un 0 en una columna es
// fácil de leer por arriba, y acá el 0 es el dato.
func conCero(n int, que string) string {
	if n == 0 {
		return "CERO " + que
	}
	return fmt.Sprintf("%d %s", n, que)
}

// resumenVueltas describe la distribución de vueltas medida. El K=3 de F4 es un JUICIO, no una
// medición: se calibra con esto o queda inventado.
func resumenVueltas(v []int) string {
	if len(v) == 0 {
		return "sin cadenas de corrección todavía (el K=3 sigue siendo un juicio, no una medición)"
	}
	partes := make([]string, 0, len(v))
	for _, k := range v {
		partes = append(partes, strconv.Itoa(k))
	}
	return fmt.Sprintf("%d cadena(s), vueltas: %s (máx %d)", len(v), strings.Join(partes, ", "), v[len(v)-1])
}
