import { useEffect, useState } from "react";
import { AlertCircle, LoaderCircle, RefreshCw, ShieldCheck, Trash2 } from "lucide-react";
import type { Account } from "../types";
import { cn, formatFullDate, statusLabel } from "../utils";
import { Modal } from "./Modal";

interface AccountManagerProps {
  open: boolean;
  accounts: Account[];
  onClose: () => void;
  onSync: (account: Account) => Promise<void>;
  onDelete: (account: Account) => Promise<void>;
  onAdd: () => void;
}

export function AccountManager({ open, accounts, onClose, onSync, onDelete, onAdd }: AccountManagerProps) {
  const [busyID, setBusyID] = useState<string | null>(null);
  const [confirmDeleteID, setConfirmDeleteID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setConfirmDeleteID(null);
      setError(null);
    }
  }, [open]);

  const run = async (account: Account, action: "sync" | "delete") => {
    setBusyID(account.id);
    setError(null);
    try {
      if (action === "sync") await onSync(account);
      else await onDelete(account);
      setConfirmDeleteID(null);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "操作失败");
    } finally {
      setBusyID(null);
    }
  };

  return (
    <Modal open={open} onClose={onClose} eyebrow="Mail accounts" title="邮箱设置">
      <div className="px-5 py-6 sm:px-7">
        {accounts.length === 0 ? (
          <div className="rounded-[18px] border border-dashed border-[#dcd8cd] bg-[#f6f4ee] px-6 py-10 text-center">
            <p className="text-sm font-semibold text-[#4d4e49]">还没有连接邮箱</p>
            <p className="mt-2 text-[11px] leading-5 text-[#929189]">添加邮箱后就能在一个地方查看所有来信。</p>
            <button
              type="button"
              onClick={onAdd}
              className="focus-ring mt-5 rounded-xl bg-[#292a27] px-4 py-2.5 text-xs font-semibold text-white hover:bg-black"
            >
              添加第一个邮箱
            </button>
          </div>
        ) : (
          <div className="space-y-3">
            {accounts.map((account) => {
              const healthy = account.monitor_status === "idle";
              const busy = busyID === account.id;
              return (
                <div key={account.id} className="rounded-[17px] border border-[#e1ded5] bg-white p-4 shadow-[0_2px_8px_rgba(36,37,33,0.025)]">
                  <div className="flex items-start gap-3">
                    <span className="relative flex size-10 shrink-0 items-center justify-center rounded-[13px] bg-[#f1efe9]">
                      <ShieldCheck className={cn("size-[18px]", healthy ? "text-[#4d8c68]" : "text-[#bd8531]")} />
                      <span className={cn("absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full border-2 border-white", healthy ? "bg-[#5dbd88]" : "bg-[#e6ad4e]")} />
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <h3 className="truncate text-[13px] font-semibold text-[#3b3c38]">{account.name}</h3>
                        {!healthy && (
                          <span className="shrink-0 rounded-full bg-[#f7edd9] px-2 py-0.5 text-[9px] font-medium text-[#976e25]">
                            {statusLabel(account.monitor_status)}
                          </span>
                        )}
                      </div>
                      <p className="mt-1 truncate text-[10px] text-[#8f8e87]">{account.username}</p>
                      <p className="mt-1 truncate text-[9px] text-[#aaa9a2]">
                        {account.host}:{account.port} · {account.tls ? "TLS" : "未加密"}
                      </p>
                    </div>
                  </div>

                  {account.latest_connection_error && (
                    <div className="mt-3 flex gap-2 rounded-xl bg-[#f9ece8] px-3 py-2 text-[10px] leading-4 text-[#a65b4d]">
                      <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
                      <span>{account.latest_connection_error}</span>
                    </div>
                  )}

                  <div className="mt-3 flex items-center justify-between border-t border-[#f0eee8] pt-3">
                    <span className="text-[9px] text-[#aaa9a2]">
                      {account.last_successful_sync_at
                        ? `上次同步 ${formatFullDate(account.last_successful_sync_at)}`
                        : "尚未完成首次同步"}
                    </span>
                    <div className="ml-3 flex shrink-0 items-center gap-1">
                      <button
                        type="button"
                        onClick={() => run(account, "sync")}
                        disabled={Boolean(busyID)}
                        className="focus-ring flex size-8 items-center justify-center rounded-lg text-[#73736d] transition hover:bg-[#f0eee8] hover:text-[#353632] disabled:opacity-40"
                        aria-label={`同步 ${account.name}`}
                      >
                        {busy ? <LoaderCircle className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
                      </button>
                      {confirmDeleteID === account.id ? (
                        <div className="flex items-center gap-1 animate-fade-in">
                          <button
                            type="button"
                            onClick={() => setConfirmDeleteID(null)}
                            className="focus-ring rounded-lg px-2 py-1.5 text-[9px] text-[#777870] hover:bg-[#f0eee8]"
                          >
                            取消
                          </button>
                          <button
                            type="button"
                            onClick={() => run(account, "delete")}
                            disabled={Boolean(busyID)}
                            className="focus-ring rounded-lg bg-[#b95748] px-2.5 py-1.5 text-[9px] font-semibold text-white hover:bg-[#a44739] disabled:opacity-50"
                          >
                            确认删除
                          </button>
                        </div>
                      ) : (
                        <button
                          type="button"
                          onClick={() => setConfirmDeleteID(account.id)}
                          disabled={Boolean(busyID)}
                          className="focus-ring flex size-8 items-center justify-center rounded-lg text-[#9b817b] transition hover:bg-[#f8eae6] hover:text-[#af5143] disabled:opacity-40"
                          aria-label={`删除 ${account.name}`}
                        >
                          <Trash2 className="size-3.5" />
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {error && <div className="mt-4 rounded-xl bg-[#f8ebe7] px-3 py-2.5 text-[10px] text-[#a65345]">{error}</div>}

        {accounts.length > 0 && (
          <button
            type="button"
            onClick={onAdd}
            className="focus-ring mt-5 h-10 w-full rounded-xl border border-dashed border-[#d5d1c6] text-[11px] font-semibold text-[#777770] transition hover:border-[#bfa761] hover:bg-[#f7f3e8] hover:text-[#6f561c]"
          >
            ＋ 添加另一个邮箱
          </button>
        )}
      </div>
    </Modal>
  );
}
