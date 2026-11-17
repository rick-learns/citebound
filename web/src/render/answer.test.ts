// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from "vitest";
import { initialState, reduce, type AnswerState } from "../state";
import { createAnswerView } from "./answer";

const XSS = "<img src=x onerror=alert(1)>";

afterEach(() => {
  document.body.replaceChildren();
});

describe("createAnswerView", () => {
  function stream(...events: Parameters<typeof reduce>[1][]): AnswerState {
    return events.reduce(reduce, reduce(initialState, { type: "start", question: XSS }));
  }

  it("says before the first question that a verified quote may not support its claim", () => {
    const view = createAnswerView(vi.fn());
    view.update(initialState);
    const intro = view.el.querySelector<HTMLElement>(".intro");
    expect(intro?.hidden).toBe(false);
    expect(intro?.textContent).toContain("it may not support the sentence");
  });

  it("streams claims in place and shows the summary", () => {
    const view = createAnswerView(vi.fn());
    document.body.append(view.el);
    const c = { id: 0, text: "A", ref: "RFC6749 §1.5 ¶1", quote: "q" };
    let s = stream({ type: "event", event: { type: "claim", data: c } });
    view.update(s);
    const first = view.el.querySelector(".claim");
    expect(first?.classList.contains("claim--pending")).toBe(true);

    s = reduce(s, { type: "event", event: { type: "claim", data: { ...c, id: 1, text: "B" } } });
    view.update(s);
    expect(view.el.querySelector(".claim")).toBe(first); // unchanged claim not re-rendered

    s = reduce(s, {
      type: "event",
      event: { type: "verdict", data: { id: 0, status: "verified", reason: "" } },
    });
    s = reduce(s, {
      type: "event",
      event: {
        type: "done",
        data: {
          mode: "extractive",
          model: "",
          claims: 2,
          verified: 1,
          struck: 0,
          latencyMs: 812,
          inputTokens: 0,
          outputTokens: 0,
          costUsd: 0,
        },
      },
    });
    view.update(s);
    const claims = [...view.el.querySelectorAll(".claim")].map((el) => el.className);
    expect(claims).toEqual(["claim claim--verified", "claim claim--unverified"]);
    expect(view.el.querySelector(".answer__summary")?.textContent).toBe(
      "1 of 2 claims verified · 812 ms · extractive",
    );
    // The question is echoed as text.
    expect(view.el.querySelector(".answer__question")?.textContent).toBe(XSS);
    expect(view.el.querySelector("img")).toBeNull();
  });

  const withheldNote = (el: HTMLElement) =>
    [...el.querySelectorAll<HTMLElement>(".notice--abstain p")].find((p) =>
      p.textContent.includes("they are not the answer"),
    );

  it("shows the abstain panel with the server's reason as text", () => {
    const view = createAnswerView(vi.fn());
    const abstain = { code: "no-passages", reason: XSS };
    view.update(stream({ type: "event", event: { type: "abstain", data: abstain } }));
    const panel = view.el.querySelector<HTMLElement>(".notice--abstain");
    expect(panel?.hidden).toBe(false);
    expect(panel?.querySelector(".notice__title")?.textContent).toBe("Answer withheld");
    expect(panel?.textContent).toContain(XSS);
    expect(withheldNote(view.el)?.hidden).toBe(true); // no claims to withhold
    expect(panel?.querySelector("img")).toBeNull();
  });

  it("marks streamed claims as withheld when the answer is withheld", () => {
    const view = createAnswerView(vi.fn());
    const c = { id: 0, text: "A", ref: "RFC6749 §1.5 ¶1", quote: "q" };
    const abstain = { code: "too-many-struck", reason: "Too few claims verified." };
    view.update(
      stream(
        { type: "event", event: { type: "claim", data: c } },
        {
          type: "event",
          event: { type: "verdict", data: { id: 0, status: "struck", reason: "" } },
        },
        { type: "event", event: { type: "abstain", data: abstain } },
      ),
    );
    const claims = view.el.querySelector(".claims");
    expect(claims?.classList.contains("claims--withheld")).toBe(true);
    expect(claims?.getAttribute("aria-label")).toBe("Withheld claims");
    expect(withheldNote(view.el)?.hidden).toBe(false);
  });

  it.each([
    ["rate_limited", "Too many questions at once"],
    ["not_recorded", "No recorded answer"],
    ["budget_exhausted", "Live mode is paused"],
    ["bad_request", "That question couldn't be processed"],
    ["unavailable", "The assistant is temporarily unavailable"],
    ["not_found", "Not found"],
    ["incomplete", "The answer was cut off"],
    ["network", "Couldn't reach the server"],
    ["weird", "Something went wrong"],
  ])("shows a friendly error for %s, with the server's message as text", (code, title) => {
    const view = createAnswerView(vi.fn());
    view.update(stream({ type: "fail", error: { code, message: XSS } }));
    const notice = view.el.querySelector<HTMLElement>(".notice--error");
    expect(notice?.hidden).toBe(false);
    expect(notice?.querySelector(".notice__title")?.textContent).toBe(title);
    expect(notice?.querySelector(".notice__detail")?.textContent).toBe(XSS);
    expect(notice?.querySelector("img")).toBeNull();
  });

  const verified = (id: number, text: string): Parameters<typeof reduce>[1][] => [
    {
      type: "event",
      event: { type: "claim", data: { id, text, ref: "RFC6749 §1.5 ¶1", quote: "q" } },
    },
    { type: "event", event: { type: "verdict", data: { id, status: "verified", reason: "" } } },
  ];
  const abstained: Parameters<typeof reduce>[1] = {
    type: "event",
    event: { type: "abstain", data: { code: "answerer-abstained", reason: "The model declined." } },
  };
  const done = {
    mode: "extractive" as const,
    model: "",
    claims: 1,
    verified: 1,
    struck: 0,
    latencyMs: 812,
    inputTokens: 0,
    outputTokens: 0,
    costUsd: 0,
  };

  it.each([
    [
      "done",
      { type: "event", event: { type: "done", data: done } },
      "Answer withheld · 1 of 1 claim verified · 812 ms · extractive",
    ],
    ["abort", { type: "abort" }, "Answer withheld. Stopped before the stream finished."],
    [
      "fail",
      { type: "fail", error: { code: "incomplete", message: "" } },
      "Answer withheld. The stream then failed.",
    ],
  ] as [string, Parameters<typeof reduce>[1], string][])(
    "ends a withheld answer by saying it was withheld (%s)",
    (_, end, want) => {
      const view = createAnswerView(vi.fn());
      view.update(stream(...verified(0, "A"), abstained, end));
      expect(view.el.querySelector(".answer__summary")?.textContent).toBe(want);
    },
  );

  it("keeps the state's claim order when an id goes and comes back", () => {
    const view = createAnswerView(vi.fn());
    const both = stream(...verified(0, "A0"), ...verified(1, "A1"));
    view.update(both);
    view.update({ ...both, claims: both.claims.slice(1) });
    view.update(both);
    const texts = [...view.el.querySelectorAll(".claim__text")].map((p) => p.textContent);
    expect(texts).toEqual(["A0", "A1"]);
  });

  it("drops the previous question's claims, even without an empty state between", () => {
    const view = createAnswerView(vi.fn());
    view.update(stream(...verified(0, "A0"), ...verified(1, "A1")));
    view.update(stream(...verified(0, "B0")));
    const texts = [...view.el.querySelectorAll(".claim__text")].map((p) => p.textContent);
    expect(texts).toEqual(["B0"]);
  });
});
