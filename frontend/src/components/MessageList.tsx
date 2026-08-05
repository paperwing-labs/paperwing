import { useState } from "react";
import {
  ChevronLeft,
  ChevronRight,
  Inbox,
  LoaderCircle,
  Menu,
  Paperclip,
  Search,
  SlidersHorizontal,
} from "lucide-react";
import type { Account, EmailSummary } from "../types";
import {
  cn,
  displayName,
  formatBytes,
  formatRelativeDate,
} from "../utils";
import { Avatar } from "./Avatar";

interface MessageListProps {
  emails: EmailSummary[];
  accounts: Account[];
  selectedAccountID: string | null;
  selectedEmailID: string | null;
  total: number;
  page: number;
  pageSize: number;
  loading: boolean;
  error: string | null;
  search: string;
  onSearchChange: (value: string) => void;
  onSelectEmail: (email: EmailSummary) => void;
  onPageChange: (page: number) => void;
  onOpenMenu: () => void;
  onRetry: () => void;
}

function MessageSkeleton() {
  return (
    <div className="border-b border-[#e5e3dc] px-5 py-4">
      <div className="flex gap-3">
        <div className="size-10 shrink-0 animate-pulse rounded-[13px] bg-[#e4e2da]" />
        <div className="min-w-0 flex-1">
          <div className="flex justify-between gap-4">
            <div className="h-3.5 w-28 animate-pulse rounded bg-[#dfddd5]" />
            <div className="h-3 w-12 animate-pulse rounded bg-[#e7e5de]" />
          </div>
          <div className="mt-3 h-3 w-4/5 animate-pulse rounded bg-[#e3e1d9]" />
          <div className="mt-2 h-3 w-full animate-pulse rounded bg-[#e9e7e0]" />
        </div>
      </div>
    </div>
  );
}

export function MessageList({
  emails,
  accounts,
  selectedAccountID,
  selectedEmailID,
  total,
  page,
  pageSize,
  loading,
  error,
  search,
  onSearchChange,
  onSelectEmail,
  onPageChange,
  onOpenMenu,
  onRetry,
}: MessageListProps) {
  const [attachmentsOnly, setAttachmentsOnly] = useState(false);
  const selectedAccount = accounts.find((account) => account.id === selectedAccountID);
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const visibleEmails = emails.filter((email) => {
    if (attachmentsOnly && email.attachment_count === 0) return false;
    if (!search.trim()) return true;
    const haystack = [email.subject, ...email.from, ...email.to].join(" ").toLowerCase();
    return haystack.includes(search.trim().toLowerCase());
  });

  return (
    <section className="flex h-full w-full min-w-0 flex-col border-r border-[#dcdad2] bg-[#f8f7f3] md:w-[390px] md:shrink-0 xl:w-[420px]">
      <header className="border-b border-[#e1dfd8] px-4 pb-4 pt-5 sm:px-5">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2.5">
            <button
              type="button"
              onClick={onOpenMenu}
              className="focus-ring flex size-9 shrink-0 items-center justify-center rounded-xl text-[#64655f] transition hover:bg-[#e9e7e0] lg:hidden"
              aria-label="打开导航"
            >
              <Menu className="size-[19px]" />
            </button>
            <div className="min-w-0">
              <h1 className="truncate text-[21px] font-semibold tracking-[-0.035em] text-[#20211f]">
                {selectedAccount?.name || "全部收件箱"}
              </h1>
              <p className="mt-0.5 text-[11px] text-[#8a8a83]">
                {total > 0 ? `${total} 封邮件` : "所有来信，一处查看"}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setAttachmentsOnly((active) => !active)}
            className={cn(
              "focus-ring relative flex size-9 shrink-0 items-center justify-center rounded-xl border shadow-sm transition",
              attachmentsOnly
                ? "border-[#d3ad5d] bg-[#f8edcf] text-[#795719]"
                : "border-[#dddbd3] bg-white text-[#73736d] hover:border-[#cbc8bd] hover:text-[#20211f]",
            )}
            aria-label="只看有附件的邮件"
            aria-pressed={attachmentsOnly}
            title="只看有附件的邮件"
          >
            <SlidersHorizontal className="size-[16px]" />
            {attachmentsOnly && <span className="absolute right-1 top-1 size-1.5 rounded-full bg-[#d89e28]" />}
          </button>
        </div>

        <label className="mt-4 flex h-10 items-center gap-2.5 rounded-[13px] border border-transparent bg-[#ebe9e2] px-3 text-[#7d7d76] transition-within focus-within:border-[#d5aa52] focus-within:bg-white focus-within:ring-3 focus-within:ring-[#e8b94f]/12">
          <Search className="size-[16px] shrink-0" />
          <input
            type="search"
            value={search}
            onChange={(event) => onSearchChange(event.target.value)}
            placeholder="搜索当前页邮件…"
            className="min-w-0 flex-1 bg-transparent text-[13px] text-[#2a2b28] outline-none placeholder:text-[#9a9992]"
          />
          {search && (
            <span className="text-[10px] text-[#aaa8a0]">{visibleEmails.length} 条</span>
          )}
        </label>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {loading ? (
          <>
            {Array.from({ length: 6 }, (_, index) => <MessageSkeleton key={index} />)}
          </>
        ) : error ? (
          <div className="flex h-full min-h-[360px] flex-col items-center justify-center px-10 text-center">
            <div className="flex size-12 items-center justify-center rounded-2xl bg-[#efe4df] text-[#b76050]">
              <Inbox className="size-5" />
            </div>
            <h2 className="mt-4 text-sm font-semibold text-[#333430]">收件箱暂时不可用</h2>
            <p className="mt-2 max-w-[260px] text-xs leading-5 text-[#85857e]">{error}</p>
            <button
              type="button"
              onClick={onRetry}
              className="focus-ring mt-5 rounded-xl bg-[#262724] px-4 py-2.5 text-xs font-medium text-white transition hover:bg-black"
            >
              重新连接
            </button>
          </div>
        ) : visibleEmails.length === 0 ? (
          <div className="flex h-full min-h-[360px] flex-col items-center justify-center px-10 text-center">
            <div className="relative flex size-[72px] items-center justify-center rounded-[24px] bg-[#eeece5] text-[#8c8b84]">
              <Inbox className="size-7" strokeWidth={1.6} />
              <span className="absolute -right-1 -top-1 size-4 rounded-full border-4 border-[#f8f7f3] bg-[#f3bd4d]" />
            </div>
            <h2 className="mt-5 text-sm font-semibold text-[#343531]">
              {search || attachmentsOnly ? "没有找到相关邮件" : "这里还没有邮件"}
            </h2>
            <p className="mt-2 max-w-[240px] text-xs leading-5 text-[#8b8a83]">
              {search || attachmentsOnly
                ? "换个关键词或关闭附件筛选试试看"
                : accounts.length === 0
                  ? "添加一个 IMAP 邮箱，Paperwing 会自动同步你的来信"
                  : "新邮件到达后会自动出现在这里"}
            </p>
          </div>
        ) : (
          visibleEmails.map((email) => {
            const account = accounts.find((item) => item.id === email.account_id);
            const selected = selectedEmailID === email.id;
            return (
              <button
                type="button"
                key={email.id}
                onClick={() => onSelectEmail(email)}
                className={cn(
                  "focus-ring relative block w-full border-b border-[#e5e3dc] px-4 py-4 text-left transition sm:px-5",
                  selected ? "bg-[#efe8d8]" : "hover:bg-[#f1efe9]",
                )}
              >
                {selected && <span className="absolute inset-y-0 left-0 w-[3px] bg-[#e8ad35]" />}
                <div className="flex gap-3">
                  <Avatar value={email.from[0] || email.subject} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between gap-3">
                      <span className="truncate text-[13px] font-semibold text-[#31322f]">
                        {displayName(email.from[0])}
                      </span>
                      <span className="shrink-0 text-[10px] font-medium text-[#96958e]">
                        {formatRelativeDate(email.received_at)}
                      </span>
                    </div>
                    <p className="mt-1.5 truncate text-[13px] font-medium text-[#555650]">
                      {email.subject || "（无主题）"}
                    </p>
                    <div className="mt-2 flex items-center gap-2 text-[10px] text-[#999890]">
                      {account && (
                        <span className="max-w-[128px] truncate rounded-md bg-black/[0.045] px-1.5 py-0.5">
                          {account.name}
                        </span>
                      )}
                      {email.attachment_count > 0 && (
                        <span className="flex items-center gap-1">
                          <Paperclip className="size-3" />
                          {email.attachment_count}
                        </span>
                      )}
                      <span className="ml-auto">{formatBytes(email.size)}</span>
                    </div>
                  </div>
                </div>
              </button>
            );
          })
        )}
      </div>

      {!loading && !error && total > pageSize && (
        <footer className="flex h-12 shrink-0 items-center justify-between border-t border-[#e1dfd8] bg-[#f8f7f3] px-5">
          <span className="text-[10px] text-[#92918a]">
            {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} / {total}
          </span>
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => onPageChange(page - 1)}
              disabled={page <= 1}
              className="focus-ring flex size-8 items-center justify-center rounded-lg text-[#696a64] transition hover:bg-[#e9e7e0] disabled:cursor-not-allowed disabled:opacity-30"
              aria-label="上一页"
            >
              <ChevronLeft className="size-4" />
            </button>
            <span className="min-w-12 text-center text-[10px] text-[#696a64]">
              {page} / {pageCount}
            </span>
            <button
              type="button"
              onClick={() => onPageChange(page + 1)}
              disabled={page >= pageCount}
              className="focus-ring flex size-8 items-center justify-center rounded-lg text-[#696a64] transition hover:bg-[#e9e7e0] disabled:cursor-not-allowed disabled:opacity-30"
              aria-label="下一页"
            >
              <ChevronRight className="size-4" />
            </button>
          </div>
        </footer>
      )}
      {loading && (
        <div className="pointer-events-none absolute bottom-4 left-1/2 -translate-x-1/2 text-[#a07b31]">
          <LoaderCircle className="size-4 animate-spin" />
        </div>
      )}
    </section>
  );
}
