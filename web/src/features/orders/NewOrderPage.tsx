import { useQuery } from '@tanstack/react-query';
import { useState, type FormEvent, type ReactNode } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';

import { api, ApiError, unwrap } from '../../shared/api/client';
import { celularValido, formatoCelular, soloCelular } from '../../shared/lib/format';
import { ayudaPlaca, normalizarPlaca, reconocerPlaca, tipoPorPlaca } from '../../shared/lib/plate';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { IconBack, IconCheck, IconWarn } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { Segmented } from '../../shared/ui/Segmented';
import { useCustomerByPhone, useOpenOrder, useVehicleByPlate, type NewOrderInput } from './api';

function Card({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4 rounded-card bg-white p-5" aria-label={title}>
      <h2 className="text-[17px] font-extrabold">{title}</h2>
      {children}
    </section>
  );
}

type SiNo = 'si' | 'no' | '';

/** HU-16 con HU-09, HU-10 y HU-11: abrir una orden escribiendo la placa y lo que dice el cliente. */
export function NewOrderPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const primera = params.get('primera') === '1';

  const [placa, setPlaca] = useState('');
  const [confirmarEspecial, setConfirmarEspecial] = useState(false);
  const [tipoEspecial, setTipoEspecial] = useState<'carro' | 'moto' | ''>('');
  const [celular, setCelular] = useState('');
  const [nombre, setNombre] = useState('');
  const [correo, setCorreo] = useState('');
  const [marca, setMarca] = useState('');
  const [linea, setLinea] = useState('');
  const [anio, setAnio] = useState('');
  const [km, setKm] = useState('');
  const [cilindraje, setCilindraje] = useState('');
  const [turbo, setTurbo] = useState<SiNo>('');
  const [sintoma, setSintoma] = useState('');
  const [estado, setEstado] = useState<'por_recibir' | 'recibido'>('por_recibir');
  const [intentado, setIntentado] = useState(false);
  const [ejemplo, setEjemplo] = useState(false);
  const taller = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')), enabled: primera });

  // HU-05 criterio 2: sin vehículo a mano, una orden de ejemplo con placa de carro o de moto según lo que atiende.
  function usarEjemplo() {
    const moto = taller.data?.atiende === 'moto';
    setPlaca(moto ? 'EJM12A' : 'EJM123');
    setCelular('3000000000');
    setNombre('Cliente de ejemplo');
    setSintoma(moto ? 'Revisión general de la moto (ejemplo)' : 'Revisión general del carro (ejemplo)');
    setEjemplo(true);
  }

  const formato = reconocerPlaca(placa);
  const placaLista = formato !== 'vacia' && formato !== 'invalida';
  const vq = useVehicleByPlate(placa, placaLista);
  const existente = vq.data?.vehiculo ?? null;
  const buscandoVehiculo = placaLista && vq.isPending;
  const esNuevo = placaLista && !buscandoVehiculo && existente === null;

  const tipo = existente?.tipo ?? tipoPorPlaca(formato) ?? (tipoEspecial || null);
  const cq = useCustomerByPhone(soloCelular(celular), esNuevo && celularValido(celular));
  const clienteExistente = cq.data ?? null;

  const open = useOpenOrder();
  const err = open.error instanceof ApiError ? open.error : null;
  const duplicada = err?.code === 'orden_abierta_existente';

  // Turbo: en moto viene en No por defecto (HU-11, criterio 3).
  const turboEfectivo: SiNo = turbo !== '' ? turbo : tipo === 'moto' ? 'no' : '';
  const motorCompletoNuevo = cilindraje.trim() !== '' && turboEfectivo !== '';
  const motorPendiente = existente ? existente.motorPendiente && !motorCompletoNuevo : !motorCompletoNuevo;

  const faltantes: string[] = [];
  if (!placaLista) faltantes.push('la placa');
  if (formato === 'especial' && esNuevo && (!confirmarEspecial || tipoEspecial === '')) faltantes.push('confirmar la placa especial');
  if (esNuevo && !celularValido(celular)) faltantes.push('el celular del cliente');
  if (esNuevo && celularValido(celular) && clienteExistente === null && nombre.trim() === '') faltantes.push('el nombre del cliente');
  if (sintoma.trim() === '') faltantes.push('lo que dice el cliente');
  const missing = faltantes.length > 0 ? `Falta ${faltantes.join(', ')}.` : undefined;

  function enviar(confirmarDuplicada: boolean) {
    const motor = {
      ...(cilindraje.trim() !== '' ? { cilindraje: cilindraje.trim() } : {}),
      ...(turboEfectivo !== '' ? { turbo: turboEfectivo === 'si' } : {}),
    };
    const input: NewOrderInput = {
      placa: normalizarPlaca(placa),
      vehiculoExistente: existente,
      etag: vq.data?.etag ?? '',
      cliente: {
        celular: soloCelular(celular),
        nombre: clienteExistente?.nombre ?? nombre.trim(),
        ...(correo.trim() !== '' ? { correo: correo.trim() } : {}),
      },
      vehiculo: {
        ...(formato === 'especial' && tipoEspecial !== '' ? { tipo: tipoEspecial } : {}),
        confirmarPlacaEspecial: confirmarEspecial,
        ...(marca.trim() !== '' ? { marca: marca.trim() } : {}),
        ...(linea.trim() !== '' ? { linea: linea.trim() } : {}),
        ...(anio !== '' ? { modeloAnio: Number(anio) } : {}),
        ...(km !== '' ? { kilometraje: Number(km) } : {}),
        ...motor,
      },
      ...(existente?.motorPendiente && motorCompletoNuevo ? { motorParaExistente: motor } : {}),
      sintoma: sintoma.trim(),
      estadoInicial: estado === 'recibido' && motorPendiente ? 'por_recibir' : estado,
      confirmarDuplicada,
      ejemplo,
    };
    open.mutate(input, { onSuccess: (o) => navigate(`/panel/ordenes/${o.id}${primera ? '?primera=1' : ''}`, { replace: true }) });
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    setIntentado(true);
    if (missing === undefined) enviar(false);
  }

  const fieldErr = (f: string) => (err?.field === f ? err.message : undefined);

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-5">
      <header className="flex flex-col gap-1">
        <Link to="/panel/ordenes" className="inline-flex min-h-[44px] items-center gap-1 self-start font-bold text-primary">
          <IconBack size={18} />
          Órdenes
        </Link>
        <h1 className="text-[28px] font-extrabold tracking-[-.02em]">{primera ? 'Tu primera orden' : 'Nueva orden'}</h1>
        <p className="text-ink-2">Escribe la placa y lo que dice el cliente. La orden queda con número y hora desde ya.</p>
        {primera ? (
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <Button variant="secondary" onClick={usarEjemplo}>
              Usar ejemplo
            </Button>
            <span className="text-sm text-ink-2">{ejemplo ? 'Orden de ejemplo: no cuenta en tus números.' : '¿No tienes un vehículo a mano?'}</span>
          </div>
        ) : null}
      </header>

      <div className="grid gap-5 lg:grid-cols-2">
        <div className="flex flex-col gap-5">
          <Card title="Vehículo">
            <Field
              label="Placa"
              value={placa}
              onChange={(e) => {
                setPlaca(e.target.value.toUpperCase());
                setConfirmarEspecial(false);
              }}
              mono
              tone="plate"
              autoFocus
              autoCapitalize="characters"
              autoComplete="off"
              maxLength={9}
              placeholder="KLM482"
              className="max-w-[240px] text-2xl"
              hint={ayudaPlaca[formato]}
              error={intentado && !placaLista ? 'Escribe la placa del vehículo.' : fieldErr('placa')}
            />
            {buscandoVehiculo ? <p role="status" className="text-sm text-ink-2">Buscando la placa en tu taller…</p> : null}
            {existente ? (
              <div className="flex flex-col gap-1 rounded-2xl bg-primary-tint p-4">
                <span className="flex items-center gap-2 font-bold text-primary">
                  <IconCheck size={18} />
                  Ya está en tu taller
                </span>
                <span>
                  <Plate value={existente.placa} />{' '}
                  {[existente.marca, existente.linea, existente.modeloAnio].filter(Boolean).join(' ') ||
                    (existente.tipo === 'moto' ? 'Moto' : 'Carro')}
                </span>
              </div>
            ) : null}
            {esNuevo && formato === 'especial' ? (
              <div className="flex flex-col gap-3 rounded-2xl bg-warn-tint p-4 text-warn-ink">
                <label className="flex min-h-[44px] items-center gap-3 font-semibold">
                  <input
                    type="checkbox"
                    className="h-5 w-5 accent-primary"
                    checked={confirmarEspecial}
                    onChange={(e) => setConfirmarEspecial(e.target.checked)}
                  />
                  Confirmo que es una placa especial
                </label>
                <Segmented
                  label="¿Es carro o moto?"
                  value={tipoEspecial}
                  onChange={setTipoEspecial}
                  options={[
                    { value: 'carro', label: 'Carro' },
                    { value: 'moto', label: 'Moto' },
                  ]}
                />
              </div>
            ) : null}
            {esNuevo ? (
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Marca" value={marca} onChange={(e) => setMarca(e.target.value)} placeholder="Chevrolet" error={fieldErr('marca')} />
                <Field label="Línea" value={linea} onChange={(e) => setLinea(e.target.value)} placeholder="Spark GT" error={fieldErr('linea')} />
                <Field
                  label="Modelo (año)"
                  inputMode="numeric"
                  value={anio}
                  onChange={(e) => setAnio(e.target.value.replace(/\D/g, '').slice(0, 4))}
                  placeholder="2015"
                  error={fieldErr('modeloAnio')}
                />
                <Field
                  label="Kilometraje"
                  inputMode="numeric"
                  mono
                  value={km}
                  onChange={(e) => setKm(e.target.value.replace(/\D/g, '').slice(0, 7))}
                  placeholder="84500"
                  error={fieldErr('kilometraje')}
                />
              </div>
            ) : null}
          </Card>

          {esNuevo || existente?.motorPendiente ? (
            <section
              aria-label="Motor"
              className={`flex flex-col gap-4 rounded-card p-5 ${motorPendiente ? 'bg-warn-tint' : 'bg-white'}`}
            >
              <h2 className="flex items-center gap-2 text-[17px] font-extrabold">
                {motorPendiente ? (
                  <>
                    <IconWarn size={18} className="text-warn-ink" />
                    Motor pendiente
                  </>
                ) : (
                  <>
                    <IconCheck size={18} className="text-ok" />
                    Motor completo
                  </>
                )}
              </h2>
              <p className="text-sm text-ink-2">Puedes dejarlo para después; se pide antes de pasar la orden a Recibido.</p>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field
                  label="Cilindraje"
                  value={cilindraje}
                  onChange={(e) => setCilindraje(e.target.value)}
                  placeholder="1200 o 1.2 L"
                  hint="Se guarda siempre en cc."
                  error={fieldErr('cilindrajeCc')}
                />
                <Segmented
                  label="Turbo"
                  value={turboEfectivo}
                  onChange={(v) => setTurbo(v)}
                  options={[
                    { value: 'no', label: 'No' },
                    { value: 'si', label: 'Sí' },
                  ]}
                />
              </div>
            </section>
          ) : null}
        </div>

        <div className="flex flex-col gap-5">
          {esNuevo ? (
            <Card title="Cliente">
              <Field
                label="Celular del cliente"
                inputMode="tel"
                value={celular}
                onChange={(e) => setCelular(e.target.value)}
                placeholder="300 123 4567"
                error={
                  (intentado && !celularValido(celular) ? 'Escribe un celular colombiano de 10 dígitos que empiece por 3.' : undefined) ??
                  fieldErr('celular')
                }
              />
              {clienteExistente ? (
                <div className="flex items-center gap-2 rounded-2xl bg-primary-tint p-4 font-bold text-primary">
                  <IconCheck size={18} />
                  {clienteExistente.nombre} · {formatoCelular(clienteExistente.celular)} ya está registrado
                </div>
              ) : celularValido(celular) && !cq.isPending ? (
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field
                    label="Nombre"
                    autoComplete="off"
                    value={nombre}
                    onChange={(e) => setNombre(e.target.value)}
                    error={(intentado && nombre.trim() === '' ? 'Escribe el nombre del cliente.' : undefined) ?? fieldErr('nombre')}
                  />
                  <Field label="Correo (opcional)" type="email" value={correo} onChange={(e) => setCorreo(e.target.value)} error={fieldErr('correo')} />
                </div>
              ) : null}
              <p className="text-sm text-ink-2">
                Queda con autorización de datos <b>pendiente</b>: no le escribimos por WhatsApp hasta que la dé.
              </p>
            </Card>
          ) : null}

          <Card title="Orden">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="sintoma" className="text-[15px] font-extrabold">
                Lo que dice el cliente
              </label>
              <textarea
                id="sintoma"
                value={sintoma}
                onChange={(e) => setSintoma(e.target.value)}
                rows={4}
                maxLength={2000}
                placeholder="Suena al frenar y vibra el timón"
                aria-invalid={intentado && sintoma.trim() === '' ? true : undefined}
                className="rounded-btn border-[1.5px] border-line bg-white p-4 text-base outline-none focus:border-primary"
              />
              {intentado && sintoma.trim() === '' ? (
                <span role="alert" className="text-sm font-semibold text-err-ink">
                  Escribe lo que dice el cliente que tiene el vehículo.
                </span>
              ) : null}
            </div>
            <Segmented
              label="Estado inicial"
              value={motorPendiente ? 'por_recibir' : estado}
              onChange={setEstado}
              options={[
                { value: 'por_recibir', label: 'Por recibir' },
                { value: 'recibido', label: 'Recibido', disabled: motorPendiente },
              ]}
              hint={motorPendiente ? 'Para Recibido falta el cilindraje o el turbo del motor.' : undefined}
            />
          </Card>

          {duplicada ? (
            <Alert kind="warn" title="Este vehículo ya tiene una orden abierta">
              <p>¿Quieres abrir otra de todos modos?</p>
              <div className="mt-3">
                <Button variant="secondary" busy={open.isPending} onClick={() => enviar(true)}>
                  Abrir otra orden
                </Button>
              </div>
            </Alert>
          ) : err ? (
            <Alert>{err.message}</Alert>
          ) : open.error ? (
            <Alert>{open.error.message}</Alert>
          ) : null}

          <Button type="submit" busy={open.isPending} missing={intentado ? missing : undefined}>
            Abrir orden
          </Button>
        </div>
      </div>
    </form>
  );
}
