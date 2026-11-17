# Recordings

Real model answers, recorded once with an API key and replayed offline by
tests, the demo, and CI. See
[decision 0007](../../docs/decisions/0007-model-providers-and-replay.md).

| File | Model | Recorded | Answers |
| --- | --- | --- | --- |
| [`nemotron-3-ultra.jsonl`](nemotron-3-ultra.jsonl) | `openrouter/nvidia/nemotron-3-ultra-550b-a55b:free` | 2026-11-17 | 19 |

Each line holds the full request (the system prompt, the question, and the
retrieved passages) and the model's reply. API keys are never recorded.

## What is in it

- **10 demo questions.** Eight are answerable from the corpus. One is off topic
  (sourdough bread), and one has a false premise (that RFC 6749 makes the
  implicit grant mandatory).
- **9 fake-citation prompts** from the red-team set. They ask for sections
  that do not exist, such as RFC 9999 §3, or ask for quotes that do not exist,
  such as RFC 7636 calling S256 NOT RECOMMENDED.

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
- **Refusal.** The model abstained on the off-topic question, the false
  premise, and all nine fake-citation prompts.
- **Real quotes struck on format.** For "What claims must a JWT access token
  contain?", all three quotes are real RFC 9068 text that contains double
  quotes (`"aud"`). The model escaped them as `\"aud\"`, even though the
  prompt says not to, so they were struck as `quote-not-found`, and the answer
  was withheld. These are correct claims rejected, not fabrications.
- **Hidden reasoning can use up the budget.** Nemotron reasons before it
  writes. At a 1,024-token limit, most answerable questions came back empty,
  so the limit is now 4,096. Recording retries a reply that comes back empty
  at the limit.

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
