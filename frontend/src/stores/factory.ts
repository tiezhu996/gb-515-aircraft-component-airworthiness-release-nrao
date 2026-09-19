
import { create } from 'zustand';
import { ApiError, request } from '../api/client';
import type { ApiEnvelope, DomainRecord, PageMeta } from '../types/domain';

export interface EntityState {
  items: DomainRecord[];
  meta: PageMeta;
  loading: boolean;
  error: string;
  blockers: string[];
  load: (path: string, search?: string) => Promise<void>;
  createRecord: (path: string, input: Partial<DomainRecord>) => Promise<void>;
  transition: (path: string, item: DomainRecord, status: string) => Promise<void>;
}
export type EntityStore = ReturnType<typeof createEntityStore>;

function errorState(error: unknown) {
  const apiError = error instanceof ApiError ? error : null;
  return {
    error: error instanceof Error ? error.message : String(error),
    blockers: apiError?.blockers ?? [],
    loading: false,
  };
}

export function createEntityStore() {
  return create<EntityState>((set, get) => ({
    items: [], meta: { page: 1, pageSize: 20, total: 0 }, loading: false, error: '', blockers: [],
    load: async (path, search = '') => {
      set({ loading: true, error: '', blockers: [] });
      try {
        const result = await request<DomainRecord[]>(`/${path}?page=1&pageSize=20&search=${encodeURIComponent(search)}`);
        set({ items: result.data, meta: result.meta || { page: 1, pageSize: 20, total: result.data.length }, loading: false });
      } catch (error) { set(errorState(error)); }
    },
    createRecord: async (path, input) => {
      set({ loading: true, error: '', blockers: [] });
      try {
        await request<DomainRecord>(`/${path}`, { method: 'POST', body: JSON.stringify(input) });
        await get().load(path);
      } catch (error) { set(errorState(error)); throw error; }
    },
    transition: async (path, item, status) => {
      set({ loading: true, error: '', blockers: [] });
      try {
        await request<DomainRecord>(`/${path}/${item.id}/transition`, { method: 'POST', body: JSON.stringify({ status, expectedVersion: item.version, reason: '前端工作台人工确认' }) });
        await get().load(path);
      } catch (error) { set(errorState(error)); throw error; }
    },
  }));
}
