# Design

Utanabi's look is **Night stage**: the feel of singing along on a dark stage. The line being sung is big and bright. The rest of the song stays dim, so the eye always knows where it is. One warm amber accent marks what you can act on.

Every screen is built for a phone first. The rules are in "Mobile first" in [AGENTS.md](../AGENTS.md).

## Light and dark

The app follows the device's setting (`prefers-color-scheme`). Dark is the signature look and the default when a device has no preference. The light theme keeps the same idea: the sung line is the darkest, boldest text, and the rest of the song is a softer grey.

A Theme control (`frontend/src/components/ThemePicker.svelte`) offers Auto, Light, and Dark, as three radio buttons drawn as one segmented control. It sits in the song's Display sheet and on About. Auto follows the device. Light or Dark sets `data-theme` on `<html>` (`frontend/src/lib/theme.ts`), and the browser's bar colour (`theme-color`) follows. The choice is remembered on the device in `localStorage`, and Auto stands in when storage is blocked.

## Colours

The tokens live on `:root` in `frontend/src/app.css`, with the light values in its `prefers-color-scheme: light` block and again under `:root[data-theme="light"]` (keep the two the same). Components use the tokens, never their own hex values, so every screen gets both themes for free.

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
- Furigana (`<rt>`) is half the size of its line, never under 10px, in the line's own colour (`--muted` on the current line). Romaji sits under its line at 16px in `--dim` (`--muted` on the current line).

## Reading aids

The song view's "Aa" button, top right, opens the Display sheet (`frontend/src/components/DisplaySheet.svelte`). It holds three switches, "Furigana" (on at first), "Romaji" (off at first), and "Words" (off at first), each a `<button role="switch" aria-checked>` row with a one-line hint, so the lyrics get the whole screen. The choice is remembered on the device in `localStorage`, and the defaults stand in when storage is blocked. The same sheet says where the lyrics came from ("LRCLIB, synced") and ends with "Delete song…", which asks before it deletes.

"Words" turns each line into an interlinear gloss (`frontend/src/components/Interlinear.svelte`), so it's always clear which English belongs to which Japanese word and which line:

- **Each word is a unit:** its Japanese, with furigana, over its own English (`Word.short`: "sky", "'s", "(subject)"). Units wrap like words in a sentence, centred.
- **The English is set apart from the Japanese:** 14px, weight 500, in the system Latin face and `--muted`. A particle's or grammar word's English is its role, in `--link` amber italics, so grammar never blurs into meaning.
- **Each line is its own soft card** (`--surface` at about half strength, no outline) with space between cards. The current line's card is full `--surface` with a thin `--link` ring, and its words are the 44px buttons that open the word sheet, underlined in dotted amber.
- Punctuation sits close to its neighbours with no English under it.
- English, numbers, and other text with no Japanese in it (an English chorus, "Monday") stay as they're sung: one plain run of text that wraps, with nothing under it and nothing to tap.

A word's English is never just the word again. Without a meaning, a conjugated word shows its dictionary form (探す under 探して), and a word already in that form shows nothing under it. Particles always show their role.

## Tapping a word

A tap on a line makes it the current line, as before. In the current line each word is its own button, at least 44px each way, marked with a dotted amber underline (`--link`) so it reads as tappable without hover. Punctuation between words is plain text. So the two taps never clash: the first tap picks the line, and a tap on a word in the current line opens the word sheet. Every word button works with a keyboard too.

The word sheet (`frontend/src/components/WordSheet.svelte`) is a bottom sheet, after mockup A, "Ink reader", built on the shared `Sheet` component (`frontend/src/components/Sheet.svelte`), as the Display sheet is. It is a native `<dialog>` opened with `showModal()`, so focus moves into it (to the Close button), stays there, and goes back to the word when it closes. Escape, the Close button, and a tap on the dimmed backdrop close it. It is padded for the safe areas and scrolls inside itself, up to 85% of the screen.

It shows, top to bottom: the word large with its furigana; its dictionary form, reading, and romaji; a sentence about its conjugation ("探す, in the 〜ている form"); for a particle or set phrase, its grammar title, meaning, and explanation; for a content word, the meaning used in this song first, in a card outlined in `--link`, then the other meanings with their parts of speech in words; each conjugation step with its title and meaning; and, when meanings show, the one-line JMdict footnote. When `dictionary.db` isn't installed, the meanings part says "Meanings aren't installed" instead of failing.

## Screens

Each screen keeps its chrome slim so the lyrics and the library get the room.

- **Top bar:** every screen starts with a sticky, blurred `.topbar`, 56px tall: Back on the left, the title (and, on a song, its artist) in small type, and at most one action on the right.
- **Home** (`#/`): the うたなび mark and an About button in the top bar, a large "Library" title with the song count, and the songs as rows with a letter tile. Search and add sit together in a dock at the bottom, within thumb reach: a search bar that opens the search screen, and a round amber "+" for your own lyrics. With search switched off, the dock is one "Add your own lyrics" button.
- **Search** (`#/search`): the search field is the top bar, focused on arrival. Results are rows with a "Synced" chip and a round "+" to add, or a check when the song is already in the library. "Can't find it? Add your own lyrics" shows only after a search.
- **Song** (`#/song/<id>`): the lyrics, the "Aa" Display sheet, and nothing else.
- **About** (`#/about`): the app's name, version, and platform, then the sources in full.

## Credits

The full credits live on the About screen, reached from the info button on Home. It lists every source the app's data asks to credit, from `AboutService.Info`: LRCLIB, JMdict (EDRDG, CC BY-SA 4.0), and the IPA dictionary with its no-warranty notice.

Every other place credits JMdict in one quiet `.footnote` line, "Definitions from JMdict · Sources", linking to About. It shows under the lyrics while "Words" is on and the song has dictionary meanings, and at the foot of the word sheet when it shows meanings, since the licence asks for a credit on every page that shows dictionary words in server mode. Search ends with a matching footnote for LRCLIB. Never put a credit paragraph on a working screen; link to About instead.

## Shared pieces

All in `frontend/src/app.css`:

- `.screen`: one column, centered and capped at 640px on wide screens, padded for the safe areas.
- `.topbar`, `.topbar-title`: the sticky top bar every screen starts with.
- `.page-title`: a large screen title, such as "Library".
- `.actions`: the bar at the bottom of a screen, within thumb reach. It sticks to the bottom while the page scrolls.
- `.sheet`: the bottom sheet's `<dialog>`, used through `components/Sheet.svelte`.
- `.chip`, `.chip.accent`: a small label, such as "Synced".
- `.footnote`: a one-line credit in small muted type, centred.
- `.button`, `.button.primary`, `.button.danger`: buttons and button-styled links, at least 52px tall.
- `.icon-button`: a 44px round button for an icon, such as Back. It always has an `aria-label`.
- `.alert`, `.muted`: an error message, and secondary text.
- `.error`: a field's error message, shown under the field.
- `.empty`, `.empty-title`: an empty state, a card with a bold title and a line of help.
- `.visually-hidden`: text for screen readers only, such as a label the layout shows another way.

Use these instead of styling the same thing again in a component.

## Mockups

The design came from the mockup canvas "Utanabi mobile mockups" on claude.ai (row B, "Night stage"). The canvas also has other styles worth borrowing from later, such as colour-coded grammar and a word-by-word gloss.
