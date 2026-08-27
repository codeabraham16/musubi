package mcp

// flota.go es FLOTA EN VIVO: el caño que lleva la telemetría de invocaciones de cada máquina al
// cerebro central, para que su panel muestre la flota entera trabajando MIENTRAS trabaja.
//
// La señal ya existía — livefeed.go publica cada invocación al terminar (tool, outcome, ms, kind;
// JAMÁS contenido, invariante L1) y cada máquina la muestra en su panel local — pero moría en la
// propia máquina. Esto agrega las dos puntas que faltaban:
//
//   REMITENTE (RunFlotaVivo): corre en el daemon local con sync configurado. Se suscribe a su
//   propio feed, filtra SOLO trabajo (medido 2026-08-26: el 99,92 % de las invocaciones locales
//   son sondeo — no cruzan la red ni una vez), junta lotes de a 32 o 2 s, y los empuja con el
//   MISMO token del sync. Best-effort estilo hooks: si el central no está, el lote se descarta y
//   la sesión ni se entera. La durabilidad es del sync de MEMORIA; la telemetría no la necesita.
//
//   RECEPTOR (handlerFlota): POST /api/flota en el central. Valida el token y RE-SELLA todo lo
//   interpretable — principal y project salen del token (nunca del body: una máquina no puede
//   disfrazarse), kind se recomputa con clasificarTool (un sondeo no se vuelve trabajo por
//   declararse trabajo), outcome se normaliza, tool se valida contra forma. Publica en el feed
//   del central con origen "flota" y NO persiste nada: el feed es el presente; la historia de
//   cada máquina vive en su ledger local.
//
// La frontera de confianza es la del sync: una máquina que ya empuja su memoria completa al
// central no revela nada nuevo contando nombres de tools. Por eso el default es activo cuando
// hay destino de sync, y flota_vivo: false lo apaga sin tocar el sync (config.SyncConfig).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"musubi/internal/logx"
)

const (
	// origenFlota marca en el feed del central los eventos que llegaron de una máquina de la
	// flota. Además es el freno estructural anti-loop: el remitente sólo manda origen "local",
	// así que un evento "flota" jamás se re-reenvía aunque alguien encendiera un remitente en
	// el propio central.
	origenFlota = "flota"

	// flotaLoteMax y flotaLoteEspera definen el batch del remitente: lo que llegue primero.
	// A ritmo real de trabajo (decenas de eventos por DÍA) esto es un POST ocasional, no un tren.
	flotaLoteMax    = 32
	flotaLoteEspera = 2 * time.Second

	// flotaBatchTope y flotaBodyTope acotan al receptor (I3). 256 eventos * ~200 bytes queda
	// lejísimos del tope del body; el doble tope existe porque miden cosas distintas (filas vs
	// bytes) y un atacante puede inflar cualquiera de las dos.
	flotaBatchTope = 256
	flotaBodyTope  = 256 << 10 // 256 KiB

	// flotaSesgoMax es cuánto puede desviar el `at` declarado del reloj del server antes de que
	// el server lo pise con su propia hora. Un reloj local roto no puede dibujar el pasado ni el
	// futuro en el riel de todos.
	flotaSesgoMax = 5 * time.Minute
)

// flotaToolRe es la forma de un nombre de tool legítimo. Lo que no matchea se descarta esa fila
// (contado, no silencioso): el nombre viaja al DOM del panel de todos los que miran.
var flotaToolRe = regexp.MustCompile(`^musubi_[a-z0-9_]{1,64}$`)

// RunFlotaVivo es el remitente. Corre como goroutine del daemon local junto a los schedulers de
// sync (mismo ciclo de vida, mismo token). Sale cuando el ctx muere.
func (s *McpServer) RunFlotaVivo(ctx context.Context) {
	if s.live == nil || s.syncClient == nil {
		return
	}
	// El backlog del ring NO viaja: «en vivo» no es «historia», y un restart no re-manda.
	id, ch, _ := s.live.subscribe("", false)
	defer s.live.unsubscribe(id)

	lote := make([]LiveEvent, 0, flotaLoteMax)
	reloj := time.NewTicker(flotaLoteEspera)
	defer reloj.Stop()

	// El log de fallo va con freno: un central caído durante horas no puede llenar el stderr de
	// la sesión — se avisa una vez por minuto como mucho, y el resto se descarta en silencio
	// contándose en `descartados`.
	var ultimoLog time.Time
	var descartados int

	enviar := func() {
		if len(lote) == 0 {
			return
		}
		pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := s.syncClient.PushFlota(pctx, lote)
		cancel()
		if err != nil {
			descartados += len(lote)
			if time.Since(ultimoLog) > time.Minute {
				logx.Error("flota: no se pudo publicar telemetría al central (se descarta y se sigue)",
					"descartados", descartados, "error", err)
				ultimoLog = time.Now()
				descartados = 0
			}
		}
		lote = lote[:0]
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !esDeFlota(ev) {
				continue
			}
			lote = append(lote, ev)
			if len(lote) >= flotaLoteMax {
				enviar()
			}
		case <-reloj.C:
			enviar()
		}
	}
}

// esDeFlota decide qué eventos del feed local viajan al central (I1 + no-loop): SOLO el trabajo
// de ESTA máquina. El sondeo es el latido de los paneles y el sync — 99,92 % del tráfico — y el
// central ya tiene el suyo; y un evento que no sea de origen local (p. ej. "flota" relayado) no
// se re-reenvía jamás.
func esDeFlota(ev LiveEvent) bool {
	return ev.Kind == KindTrabajo && ev.Origen == "local" && ev.Tool != ""
}

// PushFlota empuja un lote de telemetría al receptor /api/flota del central. Un solo POST JSON,
// el mismo bearer del sync, sin gzip (un lote pesa KB). Cualquier respuesta fuera de 2xx es
// error — el caller descarta, acá no hay reintentos ni dead-letter a propósito (ver flota.go
// cabecera: telemetría, no memoria).
func (c *SyncClient) PushFlota(ctx context.Context, lote []LiveEvent) error {
	body, err := json.Marshal(lote)
	if err != nil {
		return fmt.Errorf("flota: marshal: %w", err)
	}
	url := strings.TrimSuffix(c.url, "/mcp") + "/api/flota"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("flota: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("flota: post: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("flota: el central respondió %d", resp.StatusCode)
	}
	return nil
}

// flotaEventoEntrante es lo ÚNICO que un evento de flota puede traer. El decode es estricto
// (DisallowUnknownFields): un campo extra —content, args, lo que sea— rechaza el batch entero.
// Es la mitad receptora del invariante L1: el struct no tiene dónde poner contenido, y el que
// lo intente rebota con 400 en vez de colarse en silencio (I4).
type flotaEventoEntrante struct {
	At         string  `json:"at"`
	Tool       string  `json:"tool"`
	Outcome    string  `json:"outcome"`
	DurationMs float64 `json:"ms"`
}

// handlerFlota es el receptor central de la telemetría de la flota. Auth con el mismo patrón de
// /api/stream; identidad y clase SIEMPRE re-selladas server-side (I2, I5); topes de batch y de
// body (I3). Publica al feed y responde 202 — nada persiste.
func (s *McpServer) handlerFlota(opt httpOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var principal *Principal
		if opt.registry != nil {
			p, ok := opt.registry.resolve(bearerToken(r.Header.Get("Authorization")))
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			principal = p
		} else if opt.token != "" && !validBearer(r.Header.Get("Authorization"), opt.token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if s.live == nil {
			http.Error(w, "live feed unavailable", http.StatusServiceUnavailable)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, flotaBodyTope)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var entrantes []flotaEventoEntrante
		if err := dec.Decode(&entrantes); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(entrantes) > flotaBatchTope {
			http.Error(w, fmt.Sprintf("batch de %d eventos supera el tope de %d", len(entrantes), flotaBatchTope), http.StatusBadRequest)
			return
		}

		ahora := time.Now()
		aceptados, salteados := 0, 0
		for _, in := range entrantes {
			tool := strings.TrimSpace(in.Tool)
			if !flotaToolRe.MatchString(tool) {
				salteados++
				continue
			}
			outcome := "ok"
			if in.Outcome != "ok" {
				outcome = "error"
			}
			ms := in.DurationMs
			if ms < 0 {
				ms = 0
			}
			// El `at` declarado se honra sólo si parsea y está cerca del reloj del server: un
			// reloj local roto no puede dibujar pasado ni futuro en el riel de todos.
			at := ahora
			if t, err := time.Parse("2006-01-02T15:04:05.000Z07:00", in.At); err == nil {
				if d := ahora.Sub(t); d > -flotaSesgoMax && d < flotaSesgoMax {
					at = t
				}
			}
			ev := LiveEvent{
				At:         at.Format("2006-01-02T15:04:05.000Z07:00"),
				Tool:       tool,
				Outcome:    outcome,
				DurationMs: ms,
				Kind:       clasificarTool(tool), // I5: la clase la decide el server
				Origen:     origenFlota,
			}
			if principal != nil {
				ev.Principal = principal.Name // I2: la identidad la sella el token
				ev.Project = principal.ProjectID
			}
			s.live.publish(ev)
			aceptados++
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]int{"aceptados": aceptados, "saltados": salteados})
	}
}
