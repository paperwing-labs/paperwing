import { useMemo, useState } from "react";
import {
  ArrowLeft,
  ChevronDown,
  Download,
  File,
  FileArchive,
  FileImage,
  FileText,
  Inbox,
  MoreHorizontal,
  Paperclip,
  ShieldCheck,
} from "lucide-react";
import type { Account, Attachment, Email } from "../types";
import {
  cn,
  displayName,
  emailAddress,
  formatBytes,
  formatFullDate,
} from "../utils";
import { Avatar } from "./Avatar";

interface EmailReaderProps {
  email: Email | null;
  account?: Account;
  loading: boolean;
  error: string | null;
  attachmentURL: (emailID: string, attachmentID: string) => string;
  onBack: () => void;
  onRetry: () => void;
}

function attachmentIcon(attachment: Attachment) {
  if (attachment.content_type.startsWith("image/")) return FileImage;
  if (attachment.content_type.includes("zip") || attachment.content_type.includes("compressed")) {
    return FileArchive;
  }
  if (attachment.content_type.includes("text") || attachment.content_type.includes("pdf")) {
    return FileText;
  }
  return File;
}

function safeEmailDocument(html: string) {
  const parser = new DOMParser();
  const document = parser.parseFromString(html, "text/html");
  document
    .querySelectorAll("script, iframe, object, embed, form, input, button, video, audio, source, meta, link, base")
    .forEach((element) => element.remove());
  document.querySelectorAll("*").forEach((element) => {
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase();
      const value = attribute.value.trim().toLowerCase();
      if (
        name.startsWith("on") ||
        name === "srcset" ||
        ((name === "src" || name === "background") && !value.startsWith("data:image/"))
      ) {
        element.removeAttribute(attribute.name);
      }
      if (name === "href") {
        element.setAttribute("href", "#");
        element.setAttribute("title", "为保护隐私，邮件中的外部链接已停用");
      }
    }
  });
  return `<!doctype html>
    <html><head>
      <meta charset="utf-8">
      <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'">
      <style>
        html { color-scheme: light; }
        body { margin: 0; color: #363733; background: #fff; font: 14px/1.75 -apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC',sans-serif; overflow-wrap: anywhere; }
        img { max-width: 100%; height: auto; }
        table { max-width: 100% !important; }
        a { color: #9a6a14; text-decoration: underline; cursor: not-allowed; }
        blockquote { margin-left: 0; padding-left: 16px; border-left: 3px solid #e3e0d7; color: #71716b; }
        pre { overflow: auto; padding: 12px; border-radius: 10px; background: #f4f3ef; }
      </style>
    </head><body>${document.body.innerHTML}</body></html>`;
}

function EmptyReader() {
  return (
    <div className="paper-texture flex h-full flex-col items-center justify-center px-8 text-center">
      <div className="relative">
        <div className="absolute inset-3 rounded-full bg-[#eab748]/25 blur-2xl" />
        <svg viewBox="0 0 180 140" className="relative w-[160px]" fill="none" aria-hidden="true">
          <path d="M31 95c26 15 75 18 118-4" stroke="#D5D2C8" strokeWidth="2" strokeLinecap="round" />
          <path d="m30 41 119 26-54 13-29 38-7-43-29-34Z" fill="#EEECE4" stroke="#252623" strokeWidth="2.4" strokeLinejoin="round" />
          <path d="m59 75 90-8-70-5-49-21 29 34Z" fill="#F7C75A" stroke="#252623" strokeWidth="2.4" strokeLinejoin="round" />
          <path d="m59 75 36 5" stroke="#252623" strokeWidth="2.4" strokeLinecap="round" />
          <circle cx="145" cy="36" r="4" fill="#EAB541" />
          <circle cx="23" cy="78" r="2.5" fill="#9DA3A0" />
        </svg>
      </div>
      <h2 className="mt-5 text-[17px] font-semibold tracking-[-0.02em] text-[#30312e]">选择一封邮件开始阅读</h2>
      <p className="mt-2 max-w-[300px] text-xs leading-5 text-[#888780]">
        你的所有邮箱都在同一个安静的空间里，保持专注，也保持私密。
      </p>
      <div className="mt-6 flex items-center gap-2 rounded-full border border-[#dfddd5] bg-white/65 px-3 py-1.5 text-[10px] font-medium text-[#77776f] shadow-sm backdrop-blur-sm">
        <ShieldCheck className="size-3.5 text-[#57906e]" />
        邮件内容存储在你的设备上
      </div>
    </div>
  );
}

function ReaderSkeleton() {
  return (
    <div className="h-full animate-pulse bg-white px-7 py-8 sm:px-10 lg:px-12">
      <div className="h-8 w-4/5 rounded bg-[#eceae3]" />
      <div className="mt-4 h-4 w-2/5 rounded bg-[#f0eee8]" />
      <div className="mt-8 flex gap-3 border-b border-[#eceae3] pb-7">
        <div className="size-11 rounded-[14px] bg-[#e7e5de]" />
        <div className="flex-1">
          <div className="h-4 w-32 rounded bg-[#e5e3dc]" />
          <div className="mt-2 h-3 w-52 rounded bg-[#efede7]" />
        </div>
      </div>
      <div className="mt-8 h-3 w-full rounded bg-[#eeece6]" />
      <div className="mt-3 h-3 w-11/12 rounded bg-[#eeece6]" />
      <div className="mt-3 h-3 w-4/5 rounded bg-[#eeece6]" />
      <div className="mt-7 h-3 w-full rounded bg-[#eeece6]" />
      <div className="mt-3 h-3 w-3/4 rounded bg-[#eeece6]" />
    </div>
  );
}

export function EmailReader({
  email,
  account,
  loading,
  error,
  attachmentURL,
  onBack,
  onRetry,
}: EmailReaderProps) {
  const [showDetails, setShowDetails] = useState(false);
  const [showHeaders, setShowHeaders] = useState(false);
  const emailDocument = useMemo(
    () => (email?.html_body ? safeEmailDocument(email.html_body) : ""),
    [email?.html_body],
  );

  if (loading) return <ReaderSkeleton />;
  if (error) {
    return (
      <div className="paper-texture flex h-full flex-col items-center justify-center px-10 text-center">
        <div className="flex size-14 items-center justify-center rounded-[18px] bg-[#eee2dc] text-[#b35e4d]">
          <Inbox className="size-6" />
        </div>
        <h2 className="mt-4 text-sm font-semibold">无法打开这封邮件</h2>
        <p className="mt-2 max-w-[300px] text-xs leading-5 text-[#83837c]">{error}</p>
        <button
          type="button"
          onClick={onRetry}
          className="focus-ring mt-5 rounded-xl bg-[#242522] px-4 py-2.5 text-xs font-medium text-white hover:bg-black"
        >
          再试一次
        </button>
      </div>
    );
  }
  if (!email) return <EmptyReader />;

  const sender = email.from[0] || "未知发件人";

  return (
    <article className="flex h-full min-w-0 flex-col bg-white animate-fade-in">
      <div className="flex h-[62px] shrink-0 items-center justify-between border-b border-[#e7e5de] px-4 sm:px-6 lg:px-8">
        <div className="flex min-w-0 items-center gap-2">
          <button
            type="button"
            onClick={onBack}
            className="focus-ring flex size-9 shrink-0 items-center justify-center rounded-xl text-[#686963] transition hover:bg-[#f0eee8] md:hidden"
            aria-label="返回邮件列表"
          >
            <ArrowLeft className="size-[18px]" />
          </button>
          {account && (
            <span className="truncate rounded-full bg-[#f0eee8] px-2.5 py-1 text-[10px] font-medium text-[#6f706a]">
              {account.name}
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={() => setShowHeaders((visible) => !visible)}
          className={cn(
            "focus-ring flex size-9 items-center justify-center rounded-xl transition",
            showHeaders ? "bg-[#262724] text-white" : "text-[#72736d] hover:bg-[#f0eee8]",
          )}
          aria-label="显示邮件头信息"
          aria-pressed={showHeaders}
        >
          <MoreHorizontal className="size-[18px]" />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[900px] px-5 py-7 sm:px-8 sm:py-9 lg:px-11 lg:py-11">
          <h1 className="max-w-[760px] text-[25px] font-semibold leading-[1.28] tracking-[-0.04em] text-[#252623] sm:text-[30px]">
            {email.subject || "（无主题）"}
          </h1>
          <p className="mt-3 text-[11px] text-[#92918a]">{formatFullDate(email.received_at)}</p>

          <div className="mt-8 border-b border-[#eceae3] pb-7">
            <div className="flex items-start gap-3.5">
              <Avatar value={sender} size="lg" />
              <div className="min-w-0 flex-1 pt-0.5">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm font-semibold text-[#343532]">{displayName(sender)}</span>
                  <span className="hidden truncate text-[11px] text-[#999891] sm:inline">
                    &lt;{emailAddress(sender)}&gt;
                  </span>
                </div>
                <button
                  type="button"
                  onClick={() => setShowDetails((visible) => !visible)}
                  className="focus-ring mt-1 flex items-center gap-1 rounded text-[11px] text-[#888880] hover:text-[#393a36]"
                >
                  发送至 {email.to.length ? displayName(email.to[0]) : "我"}
                  <ChevronDown className={cn("size-3 transition", showDetails && "rotate-180")} />
                </button>
              </div>
            </div>

            {showDetails && (
              <dl className="mt-4 grid grid-cols-[52px_1fr] gap-x-3 gap-y-2 rounded-[14px] bg-[#f5f4ef] px-4 py-3 text-[11px] leading-5 animate-fade-in">
                <dt className="text-[#9b9a93]">发件人</dt>
                <dd className="min-w-0 break-all text-[#62635d]">{email.from.join(", ")}</dd>
                <dt className="text-[#9b9a93]">收件人</dt>
                <dd className="min-w-0 break-all text-[#62635d]">{email.to.join(", ") || "—"}</dd>
                {email.cc.length > 0 && (
                  <>
                    <dt className="text-[#9b9a93]">抄送</dt>
                    <dd className="min-w-0 break-all text-[#62635d]">{email.cc.join(", ")}</dd>
                  </>
                )}
              </dl>
            )}
          </div>

          {showHeaders && (
            <div className="mt-6 overflow-hidden rounded-[14px] border border-[#e4e2db] bg-[#f7f6f2] animate-fade-in">
              <div className="border-b border-[#e4e2db] px-4 py-2.5 text-[10px] font-semibold uppercase tracking-[0.14em] text-[#84847d]">
                原始邮件头
              </div>
              <div className="max-h-56 overflow-auto px-4 py-3 font-mono text-[10px] leading-5 text-[#686963]">
                {Object.entries(email.headers).map(([name, values]) => (
                  <div key={name} className="break-all">
                    <span className="font-semibold text-[#40413d]">{name}:</span> {values.join(", ")}
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="mt-8 text-[14px] leading-7 text-[#3f403c]">
            {email.html_body ? (
              <iframe
                title="邮件正文"
                sandbox=""
                srcDoc={emailDocument}
                className="mail-frame min-h-[480px] w-full"
              />
            ) : (
              <div className="whitespace-pre-wrap break-words">{email.text_body || "这封邮件没有正文。"}</div>
            )}
          </div>

          {email.attachments.length > 0 && (
            <section className="mt-10 border-t border-[#eceae3] pt-7">
              <div className="flex items-center gap-2 text-xs font-semibold text-[#4d4e49]">
                <Paperclip className="size-4 text-[#8d8c85]" />
                {email.attachments.length} 个附件
              </div>
              <div className="mt-3 grid gap-2 sm:grid-cols-2">
                {email.attachments.map((attachment) => {
                  const Icon = attachmentIcon(attachment);
                  return (
                    <a
                      key={attachment.id}
                      href={attachmentURL(email.id, attachment.id)}
                      download={attachment.filename}
                      className="focus-ring group flex min-w-0 items-center gap-3 rounded-[14px] border border-[#e4e2db] bg-[#faf9f6] p-3 text-left transition hover:border-[#d3c7a9] hover:bg-[#f7f2e6]"
                    >
                      <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-white text-[#797a74] shadow-sm ring-1 ring-black/[0.04]">
                        <Icon className="size-[18px]" />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-[11px] font-semibold text-[#454642]">{attachment.filename}</span>
                        <span className="mt-1 block text-[9px] uppercase tracking-wide text-[#9a9992]">
                          {formatBytes(attachment.size)}
                        </span>
                      </span>
                      <Download className="size-4 shrink-0 text-[#aaa9a2] transition group-hover:text-[#8d6821]" />
                    </a>
                  );
                })}
              </div>
            </section>
          )}
        </div>
      </div>
    </article>
  );
}
