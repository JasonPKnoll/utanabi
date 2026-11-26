import { expect, test, type Page } from "@playwright/test";

// The song's reading aids and its delete button sit in the Display sheet, behind the Aa button.
async function openDisplay(page: Page) {
  await page.getByRole("button", { name: "Display options" }).click();
  await expect(page.getByRole("dialog", { name: "Display" })).toBeVisible();
}

async function closeDisplay(page: Page) {
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
}

async function deleteSong(page: Page) {
  await openDisplay(page);
  await page.getByRole("button", { name: "Delete song…" }).click();
  await page.getByRole("button", { name: "Delete song", exact: true }).click();
}

test("health returns 200", async ({ request }) => {
  const res = await request.get("/health");
  expect(res.status()).toBe(200);
});

test("the UI loads and the bound method answers", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));

  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Library");
  await page.getByRole("link", { name: "About Utanabi" }).click();
  await expect(page.getByTestId("about")).toContainText("server");
  expect(errors).toEqual([]);
});

test("a chosen mode and palette override the defaults and are remembered", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/#/about");
  const root = page.locator("html");
  const background = () => page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  await expect(page.getByRole("radio", { name: "Auto" })).toBeChecked();
  const dark = await background();

  await page.getByRole("radio", { name: "Light" }).check();
  await expect(root).toHaveAttribute("data-theme", "light");
  expect(await background()).not.toBe(dark);

  await page.reload();
  await expect(root).toHaveAttribute("data-theme", "light");
  await expect(page.getByRole("radio", { name: "Light" })).toBeChecked();

  await page.getByRole("radio", { name: "Auto" }).check();
  await expect(root).not.toHaveAttribute("data-theme");
  expect(await background()).toBe(dark);

  // A palette changes the colours in either mode, and is remembered too.
  await page.getByRole("radio", { name: "Nord" }).check();
  await expect(root).toHaveAttribute("data-palette", "nord");
  expect(await background()).toBe("rgb(46, 52, 64)");
  await page.reload();
  await expect(page.getByRole("radio", { name: "Nord" })).toBeChecked();
  expect(await background()).toBe("rgb(46, 52, 64)");
  await page.getByRole("radio", { name: "Utanabi" }).check();
  await expect(root).not.toHaveAttribute("data-palette");
  expect(await background()).toBe(dark);
});

test("a song with your own lyrics can be added, read, and deleted", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));
  // The smoke data folder survives between runs, so each run uses its own title.
  const title = `星の歌 ${Date.now()}`;

  await page.goto("/");
  await page.getByRole("link", { name: "Add your own lyrics" }).click();

  await page.getByRole("button", { name: "Save song" }).click();
  await expect(page.getByText("title is required")).toBeVisible();

  await page.getByLabel("Song title").fill(title);
  await page.getByLabel("Artist").fill("Demo Artist");
  await page.getByLabel("Lyrics").fill("夜の空に\n星が光る\n\n君の声を探している");
  await page.getByRole("button", { name: "Save song" }).click();

  await expect(page.getByRole("heading", { level: 1 })).toHaveText(title);
  // The current line is a group of word buttons; the other lines are buttons that make a line current.
  await expect(page.getByRole("group", { name: "夜の空に" })).toHaveAttribute("aria-current", "true");
  await page.getByRole("button", { name: "星が光る" }).click();
  await expect(page.getByRole("group", { name: "星が光る" })).toHaveAttribute("aria-current", "true");

  const night = page.getByRole("button", { name: "夜の空に" });
  await expect(night.locator("ruby").first()).toBeVisible();
  await expect(night.locator("rt").first()).toHaveText("よる");
  await openDisplay(page);
  const romaji = page.getByRole("switch", { name: "Romaji" });
  const furigana = page.getByRole("switch", { name: "Furigana" });
  await expect(romaji).toHaveAttribute("aria-checked", "false");
  await romaji.click();
  await furigana.click();
  await expect(furigana).toHaveAttribute("aria-checked", "false");
  await closeDisplay(page);
  await expect(night.getByText("yoru no sora ni")).toBeVisible();
  await expect(night.locator("rt")).toHaveCount(0);
  await expect(night).toContainText("夜の空に");

  await page.reload();
  await openDisplay(page);
  await expect(romaji).toHaveAttribute("aria-checked", "true");
  await expect(furigana).toHaveAttribute("aria-checked", "false");
  await furigana.click();
  await closeDisplay(page);
  // After a reload the first line is current again.
  await expect(page.getByRole("group", { name: "夜の空に" }).locator("rt").first()).toHaveText("よる");

  await page.getByRole("link", { name: "Back to your songs" }).click();
  await expect(page.getByRole("link", { name: new RegExp(title) })).toBeVisible();

  await page.getByRole("link", { name: new RegExp(title) }).click();
  await deleteSong(page);
  await expect(page.getByRole("link", { name: new RegExp(title) })).toHaveCount(0);

  expect(errors).toEqual([]);
});

// The smoke server has no dictionary.db, so particles are explained from the built-in grammar list and content words
// say their meanings aren't installed.
test("words show each word's English under it and open a sheet when tapped", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));
  const title = `言葉の歌 ${Date.now()}`;

  await page.goto("/");
  await page.getByRole("link", { name: "Add your own lyrics" }).click();
  await page.getByLabel("Song title").fill(title);
  await page.getByLabel("Lyrics").fill("夜の空に星が光る\n君の声を探している");
  await page.getByRole("button", { name: "Save song" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(title);

  const words = page.getByRole("switch", { name: "Words" });
  await expect(page.getByText("(subject)")).toHaveCount(0);
  await openDisplay(page);
  await expect(words).toHaveAttribute("aria-checked", "false");
  await words.click();
  await expect(words).toHaveAttribute("aria-checked", "true");
  await closeDisplay(page);

  // Each word's English sits under that word: the particle が carries "(subject)", and を "(object)" in the next line.
  const line = page.getByRole("group", { name: "夜の空に星が光る" });
  await expect(line.getByRole("button", { name: "が, (subject)" })).toBeVisible();
  await expect(line.getByRole("button", { name: "の, 's" })).toBeVisible();
  await expect(page.getByRole("button", { name: "君の声を探している" }).getByText("(object)")).toBeVisible();

  await line.getByRole("button", { name: "星", exact: true }).click();
  const sheet = page.getByRole("dialog");
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole("heading", { level: 2 })).toContainText("星");
  await expect(sheet.getByText("Meanings aren't installed")).toBeVisible();
  await sheet.getByRole("button", { name: "Close" }).click();
  await expect(sheet).toHaveCount(0);

  await line.getByRole("button", { name: "が, (subject)" }).click();
  await expect(sheet.getByText("Subject marker")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(sheet).toHaveCount(0);

  await page.getByRole("button", { name: "君の声を探している" }).click();
  const searching = page.getByRole("group", { name: "君の声を探している" });
  await searching.getByRole("button", { name: "探している" }).click();
  await expect(sheet.getByText("探す, in the 〜ている form")).toBeVisible();
  await expect(sheet.getByText("Ongoing action or state")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(sheet).toHaveCount(0);

  await page.reload();
  await openDisplay(page);
  await expect(words).toHaveAttribute("aria-checked", "true");
  await words.click();
  await closeDisplay(page);

  await page.getByRole("link", { name: "Back to your songs" }).click();
  await page.getByRole("link", { name: "About Utanabi" }).click();
  await expect(page.getByText(/Electronic Dictionary Research and Development Group/)).toBeVisible();
  await page.getByRole("link", { name: "Back to your songs" }).click();
  await page.getByRole("link", { name: new RegExp(title) }).click();
  await deleteSong(page);
  await expect(page.getByRole("link", { name: new RegExp(title) })).toHaveCount(0);

  expect(errors).toEqual([]);
});

// Searching sends the query to LRCLIB, so CI checks only the parts that don't touch the network.
test("the search screen opens and asks for a query before searching", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));

  await page.goto("/");
  await page.getByRole("link", { name: "Search songs or artists" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Find a song");
  await expect(page.getByRole("searchbox")).toBeFocused();

  await page.getByRole("button", { name: "Search", exact: true }).click();
  await expect(page.getByText("query is required")).toBeVisible();

  await page.getByRole("link", { name: "Back to your songs" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Library");
  expect(errors).toEqual([]);
});

// The smoke server has no dictionary.db, so a song's words have no meanings and there are no cards to study; the
// study flow itself is covered by the Go and unit tests.
test("a song's study screen opens from the song and says when there's nothing to study", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));
  const title = `勉強の歌 ${Date.now()}`;

  await page.goto("/");
  await page.getByRole("link", { name: "Add your own lyrics" }).click();
  await page.getByLabel("Song title").fill(title);
  await page.getByLabel("Lyrics").fill("夜の空に星が光る");
  await page.getByRole("button", { name: "Save song" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(title);

  await page.getByRole("link", { name: "Study this song's words" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Study");
  await expect(page.getByText(title)).toBeVisible();
  await expect(page.getByText("No words to study yet")).toBeVisible();
  await expect(page.getByText(/dictionary of word meanings/)).toBeVisible();

  await page.getByRole("link", { name: "Back to the song" }).first().click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(title);
  await deleteSong(page);
  expect(errors).toEqual([]);
});
