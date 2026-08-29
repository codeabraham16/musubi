package mcp

// banco_diseno.go — EL MARCADOR DEL MOTOR DE DISEÑO (Musubi Renaissance · F0).
//
// El 2026-08-21 el motor se degradó de golpe —el bloque de método pasó de 8 principios constantes a
// 30 tarjetas del acervo, 24× más texto— y nadie lo notó durante ocho días, hasta que el usuario lo
// sintió usándolo. Las suites de methods_design_test.go siguieron verdes todo el tiempo: miden que el
// brief SE ARME, no que sirva. Esto es lo que faltaba — una medida de calidad que se pueda ver
// empeorar en un PR.
//
// Va como código normal y no como helper de _test por una razón concreta: la sonda contra el central
// vive detrás del build tag `sonda` y no puede importar símbolos de archivos _test.go. De paso, deja
// el cálculo de las métricas en un solo lugar auditable, compartido por el banco y por la sonda.
//
// Model-free y sin red: todo acá es aritmética determinista sobre un brief ya armado.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────────
// EL SET DORADO
// ─────────────────────────────────────────────────────────────────────────────────────────────────

// PedidoBanco es un pedido de diseño real de un proyecto vivo, con varias formas de pedir LO MISMO.
// Las formas son el instrumento de la métrica de reproducibilidad: si el motor devuelve material
// distinto para dos maneras de decir lo mismo, no hay «la respuesta del motor», hay una lotería.
type PedidoBanco struct {
	ID       string   `json:"id"`
	Proyecto string   `json:"proyecto"`
	Target   string   `json:"target"`
	Ejes     []string `json:"ejes"`   // temas que un corpus útil debería tocar
	Formas   []string `json:"formas"` // [0] es la canónica
}

// SetBanco es el set dorado completo, con sus tres poblaciones.
type SetBanco struct {
	Armado         string        `json:"armado"`
	Nota           string        `json:"nota"`
	Pedidos        []PedidoBanco `json:"pedidos"`
	FueraDeDominio []string      `json:"fuera_de_dominio"`
	Inyecciones    []string      `json:"inyecciones"`
}

// CargarSetBanco lee el set dorado y lo valida. Un set mal formado es un banco que miente, así que
// se rechaza acá y no más adelante con una métrica rara.
func CargarSetBanco(path string) (*SetBanco, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("banco: leer set: %w", err)
	}
	var s SetBanco
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("banco: parsear set: %w", err)
	}
	if err := s.Validar(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Validar impone la forma mínima del set (I-BANCO4). Los pisos no son decorativos: con menos
// pedidos, o con un pedido de una sola forma, la métrica de paráfrasis deja de significar algo; y
// sin población de inyecciones el banco reportaría «0 payloads filtrados», que es el valor
// tranquilizador y el valor de fallo a la vez.
func (s *SetBanco) Validar() error {
	if len(s.Pedidos) < 15 {
		return fmt.Errorf("banco: el set necesita al menos 15 pedidos, tiene %d", len(s.Pedidos))
	}
	if len(s.FueraDeDominio) < 8 {
		return fmt.Errorf("banco: el set necesita al menos 8 consultas fuera de dominio, tiene %d", len(s.FueraDeDominio))
	}
	if len(s.Inyecciones) < 8 {
		return fmt.Errorf("banco: el set necesita al menos 8 payloads de inyección, tiene %d", len(s.Inyecciones))
	}
	vistos := map[string]bool{}
	for _, p := range s.Pedidos {
		if p.ID == "" || vistos[p.ID] {
			return fmt.Errorf("banco: id de pedido vacío o repetido: %q", p.ID)
		}
		vistos[p.ID] = true
		if len(p.Formas) < 3 {
			return fmt.Errorf("banco: el pedido %q necesita 3 formas, tiene %d", p.ID, len(p.Formas))
		}
		if len(p.Ejes) == 0 {
			return fmt.Errorf("banco: el pedido %q no declara ejes", p.ID)
		}
		for _, f := range p.Formas {
			if strings.TrimSpace(f) == "" {
				return fmt.Errorf("banco: el pedido %q tiene una forma vacía", p.ID)
			}
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────────────────────────
// LAS MÉTRICAS
// ─────────────────────────────────────────────────────────────────────────────────────────────────

// TokensDeBrief estima el tamaño del brief con la MISMA cuenta que usó la auditoría del 2026-08-29
// (len del JSON / 4), para que los números del banco sean comparables con los del informe. Es una
// estimación, no un tokenizador: lo que importa es la tendencia y el tope, no el decimal.
func TokensDeBrief(b designBrief) int {
	raw, err := json.Marshal(b)
	if err != nil {
		return 0
	}
	return len(raw) / 4
}

// bloquesDe descompone el brief en los bloques que el agente lee como unidades separadas. La
// comparación va bloque a bloque y no por diff de bytes sobre el JSON entero: un diff textual sobre
// JSON produce ruido de comas y llaves que no dice nada sobre el contenido.
func bloquesDe(b designBrief) map[string]string {
	corpus, _ := json.Marshal(b.Corpus)
	return map[string]string{
		"ask":          b.Ask,
		"role":         b.Role,
		"principles":   b.Principles,
		"brand":        b.Brand,
		"emit":         b.Emit,
		"instructions": b.Instructions,
		"corpus":       string(corpus),
	}
}

// FraccionVariable mide qué proporción del brief REALMENTE depende del pedido: se piden dos briefs
// de pedidos con ejes disjuntos y se cuenta cuántos bytes salieron idénticos. Un bloque idéntico es
// una constante, y una constante no transporta información sobre lo que pediste — ocupa canal y
// desplaza a lo que sí responde. Medido el 2026-08-29 contra el central: 6 %.
func FraccionVariable(a, b designBrief) float64 {
	ba, bb := bloquesDe(a), bloquesDe(b)
	total, variable := 0, 0
	for nombre, va := range ba {
		vb := bb[nombre]
		peso := len(va)
		if len(vb) > peso {
			peso = len(vb)
		}
		total += peso
		if va != vb {
			variable += peso
		}
	}
	if total == 0 {
		return 0
	}
	return float64(variable) / float64(total)
}

// Abstuvo dice si el motor reconoció que no tenía material para el pedido. Con `degraded` apagado y
// corpus lleno, el motor está afirmando que sabe — y ésa es la falla medida: «receta de empanadas»
// devolvía 6 patrones de diseño con la misma cara de confianza que un pedido legítimo.
func Abstuvo(b designBrief) bool {
	return b.Degraded || len(b.Corpus) == 0
}

// CanalInyeccion nombra POR DÓNDE entró un payload. Se reportan separados porque cada canal lo
// arregla una fase distinta del track, y una métrica única los taparía entre sí (I-BANCO5).
type CanalInyeccion struct {
	EnInstruccion bool // llegó a role/principles/emit/instructions: el agente lo lee como orden
	EnEco         bool // volvió verbatim en `ask`, el PRIMER campo del brief
	EnMaterial    bool // apareció en el corpus, donde se lee como material citado
}

// bloquesDeInstruccion son los campos que el agente que recibe el brief lee como ÓRDENES. Que un
// payload aparezca acá es la falla grave: no es material que el agente evalúa, es conducta que
// obedece.
func bloquesDeInstruccion(b designBrief) string {
	return b.Role + "\x00" + b.Principles + "\x00" + b.Emit + "\x00" + b.Instructions
}

// DondeCayo busca la marca de un payload en cada canal del brief. Se compara contra una FIRMA del
// payload —su fragmento más distintivo— y no contra el texto entero: el brief puede recortar o
// reordenar, y buscar la cadena completa daría falsos «neutralizado» ante cualquier retoque. Ese es
// el error que ya cometí una vez midiendo esto: comparar el payload crudo contra el JSON escapado
// daba «neutralizado» cuando en realidad pasaba entero.
func DondeCayo(b designBrief, payload string) CanalInyeccion {
	firma := firmaDePayload(payload)
	corpus, _ := json.Marshal(b.Corpus)
	return CanalInyeccion{
		EnInstruccion: strings.Contains(bloquesDeInstruccion(b), firma),
		EnEco:         strings.Contains(b.Ask, firma),
		EnMaterial:    strings.Contains(string(corpus), firma),
	}
}

// firmaDePayload extrae el tramo más distintivo de un payload de inyección: la línea más larga,
// normalizada. Es lo que sobrevive a un recorte o a un reordenamiento, y lo que delata que la
// instrucción ajena viajó.
func firmaDePayload(payload string) string {
	mejor := ""
	for _, linea := range strings.Split(payload, "\n") {
		linea = strings.TrimSpace(linea)
		if len(linea) > len(mejor) {
			mejor = linea
		}
	}
	if len(mejor) > 48 { // un tramo largo es específico sin ser frágil ante un recorte al final
		mejor = mejor[:48]
	}
	return mejor
}

// Jaccard es el solape entre dos conjuntos de ids del corpus. Es el instrumento de la
// reproducibilidad: dos formas de pedir lo mismo deberían traer casi el mismo material.
func Jaccard(a, b []string) float64 {
	sa, sb := map[string]bool{}, map[string]bool{}
	for _, x := range a {
		sa[x] = true
	}
	for _, x := range b {
		sb[x] = true
	}
	if len(sa) == 0 && len(sb) == 0 {
		return 0
	}
	inter := 0
	for x := range sa {
		if sb[x] {
			inter++
		}
	}
	return float64(inter) / float64(len(sa)+len(sb)-inter)
}

// IdsDeCorpus saca los ids servidos, ordenados, para comparar dos briefs.
func IdsDeCorpus(b designBrief) []string {
	out := make([]string, 0, len(b.Corpus))
	for _, h := range b.Corpus {
		out = append(out, h.ID)
	}
	sort.Strings(out)
	return out
}

// TocaLosEjes cuenta cuántos items del corpus mencionan algún eje declarado del pedido. Es una
// aproximación GRUESA de precisión, deliberadamente model-free: no juzga si la tarjeta es buena,
// sólo si el material servido habla del tema que se pidió — que es la falla gruesa que hoy tenemos
// (un pedido de trazabilidad de lotes devolvía «usá zebra stripes»). Si algún día deja de
// discriminar entre un corpus bueno y uno malo, se reemplaza por etiquetado humano.
func TocaLosEjes(b designBrief, ejes []string) float64 {
	if len(b.Corpus) == 0 {
		return 0
	}
	tocan := 0
	for _, h := range b.Corpus {
		heno := strings.ToLower(h.TopicKey + " " + h.Gist)
		for _, eje := range ejes {
			if strings.Contains(heno, strings.ToLower(eje)) {
				tocan++
				break
			}
		}
	}
	return float64(tocan) / float64(len(b.Corpus))
}

// ─────────────────────────────────────────────────────────────────────────────────────────────────
// LOS UMBRALES
// ─────────────────────────────────────────────────────────────────────────────────────────────────

// Umbrales son los pisos y techos que el banco defiende. Arrancan clavados en el valor MEDIDO hoy
// —incluidos los malos— porque un banco que naciera exigiendo el objetivo estaría rojo desde el
// primer commit y se apagaría a la semana. La regla es que sólo se APRIETAN: cada fase del track
// mueve el suyo al aterrizar, y ese movimiento se ve en el diff del PR.
type Umbrales struct {
	Fijado   string             `json:"fijado"`
	Commit   string             `json:"commit"`
	Nota     string             `json:"nota"`
	Umbrales map[string]float64 `json:"umbrales"`
}

// CargarUmbrales lee el archivo de umbrales y exige que declare cuándo y en qué commit se fijó
// (I-BANCO3): un umbral sin procedencia es una constante escondida en otro disfraz.
func CargarUmbrales(path string) (*Umbrales, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("banco: leer umbrales: %w", err)
	}
	var u Umbrales
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, fmt.Errorf("banco: parsear umbrales: %w", err)
	}
	if strings.TrimSpace(u.Fijado) == "" || strings.TrimSpace(u.Commit) == "" {
		return nil, fmt.Errorf("banco: los umbrales deben declarar `fijado` y `commit`")
	}
	if len(u.Umbrales) == 0 {
		return nil, fmt.Errorf("banco: el archivo de umbrales está vacío")
	}
	return &u, nil
}

// Piso devuelve el umbral mínimo de una métrica; falta ⇒ error explícito y no un cero silencioso,
// que sería aprobar por omisión.
func (u *Umbrales) Piso(nombre string) (float64, error) {
	v, ok := u.Umbrales[nombre]
	if !ok {
		return 0, fmt.Errorf("banco: no hay umbral declarado para %q", nombre)
	}
	return v, nil
}
