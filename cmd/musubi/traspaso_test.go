package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// El traspaso tras compactar (traspaso.go): la sesión que compacta mucho lo avisa, y la sesión nueva
// recupera su último resumen —redactado— con `musubi traspaso`. Las pruebas van de punta a punta por el
// hook de arranque real (detectOutputSegunFuente) sobre un proyecto con memoria, y con transcripts
// SINTÉTICOS en un directorio temporal: ninguna lee un transcript de verdad.

const marcaDelTraspaso = "[Musubi — traspaso]"

// salidaDelHookDelTraspaso es lo que se mira del envelope del hook.
type salidaDelHookDelTraspaso struct {
	SystemMessage      string `json:"systemMessage"`
	HookSpecificOutput struct {
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func proyectoDelTraspaso(t *testing.T) string {
	t.Helper()
	aislarDelPluginInstalado(t)
	dir := crearGoProject(t)
	memtest.Sembrar(t, dir)
	return dir
}

// transcriptDelTraspaso escribe un transcript sintético: un pedido y, después, un resumen de
// compactación por cada texto, en orden y con timestamps crecientes.
func transcriptDelTraspaso(t *testing.T, sesion string, resumenes ...string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), sesion+".jsonl")
	var b bytes.Buffer
	escribir := func(reg map[string]interface{}) {
		linea, err := json.Marshal(reg)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(linea)
		b.WriteByte('\n')
	}
	escribir(map[string]interface{}{
		"type": "user", "uuid": "u-0", "sessionId": sesion, "timestamp": "2026-10-14T09:00:00.000Z",
		"message": map[string]interface{}{"role": "user", "content": "un pedido cualquiera"},
	})
	for i, r := range resumenes {
		escribir(map[string]interface{}{
			"type": "user", "uuid": fmt.Sprintf("u-%d", i+1), "sessionId": sesion,
			"timestamp":        fmt.Sprintf("2026-10-14T1%d:00:00.000Z", i),
			"isCompactSummary": true,
			"message":          map[string]interface{}{"role": "user", "content": r},
		})
	}
	if err := os.WriteFile(ruta, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// hookDelTraspaso corre el hook de arranque con la entrada dada y devuelve lo que emite.
func hookDelTraspaso(t *testing.T, dir string, in entradaDeArranque) salidaDelHookDelTraspaso {
	t.Helper()
	out, err := detectOutputSegunFuente(dir, true, in)
	if err != nil {
		t.Fatalf("hook de arranque (%s): %v", in.Source, err)
	}
	var s salidaDelHookDelTraspaso
	if strings.TrimSpace(out) == "" {
		return s
	}
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("la salida del hook no es JSON: %v\n%s", err, out)
	}
	return s
}

func compactarDelTraspaso(t *testing.T, dir, sesion, transcript string) salidaDelHookDelTraspaso {
	t.Helper()
	return hookDelTraspaso(t, dir, entradaDeArranque{SessionID: sesion, Source: fuenteCompact, TranscriptPath: transcript})
}

// correrTraspasoEn corre `musubi traspaso` en el proyecto, como lo corre el agente de la sesión nueva.
func correrTraspasoEn(t *testing.T, dir string) (string, string, int) {
	t.Helper()
	t.Setenv("MUSUBI_HOME", dir)
	// El proyecto de prueba no trae config.yaml, y sin él raizDelProceso no lo cuenta como memoria
	// (tieneMemoria): `musubi traspaso` diría que no hay nada que traer. Uno vacío alcanza.
	if cfg := filepath.Join(dir, ".musubi", "config.yaml"); !existeElArchivo(cfg) {
		if err := os.WriteFile(cfg, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	code := traspasoCmd(nil, &out, &errOut)
	return out.String(), errOut.String(), code
}

// A las 3 compactaciones, y de nuevo a las 6, la sesión avisa: al dueño por systemMessage (una línea)
// y al agente por contexto. En la 1, 2, 4 y 5, nada.
//
// Sabotaje: avisar cada 2 compactaciones avisa en la segunda y calla en la tercera.
// arnes: archivo="cmd/musubi/traspaso.go"
// arnes: de="const cadaCuantasCompactaciones = 3"
// arnes: a="const cadaCuantasCompactaciones = 2"
func TestTraspasoAvisaEnTresYEnSeisPeroNoEnCuatro(t *testing.T) {
	dir := proyectoDelTraspaso(t)
	tr := transcriptDelTraspaso(t, "sesion-que-compacta", "resumen uno")
	for i := 1; i <= 6; i++ {
		s := compactarDelTraspaso(t, dir, "sesion-que-compacta", tr)
		alAgente := strings.Contains(s.HookSpecificOutput.AdditionalContext, marcaDelTraspaso)
		if i%3 != 0 {
			if s.SystemMessage != "" || alAgente {
				t.Fatalf("compactación %d: avisó y no tocaba: %+v", i, s)
			}
			continue
		}
		veces := fmt.Sprintf("compactó %d veces", i)
		if !strings.Contains(s.SystemMessage, veces) || strings.Contains(s.SystemMessage, "\n") {
			t.Fatalf("compactación %d: el dueño no recibió el aviso en una línea; systemMessage = %q", i, s.SystemMessage)
		}
		if !alAgente || !strings.Contains(s.HookSpecificOutput.AdditionalContext, veces) {
			t.Fatalf("compactación %d: el agente no recibió el aviso; additionalContext = %q", i, s.HookSpecificOutput.AdditionalContext)
		}
	}
}

// Dos compactaciones son el uso normal de una sesión larga: ni aviso, ni traspaso en el arranque de
// otra sesión, ni nada que traer.
func TestTraspasoCallaEnUnaSesionQueCompactoDosVeces(t *testing.T) {
	dir := proyectoDelTraspaso(t)
	tr := transcriptDelTraspaso(t, "sesion-a", "RESUMEN-DE-A")
	for i := 1; i <= 2; i++ {
		if s := compactarDelTraspaso(t, dir, "sesion-a", tr); s.SystemMessage != "" || strings.Contains(s.HookSpecificOutput.AdditionalContext, marcaDelTraspaso) {
			t.Fatalf("compactación %d: avisó: %+v", i, s)
		}
	}
	if ctx := hookDelTraspaso(t, dir, entradaDeArranque{SessionID: "sesion-b", Source: fuenteStartup}).HookSpecificOutput.AdditionalContext; strings.Contains(ctx, marcaDelTraspaso) {
		t.Fatalf("el arranque ofreció el traspaso de una sesión que compactó dos veces:\n%s", ctx)
	}
	out, errOut, code := correrTraspasoEn(t, dir)
	if code != 0 || !strings.Contains(out, "no hay ningún traspaso pendiente") || strings.Contains(out, "RESUMEN-DE-A") {
		t.Fatalf("musubi traspaso tras dos compactaciones: code=%d out=%q err=%q", code, out, errOut)
	}
}

// La sesión que llegó al aviso deja un traspaso: el arranque y el /clear de OTRA sesión lo ofrecen en
// una línea —sin el resumen—, y el de la propia sesión no.
//
// Sabotaje: no excluir a la sesión que pregunta le ofrece su propio resumen.
// arnes: archivo="cmd/musubi/traspaso.go"
// arnes: de="\t\tif id == excluir || !s.pendiente(ahora) {\n"
// arnes: a="\t\tif !s.pendiente(ahora) {\n"
func TestTraspasoDeOtraSesionApareceEnStartupYClearYElPropioNo(t *testing.T) {
	dir := proyectoDelTraspaso(t)
	const vieja = "0123abcd-1111-2222-3333-444455556666"
	tr := transcriptDelTraspaso(t, vieja, "RESUMEN-DE-LA-SESION-VIEJA")
	for i := 0; i < 3; i++ {
		compactarDelTraspaso(t, dir, vieja, tr)
	}
	for _, in := range []entradaDeArranque{
		{SessionID: "sesion-nueva", Source: fuenteStartup},
		{SessionID: "otra-sesion-nueva", Source: fuenteClear},
	} {
		ctx := hookDelTraspaso(t, dir, in).HookSpecificOutput.AdditionalContext
		if !strings.Contains(ctx, marcaDelTraspaso+" La sesión 0123abcd ") || !strings.Contains(ctx, "compactó 3 veces") || !strings.Contains(ctx, "`musubi traspaso`") {
			t.Fatalf("%s: el arranque no ofreció el traspaso de la sesión vieja:\n%s", in.Source, ctx)
		}
		if strings.Contains(ctx, "RESUMEN-DE-LA-SESION-VIEJA") {
			t.Fatalf("%s: el arranque inyectó el resumen; tiene que ofrecerlo, no traerlo", in.Source)
		}
	}
	if ctx := hookDelTraspaso(t, dir, entradaDeArranque{SessionID: vieja, Source: fuenteClear}).HookSpecificOutput.AdditionalContext; strings.Contains(ctx, marcaDelTraspaso) {
		t.Fatalf("a la sesión que dejó el traspaso se le ofreció el suyo:\n%s", ctx)
	}
}

// Un traspaso recuperado no vuelve: ni en el próximo arranque, ni en otro `musubi traspaso`, ni
// cuando la sesión vieja compacta otra vez sin llegar al próximo múltiplo.
//
// Sabotaje: no marcar el traspaso como recuperado lo vuelve a ofrecer en cada arranque.
// arnes: archivo="cmd/musubi/traspaso.go"
// arnes: de="\t\tactual.Recuperado = s.N\n"
// arnes: a="\t\t_ = s.N\n"
func TestTraspasoConsumidoNoVuelveAAparecer(t *testing.T) {
	dir := proyectoDelTraspaso(t)
	const vieja = "feedbeef-0000-1111-2222-333344445555"
	tr := transcriptDelTraspaso(t, vieja, "RESUMEN-PARA-RECUPERAR")
	for i := 0; i < 3; i++ {
		compactarDelTraspaso(t, dir, vieja, tr)
	}
	out, errOut, code := correrTraspasoEn(t, dir)
	if code != 0 || !strings.Contains(out, "RESUMEN-PARA-RECUPERAR") || !strings.Contains(out, marcaDelTraspaso+" Último resumen de la sesión feedbeef ") {
		t.Fatalf("musubi traspaso no trajo el resumen: code=%d out=%q err=%q", code, out, errOut)
	}
	compactarDelTraspaso(t, dir, vieja, tr) // la cuarta: no llega al próximo múltiplo
	if ctx := hookDelTraspaso(t, dir, entradaDeArranque{SessionID: "sesion-nueva", Source: fuenteStartup}).HookSpecificOutput.AdditionalContext; strings.Contains(ctx, marcaDelTraspaso) {
		t.Fatalf("el arranque volvió a ofrecer un traspaso ya recuperado:\n%s", ctx)
	}
	out, errOut, code = correrTraspasoEn(t, dir)
	if code != 0 || strings.Contains(out, "RESUMEN-PARA-RECUPERAR") || !strings.Contains(out, "no hay ningún traspaso pendiente") {
		t.Fatalf("un segundo musubi traspaso volvió a traer el resumen: code=%d out=%q err=%q", code, out, errOut)
	}
}

// Un traspaso de hace más de 12 h no se ofrece: el trabajo se retomó por otro lado o se abandonó. Uno
// de hace 11 h, sí.
//
// Sabotaje: sin la ventana, un traspaso de ayer aparece en el arranque de hoy.
// arnes: archivo="cmd/musubi/traspaso.go"
// arnes: de="\tif ahora.Sub(time.Unix(s.Ultima, 0)) > ventanaDelTraspaso {\n"
// arnes: a="\tif false {\n"
func TestTraspasoDeMasDeDoceHorasNoAparece(t *testing.T) {
	dir := proyectoDelTraspaso(t)
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	ahora := time.Now()
	nueva := entradaDeArranque{SessionID: "sesion-de-hoy", Source: fuenteStartup}
	ayer := entradaDeArranque{SessionID: "aaaa1111-ayer", Source: fuenteCompact, TranscriptPath: transcriptDelTraspaso(t, "aaaa1111-ayer", "RESUMEN-DE-AYER")}
	for i := 0; i < 3; i++ {
		contarCompactacion(eng, ayer, ahora.Add(-13*time.Hour))
	}
	if linea := lineaDeTraspasoPendienteA(eng, nueva, ahora); linea != "" {
		t.Fatalf("se ofreció un traspaso de hace 13 h: %q", linea)
	}
	var out, errOut bytes.Buffer
	if code := traspasar(eng, &out, &errOut, ahora); code != 0 || strings.Contains(out.String(), "RESUMEN-DE-AYER") {
		t.Fatalf("musubi traspaso trajo un resumen de hace 13 h: code=%d out=%q", code, out.String())
	}
	// El control: otra sesión que compactó hace 11 h sí se ofrece.
	manana := entradaDeArranque{SessionID: "bbbb2222-manana", Source: fuenteCompact, TranscriptPath: transcriptDelTraspaso(t, "bbbb2222-manana", "RESUMEN-DE-LA-MANANA")}
	for i := 0; i < 3; i++ {
		contarCompactacion(eng, manana, ahora.Add(-11*time.Hour))
	}
	if linea := lineaDeTraspasoPendienteA(eng, nueva, ahora); !strings.Contains(linea, "La sesión bbbb2222 ") || !strings.Contains(linea, "hace 11 h") {
		t.Fatalf("un traspaso de hace 11 h no se ofreció: %q", linea)
	}
}

// El resumen sale del transcript y pasa por el redactor antes de llegar a stdout: un secreto citado en
// el resumen no sale. Y sale el ÚLTIMO resumen, no el primero.
//
// Sabotaje: imprimir el resumen sin redactar deja el token a la vista.
// arnes: archivo="cmd/musubi/traspaso.go"
// arnes: de="\ttexto, _ := redact.Redact(resumen)\n"
// arnes: a="\ttexto, _ := resumen, []redact.Finding(nil)\n"
func TestTraspasoSaleRedactado(t *testing.T) {
	dir := proyectoDelTraspaso(t)
	secreto := "ghp_" + strings.Repeat("a1B2c3D4e5", 4)
	const sesion = "cafe0000-aaaa-bbbb-cccc-ddddeeeeffff"
	tr := transcriptDelTraspaso(t, sesion,
		"PRIMER-RESUMEN, ya superado.",
		"SEGUNDO-RESUMEN: el dueño pegó el token "+secreto+" para probar el push.")
	for i := 0; i < 3; i++ {
		compactarDelTraspaso(t, dir, sesion, tr)
	}
	out, errOut, code := correrTraspasoEn(t, dir)
	if code != 0 {
		t.Fatalf("musubi traspaso salió con %d: %s", code, errOut)
	}
	if strings.Contains(out+errOut, secreto) {
		t.Fatal("el secreto del resumen llegó sin redactar a la salida de musubi traspaso")
	}
	if !strings.Contains(out, "REDACTED") || !strings.Contains(out, "SEGUNDO-RESUMEN") {
		t.Fatalf("musubi traspaso no trajo el último resumen, redactado:\n%s", out)
	}
	if strings.Contains(out, "PRIMER-RESUMEN") {
		t.Fatalf("musubi traspaso trajo un resumen viejo, no el último:\n%s", out)
	}
}

// Un subagente compacta su propio contexto y no tiene una ventana que renovar: no cuenta. Y una
// entrada sin transcript_path cuenta y avisa igual, aunque su traspaso no se ofrezca si el transcript
// no está donde Claude Code lo deja.
func TestTraspasoNoCuentaSubagentesYSinTranscriptAvisaIgual(t *testing.T) {
	dir := proyectoDelTraspaso(t)
	sub := filepath.Join(t.TempDir(), "sesion-padre", "subagents", "agent-1.jsonl")
	for i := 1; i <= 3; i++ {
		if s := compactarDelTraspaso(t, dir, "sesion-padre", sub); s.SystemMessage != "" {
			t.Fatalf("un subagente contó su compactación %d y avisó: %q", i, s.SystemMessage)
		}
	}
	var ultima salidaDelHookDelTraspaso
	for i := 1; i <= 3; i++ {
		ultima = compactarDelTraspaso(t, dir, "sesion-sin-ruta", "")
	}
	if !strings.Contains(ultima.SystemMessage, "compactó 3 veces") {
		t.Fatalf("sin transcript_path la sesión no avisó a las 3: %+v", ultima)
	}
	if ctx := hookDelTraspaso(t, dir, entradaDeArranque{SessionID: "sesion-nueva", Source: fuenteStartup}).HookSpecificOutput.AdditionalContext; strings.Contains(ctx, marcaDelTraspaso) {
		t.Fatalf("se ofreció un traspaso sin transcript que leer:\n%s", ctx)
	}
}

// existeElArchivo dice si la ruta existe (archivo o carpeta).
func existeElArchivo(ruta string) bool {
	_, err := os.Stat(ruta)
	return err == nil
}
