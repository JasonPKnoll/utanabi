# Design

Utanabi's look is **Night stage**: the feel of singing along on a dark stage. The line being sung is big and bright. The rest of the song stays dim, so the eye always knows where it is. One warm amber accent marks what you can act on.

Every screen is built for a phone first. The rules are in "Mobile first" in [AGENTS.md](../AGENTS.md).

## Colours

The tokens live on `:root` in `frontend/src/app.css`. Components use the tokens, never their own hex values.

| Token | Value | Use |
| --- | --- | --- |
| `--bg` | `#0f1420` | The page. |
| `--surface` | `#18202f` | Cards, fields, and secondary buttons. |
| `--surface-strong` | `#273044` | Tiles, and hover on surfaces. |
| `--border` | `#273044` | Borders. |
| `--fg` | `#f2f4f8` | Body text and the current lyric line. |
| `--muted` | `#a3acbe` | Labels and secondary text. |
| `--dim` | `#8a93a6` | Lyric lines that aren't being sung. Still passes 4.5:1 contrast on `--bg`. |
| `--accent` | `#f5b544` | The main action and links. Text on it uses `--on-accent`. |
| `--on-accent` | `#0f1420` | Text on the accent. |
| `--danger` | `#ff8a80` | Errors and the delete action. |

The theme is dark only. Any new colour must keep text at 4.5:1 contrast against what it sits on.

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
