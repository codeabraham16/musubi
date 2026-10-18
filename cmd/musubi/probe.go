package main

// probe.go: el comando `musubi doctor --probe`.
//
// Abre la base en SÓLO LECTURA (NewDbEngineSoloLectura, con PRAGMA query_only) y reporta el estado
// del esquema sin migrarla. El `doctor` normal abre con NewDbEngine, que MIGRA al abrir y deja a un
// binario viejo de la flota sin poder reabrir la base; el probe es la pregunta previa, sin ese costo.
//
// La ELECCIÓN DEL CONSTRUCTOR vive acá a propósito, no dentro de Probe(): es la decisión que hay que
// cuidar (abrir para leer, no para migrar), y tenerla en esta capa es lo que deja al test sabotearla
// —cambiar NewDbEngineSoloLectura por NewDbEngine— y verla ponerse roja.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"musubi/internal/memory"
)

// runDoctorProbe abre la base en sólo lectura, arma el informe y lo imprime. Devuelve el código de
// salida: 0 si la base es legible por este binario, 1 si es más nueva e ilegible o si algo falló.
// Devolver el código (en vez de os.Exit acá) lo hace testeable sin un helper-process.
func runDoctorProbe(root string, asJSON bool) int {
	engine, err := memory.NewDbEngineSoloLectura(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor --probe: no se pudo abrir la base en sólo lectura: %v\n", err)
		return 1
	}
	defer engine.Close()

	rep, err := engine.Probe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor --probe: %v\n", err)
		return 1
	}

	formatProbe(os.Stdout, rep, asJSON)

	// Exit code scriptable (mismo espíritu que `musubi cerebro --alcance`): 0 salvo que la base esté
	// fuera del alcance de lectura de este binario, para que un script lo distinga sin parsear.
	if rep.Estado == memory.EsquemaMasNuevaIlegible {
		return 1
	}
	return 0
}

// fraseEstado traduce el estado del esquema a una línea para una persona.
func fraseEstado(e memory.EstadoEsquema) string {
	switch e {
	case memory.EsquemaAlDia:
		return "la base está al día; este binario la abre y escribe normal"
	case memory.EsquemaMigrable:
		return "la base es más vieja; un arranque normal la migraría hacia adelante"
	case memory.EsquemaMasNuevaLegible:
		return "la base es más nueva pero LEGIBLE; el daemon la sirve en sólo lectura"
	case memory.EsquemaMasNuevaIlegible:
		return "la base es más nueva y este binario NO la alcanza; actualizá musubi"
	default:
		return string(e)
	}
}

// formatProbe imprime el informe en el writer: como JSON si asJSON, o en texto para una persona.
func formatProbe(w io.Writer, rep memory.ProbeReport, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return
	}

	piso := fmt.Sprintf("v%d", rep.PisoDeLectura)
	if !rep.PisoGrabado {
		piso = "sin piso grabado"
	}
	fmt.Fprintln(w, "musubi doctor --probe (sólo lectura, la base no se toca)")
	fmt.Fprintf(w, "  esquema de la base : v%d\n", rep.SchemaActual)
	fmt.Fprintf(w, "  esquema del binario: v%d\n", rep.SchemaEsperado)
	fmt.Fprintf(w, "  piso de lectura    : %s\n", piso)
	fmt.Fprintf(w, "  estado             : %s — %s\n", rep.Estado, fraseEstado(rep.Estado))
	fmt.Fprintf(w, "  sólo lectura       : %v\n", rep.SoloLectura)
	for _, tabla := range []string{"observations", "observation_relations"} {
		n, ok := rep.Conteos[tabla]
		switch {
		case !ok:
			continue
		case n < 0:
			fmt.Fprintf(w, "  %-18s : (no se pudo contar)\n", tabla)
		default:
			fmt.Fprintf(w, "  %-18s : %d\n", tabla, n)
		}
	}
}
