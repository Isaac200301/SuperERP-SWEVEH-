import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api, ApiError, unwrap, type Sesion } from '../../shared/api/client';
import { soloCelular } from '../../shared/lib/format';

export const sessionKey = ['sesion'] as const;

/** Sesión actual. null = no hay sesión (401). */
export function useSession() {
  return useQuery({
    queryKey: sessionKey,
    queryFn: async (): Promise<Sesion | null> => {
      try {
        return unwrap(await api.GET('/me'));
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) return null;
        throw e;
      }
    },
    staleTime: 60_000,
    retry: false,
  });
}

/** ¿La sesión tiene el permiso? Solo para ocultar botones: el servidor siempre verifica (R-05). */
export function can(session: Sesion | null | undefined, permiso: string): boolean {
  return session?.permisos?.includes(permiso) ?? false;
}

/** Funciones de la entrega del 27-nov (V-01, V-02): lo apagado en el servidor no se muestra ni se monta. */
export type Funcion = 'whatsapp' | 'plataforma';

/** ¿Está encendida la función en este servidor? Sin sesión o sin el dato, apagada. */
export function feature(session: Sesion | null | undefined, f: Funcion): boolean {
  return session?.funciones?.[f] === true;
}

export function useFeature(f: Funcion): boolean {
  return feature(useSession().data, f);
}

export function usePermission(permiso: string): boolean {
  return can(useSession().data, permiso);
}

export function useRequestCode() {
  return useMutation({
    mutationFn: async (celular: string) => unwrap(await api.POST('/auth/otp', { body: { celular: soloCelular(celular) } })),
  });
}

export type VerifyVars = { celular: string; codigo: string; nombre?: string; autorizacionDatos?: boolean };

export function useVerifyCode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: VerifyVars) =>
      unwrap(await api.POST('/auth/otp/verify', { body: { ...v, celular: soloCelular(v.celular) } })),
    onSuccess: (s) => qc.setQueryData(sessionKey, s),
  });
}

// Entrada con PIN (HU-104, ADR-0037). El código por WhatsApp (useRequestCode / useVerifyCode) queda para AUTH_MODE=otp.

export type RegistroVars = { celular: string; nombreTaller: string; pin: string; autorizacion: { version: string; aceptada: boolean } };

export function useRegisterPIN() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: RegistroVars) => unwrap(await api.POST('/auth/register', { body: { ...v, celular: soloCelular(v.celular) } })),
    onSuccess: (s) => qc.setQueryData(sessionKey, s),
  });
}

export function useLoginPIN() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { celular: string; pin: string }) => unwrap(await api.POST('/auth/login', { body: { ...v, celular: soloCelular(v.celular) } })),
    onSuccess: (s) => qc.setQueryData(sessionKey, s),
  });
}

/** Cambiar el PIN propio. Con un PIN temporal (cambiarPin) no se pide el actual. */
export function useChangePIN() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { pinActual?: string; pin: string }) => {
      const r = await api.POST('/auth/pin', { body: v });
      if (!r.response.ok) throw new ApiError(r.response.status, r.error as never);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: sessionKey }),
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await api.POST('/auth/logout');
    },
    onSettled: () => {
      qc.clear();
      qc.setQueryData(sessionKey, null);
    },
  });
}
