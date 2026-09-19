import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { clearSession, readSession, saveSession } from '../api/client';
import { login } from '../api/auth';
import type { UserSession } from '../types/domain';

type Role = 'viewer' | 'operator' | 'reviewer' | 'admin';

interface AuthContextValue {
  session: UserSession | null;
  loading: boolean;
  signIn: (username: string, password: string) => Promise<void>;
  signOut: () => void;
  hasRole: (minimum: Role) => boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);
const roleRank: Record<Role, number> = { viewer: 1, operator: 2, reviewer: 3, admin: 4 };

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<UserSession | null>(() => readSession());
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const refresh = () => setSession(readSession());
    window.addEventListener('auth-session-changed', refresh);
    window.addEventListener('storage', refresh);
    return () => {
      window.removeEventListener('auth-session-changed', refresh);
      window.removeEventListener('storage', refresh);
    };
  }, []);

  const signIn = useCallback(async (username: string, password: string) => {
    setLoading(true);
    try {
      const next = await login(username, password);
      saveSession(next);
      setSession(next);
    } finally {
      setLoading(false);
    }
  }, []);

  const signOut = useCallback(() => {
    clearSession();
    setSession(null);
  }, []);

  const value = useMemo<AuthContextValue>(() => ({
    session,
    loading,
    signIn,
    signOut,
    hasRole: (minimum) => roleRank[(session?.role || 'viewer') as Role] >= roleRank[minimum],
  }), [loading, session, signIn, signOut]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used inside AuthProvider');
  return context;
}
