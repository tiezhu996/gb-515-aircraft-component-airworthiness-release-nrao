
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listCertificateRecord(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/certificates?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createCertificateRecord(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/certificates', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionCertificateRecord(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/certificates/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
