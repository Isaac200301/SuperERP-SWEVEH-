/** Logo provisional: la W de WEVEH con el punto ámbar (docs/10 §2.3 y §8). */
export function Logo({ size = 36 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true" focusable="false">
      <rect width="64" height="64" rx="15" fill="var(--color-primary)" />
      <path
        d="M17.5 27 L24.75 46 L32 33 L39.25 46 L46.5 27"
        fill="none"
        stroke="#FFFFFF"
        strokeWidth="6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle cx="32" cy="18" r="5" fill="var(--color-amber)" />
    </svg>
  );
}

export function Brand({ subtitle, dark = false }: { subtitle?: string; dark?: boolean }) {
  return (
    <span className="flex items-center gap-2.5">
      <Logo />
      <span className="flex flex-col leading-tight">
        <b className="text-lg">SWEVEH</b>
        {subtitle ? <span className={`text-xs ${dark ? 'text-night-text-2' : 'text-ink-2'}`}>{subtitle}</span> : null}
      </span>
    </span>
  );
}
