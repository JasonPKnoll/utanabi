import { describe, expect, it } from "vitest";
import { chooseDevice, devicesToOffer, herePlayer, loadDevice, saveDevice } from "./devices";

const mac = { id: "mac", name: "MacBook Pro", type: "Computer", active: true };
const phone = { id: "phone", name: "iPhone", type: "Smartphone", active: false };
const here = { id: "page", name: "Utanabi", type: "Computer", active: false };
const otherPage = { id: "other-page", name: "Utanabi", type: "Computer", active: false };

describe("chooseDevice", () => {
  it("plays on the phone from a phone, even with the computer last played on", () => {
    expect(chooseDevice([mac, phone], "", true, "")).toBe(phone);
  });

  it("plays where Spotify last played from a computer", () => {
    expect(chooseDevice([phone, mac], "", false, "")).toBe(mac);
  });

  it("plays on the device you chose, when it's open", () => {
    expect(chooseDevice([mac, phone], "MacBook Pro", true, "")).toBe(mac);
    expect(chooseDevice([phone], "MacBook Pro", false, "")).toBe(phone);
  });

  it("plays in this page when you chose it, and never in another page", () => {
    expect(chooseDevice([mac, here, otherPage], herePlayer, false, "page")).toBe(here);
    expect(chooseDevice([otherPage], "", false, "page")).toBeNull();
  });

  it("has nowhere to play with no device open", () => {
    expect(chooseDevice([], "iPhone", true, "")).toBeNull();
  });
});

describe("devicesToOffer", () => {
  it("offers this page first and leaves other pages out", () => {
    expect(devicesToOffer([mac, otherPage, here, phone], "page")).toEqual([here, mac, phone]);
  });
});

describe("the remembered device", () => {
  it("is kept by name, and is empty when storage refuses", () => {
    const kept = new Map<string, string>();
    const store = () => ({
      getItem: (k: string) => kept.get(k) ?? null,
      setItem: (k: string, v: string) => void kept.set(k, v),
    });
    expect(saveDevice("iPhone", store)).toBe(true);
    expect(loadDevice(store)).toBe("iPhone");
    const refusing = () => {
      throw new Error("denied");
    };
    expect(loadDevice(refusing)).toBe("");
    expect(saveDevice("iPhone", refusing)).toBe(false);
  });
});
