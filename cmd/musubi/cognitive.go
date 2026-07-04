package main

import (
	"os"
	"path/filepath"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/skills"

	"gopkg.in/yaml.v3"
)

// cognitive.go define el bundle de SKILLS COGNITIVAS de arranque: plantillas
// model-free (reglas que ejecuta el LLM del cliente) que hacen que, desde el
// primer momento, el agente analice el proyecto, deduzca convenciones, planee y
// mantenga un PERFIL vivo. Ese perfil es el ancla del autoconocimiento: una vez
// que existe, el priming lo surfacea barato y el hook deja de inyectar el bloque
// cognitivo (ver buildCognitiveContext en detect.go).

// profileTopicKey es el topic_key canónico del perfil del proyecto. Su existencia
// (TopicExists) marca que el proyecto ya está "perfilado".
const profileTopicKey = "project/profile"

// ecosystemGlobs mapea cada ecosistema a los globs que lo representan. Se usan
// como triggers de las skills cognitivas dependientes del stack.
var ecosystemGlobs = map[string][]string{
	"Go":      {"*.go"},
	"Node.js": {"*.js", "*.ts", "*.tsx", "*.jsx"},
	"Python":  {"*.py"},
	"Rust":    {"*.rs"},
	"Java":    {"*.java", "*.kt"},
	"Ruby":    {"*.rb"},
	"PHP":     {"*.php"},
	".NET":    {"*.cs"},
	"Dart":    {"*.dart"},
	"Elixir":  {"*.ex", "*.exs"},
	"C/C++":   {"*.c", "*.cc", "*.cpp", "*.h", "*.hpp"},
	"Docker":  {"Dockerfile"},
}

// projectTriggers deriva los globs de los ecosistemas detectados. Sin stack
// reconocido, cae al genérico "*".
func projectTriggers(stack []detector.StackResult) []string {
	var triggers []string
	for _, r := range stack {
		for _, g := range ecosystemGlobs[r.Ecosystem] {
			triggers = appendUnique(triggers, g)
		}
	}
	if len(triggers) == 0 {
		triggers = []string{"*"}
	}
	return triggers
}

// cognitiveSkills devuelve el bundle de 4 skills cognitivas, con los triggers de
// análisis/deducción adaptados al stack detectado.
func cognitiveSkills(stack []detector.StackResult) []skills.Skill {
	stackTriggers := projectTriggers(stack)
	return []skills.Skill{
		{
			Name:         "analyze-project",
			Description:  "Analiza la estructura del proyecto para arrancar el autoconocimiento.",
			Triggers:     stackTriggers,
			Capabilities: []string{},
			Rules: "Cuando empieces a trabajar en este proyecto:\n" +
				"- Mapeá la estructura: manifests, entrypoints, carpetas clave y scripts de build/test.\n" +
				"- Usá musubi_detect_stack para confirmar ecosistemas y frameworks.\n" +
				"- Capturá los hallazgos NO obvios con musubi_save_observation (topic_key 'analysis/...').\n" +
				"- No re-analices lo que ya esté en memoria: primero recuperá con musubi_recall.\n",
		},
		{
			Name:         "deduce-conventions",
			Description:  "Deduce las convenciones del proyecto a partir del código existente.",
			Triggers:     stackTriggers,
			Capabilities: []string{},
			Rules: "A partir del código existente, deducí (no inventes) las convenciones del proyecto:\n" +
				"- Naming, estructura de carpetas, estilo de tests y manejo de errores.\n" +
				"- Guardá cada convención estable como hecho con musubi_save_fact (ej. sujeto='proyecto', predicado='usa', objeto='gofmt').\n" +
				"- Ante dudas o señales contradictorias, marcá la incertidumbre en vez de asumir.\n",
		},
		{
			Name:         "plan-ahead",
			Description:  "Planea antes de actuar usando lo que el proyecto ya sabe de sí mismo.",
			Triggers:     []string{"*"},
			Capabilities: []string{},
			Rules: "Antes de implementar o cambiar algo:\n" +
				"- Recuperá contexto con musubi_recall y musubi_recall_facts.\n" +
				"- Armá un plan corto (pasos concretos) basado en lo que ya se sabe del proyecto.\n" +
				"- Identificá explícitamente los huecos de información y resolvelos antes de codear.\n",
		},
		{
			Name:         "project-profile",
			Description:  "Mantiene un perfil vivo del proyecto: el ancla del autoconocimiento.",
			Triggers:     []string{"*"},
			Capabilities: []string{},
			Rules: "Mantené un PERFIL del proyecto como memoria de alto nivel:\n" +
				"- Guardalo y actualizalo con musubi_save_observation usando el topic_key exacto 'project/profile'.\n" +
				"- El perfil resume: propósito, stack, arquitectura, convenciones clave y decisiones vigentes.\n" +
				"- Actualizalo cuando descubras algo nuevo o cambie una decisión; mantenelo conciso.\n" +
				"- Este perfil es lo que Musubi recuerda al arrancar cada sesión, así que mantenelo al día.\n",
		},
		{
			Name:         "orchestrate-multiagent",
			Description:  "Orquesta sub-agentes en paralelo usando la pizarra compartida de Musubi.",
			Triggers:     []string{"*"},
			Capabilities: []string{},
			Rules: "Cuando una tarea sea grande y paralelizable (varios archivos/áreas independientes), orquestá sub-agentes con la pizarra compartida en vez de hacerlo todo en serie:\n" +
				"1. Descomponé la tarea y posteá las unidades con musubi_work action=plan (cada unidad: title + spec claro y autónomo). Guardá el batch_id que devuelve.\n" +
				"2. Lanzá N sub-agentes con el Task tool. En CADA uno: pasá mcpServers:[musubi] e instruí el protocolo: hacer musubi_work action=claim batch=<id> agent=<etiqueta>, ejecutar la unidad, y cerrarla con musubi_work action=complete id=<id> result=<resumen>. El claim es atómico: dos sub-agentes nunca toman la misma unidad.\n" +
				"3. Monitoreá con musubi_work action=status batch=<id>. Cuando todas estén done, leé los results y CONSOLIDÁ una respuesta única.\n" +
				"- La descomposición y la consolidación son tuyas (la inteligencia); Musubi solo garantiza coordinación sin colisiones.\n" +
				"- Guardá las decisiones/aprendizajes del trabajo con musubi_save_observation.\n",
		},
		{
			Name:         "audit-structure-flow",
			Description:  "Audita la estructura y el flujo del codebase y emite hallazgos priorizados con evidencia.",
			Triggers:     []string{"*"},
			Capabilities: []string{},
			Rules: "Cuando se pida auditar la ESTRUCTURA (organización, cohesión, acoplamiento, capas, código muerto) o el FLUJO (dirección de dependencias, ciclos, entrada→salida, propagación de context/errores) del proyecto:\n" +
				"1. Mapeá módulos/paquetes (tamaño + responsabilidad) y construí el grafo de dependencias con la herramienta del stack (go list, madge, pydeps, cargo modules, jdeps). Verificá cada afirmación contra el código; no asumas por los nombres.\n" +
				"2. Severidad — ALTO: ciclo o inversión (módulo core que importa IO/transporte), estado global mutable, errores tragados. MEDIO: smell con costo real (god-file, módulo grab-bag, código muerto/huérfano). BAJO: cosmético.\n" +
				"3. Corré SIEMPRE el chequeo de código muerto/huérfanos (módulo importado por nadie y sin tests).\n" +
				"4. No marques patrones normales (un hub de wiring, un archivo grande pero cohesivo, un entrypoint con IO) salvo que tengan costo concreto. Nunca propongas reescrituras: la acción más chica de mayor impacto.\n" +
				"5. Separá estructura (forma estática) de flujo (recorrido dinámico). Cada hallazgo: severidad + evidencia (ruta:símbolo) + acción. Guardá el resultado con musubi_save_observation (topic_key 'audit/...') e incluí un Top 3 de acciones.\n",
		},
		{
			Name:         "sdd-flow",
			Description:  "Conduce features medianas-grandes con Spec-Driven Development guiado (musubi_sdd), encarnando el rol de cada fase.",
			Triggers:     []string{"*"},
			Capabilities: []string{},
			Rules: "Para un cambio no trivial (varias fases, 2+ archivos, decisiones de diseño), usá el flujo SDD guiado en vez de improvisar:\n" +
				"1. Arrancá con musubi_sdd action=start change=<nombre>. Devuelve la fase activa, su ROL y su directiva; ENCARNÁ ese rol (proponente→especificador→diseñador→planificador→implementador→verificador→archivador).\n" +
				"2. Trabajá la fase usando su plantilla (.musubi/templates/sdd/) y cerrala con musubi_sdd action=complete change=<c> phase=<f> summary=<resumen ejecutivo> [artifacts, risks, next_recommended]. Eso persiste el artefacto en memoria bajo sdd/<change>/<phase>.\n" +
				"3. En implement NO releas lo ya visto: recuperá los artefactos previos con musubi_recall query='sdd/<change>' y los archivos con musubi_recall_code.\n" +
				"4. En verify sé adversarial (ver la skill adversarial-review). Si una fase abarca trabajo paralelizable, delegá con la pizarra (orchestrate-multiagent) y medí el ahorro con musubi_work action=savings.\n" +
				"5. El run es resumible: en otra sesión retomás con musubi_sdd action=status change=<c>.\n",
		},
		{
			Name:         "adversarial-review",
			Description:  "Revisión adversarial estilo debate: escépticos con lentes distintos refutan un cambio en rondas, con veredicto por mayoría determinista y bucle de corrección.",
			Triggers:     []string{"*"},
			Capabilities: []string{},
			Rules: "Antes de dar por bueno un cambio de riesgo (o en la fase verify de un flujo SDD), sometelo a un DEBATE adversarial con veredicto determinista en vez de una sola lectura complaciente:\n" +
				"1. Abrí el debate con musubi_debate action=open topic=<el cambio a juzgar> rounds=2 quorum=<mayoría de los escépticos, p. ej. 2 de 3>. Guardá el debate_id. (musubi_debate estructura las rondas y CUENTA los votos sin sesgo; la inteligencia de refutar es tuya.)\n" +
				"2. Lanzá N sub-agentes escépticos con el Task tool (mcpServers:[musubi]), cada uno con un LENTE distinto (correctitud, seguridad, ¿reproduce el bug?, contrato de la spec). A CADA uno instruílo a intentar REFUTAR el cambio desde su lente y a postear su hallazgo con musubi_debate action=post id=<debate_id> agent=<lente> stance=<veredicto + evidencia>. Ante la duda, que refute.\n" +
				"3. Cerrá la ronda con musubi_debate action=advance id=<debate_id>: devuelve las posturas de todos los lentes. Pasáselas a los escépticos para una 2ª ronda de CRÍTICA CRUZADA (cada uno revisa las refutaciones ajenas y ajusta la suya con otro post) — es lo que una sola lectura no te da.\n" +
				"4. Recogé el veredicto: cada escéptico vota con musubi_debate action=vote id=<debate_id> agent=<lente> choice=<real|no_real>. Cerrá con action=tally: el recuento es DETERMINISTA y queda PERSISTIDO (reproducible). Si gana 'no_real' (la mayoría refuta), hay que corregir: iterá (fix → re-debate) hasta que el cambio sobreviva. Si el tally da no_consensus (empate/sin quórum), deferí el juicio final a musubi_judge.\n" +
				"5. Registrá lo aprendido con musubi_save_observation (topic_key 'review/<change>'); el debate ya conserva las posturas y el veredicto. Si el hallazgo contradice una memoria previa, resolvé la relación con musubi_judge.\n",
		},
		{
			Name:         "designing-web-ui",
			Description:  "Applies global web UI/UX design standards — visual hierarchy, spacing rhythm, typographic scale, accessible color contrast (WCAG AA), responsive layout and restrained motion. Use when editing HTML, CSS or component files or when building or restyling any user interface.",
			Triggers:     []string{"*.html", "*.css", "*.tsx", "*.jsx", "*.vue", "*.svelte", "*.astro"},
			Capabilities: []string{},
			Rules: "Diseñá interfaces con jerarquía y ritmo, no por intuición. Reglas (derivadas de Refactoring UI + WCAG 2.1):\n" +
				"1. Espaciado en escala 4/8px (4,8,12,16,24,32,48,64). Nunca valores arbitrarios: definí tokens y reusalos.\n" +
				"2. Jerarquía por PESO y COLOR antes que por tamaño: texto principal alto contraste, secundario atenuado (mismo hue, menor lightness). Máximo 2-3 tamaños de fuente por vista.\n" +
				"3. Escala tipográfica modular (12/14/16/20/24/32). line-height 1.5 en cuerpo, 1.2 en títulos.\n" +
				"4. Contraste WCAG AA: >=4.5:1 texto normal, >=3:1 texto grande/íconos. Verificá siempre, no confíes en el ojo.\n" +
				"5. Color: una familia neutra (grises) + 1 acento; estados semánticos (éxito/aviso/error) consistentes. Usá variables CSS, no hardcodees hex repetidos.\n" +
				"6. Layout: grid/flex, ancho de lectura <=70ch, mobile-first con breakpoints, alineado a la grilla.\n" +
				"7. Profundidad sutil: sombras suaves de una sola dirección (luz arriba), bordes de 1px de bajo contraste. Nada de sombras duras.\n" +
				"8. Movimiento con propósito: transiciones 150-250ms ease-out; respetá prefers-reduced-motion.\n" +
				"9. Accesibilidad: foco visible, labels/aria, targets >=44px, semántica HTML correcta.\n" +
				"10. Consistencia: componentes reutilizables con tokens; nada de estilos ad-hoc por página.\n\n" +
				"Ejemplo (tokens + jerarquía por color):\n" +
				"```css\n" +
				":root{ --space-4:16px; --text:#e8eaed; --muted:#9aa0a6; --accent:#4ade80; --line:#2a2d31; }\n" +
				".card{ padding:var(--space-4); border:1px solid var(--line); border-radius:12px; }\n" +
				".card .title{ font-size:20px; line-height:1.2; color:var(--text); font-weight:600; }\n" +
				".card .meta{ font-size:13px; color:var(--muted); }\n" +
				"```\n" +
				"Antipatrón: 5 tamaños de fuente mezclados, hex repetidos, sombras duras, gris claro sobre blanco (<4.5:1).\n",
		},
	}
}

// writeCognitiveSkills escribe el bundle cognitivo en .musubi/skills/ (un archivo
// por skill). No sobrescribe skills ya editadas por el usuario.
func writeCognitiveSkills(root string) error {
	skillsDir := filepath.Join(root, config.DirName, config.SkillsDir)
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}
	stack, _ := detector.DetectStack(root)
	for _, sk := range cognitiveSkills(stack) {
		path := filepath.Join(skillsDir, sk.Name+".yaml")
		if _, err := os.Stat(path); err == nil {
			continue // ya existe: respetar la versión del usuario
		}
		data, err := yaml.Marshal(sk)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// appendUnique agrega elem a slice si no está presente.
func appendUnique(slice []string, elem string) []string {
	for _, e := range slice {
		if e == elem {
			return slice
		}
	}
	return append(slice, elem)
}
