import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, ApiError, unwrap, type Schemas } from '../../shared/api/client';

export type Repuesto = Schemas['Repuesto'];
export type Categoria = Schemas['Categoria'];
export type Importacion = {
  filas: { fila: number; referencia: string; nombre: string; estado: 'ok' | 'error'; errores: string[] }[];
  conError: number;
  importadas: number;
  avisos: string[];
};

// Mismas claves que inventory/domain.Categories; coinciden con los sistemas del diagnóstico.
export const categorias: Record<Categoria, string> = {
  motor: 'Motor', frenos: 'Frenos', suspension: 'Suspensión', direccion: 'Dirección', electrico: 'Eléctrico', transmision: 'Caja y transmisión',
  refrigeracion: 'Refrigeración', llantas: 'Llantas', carroceria: 'Carrocería', filtros: 'Filtros', lubricantes: 'Lubricantes', otro: 'Otro',
};

export const motivos: Record<string, string> = {
  entrada: 'Entrada de mercancía', salida_orden: 'Salida por orden', pendiente_orden: 'Pendiente (sin existencia)', devolucion: 'Devolución',
  ajuste: 'Ajuste', importacion: 'Importación',
};

const key = ['repuestos'] as const;

export type Filtros = { q: string; categoria: Categoria | ''; bajo: boolean; inactivos: boolean };

/** HU-34 y HU-36: busca por referencia, nombre, marca o referencia original; "bajo" usa /parts/low. */
export function useParts(f: Filtros) {
  return useQuery({
    queryKey: [...key, f],
    queryFn: async () => {
      if (f.bajo) return unwrap(await api.GET('/parts/low')).repuestos;
      const query = { ...(f.q.trim() ? { q: f.q.trim() } : {}), ...(f.categoria ? { categoria: f.categoria } : {}), ...(f.inactivos ? { todos: '1' as const } : {}) };
      return unwrap(await api.GET('/parts', { params: { query } })).repuestos;
    },
  });
}

export function useMovements(id: string | null) {
  return useQuery({
    queryKey: ['movimientos', id],
    enabled: id !== null,
    queryFn: async () => unwrap(await api.GET('/parts/{id}/movements', { params: { path: { id: id ?? '' } } })).movimientos,
  });
}

type NuevoRepuesto = {
  referencia: string; nombre: string; tipoVehiculo: string; costo: number; precioVenta: number; existencia: number; minimo: number;
  marca: string; categoria: string; ubicacion: string; referenciaOem: string;
};
export type CambioRepuesto = Partial<Omit<NuevoRepuesto, 'referencia' | 'existencia'>> & { activo?: boolean; ofrecerWhatsapp?: boolean; version: number };

/** Foto del repuesto para la vitrina de WhatsApp (ADR-0029). v cambia la URL cuando se sube otra. */
export function fotoRepuesto(id: string, v: number) {
  return `/api/v1/parts/${id}/photo?v=${v}`;
}

export function usePartActions() {
  const qc = useQueryClient();
  const done = () => {
    void qc.invalidateQueries({ queryKey: key });
    void qc.invalidateQueries({ queryKey: ['movimientos'] });
  };
  return {
    create: useMutation({ mutationFn: async (b: NuevoRepuesto) => unwrap(await api.POST('/parts', { body: b })), onSuccess: done }),
    update: useMutation({
      mutationFn: async (v: { id: string; body: CambioRepuesto }) => unwrap(await api.PATCH('/parts/{id}', { params: { path: { id: v.id } }, body: v.body })),
      onSettled: done,
    }),
    photo: useMutation({
      mutationFn: async (v: { id: string; foto: File }) =>
        unwrap(
          await api.PUT('/parts/{id}/photo', {
            params: { path: { id: v.id } },
            body: { foto: '' },
            bodySerializer: () => {
              const fd = new FormData();
              fd.append('foto', v.foto);
              return fd;
            },
          }),
        ),
      onSettled: done,
    }),
    entry: useMutation({
      mutationFn: async (v: { id: string; cantidad: number }) =>
        unwrap(await api.POST('/parts/{id}/entries', { params: { path: { id: v.id } }, body: { cantidad: v.cantidad } })),
      onSettled: done,
    }),
  };
}

/**
 * HU-37: sube el CSV. Sin confirmar es la vista previa. Con errores el servidor responde 422 con las mismas filas,
 * así que se devuelve el cuerpo en vez de lanzar el error.
 */
export function useImport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { archivo: File; confirmar: boolean }): Promise<Importacion> => {
      const r = await api.POST('/parts/import', {
        params: { query: { confirmar: v.confirmar ? '1' : '0' } },
        body: { archivo: '' },
        bodySerializer: () => {
          const fd = new FormData();
          fd.append('archivo', v.archivo);
          return fd;
        },
      });
      if (r.data) return r.data as Importacion;
      if (r.response.status === 422 && r.error && 'filas' in (r.error as object)) return r.error as unknown as Importacion;
      throw new ApiError(r.response.status, r.error as never);
    },
    onSuccess: (res) => {
      if (res.importadas > 0) void qc.invalidateQueries({ queryKey: key });
    },
  });
}

// Encabezados y valores como los muestra la pantalla; el servidor también acepta las claves internas (domain.columnAliases).
export const plantillaCSV =
  'Referencia;Nombre;Marca;Categoría;Para;Ubicación;Referencia original (OEM);Precio de venta (COP);Costo (COP);Existencia;Mínimo\n' +
  'PF-20;Pastillas de freno delanteras;Bosch;Frenos;Carro;Estante A3;0986494;65000;40000;4;2\n';
