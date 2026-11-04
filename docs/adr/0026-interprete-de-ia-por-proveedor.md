# ADR-0026 · Intérprete de IA por proveedor (reglas por defecto; OpenRouter para la demo)

- **Estado:** Propuesto
- **Fecha:** 2026-11-04
- **Quién decide / aprueba:** Pablo (pidió OpenRouter como proveedor para la demo); revisión de Isaac en el PR. D-08 sigue abierta
- **Relacionado con:** RNF-05, RNF-06, RNF-13, HU-52, HU-53, HU-54, HU-99, HU-102, D-08, AQ-09, AQ-10, `docs/06` §5 (puerto `LLMProvider`), ADR-0025

## Contexto
El agente clasificaba con reglas dentro del caso de uso, sin puerto, y la llamada (de 0 costo) se registraba dentro de la transacción. Para T3 hace falta un modelo de lenguaje que entienda mensajes como "mi Spark está chillando cuando freno", intercambiable (RNF-06), con una sola llamada por mensaje y salida validada (RNF-05), y que si falla no deje nada roto (RNF-13). Pablo pidió OpenRouter para la demo.

## Decisión
- **La IA solo interpreta; el código redacta.** El puerto `assistant/app.Interpreter` devuelve el JSON crudo de `domain.Understanding` (intención, vehículo, placa, síntoma, servicios de la lista, otros, fecha). `domain.ParseUnderstanding` lo valida: JSON estricto (campos desconocidos o valores fuera del enum lo descartan), servicios solo de la lista (lo demás pasa a "otros") y la placa solo si el cliente la escribió. El texto al cliente lo arma `domain.BuildDraft` con precios de la lista y lo revisa `domain.Validate`. La IA nunca ve precios ni datos personales del cliente (solo el texto del mensaje y los nombres y tipos de la lista).
- **Adaptadores** en `assistant/adapters/out/llm/`:
  - `rules`: reglas locales, sin red ni costo. Es el valor por defecto y el respaldo.
  - `openrouter`: API compatible con OpenAI (`POST {AI_API_BASE}/chat/completions`) con `response_format` `json_schema` estricto, temperatura 0, el texto del cliente delimitado como dato (R-08), costo desde `usage.cost`, errores con solo el código (sin eco del cuerpo ni de la clave). Otro proveedor compatible se usa cambiando `AI_API_BASE`.
- **Elección por configuración:** `AI_PROVIDER=reglas|openrouter`, con `AI_API_KEY`, `AI_MODEL` y `AI_API_BASE`. `AI_MODEL` acepta varios modelos separados por coma, en orden de preferencia: van en el parámetro `models` y OpenRouter pasa al siguiente si uno falla, está limitado o caído (https://openrouter.ai/docs/guides/routing/model-fallbacks, consultada el 2026-11-04); el registro guarda el modelo que respondió. El razonamiento va apagado (`reasoning.enabled=false`): extraer datos no lo necesita. `openrouter` exige clave y modelo, y **no arranca en producción** hasta cerrar AQ-09.
- **La llamada sale de la transacción**, con límite de 30 s (HU-54). Con el tope mensual del taller agotado se usan las reglas. Toda llamada queda en `assistant.llamada_ia` con proveedor, modelo, tokens, costo, latencia, resultado (`ok`, `invalido`, `error`, `tope`) y el mensaje que la originó (migración 00021, HU-99).
- Salida inválida, error o tiempo vencido: el borrador queda "No pudimos armar una respuesta segura. Responde tú." y no sale nada al cliente.
- Fuente del formato de OpenRouter: https://openrouter.ai/docs/api/reference/overview (consultada el 2026-11-04).

## Consecuencias
- Cambiar de modelo o de proveedor es configuración; comparar modelos (D-08) se hace con `make agent-eval` sobre el juego de 30 mensajes de HU-102.
- Como la IA no escribe el texto, los borradores suenan a plantilla. Si se quiere redacción libre, se agrega una segunda etapa que vuelva a pasar por `Validate`, sin quitar la validación.
- OpenRouter reenvía a terceros: **solo demo**, sin datos reales, hasta verificar que no se entrena con nuestros datos (AQ-09) y el tratamiento de datos (AQ-10).
- Las pruebas del adaptador corren contra un `httptest.Server`; las del agente usan las reglas o una IA falsa. Falta medir un modelo real con una clave.
