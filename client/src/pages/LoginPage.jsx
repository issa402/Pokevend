// ============================================================
// PokémonTool — Login & Register Page
// Shown when user is not authenticated.
// ============================================================

import { useState } from 'react';
import { useDispatch } from 'react-redux';
import { useNavigate } from 'react-router-dom';
import { setCredentials } from '../store/index.js';
import { Zap, Mail, Lock, User, Eye, EyeOff } from 'lucide-react';
import api from '../services/api.js';

export default function LoginPage() {
  const dispatch  = useDispatch();
  const navigate  = useNavigate();
  const [mode,   setMode]  = useState('login');  // 'login' | 'register'
  const [loading, setLoading] = useState(false);
  const [error,   setError]   = useState('');
  const [showPass, setShowPass] = useState(false);
  const [form, setForm] = useState({ email: '', password: '', displayName: '' });

  function handleChange(e) {
    setForm(f => ({ ...f, [e.target.name]: e.target.value }));
    setError('');
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const endpoint = mode === 'login' ? '/auth/login' : '/auth/register';
      const { data } = await api.post(endpoint, form);
      dispatch(setCredentials({ user: data.user, token: data.token }));
      navigate('/');
    } catch (err) {
      setError(err.response?.data?.error || 'Something went wrong. Please try again.');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: 24,
      background: `
        radial-gradient(ellipse at 20% 30%, rgba(79,142,247,0.12) 0%, transparent 60%),
        radial-gradient(ellipse at 80% 70%, rgba(124,58,237,0.12) 0%, transparent 60%),
        var(--color-bg-base)
      `,
    }}>
      <div className="glass-card" style={{ width: '100%', maxWidth: 420, padding: 40 }}>
        {/* Logo */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 32 }}>
          <div style={{
            width: 48, height: 48,
            background: 'var(--gradient-primary)',
            borderRadius: 14,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: 'var(--shadow-glow)',
          }}>
            <Zap size={24} color="#fff" />
          </div>
          <div>
            <div style={{ fontWeight: 800, fontSize: '1.25rem' }}>PokémonTool</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>Vendor Intelligence Platform</div>
          </div>
        </div>

        <h2 style={{ fontSize: '1.25rem', fontWeight: 700, marginBottom: 6 }}>
          {mode === 'login' ? 'Welcome back 👋' : 'Create your account'}
        </h2>
        <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.875rem', marginBottom: 28 }}>
          {mode === 'login'
            ? 'Sign in to access your vendor dashboard'
            : 'Start monitoring the Pokémon card market'}
        </p>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {mode === 'register' && (
            <div className="form-group" style={{ marginBottom: 0 }}>
              <label className="label">Display Name</label>
              <div style={{ position: 'relative' }}>
                <User size={15} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: 'var(--color-text-muted)' }} />
                <input className="input" name="displayName" placeholder="Your vendor name" value={form.displayName} onChange={handleChange} style={{ paddingLeft: 36 }} />
              </div>
            </div>
          )}

          <div className="form-group" style={{ marginBottom: 0 }}>
            <label className="label">Email Address</label>
            <div style={{ position: 'relative' }}>
              <Mail size={15} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: 'var(--color-text-muted)' }} />
              <input className="input" name="email" type="email" placeholder="you@example.com" value={form.email} onChange={handleChange} required style={{ paddingLeft: 36 }} />
            </div>
          </div>

          <div className="form-group" style={{ marginBottom: 0 }}>
            <label className="label">Password</label>
            <div style={{ position: 'relative' }}>
              <Lock size={15} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: 'var(--color-text-muted)' }} />
              <input className="input" name="password" type={showPass ? 'text' : 'password'} placeholder={mode === 'register' ? 'At least 8 characters' : '••••••••'} value={form.password} onChange={handleChange} required style={{ paddingLeft: 36, paddingRight: 42 }} />
              <button type="button" onClick={() => setShowPass(s => !s)} style={{ position: 'absolute', right: 12, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', color: 'var(--color-text-muted)', cursor: 'pointer', padding: 4 }}>
                {showPass ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
            </div>
          </div>

          {error && (
            <div style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: 'var(--radius-md)', padding: '10px 14px', fontSize: '0.8125rem', color: 'var(--color-accent-falling)' }}>
              {error}
            </div>
          )}

          <button className="btn btn-primary" type="submit" disabled={loading} style={{ width: '100%', justifyContent: 'center', height: 44, marginTop: 8 }}>
            {loading ? 'Please wait...' : mode === 'login' ? 'Sign In' : 'Create Account'}
          </button>
        </form>

        <p style={{ textAlign: 'center', marginTop: 24, fontSize: '0.875rem', color: 'var(--color-text-secondary)' }}>
          {mode === 'login' ? "Don't have an account? " : 'Already have an account? '}
          <button onClick={() => setMode(m => m === 'login' ? 'register' : 'login')} style={{ background: 'none', border: 'none', color: 'var(--color-accent-primary)', fontWeight: 600, cursor: 'pointer' }}>
            {mode === 'login' ? 'Sign up free' : 'Sign in'}
          </button>
        </p>
      </div>
    </div>
  );
}
