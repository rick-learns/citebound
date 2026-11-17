import { ApiError, askStream, fetchMeta, fetchSection, type ErrorInfo } from "./api";
import { createAnswerView } from "./render/answer";
import { h } from "./render/dom";
import { createAskForm } from "./render/form";
import { createFooter, createMasthead } from "./render/header";
import { createSourcePanel } from "./render/panel";
import { initialState, reduce, type Action, type AnswerState } from "./state";

function toErrorInfo(err: unknown): ErrorInfo {
  if (err instanceof ApiError) return { code: err.code, message: err.message };
  // api.ts turns transport failures into ApiError("network"), so anything else is a bug.
  return { code: "unknown", message: err instanceof Error ? err.message : String(err) };
}

/** Builds the page into root and wires the ask → stream → render loop. */
export function mountApp(root: HTMLElement): void {
  const masthead = createMasthead();
  const footer = createFooter();
  const main = h("main", { id: "main", class: "main" });
  const panel = createSourcePanel(
    (rfc, section, signal) => fetchSection(rfc, section, signal),
    main,
  );
  const answer = createAnswerView((citation, opener) => void panel.open(citation, opener));

  let state: AnswerState = initialState;
  let controller: AbortController | null = null;

  function dispatch(action: Action): void {
    const next = reduce(state, action);
    if (next === state) return;
    state = next;
    answer.update(state);
    form.setStreaming(state.phase === "streaming");
  }

  async function ask(question: string): Promise<void> {
    controller?.abort();
    const ctrl = new AbortController();
    controller = ctrl;
    // Ignore anything a superseded request delivers after a newer one started.
    const send = (action: Action): void => {
      if (controller === ctrl) dispatch(action);
    };
    dispatch({ type: "start", question });
    // Close the previous answer's source, and cancel it if it is still loading. Its chip is gone
    // by now, so focus stays where the form put it.
    panel.close();
    try {
      // askStream returns after done or error, and rejects with "incomplete" if the stream ends
      // before either, so the reducer always sees how it stopped.
      for await (const event of askStream(question, ctrl.signal)) send({ type: "event", event });
    } catch (err) {
      send(ctrl.signal.aborted ? { type: "abort" } : { type: "fail", error: toErrorInfo(err) });
    }
  }

  const form = createAskForm({
    onAsk: (q) => void ask(q),
    // Stop settles the answer at once, rather than when the stream notices the abort.
    onStop: () => {
      controller?.abort();
      dispatch({ type: "abort" });
    },
  });

  main.append(form.el, answer.el);
  const layout = h("div", { class: "layout" }, main, panel.backdrop, panel.el);
  const skip = h("a", { class: "skip-link", href: "#question" }, "Skip to question");
  root.replaceChildren(skip, masthead.el, layout, footer.el);
  answer.update(state);

  fetchMeta().then(
    (meta) => {
      masthead.setMeta(meta);
      footer.setMeta(meta);
    },
    () => {
      masthead.setMeta(null);
    },
  );
}
