import { useState, useEffect, useRef } from 'react';
import {
  fetchModels, createModel, updateModel, setDefaultModel,
  type ModelConfig,
} from '../../lib/api';
import { useWSStore } from '../../lib/store';
import {
  IconSearch, IconPlus, IconRefresh, IconDownload, IconUpload,
  IconCpu, IconEdit, IconMoreHorizontal, IconTrash,
  IconEye, IconEyeOff, IconArrowLeft, IconChevronDown, IconX, IconCheck,
} from '../icons';

/* ── helpers ── */

function isValidURL(raw: string): boolean {
  if (!raw.trim()) return false;
  try { const u = new URL(raw.trim()); return u.protocol === 'http:' || u.protocol === 'https:'; }
  catch { return false; }
}

/* ── shared styles ── */

const inputShared =
  'flex h-10 w-full rounded-lg border bg-[var(--color-bg-tertiary)] px-3 py-2 text-sm ' +
  'text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] ' +
  'outline-none transition-colors ' +
  'border-[var(--color-border)] hover:border-[var(--color-text-muted)] ' +
  'focus-visible:border-[var(--color-accent)] focus-visible:ring-1 focus-visible:ring-[var(--color-accent)] ' +
  'disabled:cursor-not-allowed disabled:opacity-50';

const inputErr =
  'flex h-10 w-full rounded-lg border bg-[var(--color-bg-tertiary)] px-3 py-2 text-sm ' +
  'text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] ' +
  'outline-none transition-colors ' +
  'border-[var(--color-error)] focus-visible:border-[var(--color-error)] focus-visible:ring-1 focus-visible:ring-[var(--color-error)] ' +
  'disabled:cursor-not-allowed disabled:opacity-50';

const btnBase =
  'inline-flex items-center justify-center rounded-lg text-sm font-medium ' +
  'transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--color-accent)] ' +
  'disabled:opacity-40 disabled:pointer-events-none';

const btnGhost =
  `${btnBase} h-9 w-9 border border-[var(--color-border)] bg-[var(--color-bg-tertiary)] ` +
  'text-[var(--color-text-secondary)] hover:brightness-110';

const btnPrimary =
  `${btnBase} h-9 px-4 gap-1.5 text-white ` +
  'bg-[var(--color-accent)] hover:brightness-110';

const btnIcon =
  'inline-flex h-8 w-8 items-center justify-center rounded-lg transition-colors ' +
  'text-[var(--color-text-muted)] opacity-50 hover:opacity-100 hover:bg-[var(--color-bg-tertiary)]';

const sectionTitle =
  'text-[11px] font-semibold uppercase tracking-wider text-[var(--color-text-muted)]';

const fieldLabel =
  'block text-sm font-medium text-[var(--color-text-primary)]';

const fieldHint =
  'text-[13px] text-[var(--color-text-muted)]';

/* ──── AddModelModal ──── */

function AddModelModal({ onClose, onAdded }: { onClose: () => void; onAdded: () => void }) {
  const [name, setName] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [apiFormat, setApiFormat] = useState('openai');
  const [apiKey, setApiKey] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<{ name?: string; baseUrl?: string }>({});

  const validate = () => {
    const e: typeof errors = {};
    if (!name.trim()) e.name = '请输入模型名称';
    if (!baseUrl.trim()) { e.baseUrl = '请输入 Base URL'; }
    else if (!isValidURL(baseUrl)) { e.baseUrl = '请输入合法的 HTTP/HTTPS 地址'; }
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const submit = async (ev: React.FormEvent) => {
    ev.preventDefault();
    if (!validate()) return;
    setSaving(true);
    try {
      await createModel({ name: name.trim(), base_url: baseUrl.trim(), api_format: apiFormat, api_key: apiKey.trim() || undefined });
      onAdded();
    } catch (err) { setErrors({ baseUrl: err instanceof Error ? err.message : String(err) }); }
    finally { setSaving(false); }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" onClick={onClose}>
      <form onSubmit={submit}
        className="w-full max-w-md rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-6 space-y-6 shadow-2xl"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold text-[var(--color-text-primary)]">添加新模型</h2>
          <button type="button" onClick={onClose}
            className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-[var(--color-text-muted)] opacity-50 hover:opacity-80 hover:bg-[var(--color-bg-tertiary)] transition-colors">
            <IconX size={16} />
          </button>
        </div>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <label className={fieldLabel}>模型名称</label>
            <input type="text" value={name}
              onChange={e => { setName(e.target.value); if(errors.name) setErrors(p=>({...p,name:undefined})); }}
              placeholder="例如 gpt-4o, deepseek-v3"
              className={errors.name ? inputErr : inputShared}
            />
            {errors.name && <p className={fieldHint} style={{color:'var(--color-error)'}}>{errors.name}</p>}
          </div>

          <div className="space-y-1.5">
            <label className={fieldLabel}>Base URL</label>
            <input type="text" value={baseUrl}
              onChange={e => { setBaseUrl(e.target.value); if(errors.baseUrl) setErrors(p=>({...p,baseUrl:undefined})); }}
              placeholder="https://api.openai.com/v1"
              className={errors.baseUrl ? inputErr : inputShared}
            />
            {errors.baseUrl && <p className={fieldHint} style={{color:'var(--color-error)'}}>{errors.baseUrl}</p>}
          </div>

          <div className="space-y-1.5">
            <label className={fieldLabel}>API 格式</label>
            <select value={apiFormat} onChange={e => setApiFormat(e.target.value)}
              className={`${inputShared} cursor-pointer appearance-none`}>
              <option value="openai">OpenAI Compatible</option>
              <option value="anthropic">Anthropic Messages</option>
            </select>
          </div>

          <div className="space-y-1.5">
            <label className={fieldLabel}>API Key</label>
            <div className="relative">
              <input type={showKey?'text':'password'} value={apiKey}
                onChange={e => setApiKey(e.target.value)} placeholder="sk-..."
                className={`${inputShared} pr-10`}
              />
              <button type="button" onClick={()=>setShowKey(!showKey)} tabIndex={-1}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-[var(--color-text-muted)] opacity-40 hover:opacity-70 transition-opacity">
                {showKey ? <IconEyeOff size={16} /> : <IconEye size={16} />}
              </button>
            </div>
          </div>
        </div>

        <button type="submit" disabled={saving}
          className={`${btnPrimary} w-full h-10`}>
          {saving ? '保存中...' : '添加模型'}
        </button>
      </form>
    </div>
  );
}

/* ──── EditModelPage ──── */

function EditModelPage({ model, onBack, onSaved }: { model: ModelConfig; onBack: () => void; onSaved: () => void }) {
  const [name, setName] = useState(model.name);
  const [baseUrl, setBaseUrl] = useState(model.base_url);
  const [apiFormat, setApiFormat] = useState(model.api_format || 'openai');
  const [apiKey, setApiKey] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<{ name?: string; baseUrl?: string }>({});
  const [advOpen, setAdvOpen] = useState(false);

  const save = async () => {
    const e: typeof errors = {};
    if (!name.trim()) e.name = '请输入模型名称';
    if (!baseUrl.trim()) { e.baseUrl = '请输入 Base URL'; }
    else if (!isValidURL(baseUrl)) { e.baseUrl = '请输入合法的 HTTP/HTTPS 地址'; }
    setErrors(e);
    if (Object.keys(e).length) return;
    setSaving(true);
    try {
      await updateModel(model.id, { name: name.trim(), base_url: baseUrl.trim(), api_format: apiFormat, api_key: apiKey.trim() || undefined });
      onSaved();
    } catch (err) { setErrors({ baseUrl: err instanceof Error ? err.message : String(err) }); }
    finally { setSaving(false); }
  };

  return (
    <div className="flex-1 flex flex-col items-center overflow-y-auto py-8 px-6">
      <div className="w-full max-w-[640px] space-y-6">

        {/* header */}
        <div className="flex items-center gap-3">
          <button onClick={onBack} className={btnGhost}>
            <IconArrowLeft size={18} />
          </button>
          <div>
            <h2 className="text-base font-semibold text-[var(--color-text-primary)]">编辑模型</h2>
            <p className={fieldHint}>{model.name}</p>
          </div>
        </div>

        {/* form card */}
        <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] divide-y divide-[var(--color-border)]">
          {/* 基本信息 */}
          <div className="p-6 space-y-4">
            <h3 className={sectionTitle}>基本信息</h3>
            <div className="space-y-1.5">
              <label className={fieldLabel}>模型名称</label>
              <input type="text" value={name}
                onChange={e => { setName(e.target.value); if(errors.name) setErrors(p=>({...p,name:undefined})); }}
                className={errors.name ? inputErr : inputShared}
              />
              {errors.name && <p className={fieldHint} style={{color:'var(--color-error)'}}>{errors.name}</p>}
            </div>
            <div className="space-y-1.5">
              <label className={fieldLabel}>Base URL</label>
              <input type="text" value={baseUrl}
                onChange={e => { setBaseUrl(e.target.value); if(errors.baseUrl) setErrors(p=>({...p,baseUrl:undefined})); }}
                className={errors.baseUrl ? inputErr : inputShared}
              />
              {errors.baseUrl && <p className={fieldHint} style={{color:'var(--color-error)'}}>{errors.baseUrl}</p>}
            </div>
          </div>

          {/* API 密钥 */}
          <div className="p-6 space-y-4">
            <h3 className={sectionTitle}>API 密钥</h3>
            <div className="space-y-1.5">
              <label className={fieldLabel}>API Key</label>
              <div className="relative">
                <input type={showKey?'text':'password'} value={apiKey}
                  onChange={e => setApiKey(e.target.value)}
                  placeholder="输入新 Key 或留空保持不变"
                  className={`${inputShared} pr-10`}
                />
                <button type="button" onClick={()=>setShowKey(!showKey)} tabIndex={-1}
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-[var(--color-text-muted)] opacity-40 hover:opacity-70 transition-opacity">
                  {showKey ? <IconEyeOff size={16} /> : <IconEye size={16} />}
                </button>
              </div>
              <p className={fieldHint}>留空则保持原有密钥不变</p>
            </div>
          </div>

          {/* 高级选项 */}
          <div className="p-6 space-y-4">
            <button type="button" onClick={()=>setAdvOpen(!advOpen)}
              className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wider text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] transition-colors">
              <IconChevronDown size={14} className={`transition-transform duration-200 ${advOpen?'':' -rotate-90'}`} />
              高级选项
            </button>
            {advOpen && (
              <div className="space-y-1.5">
                <label className={fieldLabel}>API 格式</label>
                <select value={apiFormat} onChange={e => setApiFormat(e.target.value)}
                  className={`${inputShared} cursor-pointer appearance-none`}>
                  <option value="openai">OpenAI Compatible</option>
                  <option value="anthropic">Anthropic Messages</option>
                </select>
              </div>
            )}
          </div>
        </div>

        {/* save */}
        <div className="flex justify-end">
          <button onClick={save} disabled={saving} className={btnPrimary}>
            {saving ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}

/* ──── MoreMenu ──── */

function MoreMenu({ model, onToggle, onSetDefault, onDelete }: {
  model: ModelConfig;
  onToggle: () => void;
  onSetDefault: () => void;
  onDelete: () => void;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const h = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); };
    document.addEventListener('mousedown', h);
    return () => document.removeEventListener('mousedown', h);
  }, [open]);

  const itemCls =
    'flex w-full items-center gap-3 px-3 py-2 text-sm transition-colors ' +
    'text-[var(--color-text-primary)] hover:bg-[var(--color-bg-tertiary)]';

  return (
    <div className="relative" ref={ref}>
      <button onClick={() => setOpen(!open)} className={btnIcon}>
        <IconMoreHorizontal size={16} />
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-1 w-44 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] shadow-lg z-30 py-1 overflow-hidden">
          {!model.is_default && (
            <button onClick={()=>{onSetDefault(); setOpen(false);}} className={itemCls}>
              <IconCheck size={14} className="text-[var(--color-text-muted)]" />
              设为默认
            </button>
          )}
          <button onClick={()=>{onToggle(); setOpen(false);}} className={itemCls}>
            <span className="flex h-3 w-3 items-center justify-center rounded-full border-2 flex-shrink-0"
              style={{
                borderColor: model.enabled ? 'var(--color-success)' : 'var(--color-text-muted)',
                background: model.enabled ? 'var(--color-success)' : 'transparent'
              }} />
            {model.enabled ? '禁用' : '启用'}
          </button>
          <div className="h-px bg-[var(--color-border)] my-1" />
          <button onClick={()=>{onDelete(); setOpen(false);}}
            className={`${itemCls} text-[var(--color-error)] hover:bg-[var(--color-error-muted)]`}>
            <IconTrash size={14} />
            删除
          </button>
        </div>
      )}
    </div>
  );
}

/* ──── Main SettingsView ──── */

export default function SettingsView() {
  const [models, setModels]       = useState<ModelConfig[]>([]);
  const [loading, setLoading]     = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [editing, setEditing]     = useState<ModelConfig | null>(null);
  const [showAdd, setShowAdd]     = useState(false);
  const [q, setQ]                 = useState('');
  const wsState = useWSStore(s => s.state);

  useEffect(() => { load(); }, []);

  async function load() {
    setLoading(true); setLoadError(null);
    try { const d = await fetchModels(); setModels(Array.isArray(d)?d:[]); }
    catch (err: any) { setLoadError(err?.message || String(err) || '加载失败'); }
    finally { setLoading(false); }
  }

  async function toggle(m: ModelConfig) {
    try { await updateModel(m.id, { enabled: !m.enabled }); setModels(prev => prev.map(x => x.id===m.id ? {...x, enabled:!x.enabled} : x)); }
    catch (err) { console.error(err); }
  }

  async function del(m: ModelConfig) {
    try { await updateModel(m.id, { enabled: false }); setModels(prev => prev.map(x => x.id===m.id ? {...x, enabled:false} : x)); }
    catch (err) { console.error(err); }
  }

  async function doSetDefault(id: string) {
    try { await setDefaultModel(id); setModels(prev => prev.map(x => ({...x, is_default: x.id===id}))); }
    catch (err) { console.error(err); }
  }

  function exportJSON() {
    const data = models.map(({id, api_format, ...r}) => r);
    const blob = new Blob([JSON.stringify(data,null,2)], {type:'application/json'});
    const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = 'models.json'; a.click();
  }

  function importJSON() {
    const inp = document.createElement('input'); inp.type='file'; inp.accept='.json';
    inp.onchange = async e => {
      const f = (e.target as HTMLInputElement).files?.[0]; if (!f) return;
      try {
        const items = JSON.parse(await f.text());
        if (!Array.isArray(items)) throw new Error('需要 JSON 数组');
        let n = 0;
        for (const it of items) { if (it.name && it.base_url) { try { await createModel({name:it.name, base_url:it.base_url, api_format:it.api_format||'openai', api_key:it.api_key||undefined}); n++; } catch {} } }
        if (n>0) load();
        alert(`导入完成：${n}/${items.length}`);
      } catch (err: any) { alert('导入失败：'+ (err?.message||String(err))); }
    };
    inp.click();
  }

  const filtered = models.filter(m => {
    if (!q.trim()) return true;
    const s = q.toLowerCase();
    return m.name.toLowerCase().includes(s) || m.base_url.toLowerCase().includes(s);
  });

  /* ── WS not connected ── */
  if (wsState !== 'connected') {
    return (
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="text-center space-y-2">
          <p className="text-base font-medium text-[var(--color-text-primary)]">连接中...</p>
          <p className={fieldHint}>
            {wsState==='connecting'||wsState==='reconnecting'?'正在建立连接':'WebSocket 未连接'}
          </p>
        </div>
      </div>
    );
  }

  /* ── edit page ── */
  if (editing) {
    return <EditModelPage model={editing} onBack={() => setEditing(null)} onSaved={() => { setEditing(null); load(); }} />;
  }

  /* ── main list page ── */
  return (
    <div className="flex-1 flex flex-col overflow-y-auto py-8 px-6">
      <div className="w-full max-w-[960px] mx-auto space-y-6">

        {/* ── page header ── */}
        <div className="space-y-1">
          <h1 className="text-lg font-semibold tracking-tight text-[var(--color-text-primary)]">模型配置</h1>
          <p className={fieldHint}>管理 AI 模型接入与配置</p>
        </div>

        {/* ── toolbar ── */}
        <div className="flex items-center gap-3">
          <div className="relative flex-1 max-w-[320px]">
            <IconSearch size={15} className="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none text-[var(--color-text-muted)]" />
            <input type="text" value={q} onChange={e => setQ(e.target.value)}
              placeholder="搜索模型"
              className={`${inputShared} pl-9 pr-8`}
            />
            {q && (
              <button onClick={() => setQ('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-0.5 text-[var(--color-text-muted)] opacity-40 hover:opacity-70 transition-opacity">
                <IconX size={13} />
              </button>
            )}
          </div>
          <div className="flex items-center gap-2 ml-auto">
            <button onClick={load} title="刷新" className={btnGhost}>
              <IconRefresh size={16} />
            </button>
            <button onClick={importJSON} title="导入" className={btnGhost}>
              <IconUpload size={16} />
            </button>
            <button onClick={exportJSON} title="导出" disabled={models.length===0} className={btnGhost}>
              <IconDownload size={16} />
            </button>
            <button onClick={() => setShowAdd(true)} className={btnPrimary}>
              <IconPlus size={16} />
              添加模型
            </button>
          </div>
        </div>

        {/* ── states ── */}

        {loading && (
          <div className="flex items-center justify-center gap-3 py-20">
            <div className="h-4 w-4 animate-spin rounded-full border-2 border-[var(--color-border)]" style={{borderTopColor:'var(--color-accent)'}} />
            <span className="text-sm text-[var(--color-text-muted)]">加载中...</span>
          </div>
        )}

        {!loading && loadError && (
          <div className="flex items-center justify-between rounded-lg border border-[var(--color-error)] px-4 py-3 text-sm" style={{background:'rgba(239,68,68,0.06)', color:'var(--color-error)'}}>
            <span>加载失败: {loadError}</span>
            <button onClick={load} className="ml-4 font-medium underline underline-offset-2 hover:no-underline">重试</button>
          </div>
        )}

        {!loading && !loadError && models.length === 0 && (
          <div className="flex flex-col items-center justify-center rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] py-16 space-y-3">
            <IconCpu size={32} className="text-[var(--color-text-muted)] opacity-20" />
            <p className="text-sm font-medium text-[var(--color-text-muted)]">还未配置任何模型</p>
            <p className={fieldHint}>点击「添加模型」开始配置</p>
          </div>
        )}

        {!loading && q && filtered.length === 0 && models.length > 0 && (
          <div className="py-16 text-center">
            <p className="text-sm text-[var(--color-text-muted)]">未找到匹配「{q}」的模型</p>
          </div>
        )}

        {/* ── model cards ── */}
        {!loading && filtered.length > 0 && (
          <div className="space-y-3">
            {filtered.map(m => (
              <div key={m.id}
                className="group flex items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-5 py-3.5 transition-colors duration-150 hover:bg-[var(--color-bg-hover)]"
              >
                {/* left: icon + name + URL */}
                <div className="flex items-center gap-3 min-w-0 flex-1">
                  <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg"
                    style={{
                      background: m.enabled ? 'var(--color-accent-muted)' : 'var(--color-bg-tertiary)',
                      color: m.enabled ? 'var(--color-accent)' : 'var(--color-text-muted)',
                    }}>
                    <IconCpu size={16} />
                  </div>
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium truncate text-[var(--color-text-primary)]">{m.name}</span>
                      {m.api_format && m.api_format !== 'openai' && (
                        <span className="inline-flex flex-shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide"
                          style={{background:'var(--color-accent-muted)', color:'var(--color-accent)'}}>
                          {m.api_format}
                        </span>
                      )}
                    </div>
                    <p className="text-xs truncate mt-0.5 text-[var(--color-text-muted)]">{m.base_url}</p>
                  </div>
                </div>

                {/* middle: status badges */}
                <div className="flex items-center gap-3 flex-shrink-0">
                  <span className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-normal"
                    style={{
                      background: m.enabled ? 'var(--color-success-muted)' : 'rgba(255,255,255,0.04)',
                      color: m.enabled ? 'var(--color-success)' : 'var(--color-text-muted)',
                    }}>
                    {m.enabled ? '已启用' : '已禁用'}
                  </span>
                  {m.is_default && (
                    <span className="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold leading-normal"
                      style={{background:'var(--color-success-muted)', color:'var(--color-success)'}}>
                      默认
                    </span>
                  )}
                </div>

                {/* right: actions */}
                <div className="flex items-center gap-2 flex-shrink-0">
                  {!m.is_default && m.enabled && (
                    <button onClick={() => doSetDefault(m.id)}
                      className="text-[11px] font-medium px-2.5 py-1 rounded-md transition-colors"
                      style={{color:'var(--color-text-muted)', background:'var(--color-bg-tertiary)'}}>
                      设为默认
                    </button>
                  )}
                  <button onClick={() => setEditing(m)} title="编辑" className={btnIcon}>
                    <IconEdit size={14} />
                  </button>
                  <MoreMenu model={m} onToggle={() => toggle(m)} onSetDefault={() => doSetDefault(m.id)} onDelete={() => del(m)} />
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {showAdd && <AddModelModal onClose={() => setShowAdd(false)} onAdded={() => { setShowAdd(false); load(); }} />}
    </div>
  );
}