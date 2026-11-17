# Design

Utanabi's look is **Night stage**: the feel of singing along on a dark stage. The line being sung is big and bright. The rest of the song stays dim, so the eye always knows where it is. One warm amber accent marks what you can act on.

Every screen is built for a phone first. The rules are in "Mobile first" in [AGENTS.md](../AGENTS.md).

## Light and dark

The app follows the device's setting (`prefers-color-scheme`). Dark is the signature look and the default when a device has no preference. The light theme keeps the same idea: the sung line is the darkest, boldest text, and the rest of the song is a softer grey. There is no in-app switch yet.

## Colours

The tokens live on `:root` in `frontend/src/app.css`, with the light values in its `prefers-color-scheme: light` block. Components use the tokens, never their own hex values, so every screen gets both themes for free.

| Token | Dark | Light | Use |
| --- | --- | --- | --- |
| `--bg` | `#0f1420` | `#f5f6f9` | The page. |
| `--surface` | `#18202f` | `#ffffff` | Cards, fields, and secondary buttons. |
| `--surface-strong` | `#273044` | `#e6e9f0` | Tiles, and hover on surfaces. |
| `--border` | `#273044` | `#d5dae3` | Borders. |
| `--fg` | `#f2f4f8` | `#121826` | Body text and the current lyric line. |
| `--muted` | `#a3acbe` | `#4a5468` | Labels and secondary text. |
| `--dim` | `#8a93a6` | `#5c6678` | Lyric lines that aren't being sung. |
| `--accent` | `#f5b544` | `#f5b544` | The main action's background. Text on it uses `--on-accent`. |
| `--on-accent` | `#0f1420` | `#0f1420` | Text on the accent. |
| `--link` | `#f5b544` | `#8a5100` | Amber text: links, list tiles, and the focus ring. On light, the accent is too pale to read as text, so this is a deeper amber. |
| `--danger` | `#ff8a80` | `#b42318` | Errors and the delete action. |

Every text colour has at least 4.5:1 contrast on the surfaces it sits on, in both themes, and the focus ring at least 3:1. A new colour needs a value for both themes that passes the same checks. Check every UI change in both themes.

## Type

- The system font stack, with Hiragino Sans, Noto Sans JP, or Yu Gothic for Japanese. No web fonts yet: fonts must be bundled, never loaded from a CDN, and the system fonts already cover Japanese well.
- Body text and form fields are at least 16px.
- Lyrics: other lines at about 18px, the current line at about 26px and bold. Leave line height for furigana.

## Shared pieces

All in `frontend/src/app.css`:

- `.screen`: one column, centered and capped at 640px on wide screens, padded for the safe areas.
- `.actions`: the bar at the bottom of a screen, within thumb reach. It sticks to the bottom while the page scrolls.
- `.button`, `.button.primary`, `.button.danger`: buttons and button-styled links, at least 52px tall.
- `.icon-button`: a 44px round button for an icon, such as Back. It always has an `aria-label`.
- `.alert`, `.muted`: an error message, and secondary text.

## Mockups

The design came from the mockup canvas "Utanabi mobile mockups" on claude.ai (row B, "Night stage"). The canvas also has other styles worth borrowing from later, such as colour-coded grammar and a word-by-word gloss.
