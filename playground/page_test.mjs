// Runs the page's script with a stand-in for the DOM and checks the shell's
// behaviour. Run by TestPageShell (page_test.go): node playground/page_test.mjs
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const html = readFileSync(new URL('./web/index.html', import.meta.url), 'utf8');
const script = html.match(/<script>([\s\S]*)<\/script>/)[1];

// Any element, property or call the page reaches for answers with another stub.
const stub = () => new Proxy(function () {}, {
  get: (_, key) => key === Symbol.toPrimitive ? () => '' : key === 'then' ? undefined : stub(),
  set: () => true,
  apply: () => stub(),
});
const storage = new Map();
const context = vm.createContext({
  console, queueMicrotask, setTimeout, clearTimeout, TextEncoder, TextDecoder, URL, AbortController,
  document: stub(),
  localStorage: {getItem: k => storage.get(k) ?? null, setItem: (k, v) => storage.set(k, String(v)), removeItem: k => storage.delete(k)},
  fetch: () => new Promise(() => {}),
  EventSource: function () { return stub(); },
  location: stub(), btoa, atob, requestAnimationFrame: () => 0,
});
context.window = context.globalThis = context;
// Expose the page's top-level bindings to the checks below.
vm.runInContext(script + `
globalThis.page = {tokenize, builtins, readOnlyLine, setMode, submit, lookup, HOME,
  get lastRunLine() { return lastRunLine; }, set lastRunLine(v) { lastRunLine = v; },
  get lastRunSafe() { return lastRunSafe; }, set lastRunSafe(v) { lastRunSafe = v; },
  set preview(v) { preview = v; },
  get busy() { return busy; }, get entries() { return state.entries; }};`, context);
const {page} = context;

// Only commands that only read re-run on a rebuild, matched by full path.
// The page learns the command tree from the help pages.
const help = {};
for (const path of ['', 'page', 'page check', 'tunnel', 'tunnel check', 'sessions', 'sessions start', 'sessions list']) help[path] = {stdout: 'Usage:'};
page.preview = {help};
assert.equal(page.readOnlyLine('notte sessions list'), true);
for (const line of ['notte page check --id I1', 'notte tunnel check', 'notte sessions start']) {
  assert.equal(page.readOnlyLine(line), false, line);
}

// A command tried against fixtures does not re-run against the real API.
page.setMode('fixtures');
page.lastRunLine = 'notte sessions start'; page.lastRunSafe = true;
page.setMode('live');
assert.equal(page.lastRunLine, '');
assert.equal(page.lastRunSafe, false);

// An unfinished ${ is an error, not an endless loop.
assert.throws(() => page.tokenize('echo ${HOME'), /bad substitution/);

// Moving a file onto itself, or a directory into itself, keeps it.
const home = () => page.lookup(page.HOME).children;
page.builtins.touch(['kept']);
assert.equal(page.builtins.mv(['kept', 'kept']).code, 1);
assert.ok(home().kept);
page.builtins.mkdir(['dir']);
assert.equal(page.builtins.mv(['dir', 'dir/sub']).code, 1);
assert.ok(home().dir);
assert.equal(page.builtins.mkdir(['kept/x']).code, 1);

// A command that throws is reported and leaves the terminal usable.
page.builtins.boom = () => { throw new Error('exploded'); };
await page.submit('boom');
assert.equal(page.busy, false);
assert.match(page.entries.at(-1).output.map(chunk => chunk.text).join(''), /exploded/);
await page.submit('echo still works');
assert.match(page.entries.at(-1).output.map(chunk => chunk.text).join(''), /still works/);

console.log('page checks passed');
