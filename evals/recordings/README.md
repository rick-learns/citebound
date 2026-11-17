# Recordings

Real model answers, recorded once with an API key and replayed offline by
tests, the demo, and CI. See
[decision 0007](../../docs/decisions/0007-model-providers-and-replay.md).

| File | Model | Recorded | Answers |
| --- | --- | --- | --- |
| [`nemotron-3-ultra.jsonl`](nemotron-3-ultra.jsonl) | `openrouter/nvidia/nemotron-3-ultra-550b-a55b:free` | 2026-11-17 | 180 |

Each line holds the full request (the system prompt, the question, and the
retrieved passages) and the model's reply. API keys are never recorded.

## What is in it

- **10 demo questions.** Eight are answerable from the corpus. One is off topic
  (sourdough bread), and one has a false premise (that RFC 6749 makes the
  implicit grant mandatory).
- **Every eval question:** the 100 answerable questions, the 30 unanswerable
  ones, and the 40 red-team prompts in [`evals/`](../README.md). These include
  the fake-citation prompts, which ask for sections that do not exist, such as
  RFC 9999 §3, or for quotes that do not exist, such as RFC 7636 calling S256
  NOT RECOMMENDED.

Replay any of them by asking the same question:

```sh
go run ./cmd/citebound ask -model openrouter/nvidia/nemotron-3-ultra-550b-a55b:free \
  -replay evals/recordings/nemotron-3-ultra.jsonl "What must a DPoP proof contain?"
```

A question that was not recorded fails with `no recording for this request`.
So does a recorded question after any change to the prompt, the retrieved
passages, or the token limit, because each of these is part of the recording
key.

## What it shows

- **Synthesis with citations.** Answers on refresh-token binding, the implicit
  grant, and introspection authentication combine verified claims from two or
  three RFCs.
- **Refusal.** The model abstained on every unanswerable question, every
  false premise, and every fake-citation prompt. It also abstained on 14 of
  the 100 answerable questions.
- **No invented quotes yet.** Of 206 claims, three were struck, and none of
  the three is an invented quote:
  - For RFC 9449 §10 ¶4, the model quoted a real figure but JSON-escaped it:
    each line-continuation backslash and line break became the literal text
    `\\\n`. It was struck as `quote-not-found`, so a real quote was
    rejected.
  - Two claims were struck as `uncited`. Both replies hit the 4,096-token
    limit (`"truncated": true` in the recording) and stop mid-sentence, one
    inside a quote and one before its citation.
- **Escaped double quotes.** For "What claims must a JWT access token
  contain?", all three quotes are real RFC 9068 text that contains double
  quotes (`"aud"`). The model escaped them as `\"aud\"`, even though the
  prompt says not to. Before
  [decision 0008](../../docs/decisions/0008-escaped-double-quotes.md), they were
  struck as `quote-not-found` and a correct answer was withheld. Now they
  verify.
- **Hidden reasoning can use up the budget.** Nemotron reasons before it
  writes. At a 1,024-token limit, most answerable questions came back empty,
  so the limit is now 4,096. `-record` does not retry: an empty reply is an
  error and is not recorded, so ask the question again.

## Re-recording

After changing the prompt, retrieval, or the token limit, re-record with a key
and commit the new file. The diff shows how each answer changed.

```sh
OPENROUTER_API_KEY=... go run ./cmd/citebound ask \
  -model openrouter/nvidia/nemotron-3-ultra-550b-a55b:free \
  -record evals/recordings/nemotron-3-ultra.jsonl "<question>"
```

Free models are rate limited upstream, so recording a long set can take
several tries.
