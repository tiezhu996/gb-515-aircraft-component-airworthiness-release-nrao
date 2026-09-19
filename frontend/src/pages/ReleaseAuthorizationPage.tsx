
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useReleaseAuthorizationStore } from '../stores/release-authorization';
export default function ReleaseAuthorizationPage() { return <EntityPage config={ENTITY_CONFIGS[3]} useStore={useReleaseAuthorizationStore} />; }
