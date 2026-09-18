const FALLBACK_CLASS =
  "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300";

type AvatarProps = {
  src: string | null;
  username: string;
  /** Sizing classes, e.g. "h-8 w-8" or "h-full w-full". */
  className?: string;
  /** Colors for the initial-letter fallback. */
  fallbackClassName?: string;
};

export default function Avatar({
  src,
  username,
  className = "h-8 w-8",
  fallbackClassName = FALLBACK_CLASS,
}: AvatarProps) {
  if (!src) {
    return (
      <span
        className={`flex items-center justify-center rounded-full text-sm font-semibold ${className} ${fallbackClassName}`}
      >
        {username.charAt(0).toUpperCase()}
      </span>
    );
  }
  return (
    // eslint-disable-next-line @next/next/no-img-element -- avatars are hosted media, <img> is fine here
    <img src={src} alt="" className={`rounded-full object-cover ${className}`} />
  );
}
