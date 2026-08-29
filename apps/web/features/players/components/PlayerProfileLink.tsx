import Link from "next/link";
import type { MouseEvent, ReactNode } from "react";

export default function PlayerProfileLink({
  userId,
  nickname,
  children,
  className = "",
  disabled = false,
  stopPropagation = false,
  title,
}: {
  userId?: string;
  nickname?: string;
  children: ReactNode;
  className?: string;
  disabled?: boolean;
  stopPropagation?: boolean;
  title?: string;
}) {
  const handleClick = (event: MouseEvent) => {
    if (stopPropagation) event.stopPropagation();
  };
  if (!userId || disabled) {
    return <span className={className}>{children}</span>;
  }
  const publicNickname = (nickname || (typeof children === "string" ? children : "")).trim();
  if (!publicNickname) {
    return <span className={className}>{children}</span>;
  }
  return (
    <Link
      href={`/players/${encodeURIComponent(publicNickname)}`}
      prefetch={false}
      className={className}
      onClick={handleClick}
      title={title || "View player profile"}
    >
      {children}
    </Link>
  );
}
