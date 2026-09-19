
import { NavLink, Outlet } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
const navigation = [{ to: '/parts', label: '航空部件' }, { to: '/inspections', label: '检查任务' }, { to: '/certificates', label: '证书记录' }, { to: '/authorizations', label: '放行授权' }, { to: '/audit', label: '审计记录' }];
export default function App() {
	const { session, loading, signOut } = useAuth();
	if (loading) return <div className="app-loading">正在建立安全会话…</div>;
	return <div className="app-shell"><aside><div className="brand"><span>CONTROL DESK</span><strong>航空部件适航放行</strong></div><nav>{navigation.map((item) => <NavLink key={item.to} to={item.to}>{item.label}</NavLink>)}</nav><div className="user-panel"><span>{session?.displayName}</span><small>{session?.role}</small><button onClick={signOut}>退出会话</button></div></aside><section className="content"><header className="topbar"><span>适航控制台</span><span className="live-dot">服务已连接</span></header><Outlet /></section></div>;
}
