import { useCallback, useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './style.css';

type Session = {
  id: string; repo: string; title: string; agent: string; status: string;
  connected: boolean | null; writable: boolean | null;
  lastMessage: string; lastActivityAt: number | null;
};
type Snapshot = { sessions: Session[]; meta: { partial: boolean; demo: boolean; fetchedAt: number } };
const statuses: Record<string, { label: string; note: string }> = {
  running: { label: 'Running', note: 'エージェント実行中' },
  done: { label: 'Done', note: '作業完了' },
  interrupted: { label: 'Interrupted', note: '処理が中断されました' },
  offline: { label: 'Offline', note: 'ターミナル未接続' },
  active: { label: 'Active workspace', note: '作業中のワークスペース' },
  unknown: { label: 'Unknown', note: '状態を確認できません' },
};
const errors: Record<string, string> = {
  orca_unavailable: 'Orcaに接続できません。MacでOrcaとCLIを確認してください。',
  timeout: '状態の取得がタイムアウトしました。',
  invalid_response: 'Orcaからの応答を読み取れませんでした。',
  upstream_error: 'Orcaから状態を取得できませんでした。',
};
function clock(at: number | null) {
  return at == null ? '時刻不明' : new Date(at).toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit' });
}

function App() {
  const [data, setData] = useState<Snapshot | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [showUnknown, setShowUnknown] = useState(false);
  const active = useRef<AbortController | null>(null);
  const mounted = useRef(true);

  const refresh = useCallback(async () => {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setLoading(true);
    const timeout = window.setTimeout(() => controller.abort(), 12000);
    try {
      const response = await fetch('/api/sessions', { signal: controller.signal, cache: 'no-store' });
      const body = await response.json();
      if (!response.ok) throw new Error(errors[body.error?.code] || '状態を取得できませんでした。');
      if (!Array.isArray(body.sessions) || typeof body.meta?.fetchedAt !== 'number') throw new Error('応答の形式を確認できませんでした。');
      if (mounted.current) { setData(body); setError(''); }
    } catch (cause) {
      if (mounted.current) setError(controller.signal.aborted ? '通信がタイムアウトしました。' : cause instanceof Error && cause.message !== 'Failed to fetch' ? cause.message : 'Gatewayに接続できません。');
    } finally {
      window.clearTimeout(timeout);
      active.current = null;
      if (mounted.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    void refresh();
    const timer = window.setInterval(() => { if (!document.hidden) void refresh(); }, 5000);
    const visible = () => { if (!document.hidden) void refresh(); };
    document.addEventListener('visibilitychange', visible);
    return () => { mounted.current = false; window.clearInterval(timer); document.removeEventListener('visibilitychange', visible); active.current?.abort(); };
  }, [refresh]);

  const sessions = data?.sessions.filter(s => showUnknown || s.agent !== 'unknown') ?? [];
  const hidden = data?.sessions.filter(s => s.agent === 'unknown').length ?? 0;
  return <div className="deck">
    <header>
      <div className="brand"><span className="brand-mark" aria-hidden="true">▥</span><div><h1>Agent Deck</h1><p>ORCA SESSION MONITOR</p></div></div>
      <div className="connection"><span className={`signal ${error ? 'signal-error' : data ? 'signal-live' : ''}`} />{data?.meta.demo ? 'DEMO' : error ? 'DISCONNECTED' : data ? 'CONNECTED' : 'CONNECTING'}</div>
    </header>

    <main>
      <div className="section-heading"><h2>Sessions <span>{sessions.length.toString().padStart(2, '0')}</span></h2><span className="read-only">READ ONLY</span></div>
      {error && <div role="alert" className="notice error">{error}{data && <strong> 情報が古い可能性があります · 最終取得 {clock(data.meta.fetchedAt)}</strong>}</div>}
      {data?.meta.partial && <div role="status" className="notice">一部のホストまたはセッションを取得できていません。</div>}
      {loading && !data && !error && <div role="status" className="empty"><span className="loader" />セッションを取得しています…</div>}
      {!loading && !error && sessions.length === 0 && <div className="empty"><span className="empty-icon" aria-hidden="true">◇</span><h3>表示するエージェントはありません</h3><p>{hidden ? '不明なTerminalは下の切り替えで確認できます。' : 'Orcaでエージェントを起動すると、ここに表示されます。'}</p></div>}
      <div className={`session-grid ${error ? 'stale' : ''}`}>
        {sessions.map(s => {
          const state = statuses[s.status] ?? statuses.unknown;
          const style = statuses[s.status] ? s.status : 'unknown';
          return <article className={`session ${style}`} key={s.id} aria-label={`${s.title} ${state.label}`}>
            <div className="card-top"><span className="agent">{s.agent === 'unknown' ? 'TERMINAL' : s.agent.toUpperCase()}</span><span className="status"><span className="status-dot" />{state.label}</span></div>
            <h3 title={s.title}>{s.title}</h3>
            <p className="message">{s.lastMessage || state.note}</p>
            <div className="card-bottom"><span className="repo">{s.repo || 'ワークスペース不明'}</span><time>{clock(s.lastActivityAt)}</time></div>
          </article>;
        })}
      </div>
    </main>

    <footer>
      <label className="unknown-toggle"><input type="checkbox" checked={showUnknown} onChange={event => setShowUnknown(event.target.checked)} />不明なTerminal{hidden > 0 && <span>({hidden})</span>}</label>
      <span className="updated">{data ? `取得 ${clock(data.meta.fetchedAt)}` : '5秒ごとに自動更新'}</span>
      <button onClick={() => void refresh()} disabled={loading}><span aria-hidden="true" className={loading ? 'spin' : ''}>↻</span>{loading ? '更新中' : 'Refresh'}</button>
    </footer>
  </div>;
}

createRoot(document.getElementById('root')!).render(<App />);
