import { type FormEvent, useState } from "react";
import { Eye, EyeOff, KeyRound, LoaderCircle, LockKeyhole, ShieldCheck } from "lucide-react";
import { api } from "../api";
import type { AuthStatus } from "../types";
import { BrandMark } from "./Brand";

interface AuthScreenProps {
  setup: boolean;
  initialUsername: string;
  onAuthenticated: (status: AuthStatus) => void;
}

export function AuthScreen({ setup, initialUsername, onAuthenticated }: AuthScreenProps) {
  const [username, setUsername] = useState(initialUsername || "admin");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!username.trim()) {
      setError("请输入管理员用户名");
      return;
    }
    if (setup && password.length < 12) {
      setError("密码至少需要 12 个字符");
      return;
    }
    if (setup && password !== confirmation) {
      setError("两次输入的密码不一致");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const result = setup
        ? await api.setupAuth(username.trim(), password)
        : await api.login(username.trim(), password);
      onAuthenticated(result);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : setup ? "初始化失败" : "登录失败");
    } finally {
      setLoading(false);
    }
  };

  const inputClass =
    "focus-ring mt-1.5 h-11 w-full rounded-[13px] border border-[#dcd9d0] bg-white px-3.5 text-[13px] text-[#30312e] outline-none transition placeholder:text-[#aaa9a1] focus:border-[#c99b3c]";

  return (
    <main className="paper-texture relative flex h-dvh min-h-[620px] overflow-y-auto bg-[#f4f3ee] px-5 py-8 sm:items-center sm:justify-center">
      <div className="pointer-events-none absolute -left-24 -top-24 size-[360px] rounded-full bg-[#f0ba48]/10 blur-3xl" />
      <div className="pointer-events-none absolute -bottom-32 -right-24 size-[420px] rounded-full bg-[#8d9f91]/10 blur-3xl" />

      <section className="relative mx-auto my-auto w-full max-w-[440px]">
        <div className="mb-7 flex items-center justify-center gap-3">
          <BrandMark />
          <div>
            <div className="font-semibold tracking-[-0.025em] text-[#292a27]">Paperwing</div>
            <div className="mt-0.5 text-[9px] font-semibold uppercase tracking-[0.18em] text-[#929189]">
              Private inbox
            </div>
          </div>
        </div>

        <div className="rounded-[24px] border border-[#dedbd2] bg-[#fbfaf7]/95 p-6 shadow-[0_24px_70px_rgba(39,40,36,0.12)] backdrop-blur-md sm:p-8">
          <div className="flex size-11 items-center justify-center rounded-[14px] bg-[#262724] text-[#f4bd4b] shadow-[0_8px_20px_rgba(25,26,23,0.16)]">
            {setup ? <ShieldCheck className="size-5" /> : <KeyRound className="size-5" />}
          </div>
          <h1 className="mt-5 text-[24px] font-semibold tracking-[-0.04em] text-[#292a27]">
            {setup ? "保护你的收件箱" : "欢迎回来"}
          </h1>
          <p className="mt-2 text-[11px] leading-5 text-[#85857e]">
            {setup
              ? "首次使用，请创建 Paperwing 管理员。设置完成后，所有邮件和账号接口都需要登录。"
              : "登录后继续查看你的统一收件箱。"}
          </p>

          <form onSubmit={submit} className="mt-6">
            <label className="block">
              <span className="text-[11px] font-semibold text-[#565751]">用户名</span>
              <input
                autoFocus
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                autoComplete="username"
                placeholder="admin"
                className={inputClass}
              />
            </label>

            <label className="mt-4 block">
              <span className="text-[11px] font-semibold text-[#565751]">密码</span>
              <span className="relative block">
                <input
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  autoComplete={setup ? "new-password" : "current-password"}
                  placeholder={setup ? "至少 12 个字符" : "输入管理员密码"}
                  className={`${inputClass} pr-11`}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((visible) => !visible)}
                  className="focus-ring absolute right-1.5 top-3 flex size-8 items-center justify-center rounded-lg text-[#929189] hover:bg-[#f0eee8] hover:text-[#4a4b47]"
                  aria-label={showPassword ? "隐藏密码" : "显示密码"}
                >
                  {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                </button>
              </span>
            </label>

            {setup && (
              <label className="mt-4 block">
                <span className="text-[11px] font-semibold text-[#565751]">确认密码</span>
                <input
                  type={showPassword ? "text" : "password"}
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                  autoComplete="new-password"
                  placeholder="再次输入密码"
                  className={inputClass}
                />
              </label>
            )}

            {error && (
              <div className="mt-4 rounded-xl border border-[#e8c9bf] bg-[#f8ece8] px-3.5 py-2.5 text-[11px] leading-5 text-[#a65345]">
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="focus-ring mt-5 flex h-11 w-full items-center justify-center gap-2 rounded-[13px] bg-[#292a27] text-xs font-semibold text-white shadow-[0_9px_24px_rgba(29,30,27,0.18)] transition hover:bg-black active:scale-[0.99] disabled:cursor-wait disabled:opacity-65"
            >
              {loading ? <LoaderCircle className="size-4 animate-spin" /> : <LockKeyhole className="size-4" />}
              {loading ? (setup ? "正在创建…" : "正在登录…") : setup ? "创建管理员并进入" : "登录"}
            </button>
          </form>

          <div className="mt-5 flex items-center justify-center gap-1.5 text-[9px] text-[#9a9992]">
            <ShieldCheck className="size-3 text-[#5e8f70]" />
            密码受 Argon2id 哈希保护，登录有效期为 30 天
          </div>
        </div>
      </section>
    </main>
  );
}
