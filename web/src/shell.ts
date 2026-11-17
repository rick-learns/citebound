/** Renders the placeholder application shell into root. */
export function renderShell(root: HTMLElement): void {
  const heading = document.createElement("h1");
  heading.textContent = "Citebound";
  const tagline = document.createElement("p");
  tagline.textContent = "An RFC assistant that checks every quotation against pinned RFC text.";
  root.replaceChildren(heading, tagline);
}
