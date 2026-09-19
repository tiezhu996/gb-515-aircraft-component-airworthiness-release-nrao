
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listAircraftPart(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/parts?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createAircraftPart(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/parts', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionAircraftPart(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/parts/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
