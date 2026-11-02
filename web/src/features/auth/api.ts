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
