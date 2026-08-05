import { AlertCircle, CheckCircle2, Info, X } from "lucide-react";
import type { ToastMessage } from "../types";

export function Toasts({ toasts, onDismiss }: { toasts: ToastMessage[]; onDismiss: (id: number) => void }) {
  return (
    <div className="pointer-events-none fixed left-1/2 top-4 z-[100] flex w-[calc(100%-2rem)] max-w-sm -translate-x-1/2 flex-col gap-2">
      {toasts.map((toast) => {
        const Icon = toast.tone === "success" ? CheckCircle2 : toast.tone === "error" ? AlertCircle : Info;
        const color = toast.tone === "success" ? "text-[#57906d]" : toast.tone === "error" ? "text-[#b75b4c]" : "text-[#9a752f]";
        return (
          <div key={toast.id} className="pointer-events-auto flex items-center gap-3 rounded-[14px] border border-black/8 bg-white/95 px-3.5 py-3 shadow-[0_14px_45px_rgba(26,27,24,0.18)] backdrop-blur-xl animate-toast-in">
            <Icon className={`size-[18px] shrink-0 ${color}`} />
            <span className="min-w-0 flex-1 text-[11px] font-medium leading-5 text-[#454642]">{toast.message}</span>
            <button
              type="button"
              onClick={() => onDismiss(toast.id)}
              className="focus-ring flex size-7 shrink-0 items-center justify-center rounded-lg text-[#9a9992] hover:bg-[#efede7] hover:text-[#4b4c48]"
              aria-label="关闭提示"
            >
              <X className="size-3.5" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
