import { useEffect, useState } from "react";
import {
  AlertCircle,
  Check,
  Copy,
  KeyRound,
  LoaderCircle,
  Plus,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import { api } from "../api";
import type { APIToken, APITokenScope, IssuedAPIToken } from "../types";
import { cn, formatFullDate } from "../utils";
import { Modal } from "./Modal";

interface APITokenManagerProps {
  open: boolean;
  onClose: () => void;
}

const SCOPE_OPTIONS: Array<{ value: APITokenScope; label: string; description: string }> = [
  { value: "mail:read", label: "读取邮件", description: "查看邮件列表、正文和附件" },
  { value: "accounts:read", label: "查看邮箱", description: "查看已连接邮箱及同步状态" },
  { value: "accounts:write", label: "添加邮箱", description: "测试和添加新的邮箱" },
  { value: "sync:write", label: "触发同步", description: "请求邮箱立即同步" },
];

const SCOPE_LABELS = Object.fromEntries(
  SCOPE_OPTIONS.map((scope) => [scope.value, scope.label]),
) as Record<APITokenScope, string>;

export function APITokenManager({ open, onClose }: APITokenManagerProps) {
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [revokingID, setRevokingID] = useState<string | null>(null);
  const [confirmRevokeID, setConfirmRevokeID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("Default");
  const [scopes, setScopes] = useState<APITokenScope[]>(["mail:read", "accounts:read"]);
  const [expiresInDays, setExpiresInDays] = useState(365);
  const [issued, setIssued] = useState<IssuedAPIToken | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!open) {
      setIssued(null);
      setCopied(false);
      setConfirmRevokeID(null);
      setError(null);
      return;
    }
    setLoading(true);
    setError(null);
    void api
      .listAPITokens()
      .then(setTokens)
      .catch((caught) => setError(caught instanceof Error ? caught.message : "无法加载 API Token"))
      .finally(() => setLoading(false));
  }, [open]);

  const toggleScope = (scope: APITokenScope) => {
    setScopes((current) =>
      current.includes(scope) ? current.filter((item) => item !== scope) : [...current, scope],
    );
  };

  const create = async () => {
    if (!name.trim()) {
      setError("请填写 Token 名称");
      return;
    }
    if (scopes.length === 0) {
      setError("请至少选择一项权限");
      return;
    }
    setCreating(true);
    setError(null);
    try {
      const token = await api.createAPIToken(name.trim(), scopes, expiresInDays);
      setIssued(token);
      setTokens((current) => [
        {
          id: token.id,
          name: token.name,
          token_prefix: token.token_prefix,
          scopes: token.scopes,
          created_at: token.created_at,
          last_used_at: token.last_used_at,
          expires_at: token.expires_at,
        },
        ...current,
      ]);
      setCopied(false);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法创建 API Token");
    } finally {
      setCreating(false);
    }
  };

  const copyToken = async () => {
    if (!issued) return;
    try {
      await navigator.clipboard.writeText(issued.token);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2400);
    } catch {
      setError("复制失败，请手动选择并复制 Token");
    }
  };

  const revoke = async (token: APIToken) => {
    setRevokingID(token.id);
    setError(null);
    try {
      await api.revokeAPIToken(token.id);
      setTokens((current) => current.filter((item) => item.id !== token.id));
      if (issued?.id === token.id) setIssued(null);
      setConfirmRevokeID(null);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法撤销 API Token");
    } finally {
      setRevokingID(null);
    }
  };

  return (
    <Modal open={open} onClose={onClose} eyebrow="Personal access" title="API Token">
      <div className="space-y-6 px-5 py-6 sm:px-7">
        {issued && (
          <section className="rounded-[17px] border border-[#d9c488] bg-[#fbf4df] p-4 animate-fade-in">
            <div className="flex gap-3">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-[#ebd28f]/45 text-[#805d16]">
                <KeyRound className="size-4" />
              </span>
              <div className="min-w-0 flex-1">
                <h3 className="text-xs font-semibold text-[#4c3c18]">立即保存这个 Token</h3>
                <p className="mt-1 text-[10px] leading-4 text-[#816f45]">出于安全原因，关闭窗口后无法再次查看完整内容。</p>
              </div>
            </div>
            <div className="mt-3 flex gap-2">
              <input
                readOnly
                value={issued.token}
                onFocus={(event) => event.currentTarget.select()}
                className="min-w-0 flex-1 rounded-xl border border-[#ddc98e] bg-white px-3 py-2.5 font-mono text-[11px] text-[#4b4026] outline-none"
                aria-label="新 API Token"
              />
              <button
                type="button"
                onClick={() => void copyToken()}
                className="focus-ring flex size-10 shrink-0 items-center justify-center rounded-xl bg-[#2a2b28] text-white transition hover:bg-black"
                aria-label="复制 Token"
              >
                {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
              </button>
            </div>
          </section>
        )}

        <section>
          <div className="flex items-center gap-2">
            <Plus className="size-4 text-[#a97720]" />
            <h3 className="text-xs font-semibold text-[#3d3e39]">创建 Token</h3>
          </div>
          <label className="mt-4 block">
            <span className="text-[10px] font-medium text-[#777770]">名称</span>
            <input
              value={name}
              onChange={(event) => setName(event.target.value)}
              maxLength={64}
              className="focus-ring mt-1.5 h-10 w-full rounded-xl border border-[#dedbd2] bg-white px-3 text-xs text-[#343531] outline-none"
              placeholder="例如：家庭助理"
            />
          </label>

          <fieldset className="mt-4">
            <legend className="text-[10px] font-medium text-[#777770]">权限</legend>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              {SCOPE_OPTIONS.map((scope) => {
                const selected = scopes.includes(scope.value);
                return (
                  <label
                    key={scope.value}
                    className={cn(
                      "flex cursor-pointer gap-2.5 rounded-xl border p-3 transition",
                      selected ? "border-[#d7b765] bg-[#fbf5e5]" : "border-[#e3e0d7] bg-white hover:border-[#cec9bc]",
                    )}
                  >
                    <input
                      type="checkbox"
                      checked={selected}
                      onChange={() => toggleScope(scope.value)}
                      className="mt-0.5 accent-[#b17c1e]"
                    />
                    <span>
                      <span className="block text-[11px] font-semibold text-[#484944]">{scope.label}</span>
                      <span className="mt-0.5 block text-[9px] leading-4 text-[#929189]">{scope.description}</span>
                    </span>
                  </label>
                );
              })}
            </div>
          </fieldset>

          <label className="mt-4 block">
            <span className="text-[10px] font-medium text-[#777770]">有效期</span>
            <select
              value={expiresInDays}
              onChange={(event) => setExpiresInDays(Number(event.target.value))}
              className="focus-ring mt-1.5 h-10 w-full rounded-xl border border-[#dedbd2] bg-white px-3 text-xs text-[#343531] outline-none"
            >
              <option value={30}>30 天</option>
              <option value={90}>90 天</option>
              <option value={365}>1 年</option>
              <option value={1095}>3 年</option>
              <option value={0}>长期</option>
            </select>
          </label>

          <button
            type="button"
            onClick={() => void create()}
            disabled={creating}
            className="focus-ring mt-4 flex h-10 w-full items-center justify-center gap-2 rounded-xl bg-[#292a27] text-xs font-semibold text-white transition hover:bg-black disabled:opacity-50"
          >
            {creating && <LoaderCircle className="size-4 animate-spin" />}
            创建 API Token
          </button>
        </section>

        <section className="border-t border-[#e8e5dd] pt-5">
          <div className="flex items-center gap-2">
            <ShieldCheck className="size-4 text-[#56866a]" />
            <h3 className="text-xs font-semibold text-[#3d3e39]">现有 Token</h3>
          </div>
          {loading ? (
            <div className="flex items-center justify-center py-8 text-[#999890]">
              <LoaderCircle className="size-5 animate-spin" />
            </div>
          ) : tokens.length === 0 ? (
            <p className="mt-3 rounded-xl bg-[#f3f1eb] px-4 py-5 text-center text-[10px] text-[#929189]">还没有 API Token</p>
          ) : (
            <div className="mt-3 space-y-2">
              {tokens.map((token) => {
                const expired = token.expires_at
                  ? new Date(token.expires_at).getTime() <= Date.now()
                  : false;
                return (
                  <div key={token.id} className="rounded-[15px] border border-[#e2dfd6] bg-white p-3.5">
                    <div className="flex items-start gap-3">
                      <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-xl", expired ? "bg-[#f2e8e5] text-[#a66356]" : "bg-[#edf3ee] text-[#56866a]")}>
                        <KeyRound className="size-4" />
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <h4 className="truncate text-[11px] font-semibold text-[#444540]">{token.name}</h4>
                          {expired && <span className="rounded-full bg-[#f4e7e3] px-1.5 py-0.5 text-[8px] text-[#9d594d]">已过期</span>}
                        </div>
                        <p className="mt-1 font-mono text-[9px] text-[#9b9a92]">{token.token_prefix}…</p>
                        <div className="mt-2 flex flex-wrap gap-1">
                          {token.scopes.map((scope) => (
                            <span key={scope} className="rounded-md bg-[#f1efe9] px-1.5 py-1 text-[8px] text-[#73736d]">
                              {SCOPE_LABELS[scope]}
                            </span>
                          ))}
                        </div>
                        <p className="mt-2 text-[8px] leading-4 text-[#aaa9a2]">
                          {token.expires_at ? `到期：${formatFullDate(token.expires_at)}` : "长期"}
                          {token.last_used_at ? ` · 最近使用：${formatFullDate(token.last_used_at)}` : " · 尚未使用"}
                        </p>
                      </div>
                      {confirmRevokeID === token.id ? (
                        <div className="flex shrink-0 flex-col gap-1">
                          <button
                            type="button"
                            onClick={() => void revoke(token)}
                            disabled={revokingID === token.id}
                            className="focus-ring rounded-lg bg-[#b95748] px-2 py-1.5 text-[8px] font-semibold text-white disabled:opacity-50"
                          >
                            {revokingID === token.id ? "撤销中" : "确认"}
                          </button>
                          <button
                            type="button"
                            onClick={() => setConfirmRevokeID(null)}
                            className="focus-ring rounded-lg px-2 py-1 text-[8px] text-[#888780] hover:bg-[#f0eee8]"
                          >
                            取消
                          </button>
                        </div>
                      ) : (
                        <button
                          type="button"
                          onClick={() => setConfirmRevokeID(token.id)}
                          className="focus-ring flex size-8 shrink-0 items-center justify-center rounded-lg text-[#a27b73] transition hover:bg-[#f7e9e5] hover:text-[#af5143]"
                          aria-label={`撤销 ${token.name}`}
                        >
                          <Trash2 className="size-3.5" />
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </section>

        {error && (
          <div className="flex gap-2 rounded-xl bg-[#f8ebe7] px-3 py-2.5 text-[10px] leading-4 text-[#a65345]">
            <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
            {error}
          </div>
        )}
      </div>
    </Modal>
  );
}
