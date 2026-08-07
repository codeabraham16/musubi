package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/config"
	"musubi/internal/skills"
)

// skillfed.go — federación del ARSENAL de skills (Fase A del track «Conocimiento unificado»).
//
// POR QUÉ VIVE EN EL CEREBRO Y NO EN EL CUERPO: la regla de reparto del track dice que si el
// cuerpo puede hacer algo que la terminal no puede, ese algo está en el lugar equivocado. El
// cerebro es dueño de QUÉ es el conocimiento; el cuerpo, de cómo se ve. Construirlo acá hace que
// la terminal lo gane el mismo día y que la mudanza al cuerpo no tenga que migrar nada.
//
// No hay transporte nuevo: SyncClient ya habla MCP-sobre-HTTP con el central (Push, PushGraph,
// Pull). Esto son dos métodos más sobre el mismo cliente, el mismo bearer y la misma config.

// skillPayload es la forma en que una skill cruza la red. Tiene tags JSON explícitos porque
// skills.Skill sólo tiene tags YAML: serializarla directo emitiría {"Name":…} y el receptor
// —que parsea en minúscula— no fallaría, guardaría una skill con todos los campos VACÍOS.
// Es exactamente la trampa que ya mordió en musubi_list_skills (spec arsenal-visible, A1).
type skillPayload struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Triggers     []string `json:"triggers"`
	Capabilities []string `json:"capabilities"`
	Rules        string   `json:"rules"`
	Source       string   `json:"source,omitempty"`
	SourceURL    string   `json:"source_url,omitempty"`
	// AlwaysBecause tiene que cruzar la red o el arreglo entero es teatro: quien mira el arsenal
	// vería una skill siempre-encendida sin poder saber por qué, que es exactamente el estado que
	// este campo vino a terminar (spec «trigger-honesto», G2).
	AlwaysBecause string `json:"always_because,omitempty"`
}

func payloadFromSkill(sk skills.Skill) skillPayload {
	return skillPayload{
		Name:          sk.Name,
		Description:   sk.Description,
		Triggers:      sk.Triggers,
		Capabilities:  sk.Capabilities,
		Rules:         sk.Rules,
		Source:        sk.Source,
		SourceURL:     sk.SourceURL,
		AlwaysBecause: sk.AlwaysBecause,
	}
}

// callCentral emite un tools/call al central y devuelve el TEXTO del primer content.
// Centraliza el sobre JSON-RPC, el bearer y la clasificación de errores para que PushSkill y
// FetchSkill no dupliquen —y no desincronicen— ese manejo.
func (c *SyncClient) callCentral(id, tool string, args any) (string, error) {
	body := struct {
		JsonRpc string `json:"jsonrpc"`
		ID      string `json:"id"`
		Method  string `json:"method"`
		Params  struct {
			Name      string `json:"name"`
			Arguments any    `json:"arguments"`
		} `json:"params"`
	}{JsonRpc: "2.0", ID: id, Method: "tools/call"}
	body.Params.Name = tool
	body.Params.Arguments = args

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("%w: serializar %s: %v", errPermanent, tool, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.http.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("%w: construir %s: %v", errPermanent, tool, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errTransient, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		kind := errPermanent
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			kind = errTransient
		}
		return "", fmt.Errorf("%w: %s HTTP %d", kind, tool, resp.StatusCode)
	}
	var rpcResp syncRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("%w: decodificar %s: %v", errTransient, tool, err)
	}
	if rpcResp.Error != nil {
		kind := errTransient
		if permanentRPCCodes[rpcResp.Error.Code] {
			kind = errPermanent
		}
		return "", fmt.Errorf("%w: %s: %s", kind, tool, rpcResp.Error.Message)
	}
	var toolResult struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil || len(toolResult.Content) == 0 {
		return "", fmt.Errorf("%w: %s sin content parseable", errTransient, tool)
	}
	return toolResult.Content[0].Text, nil
}

// PushSkill guarda una skill LOCAL en el arsenal del central (musubi_save_skill remoto).
func (c *SyncClient) PushSkill(sk skills.Skill, overwrite bool) error {
	args := struct {
		skillPayload
		Overwrite bool `json:"overwrite"`
	}{payloadFromSkill(sk), overwrite}
	_, err := c.callCentral("promote-skill:"+sk.Name, "musubi_save_skill", args)
	return err
}

// ListArsenal trae el CATÁLOGO del arsenal del central, opcionalmente filtrado por subcadena.
//
// Existe porque FetchSkill ya recibía la lista entera y descartaba todo menos un nombre: el
// descubrimiento estaba construido y escondido. Exponerlo es lo que vuelve usable a
// musubi_install_skill, que sin esto exige saber el nombre exacto de memoria (spec
// «arsenal-arranque», B1).
func (c *SyncClient) ListArsenal(query string) ([]skills.Skill, error) {
	args := map[string]any{}
	if query != "" {
		args["query"] = query
	}
	txt, err := c.callCentral("arsenal-list:"+query, "musubi_list_skills", args)
	if err != nil {
		return nil, err
	}
	txt = strings.TrimSpace(txt)
	// Un arsenal VACÍO responde "[]" —musubi_list_skills construye con make(), no con var—, así
	// que "" o "null" NO significan "no hay skills": significan un central que no entiende la
	// tool (versión vieja) o una respuesta rota. Distinguirlo evita reportar «arsenal vacío»
	// cuando el problema real es el deploy del otro lado.
	if txt == "" || txt == "null" {
		return nil, fmt.Errorf("%w: el arsenal del central no devolvió una lista", errPermanent)
	}
	var lista []skillPayload
	if err := json.Unmarshal([]byte(txt), &lista); err != nil {
		return nil, fmt.Errorf("%w: respuesta del arsenal ilegible: %v", errPermanent, err)
	}
	out := make([]skills.Skill, 0, len(lista))
	for _, p := range lista {
		out = append(out, skills.Skill{
			Name:          p.Name,
			Description:   p.Description,
			Triggers:      p.Triggers,
			Capabilities:  p.Capabilities,
			Rules:         p.Rules,
			Source:        p.Source,
			SourceURL:     p.SourceURL,
			AlwaysBecause: p.AlwaysBecause,
		})
	}
	return out, nil
}

// FetchSkill trae UNA skill del arsenal del central por nombre exacto.
//
// Pide con query=name porque musubi_list_skills filtra por SUBCADENA, así que la respuesta puede
// traer parientes ("go-rules" al pedir "go"): el match exacto se hace acá. Sin esa comparación,
// instalar "go" podría bajar cualquier cosa que empiece igual.
func (c *SyncClient) FetchSkill(name string) (skills.Skill, error) {
	lista, err := c.ListArsenal(name)
	if err != nil {
		return skills.Skill{}, err
	}
	for _, sk := range lista {
		if sk.Name == name {
			return sk, nil
		}
	}
	return skills.Skill{}, fmt.Errorf("%w: %q no está en el arsenal del central", errPermanent, name)
}

// arsenalSource es la marca de procedencia de una skill ADOPTADA del arsenal (F4). Permite
// responder «¿esto lo escribí yo o lo bajé?» mirando el archivo, sin adivinar, y es la clave
// para poder re-traerla cuando cambie en el central.
const arsenalSource = "arsenal-central"

// toolPromoteSkill sube una skill LOCAL al arsenal del central.
//
// Es EXPLÍCITA a propósito: nada sube solo. Medido sobre las 11 skills de este repo, 7 tienen
// trigger "*" —disparan en cualquier archivo—, y algunas son locales por naturaleza
// (project-profile describe ESTE proyecto). Subirlas todas ensuciaría el arsenal de todos. La
// curaduría es del dueño; la herramienta sólo la hace fácil.
// gateDePromocion es lo que se le cobra a una skill por entrar al arsenal COMPARTIDO.
//
// Hasta el spec «trigger-honesto» la promoción era la ÚNICA puerta sin gate: save_skill,
// author_skill e install_skill validaban calidad, y justo la que publica para todos no. Una skill
// escrita a mano —que nunca pasó por ninguna tool— subía tal cual.
//
// El segundo chequeo es asimétrico A PROPÓSITO: un "*" sin declarar es un warning en el score y un
// ERROR acá. En tu proyecto el alcance es tu problema; en el arsenal se lo comen todos los demás,
// que van a recibir esas reglas encendidas en cada tarea de un repo que la skill no conoce.
func gateDePromocion(sk skills.Skill) *RpcError {
	if report := skills.ValidateSkillQuality(sk); !report.OK() {
		return rpcErrorf(codeInvalidParams,
			"la skill %q no pasa el gate de calidad y no se sube al arsenal:\n%s",
			sk.Name, formatIssues(report.Errors))
	}
	if skills.WildcardUnjustified(sk) {
		return rpcErrorf(codeInvalidParams,
			"la skill %q se activa SIEMPRE ('*' entre sus triggers) sin declarar por qué, y el arsenal es compartido.\n"+
				"Acotá los triggers a donde aplique de verdad, o —si aplica siempre porque se activa por TIPO DE TAREA "+
				"y no por archivo— agregale el campo 'always_because' con la razón (mínimo %d caracteres).",
			sk.Name, skills.MinAlwaysBecauseChars)
	}
	return nil
}

func (s *McpServer) toolPromoteSkill(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Name      string `json:"name"`
		Overwrite bool   `json:"overwrite"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	if strings.TrimSpace(args.Name) == "" {
		return nil, rpcErrorf(codeInvalidParams, "name es obligatorio")
	}
	// F2: sin central, se dice. Un promote que "anda" sin subir nada es peor que uno que falla.
	if s.syncClient == nil {
		return nil, rpcErrorf(codeInvalidParams,
			"no hay cerebro central configurado: definí sync.central_url y sync.auth_token_env para promover skills al arsenal")
	}

	todas, err := s.resolver.LoadSkills()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo leer el arsenal local: %v", err)
	}
	for _, sk := range todas {
		if sk.Name != args.Name {
			continue
		}
		if rerr := gateDePromocion(sk); rerr != nil {
			return nil, rerr
		}
		if perr := s.syncClient.PushSkill(sk, args.Overwrite); perr != nil {
			return nil, rpcErrorf(codeInternalError, "no se pudo promover %q al arsenal: %v", args.Name, perr)
		}
		return textResult(fmt.Sprintf("Skill %q promovida al arsenal del cerebro central.", args.Name)), nil
	}
	// F6: no se promueve un esqueleto por un nombre que no existe.
	return nil, rpcErrorf(codeInvalidParams, "no existe una skill local llamada %q", args.Name)
}

// toolInstallSkill baja una skill del arsenal del central y la escribe en el proyecto.
//
// Convierte el «Adoptar» de la Forja —que hoy sólo REGISTRA la decisión— en una acción real.
func (s *McpServer) toolInstallSkill(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Name      string `json:"name"`
		Overwrite bool   `json:"overwrite"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	if strings.TrimSpace(args.Name) == "" {
		return nil, rpcErrorf(codeInvalidParams, "name es obligatorio")
	}
	if s.syncClient == nil {
		return nil, rpcErrorf(codeInvalidParams,
			"no hay cerebro central configurado: definí sync.central_url y sync.auth_token_env para instalar skills del arsenal")
	}

	sk, err := s.syncClient.FetchSkill(args.Name)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo traer %q del arsenal: %v", args.Name, err)
	}

	// F3 — EL INVARIANTE DE SEGURIDAD. Lo que vuelve del central es DATO REMOTO, y tratarlo como
	// confiable «porque es nuestro central» es exactamente cómo se cuela un escape de directorio.
	// El nombre se valida ANTES de tocar cualquier ruta: validateSkillStructural exige un slug
	// (sin '/', sin '..', sin unidad de Windows), así que ni el chequeo de existencia de abajo ni
	// writeSkillFile llegan a componer una ruta con basura.
	if rerr := validateSkillStructural(sk.Name, sk.Triggers, sk.Rules); rerr != nil {
		return nil, rerr
	}
	if report := skills.ValidateSkillQuality(sk); !report.OK() {
		return nil, rpcErrorf(codeInvalidParams,
			"la skill del arsenal no pasa el gate de calidad:\n%s", formatIssues(report.Errors))
	}

	// F5: sin overwrite no se pisa nada de lo que ya hay en el proyecto.
	skillsDir := filepath.Join(s.projectPath, config.DirName, config.SkillsDir)
	if _, serr := os.Stat(filepath.Join(skillsDir, sk.Name+".yaml")); serr == nil && !args.Overwrite {
		return nil, rpcErrorf(codeInvalidParams,
			"la skill %q ya existe en este proyecto; pasá overwrite=true para reemplazarla", sk.Name)
	}

	// F4: queda marcada como adoptada. SourceURL se PRESERVA — si la skill venía de un catálogo,
	// ese enlace sigue siendo el rastro a su origen; lo que se pisa es sólo el 'source', que pasa
	// a decir cómo llegó a ESTE proyecto.
	sk.Source = arsenalSource

	path, rerr := s.writeSkillFile(sk)
	if rerr != nil {
		return nil, rerr
	}
	return textResult(fmt.Sprintf("Skill %q instalada desde el arsenal del central en %s.", sk.Name, path)), nil
}
