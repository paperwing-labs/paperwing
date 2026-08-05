import { avatarColor, initials } from "../utils";

export function Avatar({
  value,
  size = "md",
}: {
  value: string;
  size?: "sm" | "md" | "lg";
}) {
  const sizes = {
    sm: "size-8 rounded-[10px] text-[10px]",
    md: "size-10 rounded-[13px] text-xs",
    lg: "size-11 rounded-[14px] text-xs",
  };
  return (
    <span
      className={`flex shrink-0 items-center justify-center font-bold tracking-[0.03em] text-white ${sizes[size]}`}
      style={{ backgroundColor: avatarColor(value) }}
      aria-hidden="true"
    >
      {initials(value)}
    </span>
  );
}
