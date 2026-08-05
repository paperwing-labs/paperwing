import { cn } from "../utils";

export function BrandMark({ className }: { className?: string }) {
  return (
    <span
      className={cn(
        "flex size-10 shrink-0 items-center justify-center rounded-[13px] bg-[#20211f] shadow-[0_7px_18px_rgba(20,21,19,0.16)]",
        className,
      )}
      aria-hidden="true"
    >
      <svg viewBox="0 0 44 44" className="size-7" fill="none">
        <path
          d="M7.6 12.3 36 20.4l-11.9 4.4-5.6 11.1-2.1-12.5-8.8-11.1Z"
          fill="#F5BE49"
          stroke="#F5BE49"
          strokeLinejoin="round"
        />
        <path d="m16.4 23.4 19.6-3-15.9.4-12.5-8.5 8.8 11.1Z" fill="#FFF3CE" />
        <path d="m16.4 23.4 7.7 1.4" stroke="#20211F" strokeWidth="1.5" />
      </svg>
    </span>
  );
}

export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className="flex items-center gap-3">
      <BrandMark />
      {!compact && (
        <div>
          <div className="font-semibold tracking-[-0.025em] text-[#f8f5e9]">Paperwing</div>
          <div className="mt-0.5 text-[10px] font-medium uppercase tracking-[0.18em] text-white/38">
            Unified inbox
          </div>
        </div>
      )}
    </div>
  );
}
