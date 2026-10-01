package recalleval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// MEDIR LO QUE LE CUESTAN AL JUEZ LOS IDS QUE VE (spec juez-ids-cortos).
//
// Dos mediciones, las dos detrás de una env var y ninguna en CI:
//
//   - TestVolcarPromptsDelJuez (MUSUBI_JUEZ_VOLCADO=<dir>) es SIN MODELO. Corre el banco con un
//     juez que sólo graba y vuelca, por llamada, el par (system, user) que el banco le manda al juez,
//     que es el mismo que manda producción (J1). Los tokens se cuentan afuera, con el tokenizador
//     que se tenga, porque el repo no trae ninguno.
//   - TestMedicionJuezOllamaLocal (MUSUBI_JUEZ_OLLAMA_MODELO=<modelo>) corre el banco con un juez
//     LOCAL de Ollama y reporta exactitud, los tokens que cuenta Ollama y la latencia de cada
//     llamada. Sólo el fixture dorado y sólo contra un Ollama en loopback: ningún texto de la
//     memoria real sale de la máquina, y una URL que no sea loopback se rechaza.
//
// LOS IDS DEL FIXTURE DORADO SON SLUGS CON SIGNIFICADO («deploy-guide-es») Y LOS DE PRODUCCIÓN SON
// UUIDs OPACOS. Eso pesa dos veces: un slug no cuesta lo que un UUID (un UUID son ~23 tokens), y un
// juez que ve «backup-restore-en» lee en el id una pista del tema que en producción no tiene. Medir
// ids largos contra cortos con los slugs le regalaría al brazo largo esa pista. Por eso las dos
// mediciones corren por default sobre fixtureConIDsOpacos, y el slug queda como variante a pedido.

// fixtureConIDsOpacos devuelve una copia del fixture con cada id cambiado por un UUID derivado del
// original, y con las relevancias y las aristas re-apuntadas. Determinista: la misma entrada da
// siempre los mismos ids, así que dos corridas del banco ven el mismo corpus.
func fixtureConIDsOpacos(fx *Fixture) *Fixture {
	opaco := func(id string) string {
		h := sha256.Sum256([]byte("recalleval:id-opaco:" + id))
		x := hex.EncodeToString(h[:16])
		// La forma de un UUID v4 (8-4-4-4-12, versión 4 y variante 10xx), que es lo que produce
		// producción al guardar una observación sin id.
		return x[0:8] + "-" + x[8:12] + "-4" + x[13:16] + "-" + string("89ab"[h[8]&3]) + x[17:20] + "-" + x[20:32]
	}
	out := &Fixture{
		Docs:       make([]Doc, len(fx.Docs)),
		Queries:    make([]Query, len(fx.Queries)),
		Relaciones: make([]Relacion, len(fx.Relaciones)),
	}
	for i, d := range fx.Docs {
		d.ID = opaco(d.ID)
		out.Docs[i] = d
	}
	for i, q := range fx.Queries {
		rel := make([]string, len(q.Relevant))
		for j, id := range q.Relevant {
			rel[j] = opaco(id)
		}
		q.Relevant = rel
		out.Queries[i] = q
	}
	for i, r := range fx.Relaciones {
		out.Relaciones[i] = Relacion{Source: opaco(r.Source), Target: opaco(r.Target)}
	}
	return out
}

// fixtureDeLaVariante elige el fixture de una medición: "uuid" (el default, ids opacos como en
// producción) o "slug" (el dorado tal cual). Cualquier otra cosa corta la prueba: una variante mal
// escrita que cayera al default mediría otra cosa con el nombre pedido.
func fixtureDeLaVariante(t *testing.T, variante string) *Fixture {
	t.Helper()
	golden := loadGolden(t)
	switch variante {
	case "", "uuid":
		return fixtureConIDsOpacos(golden)
	case "slug":
		return golden
	default:
		t.Fatalf("variante %q desconocida: es «uuid» o «slug»", variante)
		return nil
	}
}

// idsDelPrompt saca los ids que el juez ve, en el orden en que los ve: el prompt pone cada
// candidato en su línea como «[id] gist».
func idsDelPrompt(user string) []string {
	var ids []string
	for _, linea := range strings.Split(user, "\n") {
		if !strings.HasPrefix(linea, "[") {
			continue
		}
		if cierre := strings.IndexByte(linea, ']'); cierre > 1 {
			ids = append(ids, linea[1:cierre])
		}
	}
	return ids
}

// juezGrabador guarda cada prompt que recibe y contesta los ids en el orden en que los vio. El
// orden no importa: importa que la respuesta se pueda parsear, para que el banco siga de largo.
type juezGrabador struct {
	mu      sync.Mutex
	prompts [][2]string
}

func (j *juezGrabador) Name() string { return "juez-grabador" }

func (j *juezGrabador) Ask(_ context.Context, system, user string) (string, error) {
	j.mu.Lock()
	j.prompts = append(j.prompts, [2]string{system, user})
	j.mu.Unlock()
	b, _ := json.Marshal(idsDelPrompt(user))
	return string(b), nil
}

// TestVolcarPromptsDelJuez vuelca, por variante de fixture y por brazo base, lo que el juez
// recibiría en cada consulta. Con MUSUBI_POTION_DIR suma el brazo híbrido, que es la base que
// corre producción; sin ella, sólo el léxico.
func TestVolcarPromptsDelJuez(t *testing.T) {
	dir := os.Getenv("MUSUBI_JUEZ_VOLCADO")
	if dir == "" {
		t.Skip("MUSUBI_JUEZ_VOLCADO no seteado: se saltea el volcado de prompts del juez")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var embed EmbedFunc
	bases := []Config{lexicalConfig}
	if potion := os.Getenv("MUSUBI_POTION_DIR"); potion != "" {
		prov, err := embedding.NewStaticProvider(potion)
		if err != nil {
			t.Fatalf("NewStaticProvider: %v", err)
		}
		embed = func(text string) ([]float32, error) { return prov.Embed(context.Background(), text) }
		bases = append(bases, hybridConfig)
	}
	for _, variante := range []string{"slug", "uuid"} {
		fx := fixtureDeLaVariante(t, variante)
		for _, base := range bases {
			j := &juezGrabador{}
			cfg := base
			cfg.Name = base.Name + "+grabador"
			cfg.Juez = j
			if _, err := Run(context.Background(), t.TempDir(), fx, embed, []Config{cfg}, []int{10}); err != nil {
				t.Fatalf("Run(%s, %s): %v", variante, base.Name, err)
			}
			ruta := filepath.Join(dir, fmt.Sprintf("prompts-%s-%s.jsonl", variante, base.Name))
			var b bytes.Buffer
			enc := json.NewEncoder(&b)
			total := 0
			for _, p := range j.prompts {
				total += len(p[0]) + len(p[1])
				if err := enc.Encode(map[string]string{"system": p[0], "user": p[1]}); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(ruta, b.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s · %s: %d llamadas al juez, %d bytes de prompt en total → %s",
				variante, base.Name, len(j.prompts), total, ruta)
		}
	}
}

// llamadaOllama es lo que Ollama dice de una llamada: los tokens que evaluó y generó, y en qué
// se fue el tiempo. Los *_duration vienen en nanosegundos.
type llamadaOllama struct {
	Pared              time.Duration `json:"pared_ns"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	EvalCount          int           `json:"eval_count"`
	TotalDuration      int64         `json:"total_duration"`
	LoadDuration       int64         `json:"load_duration"`
	PromptEvalDuration int64         `json:"prompt_eval_duration"`
	EvalDuration       int64         `json:"eval_duration"`
	Candidatos         int           `json:"candidatos"`
	// IDsVistos son los ids del prompt, en su orden: con ellos se cuenta después cuántos ids de
	// la respuesta el modelo copió mal, repitió u omitió.
	IDsVistos []string `json:"ids_vistos"`
	Respuesta string   `json:"respuesta"`
	// Memoria de la máquina al salir la llamada, en MB. El swap le pega a la latencia, y hay que
	// poder decir cuánto había en cada llamada y no sólo al arrancar.
	MemDisponibleMB int `json:"mem_disponible_mb"`
	SwapUsadoMB     int `json:"swap_usado_mb"`
}

// memoriaDeLaMaquina lee MemAvailable y el swap usado de /proc/meminfo, en MB. Fuera de Linux, o
// si no se puede leer, devuelve -1 y -1: «no medí» no puede verse como «cero».
func memoriaDeLaMaquina() (disponible, swapUsado int) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return -1, -1
	}
	kb := map[string]int{}
	for _, linea := range strings.Split(string(b), "\n") {
		campos := strings.Fields(linea)
		if len(campos) >= 2 {
			if n, err := strconv.Atoi(campos[1]); err == nil {
				kb[strings.TrimSuffix(campos[0], ":")] = n
			}
		}
	}
	d, okD := kb["MemAvailable"]
	st, okT := kb["SwapTotal"]
	sf, okF := kb["SwapFree"]
	if !okD || !okT || !okF {
		return -1, -1
	}
	return d / 1024, (st - sf) / 1024
}

// juezOllama le habla a la API nativa de Ollama (/api/chat) y no a la compatible con OpenAI,
// porque la nativa devuelve los contadores de tokens y el desglose del tiempo, que es lo que esta
// medición necesita. El prompt es el que arma cognition.Rerank: el transporte no lo toca.
type juezOllama struct {
	url, modelo string
	numCtx      int
	cli         *http.Client
	mu          sync.Mutex
	llamadas    []llamadaOllama
}

func (j *juezOllama) Name() string { return "llm:" + j.modelo }

func (j *juezOllama) Ask(ctx context.Context, system, user string) (string, error) {
	cuerpo := map[string]any{
		"model": j.modelo,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream":     false,
		"keep_alive": "10m",
		// Temperatura 0 y semilla fija, para que los dos brazos difieran en los ids y no en el azar.
		// ACHICA el azar, no lo elimina: en CPU, el mismo prompt dos veces invirtió dos vecinos del
		// orden (medicion.md, «Condiciones»). Una posición de diferencia entre brazos puede ser eso.
		"options": map[string]any{"temperature": 0, "seed": 42, "num_ctx": j.numCtx},
	}
	if strings.HasPrefix(j.modelo, "qwen3") {
		// qwen3 razona por defecto, y en esta máquina eso son minutos por consulta.
		cuerpo["think"] = false
	}
	b, err := json.Marshal(cuerpo)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.url+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	memDisp, swapUsado := memoriaDeLaMaquina()
	t0 := time.Now()
	resp, err := j.cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
		llamadaOllama
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("ollama: respuesta ilegible: %w", err)
	}
	if resp.StatusCode != http.StatusOK || r.Error != "" {
		return "", fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, r.Error)
	}
	l := r.llamadaOllama
	l.Pared = time.Since(t0)
	l.IDsVistos = idsDelPrompt(user)
	l.Candidatos = len(l.IDsVistos)
	l.MemDisponibleMB, l.SwapUsadoMB = memDisp, swapUsado
	l.Respuesta = r.Message.Content
	j.mu.Lock()
	j.llamadas = append(j.llamadas, l)
	j.mu.Unlock()
	return r.Message.Content, nil
}

// esOllamaLocal dice si raw apunta a un Ollama de ESTA máquina. Decide por el host al que se va a
// conectar el cliente, que es lo que importa, y no por cómo empieza el texto: en
// «http://127.0.0.1:11434@otro.host/» lo de antes de la arroba es usuario y clave, y la conexión
// va a otro.host.
func esOllamaLocal(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}

// La guarda de loopback de TestMedicionJuezOllamaLocal es lo único que impide mandarle el fixture a
// otra máquina, así que se prueba con las URLs que EMPIEZAN como loopback y no lo son.
//
// Sabotaje: la guarda vuelve a decidir por el texto que sigue a «http://» y no por el host.
// arnes: archivo="internal/recalleval/juez_ids_medicion_test.go"
// arnes: de="\tswitch u.Hostname() {"
// arnes: a="\tswitch strings.Split(strings.TrimPrefix(raw, \"http://\"), \":\")[0] {"
func TestEsOllamaLocal(t *testing.T) {
	casos := []struct {
		url   string
		local bool
	}{
		// Primero el caso que la guarda vieja dejaba pasar, así el motivo del rojo lo nombra a él.
		{"http://127.0.0.1:11434@example.com/", false},
		{"http://127.0.0.1:11434", true},
		{"http://localhost:11434", true},
		{"http://[::1]:11434", true},
		{"http://127.0.0.1.example.com:11434", false},
		{"http://localhost.example.com:11434", false},
		{"https://127.0.0.1:11434", false},
		{"http://100.79.126.62:11434", false},
	}
	for _, c := range casos {
		if got := esOllamaLocal(c.url); got != c.local {
			t.Errorf("esOllamaLocal(%q) = %v, quería %v", c.url, got, c.local)
		}
	}
}

// TestMedicionJuezOllamaLocal corre la base léxica y la base con el juez local sobre el MISMO
// corpus, consulta por consulta.
//
// POR QUÉ CONSULTA POR CONSULTA Y NO CON Run: el banco ABORTA la corrida entera ante un juez que
// contesta algo imparseable, y con un modelo de 3B eso pasa. Con Run, una respuesta mala en la
// consulta 11 tiraría media hora de CPU. Acá cada consulta se evalúa sola; si el juez falla, esa
// consulta cuenta con el orden model-free —lo mismo que hace producción, que degrada— y la falla
// se CUENTA y se informa, que es la parte que Run no permite ver.
//
// POR QUÉ LÉXICA Y NO HÍBRIDA: la híbrida necesita un embebedor residente (POTION o bge-m3) al
// lado del modelo generativo, y esta máquina no tiene RAM para los dos. La base sólo decide qué
// candidatos ve el juez; es la misma en los dos brazos de la comparación (ids largos y cortos),
// así que la diferencia entre ellos no depende de ella. Los ABSOLUTOS no se comparan con los del
// central, que corre híbrida.
//
// MUSUBI_JUEZ_CONSULTA=<id>[,<id>…] limita la corrida a esas consultas. Es lo que permite
// INTERCALAR los brazos: la comparación de ids largos contra cortos corre dos binarios de prueba
// —uno compilado antes del cambio y otro después— consulta por consulta y alternando cuál va
// primero, para que el swap y la temperatura de la máquina les peguen igual a los dos. Correr
// todos los largos y después todos los cortos le cargaría al segundo brazo lo que haya cambiado la
// máquina en el medio.
func TestMedicionJuezOllamaLocal(t *testing.T) {
	modelo := os.Getenv("MUSUBI_JUEZ_OLLAMA_MODELO")
	if modelo == "" {
		t.Skip("MUSUBI_JUEZ_OLLAMA_MODELO no seteado: se saltea la medición con un juez local")
	}
	destino := os.Getenv("MUSUBI_JUEZ_OLLAMA_URL")
	if destino == "" {
		destino = "http://127.0.0.1:11434"
	}
	if !esOllamaLocal(destino) {
		t.Fatalf("MUSUBI_JUEZ_OLLAMA_URL=%q no es loopback: esta medición sólo corre contra un Ollama LOCAL", destino)
	}
	numCtx := 2048
	if v := os.Getenv("MUSUBI_JUEZ_OLLAMA_NUM_CTX"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			t.Fatalf("MUSUBI_JUEZ_OLLAMA_NUM_CTX=%q no es un entero positivo", v)
		}
		numCtx = n
	}
	variante := os.Getenv("MUSUBI_JUEZ_VARIANTE")
	fx := fixtureDeLaVariante(t, variante)
	if variante == "" {
		variante = "uuid"
	}
	if lista := os.Getenv("MUSUBI_JUEZ_CONSULTA"); lista != "" {
		pedidas := map[string]bool{}
		for _, id := range strings.Split(lista, ",") {
			pedidas[strings.TrimSpace(id)] = true
		}
		var elegidas []Query
		for _, q := range fx.Queries {
			if pedidas[q.ID] {
				elegidas = append(elegidas, q)
				delete(pedidas, q.ID)
			}
		}
		// Una consulta mal escrita no puede medir «nada» en verde.
		if len(pedidas) > 0 {
			t.Fatalf("MUSUBI_JUEZ_CONSULTA nombra consultas que el fixture no tiene: %v", pedidas)
		}
		fx = &Fixture{Docs: fx.Docs, Queries: elegidas, Relaciones: fx.Relaciones}
	}

	eng, err := SeedEngine(t.TempDir(), fx, nil)
	if err != nil {
		t.Fatalf("SeedEngine: %v", err)
	}
	defer eng.Close()

	// no habla con el cerebro: es el Ollama local de esta máquina, y la URL se valida loopback arriba.
	juez := &juezOllama{url: destino, modelo: modelo, numCtx: numCtx, cli: &http.Client{
		Timeout: 10 * time.Minute,
		// Una redirección llevaría el prompt a donde diga el servidor, y la guarda de arriba sólo
		// miró la primera dirección.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	// El calentamiento carga el modelo y no se cuenta: la primera llamada pagaría la carga del disco
	// y ensuciaría la latencia de la primera consulta.
	if _, err := juez.Ask(context.Background(), "Contestá sólo OK.", "OK"); err != nil {
		t.Fatalf("calentamiento contra %s (%s): %v", destino, modelo, err)
	}
	juez.llamadas = nil

	base := lexicalConfig
	conJuez := lexicalConfig
	conJuez.Name = "lexical+juez:" + modelo
	conJuez.Juez = juez
	ks := []int{1, 3, 5, 10}

	type porConsulta struct {
		Consulta    string           `json:"consulta"`
		Base        MetricasConsulta `json:"base"`
		Juez        MetricasConsulta `json:"juez"`
		JuezFallo   string           `json:"juez_fallo,omitempty"`
		LlamoAlJuez bool             `json:"llamo_al_juez"`
	}
	var filas []porConsulta
	fallos := 0
	arranque := time.Now()
	for _, q := range fx.Queries {
		if len(q.Relevant) == 0 {
			continue
		}
		una := &Fixture{Docs: fx.Docs, Queries: []Query{q}}
		sb, err := Evaluate(context.Background(), eng, una, base, nil, ks)
		if err != nil {
			t.Fatalf("base, consulta %s: %v", q.ID, err)
		}
		antes := len(juez.llamadas)
		fila := porConsulta{Consulta: q.ID, Base: sb.PorConsulta[q.ID]}
		sj, err := Evaluate(context.Background(), eng, una, conJuez, nil, ks)
		fila.LlamoAlJuez = len(juez.llamadas) > antes
		if err != nil {
			fallos++
			fila.JuezFallo = err.Error()
			fila.Juez = fila.Base // degradación de producción: queda el orden model-free
		} else {
			fila.Juez = sj.PorConsulta[q.ID]
		}
		filas = append(filas, fila)
		t.Logf("%-16s RR base %.3f → juez %.3f%s", q.ID, fila.Base.RR, fila.Juez.RR, map[bool]string{true: "  (FALLÓ: " + fila.JuezFallo + ")", false: ""}[fila.JuezFallo != ""])
	}

	promedio := func(f func(porConsulta) float64) float64 {
		if len(filas) == 0 {
			return 0
		}
		s := 0.0
		for _, x := range filas {
			s += f(x)
		}
		return s / float64(len(filas))
	}
	var rep strings.Builder
	fmt.Fprintf(&rep, "juez %s · variante %s · %d consultas · %d fallos del juez · %s en total\n",
		modelo, variante, len(filas), fallos, time.Since(arranque).Round(time.Second))
	fmt.Fprintf(&rep, "%-10s %6s", "brazo", "MRR")
	for _, k := range ks {
		fmt.Fprintf(&rep, "  R@%-2d nDCG@%-2d", k, k)
	}
	rep.WriteString("\n")
	for _, brazo := range []struct {
		nombre string
		m      func(porConsulta) MetricasConsulta
	}{{"base", func(x porConsulta) MetricasConsulta { return x.Base }}, {"juez", func(x porConsulta) MetricasConsulta { return x.Juez }}} {
		fmt.Fprintf(&rep, "%-10s %6.3f", brazo.nombre, promedio(func(x porConsulta) float64 { return brazo.m(x).RR }))
		for _, k := range ks {
			fmt.Fprintf(&rep, "  %5.3f %7.3f",
				promedio(func(x porConsulta) float64 { return brazo.m(x).RecallAtK[k] }),
				promedio(func(x porConsulta) float64 { return brazo.m(x).NDCGAtK[k] }))
		}
		rep.WriteString("\n")
	}

	// Tokens y tiempo, por llamada. Mediana y p90 además del promedio: con 12 llamadas un solo
	// outlier mueve el promedio, y el que decide es el caso típico.
	stats := func(xs []float64) string {
		if len(xs) == 0 {
			return "n=0"
		}
		ys := append([]float64(nil), xs...)
		sort.Float64s(ys)
		s := 0.0
		for _, y := range ys {
			s += y
		}
		p90 := ys[(len(ys)*9+9)/10-1]
		return fmt.Sprintf("prom %.1f · med %.1f · p90 %.1f · total %.0f", s/float64(len(ys)), ys[len(ys)/2], p90, s)
	}
	var tokIn, tokOut, pared, tIn, tOut, cands []float64
	for _, l := range juez.llamadas {
		tokIn = append(tokIn, float64(l.PromptEvalCount))
		tokOut = append(tokOut, float64(l.EvalCount))
		pared = append(pared, l.Pared.Seconds())
		tIn = append(tIn, float64(l.PromptEvalDuration)/1e9)
		tOut = append(tOut, float64(l.EvalDuration)/1e9)
		cands = append(cands, float64(l.Candidatos))
	}
	fmt.Fprintf(&rep, "llamadas al juez: %d\n", len(juez.llamadas))
	fmt.Fprintf(&rep, "  candidatos       %s\n", stats(cands))
	fmt.Fprintf(&rep, "  tokens entrada   %s   (prompt_eval_count: excluye el prefijo que Ollama ya tenía en caché)\n", stats(tokIn))
	fmt.Fprintf(&rep, "  tokens salida    %s\n", stats(tokOut))
	fmt.Fprintf(&rep, "  s de pared       %s\n", stats(pared))
	fmt.Fprintf(&rep, "  s leyendo prompt %s\n", stats(tIn))
	fmt.Fprintf(&rep, "  s generando      %s\n", stats(tOut))
	t.Logf("\n%s", rep.String())

	if salida := os.Getenv("MUSUBI_JUEZ_SALIDA"); salida != "" {
		b, err := json.MarshalIndent(map[string]any{
			"modelo": modelo, "variante": variante, "num_ctx": numCtx, "fallos": fallos,
			"reporte": rep.String(), "consultas": filas, "llamadas": juez.llamadas,
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(salida, b, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("resultado completo → %s", salida)
	}
}
