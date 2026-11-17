import { describe, expect, it } from "vitest";
import { friendlyError, modeText, parseRef, reasonText, rfcEditorUrl, summaryText } from "./format";

describe("parseRef", () => {
  it("parses numeric, appendix and abstract sections", () => {
    expect(parseRef("RFC 6749 §10.4 ¶1")).toEqual({ rfc: 6749, section: "10.4", paragraph: 1 });
    expect(parseRef("RFC6749 §10.4 ¶1")).toEqual({ rfc: 6749, section: "10.4", paragraph: 1 });
    expect(parseRef("RFC9449 §A.1 ¶12")).toEqual({ rfc: 9449, section: "A.1", paragraph: 12 });
    expect(parseRef("RFC8705 §Abstract ¶1")).toEqual({
      rfc: 8705,
      section: "Abstract",
      paragraph: 1,
    });
  });

  it("rejects empty and malformed refs", () => {
    // Like internal/corpus ParseRef: one space, and only letters, digits and dots in a section.
    const bad = [
      "",
      "RFC6749",
      "RFC6749 §10.4",
      "rfc6749 §1 ¶1",
      "RFC6749 §1 ¶x",
      "RFC6749  §1 ¶1",
      " RFC6749 §1 ¶1",
      "RFC6749 §1-2 ¶1",
    ];
    for (const ref of bad) {
      expect(parseRef(ref), ref).toBeNull();
    }
  });
});

describe("format helpers", () => {
  it("builds rfc-editor links", () => {
    expect(rfcEditorUrl(6749, "10.4")).toBe("https://www.rfc-editor.org/rfc/rfc6749#section-10.4");
    expect(rfcEditorUrl(6749, "A.1")).toBe("https://www.rfc-editor.org/rfc/rfc6749#appendix-A.1");
    expect(rfcEditorUrl(6749, "B")).toBe("https://www.rfc-editor.org/rfc/rfc6749#appendix-B");
    expect(rfcEditorUrl(8705, "Abstract")).toBe("https://www.rfc-editor.org/rfc/rfc8705");
  });

  it("names the answer mode", () => {
    expect(modeText("extractive", "")).toBe("extractive");
    expect(modeText("model", "m-1")).toBe("live model m-1");
    expect(modeText("replay", "m-1")).toBe("replay of m-1's recorded answers");
    expect(modeText("replay", "")).toBe("replay of recorded answers");
  });

  it("explains strike reasons in plain words", () => {
    expect(reasonText("quote-not-found", "RFC6749 §10.4 ¶2")).toBe(
      "quote not found in RFC6749 §10.4 ¶2",
    );
    expect(reasonText("uncited", "")).toBe("no citation was given");
    expect(reasonText("unknown-ref", "RFC1 §9 ¶9")).toBe("RFC1 §9 ¶9 does not exist in the corpus");
    expect(reasonText("something-new", "")).toBe("the verifier could not confirm it");
  });

  it("has friendly messages for each error code", () => {
    expect(friendlyError({ code: "rate_limited", message: "" }).hint).toMatch(/try again shortly/u);
    expect(friendlyError({ code: "budget_exhausted", message: "" }).title).toBe(
      "Live mode is paused",
    );
    expect(friendlyError({ code: "incomplete", message: "" }).title).toBe("The answer was cut off");
    expect(friendlyError({ code: "weird", message: "" }).title).toBe("Something went wrong");
  });

  it("formats the done summary", () => {
    const done = {
      mode: "extractive" as const,
      model: "",
      claims: 3,
      verified: 2,
      struck: 1,
      latencyMs: 812,
      inputTokens: 0,
      outputTokens: 0,
      costUsd: 0,
    };
    expect(summaryText(done)).toBe("2 of 3 claims verified · 812 ms · extractive");
    expect(summaryText({ ...done, mode: "model", model: "m-1", costUsd: 0.0021 })).toBe(
      "2 of 3 claims verified · 812 ms · live model m-1 · $0.0021",
    );
    expect(summaryText({ ...done, claims: 0, verified: 0 })).toBe(
      "No claims verified · 812 ms · extractive",
    );
  });
});
