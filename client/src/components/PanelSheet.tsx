"use client";

import { useRef, type ComponentPropsWithoutRef } from "react";
import { useDialogFocus } from "@/hooks/useDialogFocus";

const BASE_CLASS =
  "absolute z-40 flex flex-col bg-white shadow-2xl shadow-zinc-900/20 " +
  "inset-x-0 bottom-0 max-h-[85dvh] rounded-t-2xl " +
  "sm:left-0 sm:right-auto sm:top-16 sm:bottom-0 sm:max-h-none sm:w-full sm:max-w-md sm:rounded-none sm:rounded-r-2xl " +
  "dark:bg-zinc-900 dark:shadow-zinc-950/60";

interface PanelSheetProps extends ComponentPropsWithoutRef<"div"> {
  /** Closes the sheet on Escape. Focus is restored to the opener on close. */
  onClose?: () => void;
}

/** Bottom-sheet (mobile) / side-panel (desktop) layout shared by the map overlays. */
export default function PanelSheet({
  className,
  children,
  onClose,
  ...rest
}: PanelSheetProps) {
  const ref = useRef<HTMLDivElement>(null);
  useDialogFocus(ref, onClose);

  return (
    <div ref={ref} {...rest} className={className ? `${BASE_CLASS} ${className}` : BASE_CLASS}>
      {children}
    </div>
  );
}