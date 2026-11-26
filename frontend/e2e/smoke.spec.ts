import { execFileSync } from "node:child_process";

import { expect, test, type Browser, type Page } from "@playwright/test";

const signupCode = "smoke-test-sign-up-code";

// Gives the page a passkey authenticator, as Chrome's DevTools can: a real WebAuthn authenticator in software, which
// says yes to Face ID or a fingerprint on its own.
async function addAuthenticator(page: Page) {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: {
      protocol: "ctap2",
      transport: "internal",
      hasResidentKey: true,
      hasUserVerification: true,
      isUserVerified: true,
      automaticPresenceSimulation: true,
    },
  });
}

// Makes a new account with a passkey and leaves the page signed in, on the library. Returns the username.
async function signUp(page: Page): Promise<string> {
  const name = `smoke-${Date.now()}-${Math.floor(Math.random() * 1e6)}`.slice(0, 32);
  await addAuthenticator(page);
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Welcome to Utanabi" })).toBeVisible();
  await page.getByLabel("Choose a username").fill(name);
  await page.getByLabel("Invite code").fill(signupCode);
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: "Save passkey" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Library");
  return name;
}

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
  await signUp(page);

  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Library");
  await page.getByRole("link", { name: "About Utanabi" }).click();
  await expect(page.getByTestId("about")).toContainText("server");
  expect(errors).toEqual([]);
});

test("a chosen mode and palette override the defaults and are remembered", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await signUp(page);
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
  await signUp(page);
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
  await signUp(page);
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
  await signUp(page);

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
  await signUp(page);
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

test("service calls need a signed-in user", async ({ request }) => {
  const res = await request.post("/wails/runtime", {
    headers: { Origin: "http://localhost:18080", "Content-Type": "application/json" },
    data: {},
  });
  expect(res.status()).toBe(401);
  expect((await res.json()).cause.code).toBe("unauthorized");
});

test("signing out, then back in with the same passkey", async ({ page, request }) => {
  const errors: string[] = [];
  page.on("pageerror", (err) => errors.push(err.message));
  const name = await signUp(page);
  const session = (await page.context().cookies()).find((c) => c.name === "utanabi_session")?.value;
  expect(session).toBeTruthy();

  await page.getByRole("link", { name: "About Utanabi" }).click();
  await expect(page.getByText(`Signed in as ${name}`)).toBeVisible();
  await page.getByRole("button", { name: "Sign out" }).click();

  // The old session is over on the server, not just forgotten by the browser.
  await expect
    .poll(async () => {
      const res = await request.post("/wails/runtime", {
        headers: { Origin: "http://localhost:18080", Cookie: `utanabi_session=${session}` },
        data: {},
      });
      return res.status();
    })
    .toBe(401);

  // The same passkey signs back in: from the username field's autofill, which Chrome's test authenticator answers
  // by itself (a real phone waits for the person to pick it), or from the button.
  const button = page.getByRole("button", { name: "Sign in with a passkey" });
  if (await button.isVisible()) {
    await button.click().catch(() => {});
  }
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Library");
  expect(errors).toEqual([]);
});

test("a new account needs the sign-up code", async ({ page }) => {
  await addAuthenticator(page);
  await page.goto("/");
  // A first visit leads with making an account, and the sign-in field offers saved passkeys in its autofill.
  await expect(page.getByRole("heading", { name: "Welcome to Utanabi" })).toBeVisible();
  await expect(page.getByLabel("Username", { exact: true })).toHaveAttribute("autocomplete", "username webauthn");
  await page.getByLabel("Choose a username").fill("smoke-wrong-code");
  await page.getByLabel("Invite code").fill("not the code at all");
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page.getByText("code isn't right")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Welcome to Utanabi" })).toBeVisible();
});

test("each user's library is their own", async ({ browser }) => {
  const hana = await browser.newPage();
  await signUp(hana);
  const title = `私の歌 ${Date.now()}`;
  await hana.getByRole("link", { name: "Add your own lyrics" }).click();
  await hana.getByLabel("Song title").fill(title);
  await hana.getByLabel("Lyrics").fill("夜の空に星が光る");
  await hana.getByRole("button", { name: "Save song" }).click();
  await expect(hana.getByRole("heading", { level: 1 })).toHaveText(title);
  const songPath = new URL(hana.url()).hash;

  const kaze = await (await browser.newContext()).newPage();
  await signUp(kaze);
  await expect(kaze.getByText(title)).toHaveCount(0);
  await kaze.goto(`/${songPath}`);
  await expect(kaze.getByText("This song isn't in your library")).toBeVisible();

  await hana.close();
  await kaze.close();
});

// Makes a user an admin with the admin command, as the server's owner would on its machine.
function makeAdmin(name: string) {
  execFileSync(
    "go",
    ["run", "./cmd/admin", "-data", process.env.SMOKE_DATA_DIR ?? ".task/smoke-data", "-grant", name],
    {
      cwd: "..",
      stdio: "pipe",
    },
  );
}

async function newcomer(browser: Browser): Promise<Page> {
  const page = await (await browser.newContext()).newPage();
  await addAuthenticator(page);
  return page;
}

test("an admin's invite link signs one new person up", async ({ browser }) => {
  test.setTimeout(120_000);
  const admin = await browser.newPage();
  const name = await signUp(admin);

  // Not an admin yet: invites are off limits.
  await admin.goto("/#/invites");
  await expect(admin.getByText("Only an admin can manage invites.")).toBeVisible();

  makeAdmin(name);
  // The page learns who's an admin when it loads.
  await admin.goto("/#/about");
  await admin.reload();
  await expect(admin.getByText("Admin", { exact: true })).toBeVisible();
  await admin.getByRole("link", { name: "Manage invites" }).click();
  await admin.getByRole("button", { name: "Make an invite link" }).click();
  const link = ((await admin.getByTestId("invite-link").textContent()) ?? "").trim();
  expect(link).toMatch(/#\/join\/[a-z2-9]{4}(-[a-z2-9]{4}){3}$/);

  // The link opens sign-up with the code filled in.
  const guest = await newcomer(browser);
  await guest.goto(link);
  await expect(guest.getByLabel("Invite code")).toHaveValue(/^[a-z2-9]{4}(-[a-z2-9]{4}){3}$/);
  const guestName = `guest-${Date.now()}`;
  await guest.getByLabel("Choose a username").fill(guestName);
  await guest.getByRole("button", { name: "Continue" }).click();
  await guest.getByRole("button", { name: "Save passkey" }).click();
  await expect(guest.getByRole("heading", { level: 1 })).toContainText("Library");

  // It worked once; it doesn't work again.
  const late = await newcomer(browser);
  await late.goto(link);
  await late.getByLabel("Choose a username").fill(`late-${Date.now()}`);
  await late.getByRole("button", { name: "Continue" }).click();
  await expect(late.getByText(/invite has been used or has run out/)).toBeVisible();

  await admin.reload();
  await expect(admin.getByText(`Used by ${guestName}`)).toBeVisible();
});
