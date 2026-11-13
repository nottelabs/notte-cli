# page observe parity

`observe_parity.py` checks `notte page observe` against the Python SDK's observation of the same page. Both clients read the same API, so they must agree:

- The CLI prints the same description lines, in the same order, with the same ids.
- It changes only an element's state: `checked`, `mixed`, `pressed`, `selected`, `expanded`, `filled`, `invalid`, `value`, `error`, `sort`.
- Every state in the SDK's `element_states` shows on the CLI's line.
- A line the API already wrote with its state stays exactly as the API wrote it.
- A typed password never appears in the output.
- `-o json` has the same `element_states` as the SDK.

The pages are public act-bench tasks (`pages.json`). Each one is opened in one staging session and put into a known state with `setup_js`. The whole run takes about 30 seconds of one browser session.

```sh
NOTTE_API_KEY=... NOTTE_API_URL=https://us-staging.notte.cc make test-parity
# or one page, with a CLI you built:
python -I tests/parity/observe_parity.py --cli ./notte --only webform
```

CI runs it in the `observe-parity` job of the Integration Tests workflow.
