import { type FormEvent, useState } from "react";
import { Check, Eye, EyeOff, LoaderCircle, LockKeyhole, PlugZap, ShieldCheck } from "lucide-react";
import type { NewAccount } from "../types";
import { Modal } from "./Modal";

interface AccountDialogProps {
  open: boolean;
  onClose: () => void;
  onTest: (account: NewAccount) => Promise<void>;
  onCreate: (account: NewAccount) => Promise<void>;
}

const INITIAL_ACCOUNT: NewAccount = {
  name: "",
  host: "",
  port: 993,
  tls: true,
  username: "",
  password: "",
};

function suggestedHost(username: string) {
  const domain = username.split("@")[1]?.toLowerCase();
  const hosts: Record<string, string> = {
    "gmail.com": "imap.gmail.com",
    "outlook.com": "outlook.office365.com",
    "hotmail.com": "outlook.office365.com",
    "qq.com": "imap.qq.com",
    "163.com": "imap.163.com",
    "icloud.com": "imap.mail.me.com",
  };
  return domain ? hosts[domain] || `imap.${domain}` : "";
}

export function AccountDialog({ open, onClose, onTest, onCreate }: AccountDialogProps) {
  const [account, setAccount] = useState<NewAccount>(INITIAL_ACCOUNT);
  const [showPassword, setShowPassword] = useState(false);
  const [status, setStatus] = useState<"idle" | "testing" | "tested" | "saving">("idle");
  const [error, setError] = useState<string | null>(null);

  const setField = <K extends keyof NewAccount>(field: K, value: NewAccount[K]) => {
    setAccount((current) => ({ ...current, [field]: value }));
    setStatus("idle");
    setError(null);
  };

  const handleClose = () => {
    if (status === "testing" || status === "saving") return;
    setAccount(INITIAL_ACCOUNT);
    setStatus("idle");
    setError(null);
    onClose();
  };

  const validate = () => {
    if (!account.name.trim()) return "给这个邮箱起个容易识别的名字";
    if (!account.host.trim()) return "请填写 IMAP 服务器地址";
    if (!account.username.trim()) return "请填写邮箱账号";
    if (!account.password) return "请填写密码或应用专用密码";
    if (account.port < 1 || account.port > 65535) return "端口必须在 1 到 65535 之间";
    return null;
  };

  const handleTest = async () => {
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }
    setStatus("testing");
    setError(null);
    try {
      await onTest(account);
      setStatus("tested");
    } catch (caught) {
      setStatus("idle");
      setError(caught instanceof Error ? caught.message : "连接测试失败");
    }
  };

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }
    setStatus("saving");
    setError(null);
    try {
      await onCreate(account);
      setAccount(INITIAL_ACCOUNT);
      setStatus("idle");
      onClose();
    } catch (caught) {
      setStatus("idle");
      setError(caught instanceof Error ? caught.message : "添加邮箱失败");
    }
  };

  const inputClass =
    "focus-ring mt-1.5 h-10 w-full rounded-xl border border-[#ddd9cf] bg-white px-3 text-[13px] text-[#343531] outline-none transition placeholder:text-[#aaa9a1] focus:border-[#c99b3c]";

  return (
    <Modal open={open} onClose={handleClose} eyebrow="Connect an inbox" title="添加邮箱">
      <form onSubmit={handleSubmit} className="px-5 py-6 sm:px-7">
        <div className="grid gap-5 sm:grid-cols-2">
          <label className="block sm:col-span-2">
            <span className="text-[11px] font-semibold text-[#565751]">邮箱名称</span>
            <input
              autoFocus
              value={account.name}
              onChange={(event) => setField("name", event.target.value)}
              placeholder="例如：工作邮箱"
              className={inputClass}
            />
          </label>

          <label className="block sm:col-span-2">
            <span className="text-[11px] font-semibold text-[#565751]">邮箱账号</span>
            <input
              type="email"
              value={account.username}
              onChange={(event) => setField("username", event.target.value)}
              onBlur={() => {
                if (!account.host && account.username.includes("@")) {
                  setField("host", suggestedHost(account.username));
                }
              }}
              placeholder="you@example.com"
              autoComplete="username"
              className={inputClass}
            />
          </label>

          <label className="block sm:col-span-2">
            <span className="text-[11px] font-semibold text-[#565751]">密码</span>
            <span className="relative block">
              <input
                type={showPassword ? "text" : "password"}
                value={account.password}
                onChange={(event) => setField("password", event.target.value)}
                placeholder="密码或应用专用密码"
                autoComplete="current-password"
                className={`${inputClass} pr-11`}
              />
              <button
                type="button"
                onClick={() => setShowPassword((visible) => !visible)}
                className="focus-ring absolute right-1.5 top-3 flex size-8 items-center justify-center rounded-lg text-[#93928b] hover:bg-[#f0eee8] hover:text-[#4a4b47]"
                aria-label={showPassword ? "隐藏密码" : "显示密码"}
              >
                {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
              </button>
            </span>
            <span className="mt-2 flex items-center gap-1.5 text-[10px] leading-4 text-[#96958e]">
              <LockKeyhole className="size-3 shrink-0" />
              开启两步验证的邮箱通常需要使用应用专用密码
            </span>
          </label>

          <label className="block">
            <span className="text-[11px] font-semibold text-[#565751]">IMAP 服务器</span>
            <input
              value={account.host}
              onChange={(event) => setField("host", event.target.value)}
              placeholder="imap.example.com"
              className={inputClass}
            />
          </label>

          <label className="block">
            <span className="text-[11px] font-semibold text-[#565751]">端口</span>
            <input
              type="number"
              min={1}
              max={65535}
              value={account.port}
              onChange={(event) => setField("port", Number(event.target.value))}
              className={inputClass}
            />
          </label>
        </div>

        <label className="mt-5 flex cursor-pointer items-center justify-between rounded-[14px] border border-[#e1ded5] bg-[#f5f3ed] px-4 py-3">
          <span>
            <span className="flex items-center gap-2 text-[11px] font-semibold text-[#555650]">
              <ShieldCheck className="size-4 text-[#548a69]" />
              使用 TLS 安全连接
            </span>
            <span className="mt-1 block text-[9px] text-[#98978f]">推荐，通常对应 993 端口</span>
          </span>
          <input
            type="checkbox"
            checked={account.tls}
            onChange={(event) => setField("tls", event.target.checked)}
            className="peer sr-only"
          />
          <span className="relative h-6 w-10 rounded-full bg-[#c8c6be] transition peer-checked:bg-[#343632] peer-focus-visible:ring-3 peer-focus-visible:ring-[#e2af43]/25 after:absolute after:left-1 after:top-1 after:size-4 after:rounded-full after:bg-white after:shadow-sm after:transition peer-checked:after:translate-x-4" />
        </label>

        {error && (
          <div className="mt-4 rounded-xl border border-[#e7c8be] bg-[#f8ece8] px-3.5 py-2.5 text-[11px] leading-5 text-[#a55243]">
            {error}
          </div>
        )}

        {status === "tested" && (
          <div className="mt-4 flex items-center gap-2 rounded-xl border border-[#cce2d5] bg-[#edf7f1] px-3.5 py-2.5 text-[11px] font-medium text-[#42775a]">
            <Check className="size-4" />
            连接成功，可以添加这个邮箱
          </div>
        )}

        <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={handleTest}
            disabled={status === "testing" || status === "saving"}
            className="focus-ring flex h-11 items-center justify-center gap-2 rounded-[13px] border border-[#d9d6cc] bg-white px-4 text-xs font-semibold text-[#555650] transition hover:border-[#c8c4b8] hover:bg-[#f6f4ee] disabled:cursor-wait disabled:opacity-60"
          >
            {status === "testing" ? <LoaderCircle className="size-4 animate-spin" /> : <PlugZap className="size-4" />}
            {status === "testing" ? "正在测试…" : "测试连接"}
          </button>
          <button
            type="submit"
            disabled={status === "testing" || status === "saving"}
            className="focus-ring flex h-11 items-center justify-center gap-2 rounded-[13px] bg-[#272825] px-5 text-xs font-semibold text-white shadow-[0_8px_20px_rgba(0,0,0,0.16)] transition hover:bg-black active:scale-[0.99] disabled:cursor-wait disabled:opacity-60"
          >
            {status === "saving" && <LoaderCircle className="size-4 animate-spin" />}
            {status === "saving" ? "正在添加…" : "添加邮箱"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
