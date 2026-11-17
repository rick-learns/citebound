// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ClaimState } from "../state";
import { renderClaim } from "./claim";

const XSS = "<img src=x onerror=alert(1)>";

const claim = (patch: Partial<ClaimState>): ClaimState => ({
  id: 0,
  text: "Refresh tokens are bound to the client.",
  ref: "RFC6749 §10.4 ¶2",
  quote: "MUST maintain the binding",
  status: "pending",
  reason: "",
  ...patch,
});

afterEach(() => {
  document.body.replaceChildren();
});

describe("renderClaim", () => {
  it("renders a pending claim without an interactive chip", () => {
    const li = renderClaim(claim({}), vi.fn());
    expect(li.classList.contains("claim--pending")).toBe(true);
    expect(li.querySelector("button")).toBeNull();
    expect(li.querySelector(".claim__status")?.textContent).toBe("Checking quote…");
  });

  it("renders a verified claim with a clickable citation chip", () => {
    const onCite = vi.fn();
    const li = renderClaim(claim({ status: "verified" }), onCite);
    const chip = li.querySelector("button");
    expect(li.querySelector(".claim__icon")?.textContent).toBe("✓");
    expect(chip?.textContent).toBe("RFC6749 §10.4 ¶2");
    chip?.click();
    expect(onCite).toHaveBeenCalledWith(
      { ref: "RFC6749 §10.4 ¶2", quote: "MUST maintain the binding" },
      chip,
    );
  });

  it("renders a struck claim struck through, with the reason in words", () => {
    const li = renderClaim(claim({ status: "struck", reason: "quote-not-found" }), vi.fn());
    expect(li.querySelector(".claim__text s")?.textContent).toBe(
      "Refresh tokens are bound to the client.",
    );
    expect(li.querySelector("button")).toBeNull();
    expect(li.querySelector(".claim__status")?.textContent).toBe(
      "Struck: quote not found in RFC6749 §10.4 ¶2",
    );
  });

  it("labels uncited claims", () => {
    const li = renderClaim(claim({ ref: "", status: "struck", reason: "uncited" }), vi.fn());
    expect(li.querySelector(".chip")?.textContent).toBe("no citation");
  });

  it("renders server text as text, never as HTML", () => {
    const li = renderClaim(claim({ text: XSS, ref: XSS, status: "struck" }), vi.fn());
    expect(li.querySelector("img")).toBeNull();
    expect(li.querySelector(".claim__text")?.textContent).toBe(XSS);
    expect(li.textContent).toContain(XSS);
  });
});
