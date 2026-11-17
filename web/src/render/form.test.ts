// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import recordings from "../../../evals/recordings/nemotron-3-ultra.jsonl?raw";
import { createAskForm, EXAMPLE_QUESTIONS } from "./form";

afterEach(() => {
  document.body.replaceChildren();
});

describe("createAskForm", () => {
  it("submits on Enter, not on Shift+Enter, and disables while streaming", () => {
    const onAsk = vi.fn();
    const form = createAskForm({ onAsk, onStop: vi.fn() });
    document.body.append(form.el);
    const input = form.el.querySelector("textarea");
    if (!input) throw new Error("no textarea");
    input.value = "Is PKCE required?";
    input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", shiftKey: true }));
    expect(onAsk).not.toHaveBeenCalled();
    input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
    expect(onAsk).toHaveBeenCalledWith("Is PKCE required?");

    form.setStreaming(true);
    expect(input.disabled).toBe(true);
    const stop = [...form.el.querySelectorAll("button")].find((b) => b.textContent === "Stop");
    expect(stop?.disabled).toBe(false);
  });

  it("asks an example question when its chip is clicked", () => {
    const onAsk = vi.fn();
    const form = createAskForm({ onAsk, onStop: vi.fn() });
    form.el.querySelector<HTMLButtonElement>(".example")?.click();
    expect(onAsk).toHaveBeenCalledWith("Can a refresh token be bound to a client?");
  });
});

describe("EXAMPLE_QUESTIONS", () => {
  it("are all recorded, so they work in replay mode", () => {
    for (const q of EXAMPLE_QUESTIONS) {
      expect(recordings, q).toContain(JSON.stringify(q).slice(1, -1));
    }
  });
});
