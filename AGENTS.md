# SMS Relay — Agent Instructions

## UI / Frontend

Before writing or modifying any UI in `web/`, read and follow [DESIGN.md](./DESIGN.md).

### Design rules

- **Surfaces**: page on `canvas-soft` (#fafafa), cards on `canvas` (#ffffff), dividers on `hairline` (#ebebeb)
- **Typography**: Inter for all UI text; JetBrains Mono for code and technical labels only
- **Elevation**: stacked subtle shadows + inset hairline — never heavy drop shadows or glassmorphism
- **Buttons**: ink primary (`#171717`) for CTAs; 6px radius in-app, pill shape for marketing-scale actions
- **Links**: accent blue (`#0070f3`), underlined on hover
- **Dark mode**: polarity-flip surfaces (near-black canvas, white primary CTA) — keep the same calm elevation system

### Project layout

- `web/` — React + Vite frontend (console UI)
- `DESIGN.md` — design system reference (Vercel-inspired)
