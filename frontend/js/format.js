// format.js — 選んだ文字の修飾（FR-146, FR-147, UI-020, UI-021, UI-090, IMP-265, DSP-321）。
//
// **画面の選択をソースの位置に対応付けるのは Go 側である**（IMP-123, IMP-124）。ここは、選択を「ブロックの
// 目印・見える文字の上の位置・見える文字の全体」に変えて送り、見た目を先に変えて結果で整えることだけを行う。
// **値の正規化（両端の空白・部品の全体への広げ）はしない**——2 か所で整えると、見た目と書き込まれる内容が
// 食い違う（IMP-265）。**目印の鍵を読まない**（照合は refs.js。IMP-260）。**色の値（#d1242f など）を書かない**
// ——ファイルへ書く値は Go 側が、画面の色は CSS が決める（IMP-265）。
//
// **viewer.js / overlay.js / main.js / navigate.js / docswitch.js / editmode.js を import しない**（IMP-250）。
// editmode.js（ツールバーの淡色）・viewer.js（手順 6d）・docswitch.js（leaveDocument）がこのモジュールを import する。

import { isEditBoxFocused, refuseIfEditing } from "./blockedit.js";
import { openColorList } from "./contextmenu.js";
import { isOwnRef, ownRef } from "./refs.js";
import { state } from "./state.js";
import { S } from "./strings.js";
import { $ } from "./util.js";
import { offsetOf, rangeAt, visibleText } from "./visible.js";

// COLORS は色の一覧（FR-147 の表の名前と順。Go 側は IMP-123 の TextColors、CSS は DSP-010 のトークン）。
export const COLORS = [
  { name: "red", label: S.colorRed },
  { name: "orange", label: S.colorOrange },
  { name: "yellow", label: S.colorYellow },
  { name: "green", label: S.colorGreen },
  { name: "blue", label: S.colorBlue },
  { name: "purple", label: S.colorPurple },
  { name: "gray", label: S.colorGray },
];
const DEFAULT_COLOR = "default";

// KINDS は修飾できるブロックの目印の種類（IMP-265 の使える条件の 3。段落・見出しなどの text と表のセル）
const KINDS = ["text", "cell"];
// TYPING は入力欄（検索バー・編集欄。FR-146 の「入力欄の中では働かない」。IMP-244 の入力欄と同じ）
const TYPING = "input.search-input, input.cell-editor, textarea.code-editor";
const TYPING_AREAS = "#searchbar, .cell-editor-box, .code-editor-box";
const BUTTONS = ["btn-strike", "btn-color"];
// STRUCK は取り消し線の要素（~~ と <del> は del、手書きの <s> と <strike>。IMP-265 の「外す操作とみなす」）
const STRUCK = "del, s, strike";
// UNWRAPPED は仮の見た目で包まない要素（画面が足した要素と、1 文字と数える部品。IMP-266）
const UNWRAPPED = ".heading-anchor, .sort-btn, .media-actions, .copy-btn, .math-inline, .img-broken, sup";
const COLOR_PREFIX = "md-color-";

let deps = { api: null, notify() {} };

// saved は、修飾できる選択の Range の控え（IMP-265 の「L1 の選択範囲」）。**ボタンへフォーカスが移ると
// WebKit は選択を解除する**ため、ボタンにフォーカスがある間はこれで判定し、実行の前に当て直す。
let saved = null;
// kept は、送った修飾の控え { kind, index, start, end, block, wraps }。自分の書き込みの直後の再描画で、
// 同じ目印のブロックの同じ位置へ選択を当て直す（restoreSelection）。
let kept = null;
let scheduled = false;

// initFormat はツールバーの修飾のボタンと選択の監視を配線する（IMP-265）。given: { api, notify(error) }。
export function initFormat(given) {
  deps = { ...deps, ...given };

  for (const id of BUTTONS) {
    const button = $(id);
    // **フォーカスを移さない**（UI-021, BUG-019）。WebKit は要素へフォーカスを移すと文書の選択範囲を解除する。
    // ここを外すと、L1 でだけ「押しても何も起きない」になる。mousedown も止める（NFR-061。IMP-249 と同じ）
    for (const type of ["pointerdown", "mousedown"]) button.addEventListener(type, (event) => event.preventDefault());
    // Tab で移ってきたときも控えた選択を当て直す（淡色の切り替えが選択の解除で外れないように。4.76.0）
    button.addEventListener("focus", () => select(liveSaved()));
  }
  $("btn-strike").addEventListener("click", () => toggleStrike());
  $("btn-color").addEventListener("click", onColorButton);

  document.addEventListener("selectionchange", onSelectionChange);
}

// canFormat は修飾を使えるかを返す（IMP-265 の使える条件の 1〜4）。
export function canFormat() {
  return target() !== null;
}

// selectionState は選択の全体の修飾を返す（右クリックメニューと色の一覧の印。UI-085, DSP-141）。
//   strike: 選択の中の文字（空白を除く）がすべて取り消し線の中、color: すべてが同じ色ならその名前（無ければ null）
export function selectionState() {
  const found = target();
  const result = { strike: false, color: null };
  if (!found) return result;

  const marks = charsOf(found).map((element) => ({ struck: inside(element, STRUCK, found.block), color: colorOf(element, found.block) }));
  if (marks.length === 0) return result;

  result.strike = marks.every((mark) => mark.struck);
  const first = marks[0].color;
  result.color = first && marks.every((mark) => mark.color === first) ? first : null;
  return result;
}

// toggleStrike は取り消し線を付ける・外す（Ctrl+Shift+X・ツールバー・メニュー）。使えなければ false。
//
// **外す操作では仮の見た目を出さない**（祖先の <del> が引いた線は子孫から消せない。FR-146, NFR-012。4.81.0）。
// 外すかどうかの正は Go 側（IMP-124 の 5）であり、ここは見た目だけを決める。
//
// **編集欄が開いている間は、入口で拒む**（IMP-244 の「各ハンドラの入口で refuseIfEditing()」。選択が無くても）。
// 編集欄の中（Save / Cancel を含む）にフォーカスがあれば入力欄の扱いで、何もしない（FR-146）。
export function toggleStrike() {
  if (isEditBoxFocused()) return false;
  if (refuseIfEditing()) return true;
  const found = target();
  if (!found) return false;

  const removing = selectionState().strike;
  reselect(found);
  send(found, { strike: true }, removing ? null : "fmt-pending-strike");
  return true;
}

// applyColor は色を付ける・替える・外す（色の一覧・メニューのサブメニュー）。name は COLORS の名前か "default"。
export function applyColor(name) {
  if (name !== DEFAULT_COLOR && !COLORS.some((color) => color.name === name)) return false;

  const found = target();
  if (!found) return false;
  if (refuseIfEditing()) return true;

  reselect(found);
  send(found, { color: name }, name === DEFAULT_COLOR ? "fmt-pending-default" : `fmt-pending-color ${COLOR_PREFIX}${name}`);
  return true;
}

// updateFormatButtons はツールバーの 2 つのボタンの淡色を切り替える（UI-021）。selectionchange（rAF でまとめる）と
// editmode.js の描画から呼ぶ。
export function updateFormatButtons() {
  const usable = canFormat();
  for (const id of BUTTONS) $(id).disabled = !usable;
}

// restoreSelection は、自分の修飾の書き込みの直後の再描画（trigger が edit）で、控えた選択を同じ目印のブロックの
// 同じ位置へ当て直す（IMP-220 の手順 6d, FR-146）。**それ以外の再描画が先に届いたら当て直さず、控えを捨てる**
// （DSP-352。4.76.0）。修飾は見える文字を変えない（IMP-124 の 9）ため、同じ位置が同じ文字を指す。
export function restoreSelection(trigger) {
  const op = kept;
  kept = null;
  if (!op || trigger !== "edit") return;

  const block = nthBlock(op.kind, op.index);
  select(block ? rangeAt(block, op.start, op.end) : null);
}

// clearFormatState は控えた選択と Range を捨てる（leaveDocument から。IMP-250。4.76.0）。
export function clearFormatState() {
  saved = null;
  kept = null;
}

function onColorButton() {
  const found = target();
  if (!found) return;
  if (refuseIfEditing()) return;

  // 控えた選択を当て直してから開く（一覧は開く前の選択を控える。BUG-019）
  reselect(found);
  openColorList($("btn-color"));
}

// reselect は、ボタンにフォーカスがある間（控えた Range で判定したとき）、控えた Range を選択に当て直す
// （IMP-265 の「ボタンの実行では控えた Range を当て直してから使う」）。
function reselect(found) {
  if (onButton()) select(found.range);
}

// onSelectionChange は修飾できる選択の Range を控え、ボタンの淡色を requestAnimationFrame でまとめて切り替える。
// **修飾できない選択になっても控えは捨てない**——WebKit はボタンへフォーカスが移ると選択を解除し、そこで捨てると
// ボタンが押せなくなる。修飾できない選択の間はボタンが淡色でフォーカスを受けないため、古い控えが使われることはない
// （控えを読むのはボタンにフォーカスがある間だけ）。
function onSelectionChange() {
  const found = selectionTarget(currentRange());
  if (found) saved = found.range.cloneRange();

  if (scheduled) return;
  scheduled = true;
  requestAnimationFrame(() => {
    scheduled = false;
    updateFormatButtons();
  });
}

// target は修飾の対象 { block, kind, range, start, end } を返す。使えなければ null。
// **ボタンにフォーカスがある間は控えた Range で判定する**（IMP-265 の使える条件の 2。4.76.0）。
function target() {
  return selectionTarget(onButton() ? liveSaved() : currentRange());
}

function selectionTarget(range) {
  if (!state.editMode || !state.doc || !state.doc.textRefs) return null;
  if (!range || range.collapsed || isTyping(range)) return null;

  // 両端が同じ 1 つの修飾できるブロックの見える文字の中にある（使える条件の 3）。**ブロックは始まりの端から決め、
  // 終わりの端はそのブロックの中の位置として求める**——別のブロックや、リストの項目の数えない部分（入れ子のブロック）に
  // あれば offsetOf は -1 を返し、下の end <= start で外れる（始まりだけが -1 になることは文書の順から起こらない）。
  // 位置は、始まりを start、終わりを end の端で求める（部品の中の端は部品を含む側。IMP-266。4.92.0）
  const block = blockOf(range.startContainer);
  if (!block) return null;

  const start = offsetOf(block, range.startContainer, range.startOffset, "start");
  const end = offsetOf(block, range.endContainer, range.endOffset, "end");
  if (end <= start) return null;

  return { block, kind: KINDS.find((kind) => isOwnRef(block, kind)), range, start, end };
}

// isTyping は選択の端かフォーカスが入力欄（検索バー・編集欄）にあるかを返す（使える条件の 4）。**Chromium では
// どちらも構造が保証する**（入力欄にフォーカスがあると選択の端は入力欄の位置へ移り、検索バーは本文の外・編集欄では
// 潰れた範囲になる）が、選択の表し方をエンジンに委ねない（NFR-061）。
function isTyping(range) {
  const active = document.activeElement;
  if (active instanceof Element && active.matches(TYPING)) return true;
  return [range.startContainer, range.endContainer].some((node) => {
    const element = node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement;
    return Boolean(element && element.closest(TYPING_AREAS));
  });
}

// blockOf は node から祖先へたどり、鍵の合う text か cell の目印を持つ最も近い要素を返す（#markdown の中だけ）。
function blockOf(node) {
  const markdown = $("markdown");
  let element = node && (node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement);
  for (; element && element !== markdown; element = element.parentElement) {
    if (!markdown.contains(element)) return null;
    if (KINDS.some((kind) => isOwnRef(element, kind))) return element;
  }
  return null;
}

// nthBlock / indexOf は、同じ種類の鍵の合うブロックを文書の順に並べたときの位置で引く（再描画の後の「同じ目印」）。
// **目印の番号を読まずに数える**（鍵を読むのは refs.js だけ。IMP-260）。mediabuttons.js の diagramNumberOf と同じ考え方。
function ownBlocks(kind) {
  return [...$("markdown").querySelectorAll("[data-ref]")].filter((element) => isOwnRef(element, kind));
}

function indexOf(block, kind) {
  return ownBlocks(kind).indexOf(block);
}

function nthBlock(kind, index) {
  return index >= 0 ? ownBlocks(kind)[index] || null : null;
}

// send は修飾の指示を送る（IMP-265 の「送るもの」）。pendingClass があれば、送る前に選んだ文字を仮の span で包む。
function send(found, style, pendingClass) {
  const { block, kind, start, end } = found;
  const ref = ownRef(block, kind);
  const visible = visibleText(block);

  const op = { kind, index: indexOf(block, kind), start, end, block, wraps: [] };
  if (pendingClass) op.wraps = wrap(found.range, block, pendingClass);
  kept = op;

  deps.api
    .applyFormat(ref, start, end, visible, style)
    .then((result) => settle(op, result))
    .catch(() => settle(op, null));
}

// settle は結果で整える（DSP-321）。changed なら再描画を待つ（notice が format-widened なら警告）。changed が偽・
// stale・error なら仮の見た目を外し、当て直しの控えを捨てる。error は知らせる（format-rejected は警告。IMP-315）。
function settle(op, result) {
  if (result && result.changed) {
    if (result.notice) deps.notify({ kind: result.notice });
    return;
  }

  if (kept === op) kept = null;
  unwrap(op);
  if (result && result.error) deps.notify(result.error);
}

// wrap は範囲の中のテキストを、テキストノードごとに仮の span で包む（要素の境界をまたいで 1 つの span で包まない）。
// 包みは見える文字を変えない（IMP-266 は包みそのものを数えない）。包んだ後は、包んだ文字を選び直す。
function wrap(range, block, className) {
  const pieces = [];
  const walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode;
    if (!range.intersectsNode(node) || (node.parentElement && node.parentElement.closest(UNWRAPPED))) continue;
    const from = node === range.startContainer ? range.startOffset : 0;
    const to = node === range.endContainer ? range.endOffset : node.data.length;
    if (from < to) pieces.push({ node, from, to });
  }

  const spans = pieces.map(({ node, from, to }) => {
    if (to < node.data.length) node.splitText(to);
    const text = from > 0 ? node.splitText(from) : node;
    const span = document.createElement("span");
    span.className = className;
    text.replaceWith(span);
    span.appendChild(text);
    return span;
  });

  if (spans.length > 0) {
    const chosen = document.createRange();
    chosen.setStart(spans[0].firstChild, 0);
    const last = spans[spans.length - 1].firstChild;
    chosen.setEnd(last, last.data.length);
    select(chosen);
  }
  return spans;
}

// unwrap は仮の span を外して元に戻す。**戻すのは、その要素がまだ画面にある場合だけ**（再描画が先に届いていれば、
// 画面は既にファイルの状態を示している。DSP-321）。選択が包みの中にあったら、同じ文字を選び直す。
function unwrap(op) {
  const live = op.wraps.filter((span) => span.isConnected);
  if (live.length === 0) return;

  const range = currentRange();
  const ours = Boolean(range) && live.some((span) => span.contains(range.startContainer) || span.contains(range.endContainer));
  for (const span of live) {
    const parent = span.parentNode;
    span.replaceWith(...span.childNodes);
    parent.normalize();
  }
  if (ours && op.block.isConnected) select(rangeAt(op.block, op.start, op.end));
}

// charsOf は範囲の中の、空白でない文字を持つテキストノードの親要素を返す（selectionState の判定に使う）。
function charsOf(found) {
  const { block, range } = found;
  const parents = [];
  const walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    const node = walker.currentNode;
    if (!range.intersectsNode(node) || (node.parentElement && node.parentElement.closest(UNWRAPPED))) continue;
    const from = node === range.startContainer ? range.startOffset : 0;
    const to = node === range.endContainer ? range.endOffset : node.data.length;
    if (node.data.slice(from, to).trim() !== "") parents.push(node.parentElement);
  }
  return parents;
}

// inside は element がブロックの中で selector の要素の中にあるかを返す。
function inside(element, selector, block) {
  const found = element.closest(selector);
  return Boolean(found) && block.contains(found);
}

// colorOf は element を包む最も近い色の span の色の名前を返す（無ければ null）。class の md-color-<名前> は
// Go がサニタイズの後に付ける（IMP-116。生 HTML の class は通らない）。
function colorOf(element, block) {
  for (let node = element; node && node !== block.parentElement; node = node.parentElement) {
    const name = [...node.classList].find((cls) => cls.startsWith(COLOR_PREFIX));
    if (name) return name.slice(COLOR_PREFIX.length);
  }
  return null;
}

function currentRange() {
  const selection = getSelection();
  return selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null;
}

function onButton() {
  const active = document.activeElement;
  return Boolean(active) && BUTTONS.includes(active.id);
}

// liveSaved は控えた Range を返す。控えた位置が DOM から外れていれば（再描画・文書の切り替え）null。
function liveSaved() {
  return saved && saved.startContainer.isConnected && saved.endContainer.isConnected ? saved : null;
}

// select は Range を選択として当てる。**当て直せないことは失敗ではない**（文字が短くなっていれば例外を投げる）。
function select(range) {
  if (!range) return;
  const selection = getSelection();
  if (!selection) return;
  try {
    selection.removeAllRanges();
    selection.addRange(range);
  } catch {
    // 当て直さないだけ
  }
}
