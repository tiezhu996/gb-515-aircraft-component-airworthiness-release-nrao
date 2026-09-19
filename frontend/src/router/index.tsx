import { Navigate, createBrowserRouter } from 'react-router-dom';
import App from '../App';
import { useAuth } from '../hooks/useAuth';
import AircraftPartPage from '../pages/AircraftPartPage';
import InspectionTaskPage from '../pages/InspectionTaskPage';
import CertificateRecordPage from '../pages/CertificateRecordPage';
import ReleaseAuthorizationPage from '../pages/ReleaseAuthorizationPage';
import AuditPage from '../pages/AuditPage';
import LoginPage from '../pages/LoginPage';

function ProtectedRoute() {
  const { session, loading, hasRole } = useAuth();
  if (loading) return <div className="app-loading">正在校验会话…</div>;
  if (!session) return <Navigate to="/login" replace />;
  if (!hasRole('viewer')) return <Navigate to="/login" replace />;
  return <App />;
}

export const router = createBrowserRouter([
{ path: '/login', element: <LoginPage /> },
{ path: '/', element: <ProtectedRoute />, children: [
	{ index: true, element: <Navigate to="/parts" replace /> },
	{ path: 'parts', element: <AircraftPartPage /> }, { path: 'inspections', element: <InspectionTaskPage /> }, { path: 'certificates', element: <CertificateRecordPage /> }, { path: 'authorizations', element: <ReleaseAuthorizationPage /> },
	{ path: 'audit', element: <AuditPage /> },
] }], { future: { v7_fetcherPersist: true, v7_normalizeFormMethod: true, v7_partialHydration: true, v7_relativeSplatPath: true, v7_skipActionErrorRevalidation: true } });
