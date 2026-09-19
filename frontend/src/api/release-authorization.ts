
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listReleaseAuthorization(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/authorizations?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createReleaseAuthorization(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/authorizations', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionReleaseAuthorization(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/authorizations/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
