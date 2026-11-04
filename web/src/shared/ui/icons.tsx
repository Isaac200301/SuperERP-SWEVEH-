import type { SVGProps } from 'react';

/** Íconos SVG con trazo de 2 px y aria-hidden; el texto siempre va al lado (DR-13). */
type IconProps = SVGProps<SVGSVGElement> & { size?: number };

function Base({ size = 20, children, ...rest }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...rest}
    >
      {children}
    </svg>
  );
}

export const IconCheck = (p: IconProps) => <Base {...p}><path d="M5 12l4 4 10-10" /></Base>;
export const IconWarn = (p: IconProps) => <Base {...p}><path d="M12 4l9 16H3z" /><path d="M12 10v4M12 17h.01" /></Base>;
export const IconStop = (p: IconProps) => <Base {...p}><path d="M8 3h8l5 5v8l-5 5H8l-5-5V8z" /></Base>;
export const IconClock = (p: IconProps) => <Base {...p}><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></Base>;
export const IconOrders = (p: IconProps) => <Base {...p}><path d="M8 4h8l1 3H7z" /><rect x="5" y="7" width="14" height="14" rx="2" /><path d="M9 12h6M9 16h4" /></Base>;
export const IconPlus = (p: IconProps) => <Base {...p}><path d="M12 5v14M5 12h14" /></Base>;
export const IconLogout = (p: IconProps) => <Base {...p}><path d="M15 4h4v16h-4M10 8l-4 4 4 4M6 12h10" /></Base>;
export const IconCar = (p: IconProps) => <Base {...p}><path d="M4 17v-4l2-5h12l2 5v4" /><path d="M4 13h16" /><circle cx="7.5" cy="17" r="1.8" /><circle cx="16.5" cy="17" r="1.8" /></Base>;
export const IconChat = (p: IconProps) => <Base {...p}><path d="M4 5h16v11H9l-5 4z" /></Base>;
export const IconBack = (p: IconProps) => <Base {...p}><path d="M15 6l-6 6 6 6" /></Base>;
export const IconX = (p: IconProps) => <Base {...p}><path d="M6 6l12 12M18 6L6 18" /></Base>;
export const IconTool = (p: IconProps) => <Base {...p}><path d="M14.7 6.3a4 4 0 0 0-5.4 5.4L3 18l3 3 6.3-6.3a4 4 0 0 0 5.4-5.4l-2.5 2.5-2.4-.6-.6-2.4z" /></Base>;
export const IconTeam = (p: IconProps) => <Base {...p}><circle cx="9" cy="8" r="3.2" /><path d="M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6" /><path d="M16 4.5a3 3 0 0 1 0 6M21 20c0-2.6-1.6-4.8-4-5.6" /></Base>;
