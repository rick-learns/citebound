import type { AbstainEvent, AskEvent, DoneEvent, ErrorInfo, Passage } from "./api";

/**
 * "unverified" is client-side: the server sends each verdict right after its claim, so a
 * claim without one means the stream was cut off (internal/server/events.go).
 */
export type ClaimStatus = "pending" | "verified" | "struck" | "unverified";

export interface ClaimState {
  id: number;
  text: string;
  ref: string;
  quote: string;
  status: ClaimStatus;
  reason: string;
}

export type Phase = "idle" | "streaming" | "done" | "error" | "aborted";

export interface AnswerState {
  phase: Phase;
  question: string;
  passages: readonly Passage[];
  claims: readonly ClaimState[];
  /** Set when the answer is withheld; claims already streamed are then not the answer. */
  abstain: AbstainEvent | null;
  error: ErrorInfo | null;
  done: DoneEvent | null;
}

export type Action =
  | { type: "start"; question: string }
  | { type: "event"; event: AskEvent }
  | { type: "abort" }
  // A failed request, or a stream that ended early (the client's "incomplete", api.ts).
  | { type: "fail"; error: ErrorInfo };

export const initialState: AnswerState = {
  phase: "idle",
  question: "",
  passages: [],
  claims: [],
  abstain: null,
  error: null,
  done: null,
};

/** Pure reducer for one question/answer cycle. Unchanged claims keep object identity. */
export function reduce(state: AnswerState, action: Action): AnswerState {
  if (action.type === "start") {
    return { ...initialState, phase: "streaming", question: action.question };
  }
  // Everything else only applies to a live stream; late events after a stop are dropped.
  if (state.phase !== "streaming") return state;
  switch (action.type) {
    case "event":
      return applyEvent(state, action.event);
    case "abort":
      return settle(state, { phase: "aborted" });
    case "fail":
      return settle(state, { phase: "error", error: action.error });
  }
}

function applyEvent(state: AnswerState, event: AskEvent): AnswerState {
  switch (event.type) {
    case "retrieval":
      return { ...state, passages: event.passages };
    case "claim": {
      // The server never repeats an id. If one repeats, keep the first claim, so a verdict is
      // never applied to text it did not judge.
      if (state.claims.some((c) => c.id === event.data.id)) return state;
      const claim: ClaimState = { ...event.data, status: "pending", reason: "" };
      return { ...state, claims: [...state.claims, claim] };
    }
    case "verdict": {
      // Only a pending claim takes a verdict: a second verdict never overrides the first.
      const { id, status, reason } = event.data;
      if (!state.claims.some((c) => c.id === id && c.status === "pending")) return state;
      return {
        ...state,
        claims: state.claims.map((c) => (c.id === id ? { ...c, status, reason } : c)),
      };
    }
    case "abstain":
      return { ...state, abstain: event.data };
    case "done":
      return settle(state, { phase: "done", done: event.data });
    case "error":
      return settle(state, { phase: "error", error: event.error });
  }
}

/** Ends the stream; any claim still pending can no longer be trusted. */
function settle(state: AnswerState, patch: Partial<AnswerState>): AnswerState {
  const claims = state.claims.some((c) => c.status === "pending")
    ? state.claims.map((c) =>
        c.status === "pending" ? { ...c, status: "unverified" as const } : c,
      )
    : state.claims;
  return { ...state, ...patch, claims };
}
