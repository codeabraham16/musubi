// Command arnes corre los sabotajes que el árbol declara y lleva la cuenta.
//
// ═════════════════════════════════════════════════════════════════════════════════════════════
// EL ÁRBOL PROMETE 739 SABOTAJES Y HASTA HOY SE PODÍA CORRER UNO A LA VEZ, A MANO
//
// `deploy/pruebas/sabotaje.sh` corre UNO y es bueno en eso: sabe distinguir las ocho formas de
// falso verde que este repo ya midió. Lo que no había era la LISTA: nada recorría los sabotajes
// declarados ni llevaba la cuenta de cuáles tienen veredicto. El cabo A123 lo dice con números —38
// de 396 auditados alguna vez, el 9,9 %— y pide cerrarse «con una lista ejecutable y su cuenta,
// no con otra auditoría a mano».
//
// Esto es la lista. El que juzga sigue siendo `sabotaje.sh`: acá no se reimplementa ni una de sus
// ocho comprobaciones.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// ESTE ARNÉS CORRE SIN BUILD TAGS, Y ESO ACOTA LO QUE SU VERDE SIGNIFICA
//
// El paquete se deriva del directorio del archivo de prueba y se corre `go vet <paquete>` +
// `go test <paquete> -run ^<prueba>$` PELADO. Lo levantó otra sesión midiendo un caso suyo, y son
// dos límites distintos:
//
//	· UN SABOTAJE QUE SÓLO ROMPE BAJO TAGS PASA DESAPERCIBIDO. Medido: subir la versión de
//	  gotreesitter en `go.mod` compila sin tags —`internal/codeintel` no importa el módulo, está
//	  `treesit_off.go`— y con tags falla con `missing go.sum entry`. El arnés diría «el sabotaje
//	  funciona» con confianza, y sería cierto para el job `test` y falso para el de polyglot.
//
//	· UNA GUARDA QUE SÓLO EXISTE BAJO TAGS NO SE PUEDE VERIFICAR ACÁ, y es el peor de los dos.
//	  `treesit_test.go`, `methods_codegraph_polyglot_test.go` y `red_del_parser_test.go` no
//	  compilan sin su tag, así que el control de `sabotaje.sh` las rechaza por «no selecciona
//	  ninguna prueba» — diagnóstico correcto con el sonido equivocado, porque se lee como «el ancla
//	  está mal escrita». Por eso ese rechazo ahora imprime la causa real y el comando con el tag.
//
// O SEA QUE LA COBERTURA QUE ESTE COMANDO REPORTA ES SOBRE EL ÁRBOL SIN TAGS. Decirla sin esa
// salvedad es exactamente el defecto que el censo tuvo que corregir un piso más arriba: un número
// sin su denominador declarado se lee como si fuera del universo.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// USO
//
//	go run ./deploy/cmd/arnes                          # el censo: qué promete el árbol
//	go run ./deploy/cmd/arnes -validar                 # ¿siguen apuntando a donde dicen?
//	go run ./deploy/cmd/arnes -correr                  # corre TODOS los mecanizados
//	go run ./deploy/cmd/arnes -correr -paquete ./internal/mcp
//	go run ./deploy/cmd/arnes -correr -limite 5
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL MUTADOR SE INVOCA POR LA RUTA DEL BINARIO Y NO CON `go run`
//
// `sabotaje.sh` necesita un comando que aplique el sabotaje. Ese comando es este mismo programa en
// modo `-aplicar`, invocado por `os.Executable()`. Si se invocara con `go run ./deploy/cmd/arnes`,
// se RECOMPILARÍA desde el árbol que en ese momento está sabotado: un sabotaje que rompe la
// compilación dejaría al mutador sin poder correr, y el fallo se leería como «el sabotaje no se
// aplicó». El binario ya construido viene del árbol LIMPIO y no se entera.
//
// Y `de`/`a` viajan en ARCHIVOS, no en la línea de comandos: el texto a sabotear trae comillas,
// backticks y barras, y `sabotaje.sh` recibe su comando como un string que pasa por `bash -c`. Un
// literal de Go metido ahí sería un escapado de bash anidado en un escapado de Go — dos capas para
// equivocarse, y este repo ya pagó una (`echo "…`backticks`…"` los EJECUTÓ y la salida salió con un
// `git diff` incrustado en el medio). Una ruta de archivo no tiene metacaracteres.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"musubi/internal/arnes"
)

func main() {
	var (
		raiz       = flag.String("raiz", ".", "raíz del repo")
		validar    = flag.Bool("validar", false, "comprobar que cada directiva siga apuntando a donde dice, sin correr nada")
		correr     = flag.Bool("correr", false, "correr los sabotajes mecanizados vía deploy/pruebas/sabotaje.sh")
		paquete    = flag.String("paquete", "", "correr sólo los de este paquete (ej ./internal/mcp)")
		limite     = flag.Int("limite", 0, "correr como máximo N (0 = todos)")
		insertar   = flag.String("insertar", "", "ruta a un JSON con directivas a escribir en los comentarios")
		aplicar    = flag.Bool("aplicar", false, "modo interno: aplicar un reemplazo en un archivo")
		deArch     = flag.String("de-archivo", "", "modo interno: archivo con el texto a reemplazar")
		aArch      = flag.String("a-archivo", "", "modo interno: archivo con el reemplazo")
		detallar   = flag.Bool("detalle", false, "listar las anclas pendientes agrupadas por archivo")
		listar     = flag.Bool("listar", false, "imprimir CADA ancla con archivo:línea y su prosa")
		salidaJSON = flag.Bool("json", false, "emitir el censo como JSON (para muestrear o alimentar otra herramienta)")
		cada       = flag.Int("cada", 1, "con -json: emitir sólo una de cada N anclas pendientes (muestra sistemática)")
	)
	flag.Parse()

	if *aplicar {
		if err := modoAplicar(flag.Arg(0), *deArch, *aArch); err != nil {
			fmt.Fprintln(os.Stderr, "arnes -aplicar:", err)
			os.Exit(1)
		}
		return
	}

	censo, err := arnes.Censar(*raiz)
	if err != nil {
		fmt.Fprintln(os.Stderr, "arnes:", err)
		os.Exit(2)
	}

	if *insertar != "" {
		n, err := insertarLote(*raiz, censo, *insertar)
		if err != nil {
			fmt.Fprintln(os.Stderr, "arnes -insertar:", err)
			os.Exit(2)
		}
		fmt.Printf("escritas %d directivas. Volvé a correr el censo: las líneas se movieron.\n", n)
		return
	}
	if *salidaJSON {
		// LA MUESTRA ES SISTEMÁTICA —una de cada N en el orden del árbol— Y NO ELEGIDA A DEDO.
		// Si yo eligiera las anclas «mecanizables», el número de guardas huecas que salga estaría
		// medido sobre las fáciles y no sobre el árbol: sesgaría el hallazgo hacia arriba o hacia
		// abajo según mi olfato, que es exactamente lo que este arnés existe para no usar.
		//
		// Sin `Math.random` ni semilla: el orden del censo es estable (git ls-files ordenado + las
		// líneas del archivo), así que «una de cada N» es reproducible y auditable.
		pen := censo.Pendientes()
		var muestra []map[string]any
		for i, a := range pen {
			if *cada > 1 && i%*cada != 0 {
				continue
			}
			muestra = append(muestra, map[string]any{
				"archivo_prueba": a.Archivo,
				"linea":          a.Linea,
				"linea_fin":      a.LineaFin,
				"prueba":         a.Prueba,
				"prosa":          a.Prosa,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{
			"anclas_totales":      len(censo.Anclas),
			"pendientes":          len(pen),
			"mecanizadas":         len(censo.Mecanizadas()),
			"muestra_una_de_cada": *cada,
			"muestra":             muestra,
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}

	imprimirCenso(censo, *detallar)
	if *listar {
		// UNA POR UNA, CON SU PROSA. Es lo que permite auditar al LECTOR y no sólo al árbol: un
		// enumerador que cuenta 774 donde hay 732 infla la deuda con ruido, y la única forma de
		// verlo es leer lo que llamó ancla.
		fmt.Printf("\n── las %d anclas, una por una ──\n", len(censo.Anclas))
		for _, a := range censo.Anclas {
			estado := "PROSA"
			if a.Directiva != nil {
				estado = "arnes"
			} else if a.NoMecanizable != "" {
				estado = "exenta"
			}
			// SE CORTA POR RUNE Y NO POR BYTE. La primera versión hacía `prosa[:110]` y partía un
			// carácter acentuado al medio: el listado salía con UTF-8 inválido, `grep` lo declaraba
			// «binary file matches» y la comparación contra el grep estricto inventó cinco anclas
			// perdidas que no existían. Casi acuso al lector de un agujero que estaba en el
			// impresor. Es el mismo defecto que el árbol ya arregló en methods_design_reparto.
			prosa := []rune(a.Prosa)
			corte := ""
			if len(prosa) > 110 {
				prosa, corte = prosa[:110], "…"
			}
			fmt.Printf("%-6s %s:%d [%s] %s%s\n", estado, a.Archivo, a.Linea, a.Prueba, string(prosa), corte)
		}
	}

	// LAS QUEJAS SON UN CÓDIGO DISTINTO DE CERO, Y NO UN AVISO AL PASAR. Una directiva que no se
	// puede leer es peor que una ausente: la cuenta la suma como mecanizada y nadie la corre.
	salida := 0
	if len(censo.Quejas) > 0 || len(censo.SinUbicar) > 0 {
		salida = 1
	}

	if *validar || *correr {
		// LAS COLISIONES SE INFORMAN PERO NO FALLAN. Dos directivas sobre la misma línea pueden ser
		// dos guardas cubriéndola desde ángulos distintos —medido: el par de `internal/fleet` cae con
		// dos motivos distintos, así que las dos miden— o pueden ser un rojo falso. Lo que sí es
		// seguro es que a lo sumo UNA de las dos es sobre el comportamiento de esa línea. Falla si
		// grita en el caso legítimo; calla si no lo dice. Así que lo dice y no falla.
		if choques := arnes.Colisiones(censo); len(choques) > 0 {
			fmt.Printf("\n! %d sabotaje/s se pisan entre sí:\n", len(choques))
			for _, ch := range choques {
				fmt.Println("   · " + ch)
			}
		}
		males := arnes.Validar(censo)
		if len(males) > 0 {
			fmt.Printf("\n✗ %d directiva/s dejaron de apuntar a donde dicen:\n", len(males))
			for _, m := range males {
				fmt.Println("   ", m)
			}
			salida = 1
		} else if len(censo.Mecanizadas()) > 0 {
			fmt.Printf("\n✓ las %d directivas apuntan a un literal que existe y es único\n", len(censo.Mecanizadas()))
		}
		if *correr && len(males) == 0 {
			if rc := correrTodos(*raiz, censo, *paquete, *limite); rc != 0 {
				salida = rc
			}
		} else if *correr {
			fmt.Println("\nNO SE CORRIÓ NADA: primero hay que arreglar las directivas de arriba. Correr con una " +
				"directiva rota mide otra cosa y su resultado se lee como si midiera ésta.")
		}
	}
	os.Exit(salida)
}

// loteDeDirectivas es una directiva a escribir, identificada por DÓNDE está su ancla.
//
// El ancla se identifica por (archivo de prueba, línea) y NO por el nombre de la prueba: hay
// archivos con varias anclas sobre la misma prueba, y hay anclas que no están pegadas a ninguna.
type loteDeDirectivas struct {
	ArchivoPrueba string `json:"archivo_prueba"`
	Linea         int    `json:"linea"`

	Archivo       string `json:"archivo,omitempty"`
	De            string `json:"de,omitempty"`
	A             string `json:"a,omitempty"`
	Prueba        string `json:"prueba,omitempty"`
	ArregloDe     string `json:"arreglo_de,omitempty"`
	ArregloA      string `json:"arreglo_a,omitempty"`
	NoMecanizable string `json:"no_mecanizable,omitempty"`
}

// insertarLote escribe las directivas en los comentarios, pegadas a la prosa de su propia ancla.
//
// TRES COSAS QUE NO SON OBVIAS Y CADA UNA SALE DE UN DEFECTO DE ESTA CASA:
//
//  1. SE ESCRIBE DE ABAJO HACIA ARRIBA. Insertar líneas mueve todas las de abajo, así que aplicar
//     en orden de archivo dejaría la segunda directiva de cada archivo desplazada. El síntoma
//     sería una directiva pegada al comentario equivocado, que es peor que ninguna.
//
//  2. EL VALOR SE EMITE CON `strconv.Quote` Y NO A MANO. Es la inversa exacta del
//     `strconv.Unquote` que lo lee, así que el ida y vuelta es correcto por construcción para
//     comillas, backslashes, tabs y saltos. Un escapado escrito a mano acá sería el esquema casero
//     que todo este paquete existe para no tener.
//
//  3. SE RECHAZA ANTES DE ESCRIBIR. Si el `de` no aparece exactamente una vez en el archivo de
//     producción, la directiva no se escribe: un corpus con directivas que no se aplican produce
//     verdes que se leen como «la guarda cubre».
func insertarLote(raiz string, censo arnes.Censo, rutaJSON string) (int, error) {
	crudo, err := os.ReadFile(rutaJSON)
	if err != nil {
		return 0, err
	}
	var lote []loteDeDirectivas
	dec := json.NewDecoder(strings.NewReader(string(crudo)))
	// UN CAMPO DESCONOCIDO ES UN ERROR Y NO UN CAMPO IGNORADO: un `no_mecanizble` mal tipeado se
	// descartaría en silencio y la directiva quedaría a medias, que es la forma en que este repo
	// pierde parámetros (`project` vs `project_id`, 22 herramientas contra 3).
	dec.DisallowUnknownFields()
	if err := dec.Decode(&lote); err != nil {
		return 0, fmt.Errorf("el JSON del lote no se pudo leer: %w", err)
	}

	porAncla := map[string]arnes.Ancla{}
	for _, a := range censo.Anclas {
		porAncla[fmt.Sprintf("%s:%d", a.Archivo, a.Linea)] = a
	}

	// Agrupadas por archivo y ordenadas de abajo hacia arriba.
	porArchivo := map[string][]loteDeDirectivas{}
	for _, d := range lote {
		clave := fmt.Sprintf("%s:%d", d.ArchivoPrueba, d.Linea)
		a, ok := porAncla[clave]
		if !ok {
			return 0, fmt.Errorf("no hay ningún ancla en %s: ¿se movió el archivo? volvé a censar", clave)
		}
		if a.Directiva != nil || a.NoMecanizable != "" {
			return 0, fmt.Errorf("%s ya tiene directiva: no se sobreescribe", clave)
		}
		if d.NoMecanizable == "" {
			if d.Archivo == "" || d.De == "" {
				return 0, fmt.Errorf("%s: faltan `archivo` y/o `de`", clave)
			}
			b, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(d.Archivo)))
			if err != nil {
				return 0, fmt.Errorf("%s: no puedo leer %s: %w", clave, d.Archivo, err)
			}
			if n := strings.Count(string(b), d.De); n != 1 {
				return 0, fmt.Errorf("%s: el `de` aparece %d veces en %s y tiene que aparecer 1. "+
					"NO se escribió nada", clave, n, d.Archivo)
			}
		}
		porArchivo[d.ArchivoPrueba] = append(porArchivo[d.ArchivoPrueba], d)
	}

	escritas := 0
	for archivo, ds := range porArchivo {
		ruta := filepath.Join(raiz, filepath.FromSlash(archivo))
		b, err := os.ReadFile(ruta)
		if err != nil {
			return escritas, err
		}
		info, err := os.Stat(ruta)
		if err != nil {
			return escritas, err
		}
		lineas := strings.Split(string(b), "\n")

		sort.Slice(ds, func(i, j int) bool { return ds[i].Linea > ds[j].Linea })
		for _, d := range ds {
			a := porAncla[fmt.Sprintf("%s:%d", d.ArchivoPrueba, d.Linea)]
			if a.LineaFin < 1 || a.LineaFin > len(lineas) {
				return escritas, fmt.Errorf("%s:%d: la línea de inserción (%d) cae fuera del archivo",
					archivo, d.Linea, a.LineaFin)
			}
			// El prefijo se copia de la línea del ancla: un comentario adentro de una función
			// viene con tabulaciones, y una directiva sin ellas la reformatearía gofmt.
			prefijo := lineas[a.LineaFin-1]
			if i := strings.Index(prefijo, "//"); i >= 0 {
				prefijo = prefijo[:i+2]
			} else {
				prefijo = "//"
			}
			nuevas := lineasDeDirectiva(prefijo, d)
			resto := append([]string{}, lineas[a.LineaFin:]...)
			lineas = append(lineas[:a.LineaFin], append(nuevas, resto...)...)
			escritas++
		}
		if err := os.WriteFile(ruta, []byte(strings.Join(lineas, "\n")), info.Mode().Perm()); err != nil {
			return escritas, err
		}
	}
	return escritas, nil
}

func lineasDeDirectiva(prefijo string, d loteDeDirectivas) []string {
	var out []string
	agregar := func(clave, valor string) {
		if valor == "" {
			return
		}
		out = append(out, fmt.Sprintf("%s %s %s=%s", prefijo, arnes.PrefijoDirectiva, clave, strconv.Quote(valor)))
	}
	if d.NoMecanizable != "" {
		agregar("no_mecanizable", d.NoMecanizable)
		return out
	}
	agregar("prueba", d.Prueba)
	agregar("archivo", d.Archivo)
	agregar("de", d.De)
	// `a` puede ser la cadena vacía a propósito —borrar es un sabotaje legítimo— así que NO pasa
	// por `agregar`, que descarta los vacíos.
	out = append(out, fmt.Sprintf("%s %s a=%s", prefijo, arnes.PrefijoDirectiva, strconv.Quote(d.A)))
	agregar("arreglo_de", d.ArregloDe)
	agregar("arreglo_a", d.ArregloA)
	return out
}

func modoAplicar(ruta, deArch, aArch string) error {
	if ruta == "" || deArch == "" {
		return fmt.Errorf("faltan la ruta del archivo y/o -de-archivo")
	}
	de, err := os.ReadFile(deArch)
	if err != nil {
		return err
	}
	var a []byte
	if aArch != "" {
		if a, err = os.ReadFile(aArch); err != nil {
			return err
		}
	}
	return arnes.Aplicar(ruta, string(de), string(a))
}

func imprimirCenso(c arnes.Censo, detalle bool) {
	mec, rot, exe, pen := c.Mecanizadas(), c.Rotas(), c.Exentas(), c.Pendientes()
	porArchivo := map[string]bool{}
	for _, a := range c.Anclas {
		porArchivo[a.Archivo] = true
	}

	fmt.Printf("CENSO DE SABOTAJES DECLARADOS\n")
	fmt.Printf("  archivos de prueba mirados : %d\n", c.Archivos)
	fmt.Printf("  anclas encontradas         : %d  (en %d archivos)\n", len(c.Anclas), len(porArchivo))
	fmt.Printf("  ├─ mecanizadas (`arnes:`)  : %d\n", len(mec))
	fmt.Printf("  ├─ ROTAS (directiva ilegible): %d\n", len(rot))
	fmt.Printf("  ├─ exentas (no_mecanizable): %d\n", len(exe))
	fmt.Printf("  └─ EN PROSA Y NADA MÁS     : %d\n", len(pen))
	if len(c.Anclas) > 0 {
		// LA COBERTURA SÓLO CUENTA LAS QUE DE VERDAD SE PUEDEN CORRER. La primera versión sumaba
		// las rotas y la cobertura subía con sabotajes que nadie podía aplicar — la misma mentira
		// que este arnés viene a cazar, una vuelta más adentro.
		fmt.Printf("  cobertura ejecutable       : %.1f %%\n", 100*float64(len(mec))/float64(len(c.Anclas)))
	}

	if len(c.SinTrackear) > 0 {
		// VA ANTES QUE CUALQUIER OTRO NÚMERO: si estás escribiendo pruebas nuevas, el censo NO las
		// ve, y su «0 mecanizadas» significa «todavía no las agregaste», no «no hay ninguna». Me
		// pasó escribiendo esto.
		fmt.Printf("\n! %d `_test.go` SIN TRACKEAR: este censo deriva de `git ls-files`, así que NO los mira.\n", len(c.SinTrackear))
		for _, s := range c.SinTrackear {
			fmt.Println("   ", s)
		}
		fmt.Println("    (`git add` y volvé a correr: sus anclas no están contadas arriba)")
	}
	if len(c.SinUbicar) > 0 {
		// ESTO ES UN DEFECTO DEL LECTOR, NO DEL ÁRBOL, y va primero: un agujero en el enumerador
		// se ve idéntico a un árbol sin deuda.
		fmt.Printf("\n✗ %d ancla/s que este lector VIO en el texto y no pudo colocar en el AST:\n", len(c.SinUbicar))
		for _, s := range c.SinUbicar {
			fmt.Println("   ", s)
		}
	}
	if len(c.Quejas) > 0 {
		fmt.Printf("\n✗ %d directiva/s ilegibles (se DENUNCIAN, no se saltean):\n", len(c.Quejas))
		for _, q := range c.Quejas {
			fmt.Println("   ", q)
		}
	}
	if detalle && len(pen) > 0 {
		fmt.Printf("\n── las %d en prosa, por archivo ──\n", len(pen))
		cuenta := map[string]int{}
		for _, a := range pen {
			cuenta[a.Archivo]++
		}
		claves := make([]string, 0, len(cuenta))
		for k := range cuenta {
			claves = append(claves, k)
		}
		sort.Slice(claves, func(i, j int) bool {
			if cuenta[claves[i]] != cuenta[claves[j]] {
				return cuenta[claves[i]] > cuenta[claves[j]]
			}
			return claves[i] < claves[j]
		})
		for _, k := range claves {
			fmt.Printf("  %3d  %s\n", cuenta[k], k)
		}
	}
}

// revisarElRojo LEE LA LÍNEA DEL FALLO, QUE ES LO QUE EL EXIT CODE NO DICE.
//
// `sabotaje.sh` sale con 0 cuando la prueba se puso en rojo, y eso NO ES LO MISMO que «la guarda
// cazó el defecto declarado». Este repo ya lo tiene escrito con nombre: un rojo también miente —por
// build roto, o porque el sabotaje hizo fallar OTRA aserción de la misma prueba— y por eso la regla
// es leer la LÍNEA y no el código de salida. Contar `err == nil` era cometer, adentro del arnés,
// exactamente el atajo que el arnés existe para no dejar pasar.
//
// El guion ya imprime el motivo y dice para qué: «la primera línea de cada fallo, para poder
// compararlos». Lo que no puede hacer es compararlos, porque ve UNA corrida por vez. Quien ve las
// setenta es este comando, así que la comparación va acá.
//
// Devuelve el motivo canónico —`archivo_test.go:línea: mensaje`, SIN el nombre de la prueba, para
// que dos pruebas distintas que caen en la MISMA aserción den la misma clave— y una queja cuando lo
// que se puso en rojo no es lo que el ancla declaraba. Una queja no invalida el rojo: lo manda a
// leer a mano, que es lo que hoy no pasaba nunca.
func revisarElRojo(salida, archivoAncla, pruebaDeclarada string) (motivo, queja string) {
	var fallos, crudos []string
	seccion := ""
	for _, l := range strings.Split(salida, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.Contains(t, "ROJO, y falla en:"):
			seccion = "fallos"
			continue
		case strings.HasPrefix(t, "── motivo"):
			seccion = "motivos"
			continue
		case t == "", strings.HasPrefix(t, "✓"), strings.HasPrefix(t, "✗"), strings.HasPrefix(t, "▶"):
			seccion = ""
			continue
		}
		switch seccion {
		case "fallos":
			fallos = append(fallos, t)
		case "motivos":
			crudos = append(crudos, t)
		}
	}

	if len(fallos) == 0 {
		return "", "salió con 0 y no imprimió NINGUNA prueba fallando: no sé qué se puso en rojo"
	}
	// El `-run` filtra a la prueba declarada, así que una raíz distinta significa que el patrón no
	// es el que creo. Los subtests (`TestX/caso`) cuelgan de su raíz y no son un desvío.
	if pruebaDeclarada != "" {
		for _, f := range fallos {
			if raiz, _, _ := strings.Cut(f, "/"); raiz != pruebaDeclarada {
				return "", "cayó `" + raiz + "` y el ancla declara `" + pruebaDeclarada + "`"
			}
		}
	}
	if len(crudos) == 0 {
		return "", "se puso en rojo sin una línea de `_test.go` que ubique la aserción"
	}

	// La PRIMERA es la que el guion eligió como motivo; se le saca el nombre de la prueba de
	// adelante para que la clave sea la ASERCIÓN y no quién la ejercitó.
	primera := crudos[0]
	if _, resto, ok := strings.Cut(primera, " · "); ok {
		primera = resto
	}
	arch, ok := archivoDelMotivo(primera)
	if !ok {
		return "", "el motivo no nombra un `_test.go:línea`: " + primerasRunas(primera, 80)
	}
	if base := archivoAncla[strings.LastIndex(archivoAncla, "/")+1:]; arch != base {
		queja = "la aserción que cayó vive en " + arch + " y el ancla está en " + base
	}
	return primera, queja
}

// archivoDelMotivo saca el `foo_test.go` de una línea `foo_test.go:123: mensaje`.
//
// A mano y no con una expresión regular porque la condición es exacta y corta —`_test.go:` seguido
// de al menos un dígito— y `regexp` sería un import nuevo que el primer sabotaje sobre esta función
// dejaría huérfano, que es la clase que ya dejó una guarda de este paquete sin poder medirse.
func archivoDelMotivo(l string) (string, bool) {
	const marca = "_test.go:"
	i := strings.Index(l, marca)
	if i < 0 {
		return "", false
	}
	digitos := 0
	for j := i + len(marca); j < len(l) && l[j] >= '0' && l[j] <= '9'; j++ {
		digitos++
	}
	if digitos == 0 {
		return "", false
	}
	return l[strings.LastIndexAny(l[:i], " \t")+1 : i+len(marca)-1], true
}

// primerasRunas recorta POR RUNA y no por byte. Cortar por byte una prosa con acentos parte el
// carácter, y este repo ya pagó ese corte: la salida quedó UTF-8 inválido, `grep` la llamó «binary
// file matches» y la comparación que seguía inventó cinco anclas perdidas.
func primerasRunas(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// correrTodos pasa cada directiva por `sabotaje.sh` y cuenta los veredictos.
//
// EL VEREDICTO QUE IMPORTA ES EL VERDE: una guarda que queda en verde sobre su propio defecto
// declarado es una red que no está, y su prosa enseña a confiar en ella. Ése es el número que este
// comando existe para producir.
func correrTodos(raiz string, c arnes.Censo, soloPaquete string, limite int) int {
	guion := filepath.Join(raiz, "deploy", "pruebas", "sabotaje.sh")
	if _, err := os.Stat(guion); err != nil {
		fmt.Fprintln(os.Stderr, "no encuentro deploy/pruebas/sabotaje.sh:", err)
		return 2
	}
	yo, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "no sé cuál es mi propio binario, que es el que aplica el sabotaje:", err)
		return 2
	}

	tmp, err := os.MkdirTemp("", "arnes-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer os.RemoveAll(tmp)

	// La foto del árbol ANTES de tocar nada. Se compara al final: ver el bloque del cierre.
	antes, err := estadoDelArbol(raiz)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude fotografiar el estado del árbol antes de empezar:", err)
		fmt.Fprintln(os.Stderr, "sin esa foto no se puede saber si la corrida dejó un sabotaje puesto")
		return 2
	}

	var corridas, rojos, verdes, errores int
	// EL MOTIVO DE CADA ROJO, PARA PODER COMPARARLOS. Ver `revisarElRojo`: el exit code dice que
	// la prueba cayó, no POR QUÉ, y dos sabotajes que caen por lo mismo son uno contado dos veces.
	motivos := map[string][]string{}
	var sospechas []string
	var huecas []string
	for _, a := range c.Mecanizadas() {
		d := a.Directiva
		if soloPaquete != "" && d.Paquete != soloPaquete {
			continue
		}
		if limite > 0 && corridas >= limite {
			break
		}
		corridas++
		fmt.Printf("\n══ %d · %s:%d · %s\n", corridas, a.Archivo, a.Linea, d.Prueba)

		cmdSab, err := comandoMutador(yo, tmp, fmt.Sprintf("%d", corridas), d.De, d.A)
		if err != nil {
			fmt.Println("   ✗ no pude preparar el sabotaje:", err)
			errores++
			continue
		}
		args := []string{d.Paquete, "^" + d.Prueba + "$", d.Archivo, cmdSab}
		if d.ArregloDe != "" {
			cmdArr, err := comandoMutador(yo, tmp, fmt.Sprintf("%d-arreglo", corridas), d.ArregloDe, d.ArregloA)
			if err != nil {
				fmt.Println("   ✗ no pude preparar el arreglo:", err)
				errores++
				continue
			}
			args = append(args, cmdArr)
		}

		cmd := exec.Command(guion, args...)
		cmd.Dir = raiz
		salida, err := cmd.CombinedOutput()
		for _, l := range strings.Split(strings.TrimRight(string(salida), "\n"), "\n") {
			fmt.Println("   " + l)
		}
		switch {
		case err == nil:
			rojos++
			// NO ALCANZA CON QUE `sabotaje.sh` HAYA SALIDO CON 0. Ver `revisarElRojo`.
			motivo, queja := revisarElRojo(string(salida), a.Archivo, d.Prueba)
			if queja != "" {
				fmt.Println("   ! " + queja)
				sospechas = append(sospechas, fmt.Sprintf("%s:%d %s — %s", a.Archivo, a.Linea, d.Prueba, queja))
			}
			if motivo != "" {
				motivos[motivo] = append(motivos[motivo], fmt.Sprintf("%s:%d %s", a.Archivo, a.Linea, d.Prueba))
			}
		case strings.Contains(string(salida), "EL SABOTAJE NO LA PONE EN ROJO"):
			// LA GUARDA QUEDÓ VERDE SOBRE SU PROPIO DEFECTO. Es el desenlace que hay que contar
			// aparte: no es un error de la herramienta, es un hallazgo.
			verdes++
			huecas = append(huecas, fmt.Sprintf("%s:%d %s", a.Archivo, a.Linea, d.Prueba))
		default:
			// Todo lo demás es la herramienta diciendo «no pude medir»: control en rojo, build
			// roto, cero pruebas ejecutadas, guarda que castiga el arreglo. NO se cuenta como
			// veredicto, porque «no medí» y «está bien» no son lo mismo.
			errores++
			// Y UN CASO MERECE QUE SE LE DIGA LA CAUSA REAL, porque el diagnóstico de `sabotaje.sh`
			// es correcto y suena a otra cosa. Cuando el patrón no selecciona ninguna prueba, la
			// causa más probable en este árbol NO es un ancla mal escrita: es que esa prueba vive
			// detrás de un build tag (`treesitter`, `race`) y ESTE ARNÉS CORRE SIN TAGS. «no test
			// files» se lee como «te equivocaste de nombre»; la verdad es «este árbol no compila
			// esa prueba». Es la diferencia entre un límite declarado y un error aparente, y si la
			// primera frase entra a un informe alguien va a ir a arreglar el ancla que está bien.
			if strings.Contains(string(salida), "NO SELECCIONA NINGUNA PRUEBA") {
				fmt.Println("   → OJO CON LA CAUSA: este arnés corre SIN BUILD TAGS. Si la prueba vive detrás")
				fmt.Println("     de un tag (`treesitter`, `race`), no existe en este árbol y el ancla puede estar")
				fmt.Println("     perfecta. Comprobalo con: go test -tags '<el tag>' " + d.Paquete + " -run '^" + d.Prueba + "$'")
			}
		}
	}

	fmt.Printf("\n════════════════════════════════════════════════════\n")
	fmt.Printf("corridas          : %d\n", corridas)
	fmt.Printf("en ROJO (sanas)   : %d\n", rojos)
	fmt.Printf("en VERDE (huecas) : %d   ← el hallazgo\n", verdes)
	fmt.Printf("sin veredicto     : %d   (la herramienta no pudo medir; NO es un verde)\n", errores)
	if len(huecas) > 0 {
		fmt.Println("\nguardas en verde sobre su propio sabotaje declarado:")
		for _, h := range huecas {
			fmt.Println("   ", h)
		}
	}
	// ── LOS ROJOS, LEÍDOS ─────────────────────────────────────────────────────────────────────
	//
	// Dos sabotajes distintos que hacen fallar la MISMA aserción con el MISMO mensaje son un
	// sabotaje solo contado dos veces: la cobertura sube y lo cubierto no. Lo dice `sabotaje.sh`
	// en su propia salida —imprime el motivo «para poder compararlos»— pero no tiene con qué
	// compararlo, porque ve una corrida por vez. Este arnés ve las setenta.
	var repes int
	for motivo, quienes := range motivos {
		if len(quienes) > 1 {
			repes++
			fmt.Printf("\n! MISMO MOTIVO en %d directivas — o es un sabotaje contado de más:\n", len(quienes))
			fmt.Println("   " + motivo)
			for _, q := range quienes {
				fmt.Println("     · " + q)
			}
		}
	}
	fmt.Printf("motivos repetidos : %d   (dos sabotajes que fallan igual son uno solo)\n", repes)
	fmt.Printf("rojos sospechosos : %d   (cayó otra prueba, u otro archivo, o sin línea)\n", len(sospechas))
	for _, s := range sospechas {
		fmt.Println("   " + s)
	}
	// ── EL ÁRBOL TIENE QUE QUEDAR LIMPIO, Y SE COMPRUEBA ─────────────────────────────────────
	//
	// `sabotaje.sh` restaura con `cp` desde un respaldo y lo hace en un `trap EXIT INT TERM`, así
	// que un Ctrl-C o un error restauran. Lo que NO restaura es un SIGKILL: un OOM, o la red que
	// se cae con el proceso adentro. Este repo lo midió: cayó la red y quedaron cuatro worktrees
	// sucios con dos sabotajes puestos, y UNO DE ELLOS MENTÍA EN VERDE.
	//
	// El modo de falla es silencioso y SOBREVIVE A LA SESIÓN: el próximo que corra las pruebas las
	// va a ver fallar por un sabotaje que nadie sabe que está puesto, o —peor— en verde sobre
	// código saboteado. Por eso la corrida termina preguntándole a git si quedó algo, y grita.
	//
	// Se pregunta por `--porcelain` y no por `git diff`: `diff` NO VE los archivos sin trackear, y
	// aunque hoy ningún sabotaje crea archivos, un `de`→`a` que parta un archivo en dos sí lo
	// haría, y ése es exactamente el caso que `git checkout` tampoco sabe deshacer.
	//
	// Y SE COMPARA EL ANTES CONTRA EL DESPUÉS, NO CONTRA VACÍO. Exigir un árbol limpio al final
	// sería un falso positivo cada vez que alguien corre esto con trabajo sin commitear —que es
	// SIEMPRE, porque las directivas recién escritas son exactamente eso—, y una guarda que grita
	// en el caso normal enseña a ignorarla. Lo que importa no es si el árbol está sucio: es si esta
	// corrida lo ensució.
	if despues, err := estadoDelArbol(raiz); err != nil {
		fmt.Printf("\n! no pude preguntarle a git si el árbol quedó como estaba: %v\n", err)
		fmt.Println("  REVISALO A MANO. Un sabotaje que queda puesto es el peor desenlace de esta herramienta.")
		return 1
	} else if despues != antes {
		fmt.Println("\n✗ ESTA CORRIDA LE DEJÓ CAMBIOS AL ÁRBOL. Antes y después no coinciden:")
		fmt.Println("  ── antes ──")
		fmt.Println(sangrar(antes))
		fmt.Println("  ── después ──")
		fmt.Println(sangrar(despues))
		// LO QUE SE SABE ES QUE EL ÁRBOL CAMBIÓ. Por qué cambió NO se sabe, y decirlo como si se
		// supiera manda a mirar el lugar equivocado. La primera versión afirmaba «es un sabotaje
		// puesto» y la primera vez que saltó de verdad la causa era otra: alguien —yo— editando un
		// `_test.go` mientras la corrida iba por la 56 de 73. Diagnóstico correcto, causa inventada.
		// Es la misma forma que ya se corrigió un piso más abajo con «NO SELECCIONA NINGUNA PRUEBA»,
		// que también acertaba y también sonaba a otra cosa.
		fmt.Println("  QUÉ SE SABE: el árbol no quedó como estaba. Las causas, de más grave a más común:")
		fmt.Println("   · UN SABOTAJE QUE QUEDÓ PUESTO. `sabotaje.sh` restaura en un `trap EXIT INT TERM`,")
		fmt.Println("     así que esto pasa cuando el proceso muere por SIGKILL —un OOM, la red— que se")
		fmt.Println("     saltea el trap. Es el peor desenlace: el próximo que corra las pruebas las va a")
		fmt.Println("     ver fallar por algo que nadie sabe que está puesto, o peor, en verde sobre código")
		fmt.Println("     saboteado. Si el archivo de arriba es de PRODUCCIÓN, es casi seguro esto.")
		fmt.Println("   · ALGUIEN EDITÓ EL ÁRBOL MIENTRAS ESTO CORRÍA —otra sesión, o vos en otra ventana.")
		fmt.Println("     Esta corrida tarda minutos y no toma el árbol para sí. Si el archivo de arriba es")
		fmt.Println("     uno que estabas tocando, es esto, y la corrida ya no es homogénea: re-corré.")
		fmt.Println("  Mirá el diff antes de restaurar nada: `git diff` te dice cuál de las dos es.")
		return 1
	}
	fmt.Println("\n✓ el árbol quedó como estaba: ningún sabotaje se quedó puesto")

	// SIN VEREDICTO TAMBIÉN ES SALIDA DISTINTA DE CERO. Si esto saliera 0, una corrida donde la
	// herramienta no pudo medir NADA se leería como una corrida limpia.
	if verdes > 0 || errores > 0 {
		return 1
	}
	return 0
}

// estadoDelArbol es la foto que se compara antes y después de la corrida.
//
// `status --porcelain` y no `diff`: `diff` no ve los archivos sin trackear, y el caso que más duele
// —un sabotaje que queda puesto en un archivo que git no conoce— es justamente el que `diff` y
// `git checkout` no alcanzan.
func estadoDelArbol(raiz string) (string, error) {
	out, err := exec.Command("git", "-C", raiz, "status", "--porcelain").Output()
	if err != nil {
		return "", err
	}
	lineas := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	sort.Strings(lineas) // el orden de git es estable, pero no depender de eso cuesta una línea
	return strings.Join(lineas, "\n"), nil
}

func sangrar(s string) string {
	if strings.TrimSpace(s) == "" {
		return "    (limpio)"
	}
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}

func comandoMutador(binario, tmp, id, de, a string) (string, error) {
	deArch := filepath.Join(tmp, id+".de")
	aArch := filepath.Join(tmp, id+".a")
	if err := os.WriteFile(deArch, []byte(de), 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(aArch, []byte(a), 0o600); err != nil {
		return "", err
	}
	// `"$1"` es el archivo que `sabotaje.sh` le pasa al comando. Las tres rutas van entre comillas
	// dobles: las dos primeras son nuestras y no tienen metacaracteres, pero citarlas igual es lo
	// que hace que esto no dependa de dónde puso el temporal el sistema.
	return fmt.Sprintf(`%q -aplicar -de-archivo %q -a-archivo %q "$1"`, binario, deArch, aArch), nil
}
