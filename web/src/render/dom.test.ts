// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { h, setText } from "./dom";

describe("h", () => {
  it("sets attributes, skipping false and undefined", () => {
    const el = h("button", { class: "ask", disabled: true, hidden: false, title: undefined });
    expect(el.getAttribute("class")).toBe("ask");
    expect(el.getAttribute("disabled")).toBe("");
    expect(el.hasAttribute("hidden")).toBe(false);
    expect(el.hasAttribute("title")).toBe(false);
  });

  it("appends strings as text, never as markup", () => {
    const el = h("p", {}, "<img src=x onerror=alert(1)>", null, false, h("b", {}, "bold"));
    expect(el.querySelector("img")).toBeNull();
    expect(el.textContent).toBe("<img src=x onerror=alert(1)>bold");
    expect(el.children).toHaveLength(1);
  });
});

describe("setText", () => {
  it("leaves the node alone when the text is unchanged", () => {
    const el = h("p", {}, "same");
    const node = el.firstChild;
    setText(el, "same");
    expect(el.firstChild).toBe(node);
    setText(el, "new");
    expect(el.textContent).toBe("new");
  });
});
