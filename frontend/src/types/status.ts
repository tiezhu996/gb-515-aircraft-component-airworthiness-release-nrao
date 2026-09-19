import type { EntityConfig } from './domain';

export type PartState = 'received' | 'inspection' | 'hold' | 'released' | 'retired';
export const ALL_PART_STATE: readonly PartState[] = ['received', 'inspection', 'hold', 'released', 'retired'];
export type AuthorizationState = 'draft' | 'review' | 'approved' | 'restricted' | 'revoked';
export const ALL_AUTHORIZATION_STATE: readonly AuthorizationState[] = ['draft', 'review', 'approved', 'restricted', 'revoked'];

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
	{ key: 'aircraftPart', path: 'parts', label: '航空部件', statuses: ['received', 'inspection', 'hold', 'released', 'retired'] as const, primaryTransitions: { received: 'inspection', inspection: 'hold', hold: 'released', released: 'retired' } },
	{ key: 'inspectionTask', path: 'inspections', label: '检查任务', statuses: ['planned', 'running', 'passed', 'failed'] as const, primaryTransitions: { planned: 'running', running: 'passed', failed: 'running' } },
	{ key: 'certificateRecord', path: 'certificates', label: '证书记录', statuses: ['draft', 'valid', 'expired', 'revoked'] as const, primaryTransitions: { draft: 'valid', valid: 'expired', expired: 'revoked' } },
	{ key: 'releaseAuthorization', path: 'authorizations', label: '放行授权', statuses: ['draft', 'review', 'approved', 'restricted', 'revoked'] as const, primaryTransitions: { draft: 'review', review: 'approved', approved: 'restricted', restricted: 'revoked' } }
];
