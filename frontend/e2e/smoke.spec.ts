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
