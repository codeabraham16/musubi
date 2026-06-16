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
