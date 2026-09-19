
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useAircraftPartStore } from '../stores/aircraft-part';
export default function AircraftPartPage() { return <EntityPage config={ENTITY_CONFIGS[0]} useStore={useAircraftPartStore} />; }
