"use client";

import SearchBar from "./SearchBar";
import { AccountMenu } from "./AccountMenu";
import { CategoryChipStrip } from "./CategoryChipStrip";
import type { Category } from "@/shared/types/category"
import type { PinListEntry } from "@/shared/types/pin";

interface MapNavBarProps {
  center: { lat: number; lng: number };
  onSelectPlace: (center: { lat: number; lng: number }, bbox?: [number, number, number, number]) => void;
  onSelectPin: (entry: PinListEntry) => void;
  categories: Category[];
  selectedCategory: number | null;
  onSelectCategory: (id: number | null) => void;
  onOpenSaved: () => void;
}

export default function MapNavBar({
  center,
  onSelectPlace,
  onSelectPin,
  categories,
  selectedCategory,
  onSelectCategory,
  onOpenSaved,
}: MapNavBarProps) {
  return (
    <header className="pointer-events-none absolute inset-x-0 top-0 z-30">
      <div className="pointer-events-auto flex flex-wrap items-center gap-2.5 px-2.5 py-2 sm:gap-3 sm:px-3">
          <div className="w-full min-w-0 flex-1 sm:w-72 sm:flex-none sm:min-w-64">
            <SearchBar
              center={center}
              onSelectPlace={onSelectPlace}
              onSelectPin={onSelectPin}
              placeholder="Search Maps"
              className="w-full min-w-0"
            />
          </div>

          <CategoryChipStrip
            categories={categories}
            selectedCategory={selectedCategory}
            onSelectCategory={onSelectCategory}
          />

          <AccountMenu onOpenSaved={onOpenSaved} />
        </div>
    </header>
  );
}
