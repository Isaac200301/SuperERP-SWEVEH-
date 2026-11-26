// IA falsa para el e2e del asistente del piso (R8): responde como la API de chat de OpenRouter
// (POST /chat/completions) con un guion fijo, para que el recorrido en navegador no dependa de un modelo real ni
// gaste cupo. Solo la usa scripts/e2e.sh; nunca corre en un servidor. Datos Ficticio.
//
//   node scripts/fake-ai.mjs 8091
import { appendFileSync } from 'node:fs';
import { createServer } from 'node:http';

const port = Number(process.argv[2] ?? 8091);
const SIDES = ['frente', 'trasera', 'izquierdo', 'derecho'];

function text(c) {
  if (typeof c === 'string') return c;
  return (c ?? []).filter((p) => p.type === 'text').map((p) => p.text).join(' ');
}

function plan(messages) {
  let last = messages.length - 1;
  while (last >= 0 && messages[last].role !== 'user') last--;
  const u = text(messages[last]?.content);
  const images = Array.isArray(messages[last]?.content) ? messages[last].content.filter((p) => p.type === 'image_url').length : 0;
  // Las consultas que hace el código antes del modelo ("pre-…": la placa y la ficha) no cuentan como tools pedidas.
  const results = messages.slice(last + 1).filter((m) => m.role === 'tool' && !String(m.tool_call_id).startsWith('pre-')).map((m) => m.content).join(' | ');
  const plate = u.match(/\b[A-Z]{3}[0-9]{2}[0-9A-Z]?\b/)?.[0];
  const call = (name, args) => ({ tool_calls: [{ id: `c-${name}`, type: 'function', function: { name, arguments: JSON.stringify(args) } }] });
  if (plate && !results.includes('encontrado')) return call('buscar_vehiculo', { placa: plate });
  if (results.includes('"encontrado":false') && !results.includes('propuesta')) {
    return call('crear_orden', {
      placa: plate, sintoma: 'le suenan los frenos', tipo: 'carro', celular: u.match(/3\d{9}/)?.[0] ?? '', nombre: 'Carlos Gómez',
    });
  }
  const km = u.match(/(\d{3,6})\s?km/)?.[1];
  if ((km || images > 0) && !results.includes('"tiene"')) {
    return call('proponer_recepcion', {
      ...(km ? { kilometraje: Number(km), combustible: '1/2' } : {}),
      lados: Array.from({ length: Math.min(images, 4) }, (_, i) => ({ foto: i + 1, lado: SIDES[i] })),
    });
  }
  return { content: 'Listo, te lo dejé en la ficha. Revísalo y toca Confirmar.' };
}

createServer((req, res) => {
  let body = '';
  req.on('data', (c) => (body += c));
  req.on('end', () => {
    if (req.method !== 'POST' || !req.url?.endsWith('/chat/completions')) {
      res.writeHead(404).end();
      return;
    }
    const r = JSON.parse(body || '{}');
    // FAKE_AI_LOG=archivo: deja qué recibió (rol, tipo de contenido y tools pedidas), sin las fotos, para depurar el e2e.
    if (process.env.FAKE_AI_LOG) {
      const resumen = (r.messages ?? []).map((m) => ({ rol: m.role, contenido: typeof m.content === 'string' ? m.content.slice(0, 80) : (m.content ?? []).map((p) => p.type) }));
      appendFileSync(process.env.FAKE_AI_LOG, JSON.stringify(resumen) + '\n');
    }
    const msg = plan(r.messages ?? []);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ model: 'falso', choices: [{ message: { content: msg.content ?? '', tool_calls: msg.tool_calls } }], usage: { prompt_tokens: 10, completion_tokens: 5, cost: 0 } }));
  });
}).listen(port);
