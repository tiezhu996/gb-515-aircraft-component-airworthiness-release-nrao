
import type { DomainRecord } from '../../types/domain';
import { StatusBadge } from './StatusBadge';
export function CertificatePanel({ records }: { records: DomainRecord[] }) {
	if (!records.length) return <div className="empty">暂无可展示的业务证据</div>;
	return <div className="evidence-strip">{records.slice(0, 4).map((item) => {
		const latest = item.revisions?.at(-1);
		return <article key={item.id}>
			<div className="certificate-title"><strong>{item.code}</strong><StatusBadge status={item.status} /></div>
			<span>{item.name}</span>
			<small>版本 v{item.version} · {latest?.actor || item.verifiedBy || item.preparedBy || item.owner}</small>
			<code title={latest?.requestId}>{latest?.requestId || '种子证据'}</code>
		</article>;
	})}</div>;
}
