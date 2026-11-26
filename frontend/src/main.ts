import { mount } from "svelte";

import App from "./App.svelte";
import "./app.css";
import { applyTheme, loadTheme } from "./lib/theme";

applyTheme(loadTheme());

const target = document.getElementById("app");
if (!target) {
  throw new Error("missing #app element");
}

mount(App, { target });
