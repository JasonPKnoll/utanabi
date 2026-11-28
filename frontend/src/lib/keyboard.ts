// iOS raises the on-screen keyboard only for a focus that happens during a tap. A tap that opens a screen whose
// field focuses itself on arrival, such as search, loses that: the focus comes a moment later and the keyboard stays
// down. So the tap focuses this stand-in field first, outside the screens so it isn't removed with the old one: the
// keyboard comes up, and when the new screen focuses its own field, iOS moves the keyboard over to it. The stand-in
// removes itself then, or after a moment if nothing takes the focus.
export function holdKeyboard(doc: Document = document): void {
  const stand = doc.createElement("input");
  stand.setAttribute("aria-hidden", "true");
  stand.setAttribute("autocomplete", "off");
  stand.tabIndex = -1;
  stand.dataset.keyboardHold = "";
  // 16px so iOS doesn't zoom in on it; fixed and see-through so nothing moves or shows.
  Object.assign(stand.style, {
    position: "fixed",
    top: "0",
    left: "0",
    width: "1px",
    height: "1px",
    padding: "0",
    border: "0",
    opacity: "0",
    fontSize: "16px",
  });
  doc.body.appendChild(stand);
  stand.focus({ preventScroll: true });

  const done = () => {
    doc.removeEventListener("focusin", moved);
    clearTimeout(timer);
    stand.remove();
  };
  const moved = (event: FocusEvent) => {
    if (event.target !== stand) {
      done();
    }
  };
  doc.addEventListener("focusin", moved);
  const timer = setTimeout(done, 1500);
}
