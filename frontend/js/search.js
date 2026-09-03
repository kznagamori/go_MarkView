// search.js — 文書内検索（FR-080, UI-080, IMP-241, DSP-160, DSP-161）。
//
// 検索語をディスクへ書かない（NFR-042）。state に持つだけで、UpdateConfig
// にも含めない（IMP-303 の ConfigDTO に検索の項目はない）。
//
// **ハイライトはテキストノードの分割だけで行う**（IMP-241）。要素の入れ子を
// 変更すると、本文の見た目とアンカーの位置が壊れる。解除は分割を元へ戻す
// 形で必ず行い、解除できないハイライト実装を採らない。

import { S } from "./strings.js";
import { state } from "./state.js";
import { syncActive } from "./outline.js";
import { $, clear, icon } from "./util.js";

const HIT = "search-hit";
const CURRENT = "search-hit-current";

// 走査から外す領域（IMP-241）。
//
//   svg     … Mermaid と PlantUML の描画結果。HTML の <mark> を差し込むと
//             図が壊れる。**PlantUML の図は文字を多く含む**ため、外し忘れると
//             図の中の語が大量にヒットする（IMP-241）
//   .katex  … KaTeX の描画結果。MathML と HTML に同じ文字が二重に入って
//             おり、包むと数式が崩れるうえヒット数も倍になる
//
// どちらも「Go が出力した本文」ではなく、フロントエンドが後から描いたもので
// ある。原文は data-source と TeX ソースとして別に残っている。
const EXCLUDED = "svg, .katex, script, style";

// initSearch は検索バーを組み立てて配線する（IMP-211）。
//
// 文言は strings.js から採る（IMP-290）。textContent と setAttribute で
// 入れ、innerHTML を使わない（IMP-220）。
//
// **他のモジュールへの依存を受け取らない。** 検索は本文の DOM だけで
// 完結しており、Go 側へも通知しない（検索語を保存しない。NFR-042）。
export function initSearch() {
  const bar = $("searchbar");
  clear(bar);

  bar.appendChild(icon("icon-search", "icon search-icon"));
  bar.appendChild(createInput());
  bar.appendChild(createCount());
  bar.appendChild(createButton("search-prev", "icon-chevron-up", S.searchPrevious, () => jump(-1)));
  bar.appendChild(createButton("search-next", "icon-chevron-down", S.searchNext, () => jump(1)));
  bar.appendChild(createButton("search-close", "icon-close", S.searchClose, closeSearch));
}

// openSearch は検索バーを開く（FR-080, UI-080）。
//
// **本文を表示している状態でのみ開く**（DSP-301）。状態画面を表示中は
// 何も起きない。走査対象そのものが存在しないためである。
//
// 何もしなかった場合は false を返す（IMP-244）。
export function openSearch() {
  if (!$("state-screen").hidden) return false;

  const input = $("search-input");

  // 本文が選択されていればそれを初期値にする（UI-080）。
  if (!state.search.open) {
    const selected = selectionText();
    if (selected) input.value = selected;
  }

  state.search.open = true;
  $("searchbar").hidden = false;

  input.focus();
  input.select();

  find(input.value);
}

// closeSearch は検索を終了する（FR-080, UI-080, IMP-241）。
//
// 文書の切り替えと再描画からも呼ぶ。**ハイライトを必ず解除する。**
export function closeSearch() {
  const input = $("search-input");
  if (!input) return false; // initSearch より前に呼ばれた場合

  clearHighlights();

  state.search.open = false;
  state.search.query = "";
  state.search.hits = [];
  state.search.index = -1;

  input.value = "";
  updateCount();

  // フォーカスが検索欄にあるときだけ本文へ戻す（UI-080）。ツリーから
  // 開いた場合などにフォーカスを奪わない。
  if (document.activeElement === input) $("viewer").focus();

  $("searchbar").hidden = true;
}

// find は検索語を適用する（FR-080）。入力のたびに呼ばれる。
export function find(query) {
  clearHighlights();

  state.search.query = query;
  state.search.hits = [];
  state.search.index = -1;

  // **正規表現を組み立てない**（IMP-241）。メタ文字を含む入力をそのまま
  // 扱えるうえ、エスケープ漏れの余地が残らない。
  const needle = query.toLowerCase();

  if (needle) {
    for (const node of textNodes($("markdown"))) {
      for (const mark of wrap(node, needle)) state.search.hits.push(mark);
    }
  }

  updateCount();

  // 画面に見えている位置から探し始める。先頭へ戻すと、入力を 1 文字足す
  // たびに文書の冒頭へ引き戻される。
  if (state.search.hits.length > 0) select(firstFromViewport());
}

// jump は次・前のヒットへ移動する（FR-080）。delta は +1 / -1。
//
// 端では反対側へ回り込む。ヒットが 1 件でも操作が空振りしない。
export function jump(delta) {
  // 検索を開いていないときの Enter はここへ来る。false を返して
  // 既定の動作（フォーカス中のボタンの実行。UI-021）を妨げない。
  if (!state.search.open || state.search.hits.length === 0) return false;

  select(state.search.index + delta);
  return true;
}

// isSearchOpen は検索バーが開いているかを返す。
export function isSearchOpen() {
  return state.search.open;
}

// select は現在位置を移す（IMP-241, DSP-161）。
//
// **クラスの付け替えだけで済ませる。** ハイライトを作り直すと、文書が
// 大きいほど入力の反応が鈍る。
function select(index) {
  const hits = state.search.hits;

  const previous = hits[state.search.index];
  if (previous) previous.classList.remove(CURRENT);

  const count = hits.length;
  const wrapped = ((index % count) + count) % count;

  state.search.index = wrapped;
  hits[wrapped].classList.add(CURRENT);

  // **スムーススクロールを使わない**（FR-041 と同じ理由）。
  hits[wrapped].scrollIntoView({ block: "center" });
  syncActive();

  updateCount();
}

// updateCount は件数表示を更新する（UI-080, DSP-160）。
function updateCount() {
  const element = $("search-count");
  const { query, hits, index } = state.search;

  element.dataset.empty = String(Boolean(query) && hits.length === 0);

  if (!query) {
    element.textContent = "";
    return;
  }

  element.textContent = hits.length === 0 ? S.searchNoResults : S.searchCount(index + 1, hits.length);
}

// textNodes は走査対象のテキストノードを集める（IMP-241）。
//
// **先にすべて集めてから包む。** 包む処理はテキストノードを分割するため、
// 走査しながら変更すると同じ箇所を二重に処理する。
function textNodes(root) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      if (!node.nodeValue) return NodeFilter.FILTER_REJECT;

      const parent = node.parentElement;
      if (!parent || parent.closest(EXCLUDED)) return NodeFilter.FILTER_REJECT;

      return NodeFilter.FILTER_ACCEPT;
    },
  });

  const nodes = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) nodes.push(node);

  return nodes;
}

// wrap は 1 つのテキストノード内の一致箇所を <mark> で包む（IMP-241）。
//
// splitText で切り出したテキストノードをそのまま <mark> の中へ移す。
// 新しいテキストを作らないため、原文の文字がここで変わることはない。
function wrap(node, needle) {
  const lower = node.nodeValue.toLowerCase();
  const marks = [];

  let current = node; // まだ調べていない残りのテキストノード
  let consumed = 0; // current が元のテキストの何文字目から始まるか
  let from = 0;

  for (;;) {
    const at = lower.indexOf(needle, from);
    if (at < 0) break;

    // 一致部分を単独のテキストノードとして切り出す。
    const hit = current.splitText(at - consumed);
    const rest = hit.splitText(needle.length);

    const mark = document.createElement("mark");
    mark.className = HIT;
    hit.replaceWith(mark);
    mark.appendChild(hit);

    marks.push(mark);

    current = rest;
    consumed = at + needle.length;
    from = consumed;
  }

  return marks;
}

// clearHighlights は包んだ <mark> を解いて元へ戻す（IMP-241）。
function clearHighlights() {
  const root = $("markdown");
  const parents = new Set();

  for (const mark of root.querySelectorAll(`mark.${HIT}`)) {
    const parent = mark.parentNode;

    while (mark.firstChild) parent.insertBefore(mark.firstChild, mark);
    parent.removeChild(mark);
    parents.add(parent);
  }

  // **分割したテキストノードを 1 つへ結合し直す。** これを省くと、次の
  // 検索で分割の境界をまたぐ語が見つからなくなる。
  for (const parent of parents) parent.normalize();
}

// firstFromViewport は本文ペインの上端以降にある最初のヒットを返す。
function firstFromViewport() {
  const top = $("viewer").getBoundingClientRect().top;
  const index = state.search.hits.findIndex((mark) => mark.getBoundingClientRect().bottom >= top);

  return index < 0 ? 0 : index;
}

// selectionText は本文中の選択文字列を返す（UI-080）。
function selectionText() {
  const selection = window.getSelection();
  if (!selection || selection.isCollapsed) return "";
  if (!$("markdown").contains(selection.anchorNode)) return "";

  const text = selection.toString().trim();

  // 行をまたぐ選択は初期値にしない。走査は 1 つのテキストノードの中で
  // 行うため、必ず 0 件になる。
  return text.includes("\n") ? "" : text;
}

function createInput() {
  const input = document.createElement("input");
  input.id = "search-input";
  input.className = "search-input";
  input.type = "text";
  input.spellcheck = false;
  input.autocomplete = "off";
  input.placeholder = S.searchPlaceholder;
  input.setAttribute("aria-label", S.searchPlaceholder);

  input.addEventListener("input", () => find(input.value));

  return input;
}

function createCount() {
  const element = document.createElement("span");
  element.id = "search-count";
  element.className = "search-count";
  // 件数が変わったことを支援技術へ伝える（UI-080）。
  element.setAttribute("aria-live", "polite");

  return element;
}

function createButton(id, symbol, label, handler) {
  const button = document.createElement("button");
  button.id = id;
  button.className = "search-btn";
  button.type = "button";
  button.title = label;
  button.setAttribute("aria-label", label);
  button.appendChild(icon(symbol));

  button.addEventListener("click", () => {
    handler();
    // 操作の続きをキーボードで行えるよう、入力欄へ戻す（UI-080）。
    if (state.search.open) $("search-input").focus();
  });

  return button;
}
