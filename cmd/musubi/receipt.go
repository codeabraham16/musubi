package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"musubi/internal/memory"
	"musubi/internal/receipt"
)

// receipt.go implementa `musubi receipt`: el GATE DE ENTREGA de RDD.
//
// La idea, en una línea: un permiso de entrega vale para UNA huella del árbol, y muere cuando el
// árbol cambia. Ver internal/receipt para el porqué del mecanismo.
//
// POR QUÉ pre-push Y NO pre-commit. El commit es el cuaderno de trabajo: frenarlo obliga a pedir
// permiso para cada paso intermedio y el gate termina desactivado a la semana. El push es la
// frontera real —lo que sale del repo y le llega a otro— y es donde una decisión humana merece ser
// vinculante. Un gate que molesta en el lugar equivocado no se respeta: se saltea.
//
// ES OPT-IN Y NO LO TOCA `musubi setup`. Instalar un hook de git cambia el día a día de quien
// trabaja en el repo, y eso no se hace por default en nombre de nadie: hay que pedirlo con
// `musubi receipt install-hook`.

// gitOut corre un comando git en root y devuelve su stdout. Los errores llevan el comando para que
// se sepa cuál de los tres falló.
func gitOut(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// treeFingerprint deriva la huella del árbol de trabajo con las tres piezas que lo definen.
// Ver receipt.Compute para por qué hacen falta las tres.
func treeFingerprint(ctx context.Context, root string) (fp, head string, err error) {
	head, err = gitOut(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	head = strings.TrimSpace(head)
	// `diff HEAD` cubre lo stageado Y lo no stageado de los archivos trackeados, de una.
	diff, err := gitOut(ctx, root, "diff", "HEAD")
	if err != nil {
		return "", "", err
	}
	// Los untracked no aparecen en ningún diff: sin esto, agregar un archivo nuevo no invalidaría
	// el recibo, que es justo por donde se cuela código sin revisar.
	unt, err := gitOut(ctx, root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return "", "", err
	}
	return receipt.Compute(head, diff, unt), head, nil
}

// openReceiptStore abre la memoria del workspace, que es donde vive el recibo.
func openReceiptStore() (*memory.DbEngine, string, error) {
	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		return nil, "", err
	}
	eng, err := memory.NewDbEngine(root)
	if err != nil {
		return nil, "", fmt.Errorf("no pude abrir la memoria: %w", err)
	}
	return eng, root, nil
}

func runReceipt(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: musubi receipt <emit|check|show|freeze|verify|install-hook|uninstall-hook>")
		os.Exit(2)
	}
	switch args[0] {
	case "emit":
		receiptEmit(args[1:])
	case "check":
		receiptCheck()
	case "show":
		receiptShow()
	case "freeze":
		receiptFreeze(args[1:])
	case "verify":
		receiptVerify(args[1:])
	case "install-hook":
		receiptInstallHook(false)
	case "uninstall-hook":
		receiptInstallHook(true)
	default:
		fmt.Fprintf(os.Stderr, "musubi receipt: subcomando desconocido %q\n", args[0])
		os.Exit(2)
	}
}

func receiptEmit(args []string) {
	verdict, reason, by := receipt.Approved, "", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--verdict":
			if i+1 < len(args) {
				i++
				verdict = args[i]
			}
		case "--reason":
			if i+1 < len(args) {
				i++
				reason = args[i]
			}
		case "--by":
			if i+1 < len(args) {
				i++
				by = args[i]
			}
		}
	}
	if verdict != receipt.Approved && verdict != receipt.Rejected {
		fmt.Fprintf(os.Stderr, "veredicto inválido %q: usá %s o %s\n", verdict, receipt.Approved, receipt.Rejected)
		os.Exit(2)
	}

	eng, root, err := openReceiptStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer eng.Close()

	fp, head, err := treeFingerprint(context.Background(), root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude calcular la huella del árbol:", err)
		os.Exit(1)
	}
	r := receipt.Receipt{
		Fingerprint: fp, Head: head, Verdict: verdict,
		Reason: reason, IssuedBy: by, IssuedAt: time.Now().UTC(),
	}
	raw, err := receipt.Encode(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := eng.SetMeta(receipt.MetaKey, raw); err != nil {
		fmt.Fprintln(os.Stderr, "no pude guardar el recibo:", err)
		os.Exit(1)
	}
	fmt.Printf("recibo %s para la huella %s (HEAD %s)\n", verdict, fp[:12], shortHead(head))
	if verdict == receipt.Approved {
		fmt.Println("vale hasta que cambie un byte del árbol.")
	}
}

func receiptCheck() {
	eng, root, err := openReceiptStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer eng.Close()

	fp, _, err := treeFingerprint(context.Background(), root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude calcular la huella del árbol:", err)
		os.Exit(1)
	}
	raw, _, _ := eng.GetMeta(receipt.MetaKey)
	d := receipt.Check(receipt.Decode(raw), fp)
	if !d.Allowed {
		fmt.Fprintln(os.Stderr, "✗ entrega bloqueada:", d.Reason)
		fmt.Fprintln(os.Stderr, "  para habilitarla: musubi receipt emit --reason \"...\"")
		os.Exit(1)
	}
	fmt.Println("✓", d.Reason)
}

func receiptShow() {
	eng, root, err := openReceiptStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer eng.Close()

	raw, _, _ := eng.GetMeta(receipt.MetaKey)
	r := receipt.Decode(raw)
	if r == nil {
		fmt.Println("no hay recibo en este workspace")
		return
	}
	fp, _, err := treeFingerprint(context.Background(), root)
	if err != nil {
		fp = ""
	}
	fmt.Printf("veredicto : %s\n", r.Verdict)
	fmt.Printf("huella    : %s\n", r.Fingerprint[:12])
	fmt.Printf("HEAD      : %s\n", shortHead(r.Head))
	fmt.Printf("emitido   : %s por %s\n", r.IssuedAt.Format(time.RFC3339), orNone(r.IssuedBy))
	if r.Reason != "" {
		fmt.Printf("motivo    : %s\n", r.Reason)
	}
	if fp != "" {
		fmt.Printf("árbol hoy : %s  =>  %s\n", fp[:12], receipt.Check(r, fp).Reason)
	}
}

// hookMarca identifica al hook como nuestro: sin esto no se puede distinguir un hook de Musubi de
// uno que escribió una persona, y pisarlo sería destruir trabajo ajeno en silencio.
const hookMarca = "# musubi-receipt-hook"

func receiptInstallHook(uninstall bool) {
	root := workspaceDir()
	ctx := context.Background()
	// `--git-path hooks` resuelve bien también en worktrees, donde .git es un ARCHIVO y no un
	// directorio: ahí un filepath.Join(root, ".git", "hooks") apuntaría a la nada.
	rel, err := gitOut(ctx, root, "rev-parse", "--git-path", "hooks")
	if err != nil {
		fmt.Fprintln(os.Stderr, "esto no parece un repositorio git:", err)
		os.Exit(1)
	}
	dir := strings.TrimSpace(rel)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	path := filepath.Join(dir, "pre-push")

	existing, _ := os.ReadFile(path)
	esNuestro := strings.Contains(string(existing), hookMarca)

	if uninstall {
		if len(existing) == 0 {
			fmt.Println("no hay hook pre-push instalado")
			return
		}
		if !esNuestro {
			fmt.Fprintln(os.Stderr, "el pre-push existente NO es de Musubi: no lo toco. Borralo a mano si querés.")
			os.Exit(1)
		}
		if err := os.Remove(path); err != nil {
			fmt.Fprintln(os.Stderr, "no pude quitar el hook:", err)
			os.Exit(1)
		}
		fmt.Println("hook pre-push quitado; el gate de entrega queda apagado")
		return
	}

	if len(existing) > 0 && !esNuestro {
		fmt.Fprintf(os.Stderr, "ya hay un hook pre-push que no es de Musubi en %s.\n", path)
		fmt.Fprintln(os.Stderr, "no lo piso: revisalo y, si querés el gate, agregale la línea `musubi receipt check`.")
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		exe = "musubi"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "no pude crear el directorio de hooks:", err)
		os.Exit(1)
	}
	cuerpo := "#!/bin/sh\n" + hookMarca + "\n" +
		"# Gate de entrega RDD: el push pasa sólo si hay un recibo aprobado para ESTA huella\n" +
		"# del árbol. Quitalo con `musubi receipt uninstall-hook`.\n" +
		hookExeCommand(exe, "receipt check") + "\n"
	if err := os.WriteFile(path, []byte(cuerpo), 0755); err != nil {
		fmt.Fprintln(os.Stderr, "no pude escribir el hook:", err)
		os.Exit(1)
	}
	fmt.Printf("hook pre-push instalado en %s\n", path)
	fmt.Println("desde ahora el push exige un recibo aprobado. Emitilo con: musubi receipt emit --reason \"...\"")
}

func shortHead(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	if h == "" {
		return "(sin HEAD)"
	}
	return h
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(anónimo)"
	}
	return s
}

// ─── HALLAZGOS CONGELADOS (F5) ──────────────────────────────────────────────────────────
//
// `receipt freeze` y `receipt verify` aplican RDD a los HALLAZGOS de un panel, no al árbol.
// Son subcomandos NUEVOS y no tocan `receiptCheck` ni `receipt.Check`: el hook pre-push está
// instalado y vivo, y ése es el camino que decide si esta gente puede pushear.
//
// El cuerpo entra por STDIN a propósito. Pasarlo por bandera lo metería en la línea de
// comandos —y en el historial del shell, y en los logs— y además obligaría a escapar un texto
// multilínea, que es exactamente la forma del hallazgo típico.

// idDeArgs extrae --id de los argumentos.
func idDeArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "--id" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// cuerpoDeStdin lee el cuerpo del hallazgo.
func cuerpoDeStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("no pude leer el cuerpo del hallazgo por stdin: %w", err)
	}
	return string(data), nil
}

// abrirHallazgos abre la memoria, deriva la huella del árbol y lee la colección congelada.
func abrirHallazgos() (*memory.DbEngine, string, map[string]receipt.Finding, error) {
	eng, root, err := openReceiptStore()
	if err != nil {
		return nil, "", nil, err
	}
	fp, _, ferr := treeFingerprint(context.Background(), root)
	if ferr != nil {
		eng.Close()
		return nil, "", nil, fmt.Errorf("no pude calcular la huella del árbol: %w", ferr)
	}
	raw, _, _ := eng.GetMeta(receipt.FindingsMetaKey)
	return eng, fp, receipt.DecodeFindings(raw), nil
}

// receiptFreeze congela un hallazgo: id + huella del cuerpo + huella del árbol de HOY.
func receiptFreeze(args []string) {
	id := idDeArgs(args)
	if id == "" {
		fmt.Fprintln(os.Stderr, "uso: musubi receipt freeze --id <debate>/<lente>   (el cuerpo va por stdin)")
		os.Exit(2)
	}
	cuerpo, err := cuerpoDeStdin()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	eng, fp, cong, err := abrirHallazgos()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer eng.Close()

	// Podar ANTES de agregar: tras un fix el árbol cambia y los hallazgos de la vuelta previa
	// se caen solos, así que la vuelta siguiente arranca sólo con lo de hoy. Es media respuesta
	// al alcance decreciente, sin que nadie tenga que acordarse de limpiar.
	cong, podados := receipt.PodarPorArbol(cong, fp)

	f, err := receipt.Congelar(id, cuerpo, fp, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cong[f.ID] = f

	raw, err := receipt.EncodeFindings(cong)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := eng.SetMeta(receipt.FindingsMetaKey, raw); err != nil {
		fmt.Fprintln(os.Stderr, "no pude guardar los hallazgos congelados:", err)
		os.Exit(1)
	}
	fmt.Printf("hallazgo %s congelado (cuerpo %s, árbol %s)\n", f.ID, shortHead(f.BodyHash), shortHead(f.Tree))
	if podados > 0 {
		fmt.Printf("se podaron %d hallazgo(s) de un árbol anterior: el código cambió debajo.\n", podados)
	}
	fmt.Println("el cuerpo NO se guarda: para verificarlo hay que volver a tenerlo.")
}

// receiptVerify comprueba que un cuerpo sea el que se congeló, sobre el mismo árbol.
func receiptVerify(args []string) {
	id := idDeArgs(args)
	if id == "" {
		fmt.Fprintln(os.Stderr, "uso: musubi receipt verify --id <debate>/<lente>   (el cuerpo va por stdin)")
		os.Exit(2)
	}
	cuerpo, err := cuerpoDeStdin()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	eng, fp, cong, err := abrirHallazgos()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer eng.Close()

	d := receipt.Verificar(cong, id, cuerpo, fp)
	if !d.Allowed {
		fmt.Fprintln(os.Stderr, "✗", d.Reason)
		os.Exit(1)
	}
	fmt.Println("✓", d.Reason)
}
