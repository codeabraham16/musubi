# Motor Hermes: Resolución Dinámica de Skills

## 1. Concepto
El motor Hermes de Musubi es un sistema de resolución dinámica de dependencias (DAG) en tiempo de ejecución. Permite que los agentes carguen contexto, reglas de estilo y dependencias de manera automática basándose en los archivos que están modificando, sin intervención humana.

## 2. Flujo de Resolución

1. **Detección de Triggers**: El agente informa qué archivos va a tocar (ej. `main.go`, `config.yaml`).
2. **Matching Topológico**: Hermes lee los triggers de todas las skills disponibles en `.musubi/skills/*.yaml`.
3. **Chequeo de Capacidades (System Check)**: Antes de cargar una skill, verifica que el host tenga las herramientas necesarias (ej. verifica que `go` está en el PATH).
4. **Agregación de Reglas**: Si la skill pasa el chequeo, se extraen sus "Compact Rules" y se inyectan en el prompt del agente bajo el bloque `## Project Standards (auto-resolved)`.

## 3. Prevención de Colapsos
Hermes protege al orquestador limitando la cantidad de reglas inyectadas (presupuesto de tokens). Si un agente requiere más de 5 skills simultáneas, Hermes prioriza las skills del lenguaje core (ej. Go/TypeScript) sobre las skills secundarias (ej. documentación), evitando saturar la ventana de contexto del LLM.
