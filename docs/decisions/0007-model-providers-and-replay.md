# 0007. Call models over plain HTTP, and replay recordings in CI

- Status: Accepted
- Date: 2026-11-17

## Context

Citebound compares answerers: the extractive baseline, and models from more
than one provider. Evals must run in CI on every pull request, but CI has no
API keys, and live calls would make results flaky and cost money on every
push. A model's answers also have to be reproducible, or a change in the
score cannot be attributed to a change in the code.

## Decision

- **One small interface.** `llm.Provider` takes a system prompt and one user
  turn and returns text, token counts, and whether the reply was truncated.
  Citebound never holds a conversation, so nothing more is needed.
- **Plain HTTP, no SDKs.** Two clients: the Anthropic Messages API, and
  OpenAI-compatible Chat Completions, which also reaches gateways such as
  OpenRouter through `BaseURL`. Each is about 100 lines.
- **Record and replay.** `llm.Recorder` wraps a live provider and appends
  each request and response to a JSON Lines file. `llm.Replay` serves those
  responses with no network. Recordings are keyed by a SHA-256 of the
  provider name and the full request, so any change to the prompt, the
  passages, or the model misses the recording and fails loudly instead of
  replaying a stale answer. Keys are checked on load, so a hand-edited
  request is rejected.
- **Keys never touch disk.** Requests hold only the prompt; API keys live in
  the provider and are never recorded or included in errors.

## Consequences

- CI evals of model answerers run offline and are deterministic: they
  measure the verifier, the policy, and the prompt against fixed model
  output.
- Changing the prompt means re-recording with a key, locally, and committing
  the new recordings. The diff of the recordings file shows the changed
  answers in review.
- Recordings age: they show how a model answered when recorded, not today.
  Each recording stores its date, and the scorecard reports the range.
- No streaming of tokens yet. Answers are short, and claims are verified
  whole; streaming can be added behind the same interface later.

## Alternatives considered

- **Vendor SDKs.** More features than needed, larger dependency trees to
  audit, and two different styles of client for one call.
- **An HTTP-level recorder (go-vcr style).** Records headers, which risks
  writing API keys to disk, and ties recordings to wire details that do not
  matter to the answer.
- **Live calls in CI with a secret.** Flaky, costs money per push, and
  forked pull requests cannot see secrets anyway.
- **Mock providers only.** Tests the plumbing but never a real model's
  output, which is the thing the evals exist to measure.
