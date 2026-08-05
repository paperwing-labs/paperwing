import { useEffect } from "react";
import { X } from "lucide-react";

interface ModalProps {
  open: boolean;
  title: string;
  eyebrow?: string;
  children: React.ReactNode;
  onClose: () => void;
}

export function Modal({ open, title, eyebrow, children, onClose }: ModalProps) {
  useEffect(() => {
    if (!open) return undefined;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [onClose, open]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-[70] flex items-end justify-center p-0 sm:items-center sm:p-5">
      <button
        type="button"
        onClick={onClose}
        className="absolute inset-0 bg-[#171815]/55 backdrop-blur-[3px] animate-fade-in"
        aria-label="关闭"
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        className="relative max-h-[94vh] w-full overflow-y-auto rounded-t-[24px] bg-[#fbfaf7] shadow-[0_24px_80px_rgba(0,0,0,0.3)] animate-modal-in sm:max-w-[560px] sm:rounded-[24px]"
      >
        <header className="sticky top-0 z-10 flex items-start justify-between border-b border-[#e7e4dc] bg-[#fbfaf7]/95 px-5 py-5 backdrop-blur-md sm:px-7">
          <div>
            {eyebrow && (
              <p className="mb-1 text-[9px] font-bold uppercase tracking-[0.18em] text-[#b07d20]">{eyebrow}</p>
            )}
            <h2 id="modal-title" className="text-xl font-semibold tracking-[-0.035em] text-[#292a27]">
              {title}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="focus-ring flex size-9 items-center justify-center rounded-xl text-[#777770] transition hover:bg-[#eceae3] hover:text-[#292a27]"
            aria-label="关闭"
          >
            <X className="size-[18px]" />
          </button>
        </header>
        {children}
      </div>
    </div>
  );
}
