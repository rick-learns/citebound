// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, askStream, fetchMeta, fetchSection, type AskEvent, type Section } from "./api";
import { mountApp } from "./app";

vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  askStream: vi.fn(),
  fetchMeta: vi.fn(),
  fetchSection: vi.fn(),
}));

const CLAIM: AskEvent = {
  type: "claim",
  data: { id: 0, text: "Refresh tokens are bound.", ref: "RFC6749 §10.4 ¶2", quote: "binding" },
};
const VERIFIED: AskEvent = { type: "verdict", data: { id: 0, status: "verified", reason: "" } };
const DONE: AskEvent = {
  type: "done",
  data: {
    mode: "extractive",
    model: "",
    claims: 1,
    verified: 1,
    struck: 0,
    latencyMs: 5,
    inputTokens: 0,
    outputTokens: 0,
    costUsd: 0,
    budgetExhausted: false,
  },
};

/** A stream that yields events, then fails with error if one is given. */
async function* stream(events: AskEvent[], error?: Error): AsyncGenerator<AskEvent> {
  for (const event of events) {
    await Promise.resolve();
    yield event;
  }
  if (error) throw error;
}

/** A stream that yields events, then waits until it is aborted, as a stalled server would. */
async function* stalled(events: AskEvent[], signal: AbortSignal): AsyncGenerator<AskEvent> {
  yield* events;
  await new Promise((_, reject) => {
    signal.addEventListener("abort", () => {
      reject(new DOMException("aborted", "AbortError"));
    });
  });
}

let root: HTMLElement;
const $ = (sel: string) => root.querySelector<HTMLElement>(sel);
const input = () => root.querySelector("textarea");
const text = (sel: string) => $(sel)?.textContent;
const settle = () => new Promise((r) => setTimeout(r, 0));
const askExample = () => {
  root.querySelector<HTMLButtonElement>(".example")?.click();
};

beforeEach(() => {
  vi.mocked(fetchMeta).mockResolvedValue({
    version: "v1",
    mode: "extractive",
    model: "",
    rfcs: [{ number: 6749, title: "OAuth 2.0" }],
  });
  root = document.createElement("div");
  document.body.append(root);
});

afterEach(() => {
  document.body.replaceChildren();
  vi.resetAllMocks();
});

describe("mountApp", () => {
  it("builds the page and fills in the mode and corpus", async () => {
    mountApp(root);
    await settle();
    expect(text("h1")).toBe("Citebound");
    expect(text(".badge")).toBe("Mode: extractive");
    expect(text(".footer__corpus")).toBe("Corpus: RFC 6749");
    expect($(".intro")?.hidden).toBe(false);
  });

  it("says when the server can't be reached for /api/meta", async () => {
    vi.mocked(fetchMeta).mockRejectedValue(new ApiError("network", "down"));
    mountApp(root);
    await settle();
    expect(text(".badge")).toBe("Mode: server unreachable");
  });

  it("streams an answer into the view and re-enables the form", async () => {
    vi.mocked(askStream).mockImplementation(() => stream([CLAIM, VERIFIED, DONE]));
    mountApp(root);
    askExample();
    await settle();
    expect(text(".claim__status")).toBe("Verified quote");
    expect(text(".answer__summary")).toBe("1 of 1 claim verified · 5 ms · extractive");
    expect(input()?.disabled).toBe(false);
  });

  it("stops the stream with Stop, leaving unjudged claims unverified", async () => {
    vi.mocked(askStream).mockImplementation((_, signal) => stalled([CLAIM], signal));
    mountApp(root);
    askExample();
    await settle();
    expect(input()?.disabled).toBe(true);
    [...root.querySelectorAll("button")].find((b) => b.textContent === "Stop")?.click();
    await settle();
    expect(text(".answer__summary")).toBe("Stopped before the answer finished.");
    expect(text(".claim__status")).toBe("Not verified: the answer stopped first");
    expect(input()?.disabled).toBe(false);
  });

  it("stops at once, and ignores what the stopped stream delivers later", async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((r) => (release = r));
    vi.mocked(askStream)
      .mockImplementationOnce(async function* () {
        await gate; // a stream that is slow to notice the abort
        yield CLAIM;
        yield VERIFIED;
        yield DONE;
      })
      .mockImplementationOnce((_, signal) => stalled([], signal));
    mountApp(root);
    askExample();
    await settle();
    [...root.querySelectorAll("button")].find((b) => b.textContent === "Stop")?.click();
    expect(text(".answer__summary")).toBe("Stopped before the answer finished.");
    askExample();
    release();
    await settle();
    expect(root.querySelectorAll(".claim")).toHaveLength(0);
    expect($(".answer__summary")?.hidden).toBe(true);
    expect(input()?.disabled).toBe(true);
  });

  it("shows an unexpected failure as a generic error, not a network one", async () => {
    vi.mocked(askStream).mockImplementation(() => stream([], new TypeError("boom")));
    mountApp(root);
    askExample();
    await settle();
    expect(text(".notice--error .notice__title")).toBe("Something went wrong");
    expect(text(".notice--error .notice__detail")).toBe("boom");
  });

  it("opens a verified claim's source in the panel", async () => {
    const section: Section = {
      rfc: 6749,
      rfcTitle: "The OAuth 2.0 Authorization Framework",
      section: "10.4",
      sectionTitle: "Refresh Tokens",
      paragraphs: [
        { ref: "RFC6749 §10.4 ¶2", paragraph: 2, text: "MUST maintain the binding", figure: false },
      ],
    };
    vi.mocked(fetchSection).mockResolvedValue(section);
    vi.mocked(askStream).mockImplementation(() => stream([CLAIM, VERIFIED, DONE]));
    mountApp(root);
    askExample();
    await settle();
    $(".claim .chip--link")?.click();
    await settle();
    expect(vi.mocked(fetchSection).mock.calls[0]?.slice(0, 2)).toEqual([6749, "10.4"]);
    expect(text(".para--cited mark")).toBe("binding");
  });

  it.each([
    ["loaded", true],
    ["still loading", false],
  ])("closes the previous answer's source (%s) when a new question starts", async (_, loaded) => {
    const section: Section = {
      rfc: 6749,
      rfcTitle: "The OAuth 2.0 Authorization Framework",
      section: "10.4",
      sectionTitle: "Refresh Tokens",
      paragraphs: [
        { ref: "RFC6749 §10.4 ¶2", paragraph: 2, text: "MUST maintain the binding", figure: false },
      ],
    };
    let resolveSection: (s: Section) => void = () => undefined;
    vi.mocked(fetchSection).mockImplementation(
      () => new Promise((resolve) => (resolveSection = resolve)),
    );
    vi.mocked(askStream).mockImplementation(() => stream([CLAIM, VERIFIED, DONE]));
    mountApp(root);
    askExample();
    await settle();
    $(".claim .chip--link")?.click();
    if (loaded) resolveSection(section);
    await settle();

    askExample();
    expect($(".panel")?.hidden).toBe(true);
    // A pending load is cancelled. (A finished one has nothing left to cancel.)
    const signal = vi.mocked(fetchSection).mock.calls[0]?.[2];
    expect(loaded || signal?.aborted).toBe(true);
    resolveSection(section);
    await settle();
    expect($(".panel")?.hidden).toBe(true);
    // A cancelled load never renders. A loaded section stays in the hidden panel until the next.
    expect(root.querySelectorAll(".para")).toHaveLength(loaded ? 1 : 0);
  });
});
