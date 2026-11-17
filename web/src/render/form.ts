import { h } from "./dom";

// Each is recorded in evals/recordings, so the chips work in replay mode, with no API key.
export const EXAMPLE_QUESTIONS = [
  "Can a refresh token be bound to a client?",
  "What claims must a JWT access token contain?",
  "How should a native app receive the authorization response?",
  "Quote RFC 9999 §3 on OAuth token lifetimes.",
];

export interface AskForm {
  el: HTMLElement;
  setStreaming: (streaming: boolean) => void;
}

export function createAskForm(handlers: {
  onAsk: (q: string) => void;
  onStop: () => void;
}): AskForm {
  const input = h("textarea", {
    id: "question",
    name: "question",
    rows: "2",
    maxlength: "1000",
    placeholder: "e.g. Can a refresh token be bound to a client?",
    "aria-describedby": "question-hint",
    required: true,
  });
  const ask = h("button", { type: "submit", class: "button button--primary" }, "Ask");
  const stop = h("button", { type: "button", class: "button", disabled: true }, "Stop");
  const chips = EXAMPLE_QUESTIONS.map((q) => {
    const chip = h("button", { type: "button", class: "example" }, q);
    chip.addEventListener("click", () => {
      input.value = q;
      submit();
    });
    return chip;
  });
  const form = h(
    "form",
    { class: "ask", "aria-label": "Ask a question" },
    h("label", { for: "question", class: "ask__label" }, "Ask about the OAuth RFCs"),
    h("div", { class: "ask__box" }, input),
    h(
      "div",
      { class: "ask__row" },
      h(
        "p",
        { id: "question-hint", class: "ask__hint" },
        "Enter to ask · Shift+Enter for a new line",
      ),
      h("div", { class: "ask__buttons" }, stop, ask),
    ),
    h(
      "div",
      { class: "examples", role: "group", "aria-label": "Example questions" },
      h("span", { class: "examples__label", "aria-hidden": "true" }, "Try"),
      ...chips,
    ),
  );

  function submit(): void {
    const q = input.value.trim();
    if (q === "" || ask.disabled) return;
    handlers.onAsk(q);
  }

  form.addEventListener("submit", (e) => {
    e.preventDefault();
    submit();
  });
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      submit();
    }
  });
  stop.addEventListener("click", () => {
    handlers.onStop();
  });

  function setStreaming(streaming: boolean): void {
    // Disabling a focused control drops focus to <body>; hand it to the control that stays useful.
    const active = document.activeElement;
    const hadFocus = active === input || active === ask || chips.some((c) => c === active);
    const stopHadFocus = active === stop;
    input.disabled = streaming;
    ask.disabled = streaming;
    for (const c of chips) c.disabled = streaming;
    stop.disabled = !streaming;
    if (streaming && hadFocus) stop.focus();
    if (!streaming && stopHadFocus) input.focus();
  }

  return { el: form, setStreaming };
}
