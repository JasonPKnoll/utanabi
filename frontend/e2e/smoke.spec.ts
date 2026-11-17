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
  await expect(page.getByRole("button", { name: "夜の空に" })).toHaveAttribute("aria-current", "true");
  await page.getByRole("button", { name: "星が光る" }).click();
  await expect(page.getByRole("button", { name: "星が光る" })).toHaveAttribute("aria-current", "true");

  await page.getByRole("link", { name: "Back to your songs" }).click();
  await expect(page.getByRole("link", { name: new RegExp(title) })).toBeVisible();

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
