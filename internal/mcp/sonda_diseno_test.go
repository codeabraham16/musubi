//go:build sonda

package mcp

// sonda_diseno_test.go — LA SONDA del motor de diseño (Musubi Renaissance · F0).
//
// Mide contra el CENTRAL REAL lo que el banco estructural honestamente no puede: estabilidad de
// paráfrasis, precisión temática, latencia y cobertura del acervo. Las tres primeras dependen del
// embebedor real (bge-m3) y la última del acervo real de 1.736 entradas; medirlas offline exigiría
// un embebedor falso, y un embebedor falso mide al embebedor falso — el modo de falla que este
// proyecto ya documentó cuatro veces («el test espera el proxy, no la cosa»).
//
// Va detrás del build tag `sonda` (I-BANCO6): sin el tag no compila, así que `go test ./...` y CI
// jamás dependen de la red ni de una credencial.
//
//	go test -tags sonda ./internal/mcp -run TestSondaDiseno -v
//
// NO FALLA por umbral, a propósito. Es un instrumento, no una compuerta: el central puede estar
// legítimamente en otro estado que el repo local, y una sonda que rompe el build por eso se apaga
// sola a la semana. Reporta; el juicio es de quien la corre.
//
// ⚠ EN kernelos-pc (Windows con NordVPN) EL HTTP DIRECTO NO SALE. NordVPN excluye por RUTA EXACTA
// del binario —medido: ni recompilar en la carpeta de `musubi.exe` alcanza, porque la regla incluye
// el NOMBRE del archivo— así que el binario efímero de `go test` recibe
// «socket ... forbidden by its access permissions» al primer dial. No es el central caído.
// Salida: pasarle un curl permitido como transporte, que es lo mismo que ya hace el resto del repo.
//
//	MUSUBI_SONDA_CURL='C:\Windows\System32\curl.exe' go test -tags sonda ./internal/mcp -run TestSondaDiseno -v
//
// Sin esa variable usa net/http directo, que es lo correcto en la laptop, en el server y en CI.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"
)

// transporte devuelve la función que entrega un cuerpo JSON-RPC y trae la respuesta cruda. Dos
// implementaciones con el mismo contrato: net/http (lo normal) y un curl externo (la salida para
// una máquina donde el firewall filtra por ruta de binario).
func transporte(t *testing.T, url, token string) func([]byte) ([]byte, error) {
	if curl := strings.TrimSpace(os.Getenv("MUSUBI_SONDA_CURL")); curl != "" {
		t.Logf("transporte: curl externo (%s)", curl)
		return func(body []byte) ([]byte, error) {
			f, err := os.CreateTemp(t.TempDir(), "sonda-*.json")
			if err != nil {
				return nil, err
			}
			if _, err := f.Write(body); err != nil {
				f.Close()
				return nil, err
			}
			f.Close()
			cmd := exec.CommandContext(t.Context(), curl, "--max-time", "90", "-s", "-X", "POST", url+"/mcp",
				"-H", "Content-Type: application/json", "-H", "Authorization: Bearer "+token, "-d", "@"+f.Name())
			out, err := cmd.Output()
			if err != nil {
				return nil, fmt.Errorf("curl: %w", err)
			}
			if len(bytes.TrimSpace(out)) == 0 {
				return nil, fmt.Errorf("curl devolvió vacío (¿el binario está permitido por el firewall?)")
			}
			return out, nil
		}
	}
	cl := &http.Client{Timeout: 90 * time.Second}
	return func(body []byte) ([]byte, error) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url+"/mcp", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := cl.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
}

func TestSondaDiseno(t *testing.T) {
	url := strings.TrimSuffix(strings.TrimSpace(os.Getenv("MUSUBI_CENTRAL_URL")), "/")
	token := strings.TrimSpace(os.Getenv("MUSUBI_TOKEN"))
	if url == "" || token == "" {
		t.Skip("la sonda necesita MUSUBI_CENTRAL_URL y MUSUBI_TOKEN")
	}
	set, err := CargarSetBanco(rutaSetBanco)
	if err != nil {
		t.Fatal(err)
	}
	entregar := transporte(t, url, token)

	pedir := func(prompt, target string) (designBrief, time.Duration, error) {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": "musubi_design",
				"arguments": map[string]any{"prompt": prompt, "target": target}},
		})
		t0 := time.Now()
		raw, err := entregar(body)
		lat := time.Since(t0)
		if err != nil {
			return designBrief{}, lat, err
		}
		var env struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return designBrief{}, lat, err
		}
		if len(env.Error) > 0 {
			return designBrief{}, lat, fmt.Errorf("rpc: %s", env.Error)
		}
		if len(env.Result.Content) == 0 {
			return designBrief{}, lat, fmt.Errorf("respuesta sin contenido")
		}
		var b designBrief
		err = json.Unmarshal([]byte(env.Result.Content[0].Text), &b)
		return b, lat, err
	}

	var (
		lat       []time.Duration
		solapes   []float64
		precision []float64
		servidos  = map[string]bool{}
		abstuvo   int
		// FALSA ABSTENCIÓN: pedidos LEGÍTIMOS que el piso dejó sin material. Es el riesgo principal de
		// F3 y por eso se mide junto al beneficio: una sonda que sólo reporta la cara que conviene es
		// un expediente, no un instrumento. Si esto no es cero, el piso está mal y baja.
		falsaAbstencion []string
		causas          = map[string]int{}
		peor            = struct {
			id string
			j  float64
		}{"", 2}
	)

	// ── M1 · estabilidad de paráfrasis · M3 · precisión temática ──────────────────────────────
	for _, p := range set.Pedidos {
		var conjuntos [][]string
		for _, forma := range p.Formas {
			b, d, err := pedir(forma, p.Target)
			if err != nil {
				t.Fatalf("sonda %s: %v", p.ID, err)
			}
			lat = append(lat, d)
			conjuntos = append(conjuntos, IdsDeCorpus(b))
			for _, id := range IdsDeCorpus(b) {
				servidos[id] = true
			}
			precision = append(precision, TocaLosEjes(b, p.Ejes))
			if Abstuvo(b) {
				falsaAbstencion = append(falsaAbstencion, p.ID+" ("+b.DegradedReason+")")
			}
			causas[b.Retrieval]++
		}
		suma, n := 0.0, 0
		for i := range conjuntos {
			for j := i + 1; j < len(conjuntos); j++ {
				suma += Jaccard(conjuntos[i], conjuntos[j])
				n++
			}
		}
		jp := suma / float64(n)
		solapes = append(solapes, jp)
		if jp < peor.j {
			peor = struct {
				id string
				j  float64
			}{p.ID, jp}
		}
		t.Logf("  %-26s paráfrasis %.2f   precisión %.2f", p.ID, jp, prom(precision[len(precision)-3:]))
	}

	// ── M2 · abstención, contra el embebedor real (donde nunca devuelve vacío) ────────────────
	for _, q := range set.FueraDeDominio {
		b, d, err := pedir(q, "web")
		if err != nil {
			t.Fatalf("sonda fuera-de-dominio: %v", err)
		}
		lat = append(lat, d)
		if Abstuvo(b) {
			abstuvo++
			causas[b.DegradedReason]++
		}
		causas[b.Retrieval]++
	}

	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	t.Logf("\n%s", strings.Join([]string{
		"SONDA DEL MOTOR DE DISEÑO — contra " + url,
		"─────────────────────────────────────────────────────────────────────────────",
		fmt.Sprintf("  M1 estabilidad de paráfrasis      %.2f   (objetivo ≥ 0,80)", prom(solapes)),
		fmt.Sprintf("     el peor pedido                 %.2f   (%s)", peor.j, peor.id),
		fmt.Sprintf("  M3 precisión temática @6          %.2f   (objetivo ≥ 0,80)", prom(precision)),
		fmt.Sprintf("  M2 abstención fuera de dominio    %.2f   (objetivo 1,00)", float64(abstuvo)/float64(len(set.FueraDeDominio))),
		fmt.Sprintf("  M7 latencia p50 / p95             %d ms / %d ms", lat[len(lat)/2].Milliseconds(), lat[int(float64(len(lat))*0.95)-1].Milliseconds()),
		fmt.Sprintf("  M8 ids distintos servidos         %d   (sobre el set entero)", len(servidos)),
	}, "\n"))
}

func prom(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}
