import { useState, type FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import Button from '@mui/material/Button';
import TextField from '@mui/material/TextField';
import { useAuth } from '../hooks/useAuth';

const accounts = [
  { username: 'admin', label: '管理员' },
  { username: 'reviewer', label: '复核员' },
  { username: 'operator', label: '操作员' },
  { username: 'viewer', label: '只读审计员' },
];

export default function LoginPage() {
  const { session, loading, signIn } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('Admin123!');
  const [error, setError] = useState('');

  if (session) return <Navigate to="/parts" replace />;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    try {
      await signIn(username.trim(), password);
      navigate('/parts', { replace: true });
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  return <main className="login-page">
    <section className="login-panel">
      <p className="eyebrow">AIRWORTHINESS CONTROL</p>
      <h1>航空部件适航放行</h1>
      <p className="login-subtitle">受控业务工作台</p>
      <form onSubmit={(event) => void submit(event)}>
        <TextField label="用户名" value={username} onChange={(event) => setUsername(event.target.value)} fullWidth required />
        <TextField label="密码" value={password} onChange={(event) => setPassword(event.target.value)} type="password" fullWidth required />
        {error && <div className="alert" role="alert">{error}</div>}
        <Button type="submit" variant="contained" size="large" disabled={loading} fullWidth>{loading ? '正在登录' : '登录'}</Button>
      </form>
      <div className="account-switcher" aria-label="演示角色">
        {accounts.map((account) => <button key={account.username} type="button" className={username === account.username ? 'active' : ''} onClick={() => setUsername(account.username)}>{account.label}</button>)}
      </div>
    </section>
  </main>;
}
