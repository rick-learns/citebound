import { describe, expect, it } from "vitest";
import type { AskEvent, DoneEvent } from "./api";
import { initialState, reduce, type Action, type AnswerState } from "./state";

// What app.ts sends when askStream rejects with ApiError "incomplete".
const INCOMPLETE: Action = {
  type: "fail",
  error: { code: "incomplete", message: "The answer stream ended before it finished." },
};

const DONE: DoneEvent = {
  mode: "extractive",
  model: "",
  claims: 2,
  verified: 1,
  struck: 1,
  latencyMs: 812,
  inputTokens: 0,
  outputTokens: 0,
  costUsd: 0,
};

const ABSTAIN = { code: "answerer-abstained", reason: "nothing relevant" };

const claim = (id: number): AskEvent => ({
  type: "claim",
  data: { id, text: `claim ${String(id)}`, ref: "RFC6749 §10.4 ¶1", quote: "q" },
});
const verdict = (id: number, status: "verified" | "struck", reason = ""): AskEvent => ({
  type: "verdict",
  data: { id, status, reason },
});
const ev = (event: AskEvent): Action => ({ type: "event", event });

function run(...actions: Action[]): AnswerState {
  return actions.reduce(reduce, initialState);
}

describe("reduce", () => {
  it("starts a fresh stream, clearing a previous answer", () => {
    const prev = run(
      { type: "start", question: "a" },
      ev(claim(0)),
      ev({ type: "done", data: DONE }),
    );
    const next = reduce(prev, { type: "start", question: "b" });
    expect(next).toEqual({ ...initialState, phase: "streaming", question: "b" });
  });

  it("adds claims as pending and resolves them by verdict", () => {
    let s = run({ type: "start", question: "q" }, ev(claim(0)), ev(claim(1)));
    expect(s.claims.map((c) => c.status)).toEqual(["pending", "pending"]);

    const before = s.claims[1];
    s = reduce(s, ev(verdict(0, "verified")));
    expect(s.claims[0]?.status).toBe("verified");
    expect(s.claims[1]).toBe(before); // untouched claims keep identity

    s = reduce(s, ev(verdict(1, "struck", "quote-not-found")));
    expect(s.claims[1]).toMatchObject({ status: "struck", reason: "quote-not-found" });

    s = reduce(s, ev({ type: "done", data: DONE }));
    expect(s.phase).toBe("done");
    expect(s.done).toEqual(DONE);
  });

  it("keeps the first claim when an id repeats", () => {
    const s = run(
      { type: "start", question: "q" },
      ev(claim(0)),
      ev({ type: "claim", data: { id: 0, text: "Something else.", ref: "", quote: "other" } }),
      ev(verdict(0, "verified")),
    );
    expect(s.claims).toHaveLength(1);
    expect(s.claims[0]).toMatchObject({ text: "claim 0", quote: "q", status: "verified" });
  });

  it("never lets a second verdict override the first", () => {
    const s = run(
      { type: "start", question: "q" },
      ev(claim(0)),
      ev(verdict(0, "struck", "quote-not-found")),
      ev(verdict(0, "verified")),
    );
    expect(s.claims[0]).toMatchObject({ status: "struck", reason: "quote-not-found" });
  });

  it("ignores verdicts for unknown claims", () => {
    const s = run({ type: "start", question: "q" }, ev(claim(0)), ev(verdict(9, "verified")));
    expect(s.claims.map((c) => c.status)).toEqual(["pending"]);
  });

  it("records retrieval passages", () => {
    const s = run({ type: "start", question: "q" }, ev({ type: "retrieval", passages: [] }));
    expect(s.passages).toEqual([]);
    expect(s.phase).toBe("streaming");
  });

  it("handles abstain followed by done", () => {
    const s = run(
      { type: "start", question: "q" },
      ev({ type: "abstain", data: ABSTAIN }),
      ev({ type: "done", data: { ...DONE, claims: 0, verified: 0, struck: 0 } }),
    );
    expect(s.phase).toBe("done");
    expect(s.abstain).toEqual(ABSTAIN);
  });

  it("reports a stream that closes after abstain but before done as incomplete", () => {
    const s = run(
      { type: "start", question: "q" },
      ev({ type: "abstain", data: ABSTAIN }),
      INCOMPLETE,
    );
    expect(s.phase).toBe("error");
    expect(s.abstain).toEqual(ABSTAIN);
  });

  it("stops on an error event mid-stream and marks pending claims unverified", () => {
    const s = run(
      { type: "start", question: "q" },
      ev(claim(0)),
      ev(verdict(0, "verified")),
      ev(claim(1)),
      ev({ type: "error", error: { code: "rate_limited", message: "slow down" } }),
      ev(verdict(1, "verified")), // late event after the error is dropped
    );
    expect(s.phase).toBe("error");
    expect(s.error).toEqual({ code: "rate_limited", message: "slow down" });
    expect(s.claims.map((c) => c.status)).toEqual(["verified", "unverified"]);
  });

  it("handles a failed request", () => {
    const s = run(
      { type: "start", question: "q" },
      {
        type: "fail",
        error: { code: "budget_exhausted", message: "" },
      },
    );
    expect(s.phase).toBe("error");
    expect(s.error?.code).toBe("budget_exhausted");
  });

  it("aborts, keeping resolved claims and dropping later events", () => {
    const s = run(
      { type: "start", question: "q" },
      ev(claim(0)),
      ev(verdict(0, "struck", "uncited")),
      ev(claim(1)),
      { type: "abort" },
      ev(verdict(1, "verified")),
      ev({ type: "done", data: DONE }),
    );
    expect(s.phase).toBe("aborted");
    expect(s.done).toBeNull();
    expect(s.claims.map((c) => c.status)).toEqual(["struck", "unverified"]);
  });

  it("reports a stream that ends without done as an error", () => {
    const s = run({ type: "start", question: "q" }, ev(claim(0)), INCOMPLETE);
    expect(s.phase).toBe("error");
    expect(s.claims[0]?.status).toBe("unverified");
  });

  it("returns the same state for actions outside a stream", () => {
    expect(reduce(initialState, { type: "abort" })).toBe(initialState);
    expect(reduce(initialState, ev(claim(0)))).toBe(initialState);
  });
});
