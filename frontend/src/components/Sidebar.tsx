import {
  Inbox,
  LogOut,
  Mail,
  Plus,
  RefreshCw,
  Settings2,
  X,
} from "lucide-react";
import type { Account } from "../types";
import { cn, statusLabel } from "../utils";
import { Brand } from "./Brand";

interface SidebarProps {
  accounts: Account[];
  selectedAccountID: string | null;
  totalEmails: number;
  open: boolean;
  syncing: boolean;
  onSelectAccount: (id: string | null) => void;
  onAddAccount: () => void;
  onManageAccounts: () => void;
  onRefresh: () => void;
  onLogout: () => void;
  onClose: () => void;
}

export function Sidebar({
  accounts,
  selectedAccountID,
  totalEmails,
  open,
  syncing,
  onSelectAccount,
  onAddAccount,
  onManageAccounts,
  onRefresh,
  onLogout,
  onClose,
}: SidebarProps) {
  const content = (
    <aside className="sidebar-glow relative flex h-full w-[264px] shrink-0 flex-col overflow-hidden bg-[#20211f] px-4 py-5 text-white">
      <div className="relative z-10 flex items-center justify-between px-2">
        <Brand />
        <button
          type="button"
          onClick={onClose}
          className="focus-ring flex size-9 items-center justify-center rounded-xl text-white/55 transition hover:bg-white/8 hover:text-white lg:hidden"
          aria-label="关闭导航"
        >
          <X className="size-[18px]" />
        </button>
      </div>

      <button
        type="button"
        onClick={onAddAccount}
        className="focus-ring relative z-10 mt-8 flex h-11 items-center justify-center gap-2 rounded-[13px] bg-[#f5bd48] px-4 text-sm font-semibold text-[#20211f] shadow-[0_9px_25px_rgba(0,0,0,0.18)] transition hover:bg-[#fac95f] active:scale-[0.98]"
      >
        <Plus className="size-[17px]" strokeWidth={2.4} />
        添加邮箱
      </button>

      <nav className="relative z-10 mt-8" aria-label="邮箱导航">
        <p className="px-3 text-[10px] font-semibold uppercase tracking-[0.19em] text-white/35">
          邮件
        </p>
        <button
          type="button"
          onClick={() => onSelectAccount(null)}
          className={cn(
            "focus-ring mt-2 flex h-11 w-full items-center gap-3 rounded-[13px] px-3 text-sm transition",
            selectedAccountID === null
              ? "bg-white/10 font-medium text-white shadow-[inset_0_0_0_1px_rgba(255,255,255,0.04)]"
              : "text-white/56 hover:bg-white/6 hover:text-white",
          )}
        >
          <Inbox className="size-[18px]" />
          <span className="flex-1 text-left">全部收件箱</span>
          {totalEmails > 0 && (
            <span className="rounded-full bg-white/9 px-2 py-0.5 text-[10px] font-semibold text-white/58">
              {totalEmails > 999 ? "999+" : totalEmails}
            </span>
          )}
        </button>
      </nav>

      <div className="relative z-10 mt-7 min-h-0 flex-1">
        <div className="flex items-center justify-between px-3">
          <p className="text-[10px] font-semibold uppercase tracking-[0.19em] text-white/35">
            已连接邮箱
          </p>
          <button
            type="button"
            onClick={onRefresh}
            className="focus-ring -mr-1 flex size-7 items-center justify-center rounded-lg text-white/35 transition hover:bg-white/7 hover:text-white"
            aria-label="刷新"
          >
            <RefreshCw className={cn("size-3.5", syncing && "animate-spin")} />
          </button>
        </div>

        <div className="mt-2 max-h-full space-y-1 overflow-y-auto pr-1">
          {accounts.map((account) => (
            <button
              type="button"
              key={account.id}
              onClick={() => onSelectAccount(account.id)}
              className={cn(
                "focus-ring flex w-full items-center gap-3 rounded-[13px] px-3 py-2.5 text-left transition",
                selectedAccountID === account.id
                  ? "bg-white/10 text-white"
                  : "text-white/55 hover:bg-white/6 hover:text-white",
              )}
            >
              <span className="relative flex size-8 shrink-0 items-center justify-center rounded-[10px] bg-white/8">
                <Mail className="size-4" />
                <span
                  className={cn(
                    "absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full border-2 border-[#20211f]",
                    account.monitor_status === "idle" ? "bg-[#5dc38d]" : "bg-[#e6aa46]",
                  )}
                />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[13px] font-medium">{account.name}</span>
                <span className="mt-0.5 block truncate text-[10px] text-white/34">
                  {account.monitor_status === "idle"
                    ? account.username
                    : statusLabel(account.monitor_status)}
                </span>
              </span>
            </button>
          ))}

          {accounts.length === 0 && (
            <div className="mx-2 mt-3 rounded-[13px] border border-dashed border-white/10 px-3 py-4 text-center text-[11px] leading-5 text-white/32">
              添加邮箱后，邮件会汇集在这里
            </div>
          )}
        </div>
      </div>

      <div className="relative z-10 mt-5 flex items-center gap-1">
        <button
          type="button"
          onClick={onManageAccounts}
          className="focus-ring flex h-10 min-w-0 flex-1 items-center gap-3 rounded-xl px-3 text-[13px] text-white/44 transition hover:bg-white/6 hover:text-white"
        >
          <Settings2 className="size-[17px]" />
          邮箱设置
        </button>
        <button
          type="button"
          onClick={onLogout}
          className="focus-ring flex size-10 shrink-0 items-center justify-center rounded-xl text-white/36 transition hover:bg-white/6 hover:text-white"
          aria-label="退出登录"
          title="退出登录"
        >
          <LogOut className="size-[16px]" />
        </button>
      </div>
      <div className="relative z-10 mt-3 px-3 text-[10px] tracking-wide text-white/20">
        PAPERWING · PRIVATE BY DESIGN
      </div>
    </aside>
  );

  return (
    <>
      <div className="hidden h-full lg:block">{content}</div>
      {open && (
        <div className="fixed inset-0 z-50 flex lg:hidden">
          <button
            type="button"
            className="absolute inset-0 bg-black/45 backdrop-blur-[2px] animate-fade-in"
            onClick={onClose}
            aria-label="关闭导航"
          />
          <div className="relative animate-modal-in">{content}</div>
        </div>
      )}
    </>
  );
}
