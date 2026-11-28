import { mount } from "svelte";

import App from "./App.svelte";
import "./themes.css";
import "./app.css";
import { applyLook, loadMode, loadPalette } from "./lib/theme";

applyLook(loadPalette(), loadMode());

const target = document.getElementById("app");
if (!target) {
  throw new Error("missing #app element");
}

mount(App, { target });
