import { statusTone } from '../../utils/format';

const labels: Record<string, string> = {
  received: '已接收', inspection: '检查中', hold: '暂停', released: '已放行', retired: '已退役',
  draft: '草稿', review: '待复核', approved: '已批准', restricted: '限制放行', revoked: '已撤销',
};

export function PartStatusBadge({ status }: { status: string }) {
  return <span className={`status part-status status--${statusTone(status)}`}>
    <span className="part-status__signal" aria-hidden="true" />
    {labels[status] || status.replaceAll('_', ' ')}
  </span>;
}
