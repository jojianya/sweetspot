export { default as CreatePinButton } from "./components/CreatePinButton";
export { PinCoverImage } from "./components/PinCoverImage";
export { default as PinDetailPanel } from "./components/PinDetailPanel";
export { default as PinPageClient } from "./components/PinPageClient";
export { ListSkeleton } from "./components/ListSkeleton";
export { SheetHeader, SheetSignIn } from "./components/SheetChrome";
export { usePins } from "./hooks/usePins";
export { usePinDetail } from "./hooks/usePinDetail";
export { searchPins, fetchTrendingPins } from "./api/pins";
export { toPinListEntry } from "./lib/pinEntry";
export { buildPinMetadata } from "./lib/metadata/pin";
// getPinServer is deliberately NOT re-exported here. api/server.ts depends on
// next/headers, and any barrel that re-exports it poisons every client graph
// that transitively reaches the barrel (Navbar -> auth -> profile -> pins).
// The one server caller, app/pin/[id]/page.tsx, imports the leaf module.
