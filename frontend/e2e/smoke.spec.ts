import { expect, test } from "@playwright/test";

test("health returns 200", async ({ request }) => {
  const res = await request.get("/health");
  expect(res.status()).toBe(200);
});

test("the UI loads and the bound method answers", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));

  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Utanabi");
  await expect(page.getByTestId("about")).toContainText("server");
  expect(errors).toEqual([]);
});

test("a song with your own lyrics can be added, read, and deleted", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));
  // The smoke data folder survives between runs, so each run uses its own title.
  const title = `星の歌 ${Date.now()}`;

  await page.goto("/");
  await page.getByRole("link", { name: "Add your own", exact: true }).click();

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
  await expect(page.getByRole("button", { name: "Romaji" })).toHaveAttribute("aria-pressed", "false");
  await page.getByRole("button", { name: "Romaji" }).click();
  await expect(night.getByText("yoru no sora ni")).toBeVisible();
  await page.getByRole("button", { name: "Furigana" }).click();
  await expect(page.getByRole("button", { name: "Furigana" })).toHaveAttribute("aria-pressed", "false");
  await expect(night.locator("rt")).toHaveCount(0);
  await expect(night).toContainText("夜の空に");

  await page.reload();
  await expect(page.getByRole("button", { name: "Romaji" })).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("button", { name: "Furigana" })).toHaveAttribute("aria-pressed", "false");
  await page.getByRole("button", { name: "Furigana" }).click();
  // After a reload the first line is current again.
  await expect(page.getByRole("group", { name: "夜の空に" }).locator("rt").first()).toHaveText("よる");

  await page.getByRole("link", { name: "Back to your songs" }).click();
  await expect(page.getByRole("link", { name: new RegExp(title) })).toBeVisible();

  await page.getByRole("link", { name: new RegExp(title) }).click();
  await page.getByRole("button", { name: "Delete song…" }).click();
  await page.getByRole("button", { name: "Delete song", exact: true }).click();
  await expect(page.getByRole("link", { name: new RegExp(title) })).toHaveCount(0);

  expect(errors).toEqual([]);
});

// The smoke server has no dictionary.db, so particles are explained from the built-in grammar list and content words
// say their meanings aren't installed.
test("words show a word-by-word gloss and open a sheet when tapped", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));
  const title = `言葉の歌 ${Date.now()}`;

  await page.goto("/");
  await page.getByRole("link", { name: "Add your own", exact: true }).click();
  await page.getByLabel("Song title").fill(title);
  await page.getByLabel("Lyrics").fill("夜の空に星が光る\n君の声を探している");
  await page.getByRole("button", { name: "Save song" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(title);

  const words = page.getByRole("button", { name: "Words" });
  await expect(words).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByText("夜 · 's · 空 · in · 星 · (subject) · 光る")).toHaveCount(0);
  await words.click();
  await expect(words).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("夜 · 's · 空 · in · 星 · (subject) · 光る")).toBeVisible();
  await expect(page.getByText("君 · 's · 声 · (object) · 探す")).toBeVisible();

  const line = page.getByRole("group", { name: "夜の空に星が光る" });
  await line.getByRole("button", { name: "星", exact: true }).click();
  const sheet = page.getByRole("dialog");
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole("heading", { level: 2 })).toContainText("星");
  await expect(sheet.getByText("Meanings aren't installed")).toBeVisible();
  await sheet.getByRole("button", { name: "Close" }).click();
  await expect(sheet).toHaveCount(0);

  await line.getByRole("button", { name: "が", exact: true }).click();
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
  await expect(page.getByRole("button", { name: "Words" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Words" }).click();

  await page.getByRole("link", { name: "Back to your songs" }).click();
  await page.getByText("Credits").click();
  await expect(page.getByText(/Electronic Dictionary Research and Development Group/)).toBeVisible();
  await page.getByRole("link", { name: new RegExp(title) }).click();
  await page.getByRole("button", { name: "Delete song…" }).click();
  await page.getByRole("button", { name: "Delete song", exact: true }).click();
  await expect(page.getByRole("link", { name: new RegExp(title) })).toHaveCount(0);

  expect(errors).toEqual([]);
});

// Searching sends the query to LRCLIB, so CI checks only the parts that don't touch the network.
test("the search screen opens and asks for a query before searching", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));

  await page.goto("/");
  await page.getByRole("link", { name: "Find a song" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Find a song");

  await page.getByRole("button", { name: "Search" }).click();
  await expect(page.getByText("query is required")).toBeVisible();

  await page.getByRole("link", { name: "Can't find it? Add your own lyrics" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Add your own lyrics");
  expect(errors).toEqual([]);
});
