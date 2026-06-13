# Bucle de Telemetría y Autocorrección (Self-Healing)

## 1. Monitorización en Caliente
Musubi no delega las tareas a ciegas. Cada sub-agente corre en un entorno controlado (Goroutine) donde `Stdout` y `Stderr` son interceptados y evaluados en tiempo real.

## 2. Bucle de Reflexión

- **Paso 1 (Fallo)**: Un agente intenta compilar un código Go, pero el compilador arroja `undefined: foo`.
- **Paso 2 (Análisis)**: El motor de Telemetría parsea el log de error y detecta un patrón de fallo sintáctico o de importación.
- **Paso 3 (Hot-Patching)**: La telemetría inyecta una regla correctiva temporal en la memoria semántica de la sesión (ej. "Asegúrate de importar el paquete X antes de usar foo").
- **Paso 4 (Reintento Seguro)**: Se relanza el agente con la regla correctiva inyectada. 

## 3. Aprendizaje Persistente (Engram-style)
Si el "Hot-Patch" resulta exitoso y compila, Musubi guarda automáticamente esa lección en su base de datos local SQLite bajo la clave de tópico del proyecto. De esta forma, el ecosistema *aprende permanentemente* de los errores específicos del repositorio, garantizando que el mismo error no se repita en futuras sesiones.
