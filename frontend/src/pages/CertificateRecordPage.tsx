
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useCertificateRecordStore } from '../stores/certificate-record';
export default function CertificateRecordPage() {
  const certificates = useCertificateRecordStore((state) => state.items);
  return <EntityPage config={ENTITY_CONFIGS[2]} useStore={useCertificateRecordStore} certificateRecords={certificates} />;
}
