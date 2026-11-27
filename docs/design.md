# Design

Utanabi's look is **Night stage**: the feel of singing along on a dark stage. The line being sung is big and bright. The rest of the song stays dim, so the eye always knows where it is. One warm amber accent marks what you can act on.

Every screen is built for a phone first. The rules are in "Mobile first" in [AGENTS.md](../AGENTS.md).

## Theme: palette and mode

The look is two choices, both in the Theme control (`frontend/src/components/ThemePicker.svelte`), which sits in the song's Display sheet and on About:

- **Palette:** Utanabi (the default, Night stage), Tokyo Night (Night, with Day for light), Nord (Polar Night, with Snow Storm for light), Catppuccin (Mocha, with Latte for light), Solarized, and Vivid (hot pink and cyan). Each is a swatch showing its own dark and light colours. Every palette has both a dark and a light set, keeping the same idea: the sung line is the strongest text, and the rest of the song is softer.
- **Mode:** Auto, Light, or Dark, as a segmented control. Auto follows the device's setting (`prefers-color-scheme`); dark is the default when a device has no preference.

Both are radio buttons, so they work with a tap and a keyboard. `frontend/src/lib/theme.ts` sets `data-palette` and `data-theme` on `<html>`, and the browser's bar colour (`theme-color`) follows the page. The choices are remembered on the device in `localStorage`; Utanabi and Auto stand in when storage is blocked.

## Colours

Each palette lives in `frontend/src/themes.css` as one block of dark values (`--d-bg`, `--d-fg`, and so on) and light values (`--l-*`), selected by `[data-palette="…"]`; Utanabi also sits on `:root`. `frontend/src/app.css` maps them onto the tokens components use (`--bg: var(--d-bg)`), switching to the `--l-*` values for light mode. Components use the tokens, never their own hex values, so every screen gets every palette in both modes for free.

**Adding a palette:** add its block to `themes.css` with all 30 values, and its name to `palettes` in `theme.ts`. `theme.test.ts` fails until every palette has every value and every text colour keeps 4.5:1 contrast on the surfaces it's drawn on, in both modes.

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

- **Each word is a unit:** its Japanese, with furigana, over its own English (`Word.short`: "sky", "'s", "(subject)"). Units wrap like words in a sentence, centred, and line up on the baseline of their Japanese, so a word with furigana sits level with one without, and so does its English. (Safari grows a line for its ruby however much line height there is, so lining up by the top doesn't hold.)
- **The English is set apart from the Japanese:** 14px, weight 500, in the system Latin face and `--muted`. A particle's or grammar word's English is its role, in `--link` amber italics, so grammar never blurs into meaning.
- **Each line is its own soft card** (`--surface` at about half strength, no outline) with space between cards. The current line's card is full `--surface` with a thin `--link` ring, and its words are the 44px buttons that open the word sheet, underlined in dotted amber.
- Punctuation sits close to its neighbours with no English under it.
- English, numbers, and other text with no Japanese in it (an English chorus, "Monday") stay as they're sung: one plain run of text that wraps, with nothing under it and nothing to tap.

A word's English is never just the word again. Without a meaning, a conjugated word shows its dictionary form (探す under 探して), and a word already in that form shows nothing under it. Particles always show their role.

## Tapping a word

A tap on a line makes it the current line, as before. In the current line each word is its own button, at least 44px each way, marked with a dotted amber underline (`--link`) so it reads as tappable without hover. Punctuation between words is plain text. So the two taps never clash: the first tap picks the line, and a tap on a word in the current line opens the word sheet. Every word button works with a keyboard too.

The word sheet (`frontend/src/components/WordSheet.svelte`) is a bottom sheet, after mockup A, "Ink reader", built on the shared `Sheet` component (`frontend/src/components/Sheet.svelte`), as the Display sheet is. It is a native `<dialog>` opened with `showModal()`, so focus moves into it (to the Close button), stays there, and goes back to the word when it closes. Escape, the Close button, and a tap on the dimmed backdrop close it. It is padded for the safe areas and scrolls inside itself, up to 85% of the screen.

It shows, top to bottom: the word large with its furigana; its dictionary form, reading, and romaji; a sentence about its conjugation ("探す, in the 〜ている form"); for a particle or set phrase, its grammar title, meaning, and explanation; for a content word, the meaning used in this song first, in a card outlined in `--link`, then the other meanings with their parts of speech in words; each conjugation step with its title and meaning; for a content word or set phrase, how it's sung; and, when meanings show, the one-line JMdict footnote. When `dictionary.db` isn't installed, the meanings part says "Meanings aren't installed" instead of failing.

**Sung differently** (`frontend/src/components/SungReading.svelte`). Lyrics sometimes give a word a reading of their own, printed as furigana in the official lyrics: gikun (義訓), such as 羅針盤 read コンパス. Plain-text lyrics lose that furigana. The sheet ends with a quiet link, "Sung differently from how it's written?", that opens a field for the sung reading, typed in kana or in romaji that turns into kana as it's typed (`frontend/src/lib/sung.ts`), with a カタカナ / ひらがな choice that starts on katakana. Once set, the sheet shows it under "Sung as" with its romaji, Change, and Remove, and the word's furigana and romaji in the song follow it. It's saved for you alone (`Songs.SetSungReading`).

The sheet opens with its meanings already there. When a song opens, the app fetches the meanings of all its words in one call (`Songs.LookupEntries`) and keeps them in memory while it runs (`frontend/src/lib/meaningsCache.ts`), so a tap needs no call to Go. A word the song's call didn't cover, such as one with no dictionary entry, is looked up the first time it's tapped, then kept too. A tap while that first call is still under way waits for it rather than asking again.

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

## The mark: the visualizer peony

Utanabi's icon is a firework whose rays are a sound spectrum, around a small spinning record: amber, strong amber, and a pink spark, slightly rough as if drawn by hand. The concepts are on the canvas "Utanabi icon concepts" on claude.ai (Firework C).

- **The loader** (`frontend/src/components/Firework.svelte`, with its rays in `frontend/src/lib/firework.ts`): 44 thin rays, each jumping on its own rhythm, the flower turning slowly and kicking at 120 bpm, glitter twinkling past the tips, and an SVG filter that wobbles every line. It's drawn on the palette's dark colours (`--d-bg`, `--d-accent`, `--d-accent-strong`, `--d-spark`) in both modes, since a firework needs a night sky. With reduced motion on, it holds still and the wobble is left out.
- **Building a song** (`frontend/src/components/SongBuilding.svelte`): while Save song or a search result's Add runs, the screen shows the loader and "Building <title>". It shows at once and stays up for at least 1.2 seconds, one full bloom (`frontend/src/lib/hold.ts`), so a fast save shows the firework for a moment instead of a flicker. Opening a song shows a smaller one, "Opening the song…", only when the open takes more than a third of a second, as it does when an older song is built on first open.
- **`--spark`** is the pink of the mark, one per palette. It's a graphics colour, never text: `theme.test.ts` checks it, the accent, and the strong accent at 3:1 on the dark surfaces.
- **The app icon** is `build/appicon.svg`: 24 rays alternating long and short, so it reads as a spectrum when small. `build/appicon.png` and the Android launcher icons are rendered from it, and `wails3 task common:generate:icons` makes the macOS `.icns` and Windows `.ico` from the PNG. The Icon Composer layer for macOS 26 is `build/appicon.icon/Assets/peony.svg`; building its `Assets.car` needs Xcode's `actool`, so `build/darwin/Assets.car` is left out until it's made on a Mac with Xcode, and macOS uses the `.icns` meanwhile.
- **The favicon** is `frontend/public/favicon.svg` (12 thick rays) with PNGs at 32 and 16 pixels; the 16 comes from `build/favicon-16.svg`, which has only 8 rays. `apple-touch-icon.png` is the app icon at 180.

## Mockups

The design came from the mockup canvas "Utanabi mobile mockups" on claude.ai (row B, "Night stage"). The canvas also has other styles worth borrowing from later, such as colour-coded grammar and a word-by-word gloss.

## Study

A song's flashcards, opened from the cards button in the song's top bar. Each card is one word with the meaning it has in the song. You look them over first, then take the quiz.

### The word list

`frontend/src/screens/Study.svelte`, `#/song/<id>/study`. Looking changes no progress; only the quiz does.

- **The list** shows every card in the order it's first sung: the word, its reading under it (when it has kanji), the song's meaning, and its streak as dots. A segmented control picks "To learn" (cards not learned yet, the default) or "All" (the default once every card is learned). The helpers are in `frontend/src/lib/studyList.ts`.
- **One card at a time** (`#/song/<id>/study/<cardId>`), opened by tapping a row or "Flip through": the reading in amber over the word, large, then the song's meaning, the word's other meanings, the line it's sung in with the word underlined, and its progress ("Not quizzed yet", "1 of 3 in a row", "Learned"). Previous and Next sit in the bottom bar, and a sideways swipe or the arrow keys do the same. Stepping replaces the card in the history, so Back returns to the list, which scrolls to the last card seen. On the last card, Next becomes "Quiz me".
- **"Quiz me"** sits in the bottom bar of the list.

### The quiz

`frontend/src/screens/Quiz.svelte`, `#/song/<id>/quiz`. It asks the way WaniKani asks:

- **The front** is the word in its dictionary form, large, over the line it's first sung in with the word underlined in dotted amber.
- **Reading, then meaning.** The reading is typed in kana: WanaKana's kana input turns romaji into hiragana as it's typed (`naoru` becomes なおる), and kana from a Japanese keyboard goes straight in. A word written only in kana has no reading to ask. The meaning is typed in English, and any of the word's meanings counts, ignoring case, punctuation, notes in parentheses, a leading "to", "be", "a", "an", or "the", and a small typo (`frontend/src/lib/answers.ts`).
- **Right goes straight on; wrong waits.** A wrong answer turns the field red and shows the right one, with "I was right" for a typo or synonym the check missed. Enter goes on either way.
- **The back** shows the reading, the song's meaning, and the card's streak as dots ("1 of 3 in a row"). A card is right only when every question on it is. Enter, or Next card, saves the answer (`Study.Answer`).
- **The rotation** (`frontend/src/lib/studyQueue.ts`) is a shuffled queue of the cards not learned yet: a right card goes to the back, a wrong one comes back after three others, and a learned one leaves. When every card is learned, "Review all again" goes round them all.
- **One answer field for the whole session,** kept focused even while an answer shows, so a phone's keyboard stays up from card to card.

The top bar counts learned cards ("12/39"), with a bar in `--success` under it. Right is `--success` and wrong is `--danger`, in every palette.

## Sign-in

On a server with accounts (`frontend/src/screens/SignIn.svelte`), nothing shows until someone signs in. The screen is one column, centred: the うたなび mark, a heading, and a line saying it's a passkey (Face ID, a fingerprint, or the device's PIN) with no password. Then two cards:

- **Make your account:** a line on what happens (pick a username, enter the sign-up code, and the device saves a passkey: no password and no QR code), "Choose a username", "Sign-up code", and "Continue", which checks both. Then "One more step" and **Save passkey**, a tap of its own, which opens the device's passkey sheet straight away: Safari refuses a passkey that isn't asked for straight from a tap. The passkey is made on this device first (the `client-device` hint), so on an iPhone it's one Face ID. "Change username" goes back.
- **Sign in:** a "Username" field whose autofill offers the site's saved passkeys (`autocomplete="username webauthn"`), and "Sign in with a passkey". On a browser that has signed in before, a sign-in waits in the background for a passkey picked from that autofill (WebAuthn's conditional mediation). A first visit never starts one, because Safari won't save a new passkey while one is waiting. The button also offers a passkey from another device, by QR code.

A browser that has never signed in leads with making an account ("Welcome to Utanabi"); one that has (remembered in `localStorage`) leads with signing in ("Sign in to Utanabi"). When the server takes no new accounts, the account card is a line saying so. Field errors show under their field; anything else shows as an alert above the cards.

An **invite link** (`#/join/<code>`) opens this screen leading with making an account, with the invite code filled in: the newcomer picks a username, taps Continue, then Save passkey.

The About screen gains an **Account** section when signed in: "Signed in as hana", a note to add a passkey on each device used, "Add a passkey on this device", and "Sign out", which goes back to the sign-in screen at the library's address. A call that finds the session has ended (`unauthorized`) goes back to the sign-in screen too. The desktop and phone apps have no accounts and never show any of this.

## Invites

For admins (`frontend/src/screens/Invites.svelte`, `#/invites`, from "Manage invites" in About's Account section, beside an Admin chip). "Make an invite link" makes an invite and shows its link, once, with **Share** (the phone's share sheet, where there is one) and **Copy link**, and a note that it works once. Under it, "All invites" lists each one: when it was made and by whom, and whether it's unused (until when), ran out, or was used by whom. An unused one has **Revoke**. Anyone else who opens the screen is told only an admin can manage invites.
