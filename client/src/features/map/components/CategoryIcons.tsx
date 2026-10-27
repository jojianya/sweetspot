type IconName =
  | "restaurant"
  | "hotel"
  | "activities"
  | "museum"
  | "transit"
  | "pharmacy"
  | "park"
  | "shopping"
  | "bar"
  | "religion"
  | "tag";

function categoryIcon(name: string): IconName {
  const n = name.toLowerCase();
  if (/restaurant|food|cafe|dining|eat|coffee|brunch/.test(n)) return "restaurant";
  if (/hotel|stay|lodge|inn|accommodation|motel|resort|hostel|apartment/.test(n)) return "hotel";
  if (/thing|attraction|tour|activity|see|fun|outdoor|adventure/.test(n)) return "activities";
  if (/museum|gallery|art|history|exhibit|heritage/.test(n)) return "museum";
  if (/transit|transport|train|bus|metro|rail|station|airport/.test(n)) return "transit";
  if (/pharm|drug|medic|chemist|clinic|health|wellness/.test(n)) return "pharmacy";
  if (/park|garden|nature|trail|playground|reserve/.test(n)) return "park";
  if (/shop|store|mall|market|retail|boutique/.test(n)) return "shopping";
  if (/bar|pub|night|club|cocktail|drink|brew/.test(n)) return "bar";
  if (/temple|church|mosque|synagogue|worship|holy|shrine/.test(n)) return "religion";
  return "tag";
}

export function ChipIcon({ name }: { name: string }) {
  const icon = categoryIcon(name);
  const common = {
    width: 14,
    height: 14,
    viewBox: "0 0 24 24",
    fill: "none" as const,
    stroke: "currentColor",
    strokeWidth: 2,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
  };
  if (name.trim().toLowerCase() === "all") {
    return (
      <svg {...common}>
        <circle cx="12" cy="12" r="10" />
        <path d="M12 2a14 14 0 0 0 0 20 14 14 0 0 0 0-20" />
        <path d="M2 12h20" />
      </svg>
    );
  }
  switch (icon) {
    case "restaurant":
      return (
        <svg {...common}>
          <path d="M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2" />
          <path d="M7 2v20" />
          <path d="M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3Zm0 0v7" />
        </svg>
      );
    case "hotel":
      return (
        <svg {...common}>
          <path d="M2 20v-8a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v8" />
          <path d="M4 10V6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v4" />
          <path d="M12 4v6" />
          <path d="M2 18h20" />
        </svg>
      );
    case "activities":
      return (
        <svg {...common}>
          <path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z" />
          <circle cx="12" cy="13" r="3" />
        </svg>
      );
    case "museum":
      return (
        <svg {...common}>
          <line x1="3" x2="21" y1="22" y2="22" />
          <line x1="6" x2="6" y1="18" y2="11" />
          <line x1="10" x2="10" y1="18" y2="11" />
          <line x1="14" x2="14" y1="18" y2="11" />
          <line x1="18" x2="18" y1="18" y2="11" />
          <polygon points="12 2 20 7 4 7" />
        </svg>
      );
    case "transit":
      return (
        <svg {...common}>
          <path d="M8 3.1V7a4 4 0 0 0 8 0V3.1" />
          <path d="m9 15-1-1" />
          <path d="m15 15 1-1" />
          <path d="M9 19c-2.8 0-5-2.2-5-5v-4a8 8 0 0 1 16 0v4c0 2.8-2.2 5-5 5Z" />
          <path d="m8 19-2 3" />
          <path d="m16 19 2 3" />
        </svg>
      );
    case "pharmacy":
      return (
        <svg {...common}>
          <path d="m10.5 20.5 10-10a4.95 4.95 0 1 0-7-7l-10 10a4.95 4.95 0 1 0 7 7Z" />
          <path d="m8.5 8.5 7 7" />
        </svg>
      );
    case "park":
      return (
        <svg {...common}>
          <path d="m17 14 3 3.3a1 1 0 0 1-.7 1.7H4.7a1 1 0 0 1-.7-1.7L7 14h-.3a1 1 0 0 1-.7-1.7L9 9h-.2A1 1 0 0 1 8 7.3L12 3l4 4.3a1 1 0 0 1-.8 1.7H15l3 3.3a1 1 0 0 1-.7 1.7H17Z" />
          <path d="M12 22v-3" />
        </svg>
      );
    case "shopping":
      return (
        <svg {...common}>
          <path d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4Z" />
          <path d="M3 6h18" />
          <path d="M16 10a4 4 0 0 1-8 0" />
        </svg>
      );
    case "bar":
      return (
        <svg {...common}>
          <path d="M8 22h8" />
          <path d="M12 11v11" />
          <path d="m19 3-7 8-7-8Z" />
        </svg>
      );
    case "religion":
      return (
        <svg {...common}>
          <path d="M10 9h4" />
          <path d="M12 7v5" />
          <path d="M14 22v-4a2 2 0 0 0-4 0v4" />
          <path d="M18 22V5.618a1 1 0 0 0-.553-.894l-4.553-2.277a2 2 0 0 0-1.788 0L6.553 4.724A1 1 0 0 0 6 5.618V22" />
          <path d="m18 7 3.447 1.724a1 1 0 0 1 .553.894V20a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9.618a1 1 0 0 1 .553-.894L6 7" />
        </svg>
      );
    default:
      return (
        <svg {...common}>
          <path d="M12.586 2.586A2 2 0 0 0 11.172 2H4a2 2 0 0 0-2 2v7.172a2 2 0 0 0 .586 1.414l8.704 8.704a2.426 2.426 0 0 0 3.42 0l6.58-6.58a2.426 2.426 0 0 0 0-3.42z" />
          <circle cx="7.5" cy="7.5" r="0.5" fill="currentColor" stroke="none" />
        </svg>
      );
  }
}
