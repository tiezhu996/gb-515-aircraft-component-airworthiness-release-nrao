
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listInspectionTask(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/inspections?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createInspectionTask(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/inspections', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionInspectionTask(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/inspections/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
