
import { useEffect } from 'react';
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useInspectionTaskStore } from '../stores/inspection-task';
import { useCertificateRecordStore } from '../stores/certificate-record';
export default function InspectionTaskPage() {
  const certificates = useCertificateRecordStore((state) => state.items);
  const loadCertificates = useCertificateRecordStore((state) => state.load);
  useEffect(() => { void loadCertificates('certificates'); }, [loadCertificates]);
  return <EntityPage config={ENTITY_CONFIGS[1]} useStore={useInspectionTaskStore} certificateRecords={certificates} />;
}
