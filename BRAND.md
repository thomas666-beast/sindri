# Brand

## Name
Set via `VITE_APP_BRAND_NAME` in `.env`. Do not hardcode the brand name
anywhere else — read it from `useBrand()`.

## Mark
- `web/public/logo.svg` — transparent, uses `currentColor`.
  Use inline in components. Inherits `--accent` via CSS `color`.
- `web/public/logo-badge.svg` — same glyph with rounded background.
  Used only for the favicon / PWA / social previews (browsers don't
  apply your CSS there, so the color is baked in).

## Accent color
`--accent` (and `--accent-hover`) in CSS variables.
Changing them recolors the inline mark automatically.

## When rebranding
1. Change `VITE_APP_BRAND_NAME` in `.env`.
2. If the accent color changes, update `--accent` + `--accent-hover`,
   plus the hardcoded `#00d9a3` in `logo-badge.svg` (one `sed`).
3. The mark (`>_`) does not need to change unless the brand direction
   stops being coding/shell-flavored.

## Verify

### 1. Frontend builds

```bash
cd web && npm run build

# 2. Dev server — visit the topbar, confirm:
#    - the >_ mark renders
#    - it colors #00d9a3 by default
#    - it shifts to --accent-hover on .brand:hover
#    - the wordmark next to it is whatever VITE_APP_BRAND_NAME is
npm run dev
```