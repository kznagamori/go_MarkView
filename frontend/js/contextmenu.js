// contextmenu.js — 右クリックメニュー（FR-063, UI-085, AR-060, AR-062, IMP-249, DSP-140）。
//
// **WebView の標準のメニューは、どの場所でも止める**（AR-060）。document の contextmenu で必ず
// preventDefault() し、そのうえで場所が PLACES に当たるときだけ自前のメニューを出す。
//
// **viewer.js / overlay.js / main.js / navigate.js / docswitch.js / editmode.js を import しない**
// （IMP-250, IMP-249）。Go の呼び出し・通知・セルの編集欄の取り消しは main.js が deps で渡す。
// **目印の鍵を読まない**——書かれたとおりのリンク先は refs.js の ownLinkTarget に聞く（IMP-260）。

import { S } from "./strings.js";
import { $, linkHref } from "./util.js";
import { ownLinkTarget } from "./refs.js";
import { isExpandOpen } from "./expand.js";
import { hasAction, restoreRange, runAction } from "./menuactions.js";

// PLACES は場所の判定（IMP-249 の表）。**上から順に見て、最初に当たったものを採る**——入力欄は本文ペインの
// 中にもある（セルの編集欄。FR-063）。kind が null の行（検索バーの入力欄以外の部分と本文中のボタン）では
// 出さない。
const PLACES = [
  { selector: "input.search-input, input.cell-editor", kind: "input" },
  { selector: ".about-licenses", kind: "licenses" },
  { selector: "#searchbar, #viewer button", kind: null },
  { selector: "#viewer", kind: "viewer" },
];

const CELL_EDITOR = "input.cell-editor";

// 右クリックした点からの距離と、ウィンドウの端から空ける幅（DSP-140）
const [OFFSET, MARGIN] = [2, 8];

// deps を渡さずに初期化しても例外にしない（DOM 検査で単独に読むため）
const NO_DEPS = { copyText: async () => null, readClipboard: async () => ({ text: "", error: null }), notify() {}, cancelCellEdit() {} };
let deps = NO_DEPS;

// menu は開いているメニューの状態。閉じていれば null。
//   kind: input / licenses / viewer、place: 場所の要素（入力欄・ライセンス欄・#viewer）、
//   focus: 開く前のフォーカス要素、start / end: 入力欄の選択範囲、link: 書かれたとおりのリンク先（無ければ null）、
//   pasteText: Paste で入れる文字列（readClipboard の応答で控える）
let menu = null;

// rightPressed は、右ボタンの pointerdown の後にまだ contextmenu が来ていないかを表す。**偽のまま contextmenu が
// 来たら、キーボード（Shift+F10 とアプリケーションキー）から開いたとみなす**（UI-085）。WebView は座標を 0 に
// することがあり、座標では見分けられない。
let rightPressed = false;

// initContextMenu は右クリックメニューを配線する（IMP-249）。
//
// given: { copyText(text), readClipboard(), notify(error), cancelCellEdit() }。
//   copyText は失敗したら ErrorDTO を、成功したら null を返す（api.js の copyToClipboard）。
//   readClipboard は { text, error } を返す（api.js の readClipboard。失敗の error は kind が paste）。
export function initContextMenu(given) {
  deps = { ...NO_DEPS, ...given };

  document.addEventListener("contextmenu", onContextMenu);

  // メニューの外の pointerdown で閉じる。**キャプチャで拾う**——本文や情報ダイアログのリスナより先に閉じる。
  window.addEventListener("pointerdown", onPointerDown, true);
  window.addEventListener("keydown", () => (rightPressed = false), true);

  // 本文ペインとライセンス欄のスクロールは、要素の scroll が泡立たないためキャプチャで拾う（IMP-249）
  document.addEventListener("scroll", onScroll, true);
  window.addEventListener("blur", () => closeContextMenu());
  window.addEventListener("resize", () => closeContextMenu());

  const root = $("contextmenu");
  // **項目を押した瞬間に本文の選択範囲が外れないよう、既定の動作を止める**（FR-063, UI-085）。
  // click は止まらない。mousedown も止める（pointerdown を止めたときに mousedown の既定の動作まで止まるかを
  // エンジンに委ねない。NFR-061）。
  for (const type of ["pointerdown", "mousedown"]) root.addEventListener(type, (event) => event.preventDefault());
  root.addEventListener("click", onClick);
  root.addEventListener("keydown", onMenuKey);
}

// isContextMenuOpen はメニューが開いているかを返す（IMP-244 の振り分け）。
export function isContextMenuOpen() {
  return menu !== null;
}

// closeContextMenu はメニューを閉じ、開く前のフォーカス要素へ戻す。閉じたら true、開いていなければ false。
//
// **閉じてから戻すのではなく、戻してから閉じる。** フォーカスのある項目を先に取り除くと、フォーカスが
// 一度 body へ落ちる。
export function closeContextMenu() {
  if (!menu) return false;

  const { focus } = menu;
  if (focus && focus.isConnected) focus.focus({ preventScroll: true });

  // **フォーカスを戻したら選択範囲を当て直す**（BUG-019）。**WebKit は要素へフォーカスを移すと
  // 文書の選択範囲を解除する。** ここは戻す処理が集まる唯一の場所であり、`Esc`・スクロール・`blur`・
  // 開き直しのどの経路もここを通る。**呼び出し側ごとに当て直さない**——1 か所でも忘れると消える。
  restoreRange(menu);

  hide();
  return true;
}

function hide() {
  menu = null;
  $("contextmenu").hidden = true;
  $("contextmenu").replaceChildren();
}

function onContextMenu(event) {
  // **場所の判定に defaultPrevented を使わない**（IMP-249）。本番ビルドの Wails のランタイムは、編集できない
  // 要素の上などで先に preventDefault() を呼ぶことがある。
  event.preventDefault();

  const fromKeyboard = !rightPressed;
  rightPressed = false;

  const root = $("contextmenu");
  const target = event.target instanceof Element ? event.target : null;
  if (!target || root.contains(target)) return;

  closeContextMenu();

  // 拡大画面の表示中は、どこでも出さない（UI-104）
  if (isExpandOpen()) return;

  const found = PLACES.find((entry) => target.closest(entry.selector));
  if (!found || !found.kind) return;

  open(found.kind, target.closest(found.selector), target, event, fromKeyboard);
}

function open(kind, place, target, event, fromKeyboard) {
  const active = document.activeElement;
  const current = { kind, place, start: 0, end: 0, pasteText: "" };
  current.focus = active instanceof HTMLElement && active !== document.body ? active : null;
  current.link = kind === "viewer" ? linkTargetOf(target.closest("a")) : null;

  // **フォーカスを移す前に、本文の選択範囲を控える**（BUG-019）。**WebKit は要素へフォーカスを移すと
  // 文書の選択範囲を解除する**ため、下の first.focus() で消える。入力欄が start / end を控えているのと
  // 同じ扱いを Range にも与える。控えたものは restoreRange で当て直す。
  const selected = kind === "input" ? null : selectionIn(place);
  current.range = selected ? selected.cloneRange() : null;

  const root = $("contextmenu");
  if (current.link !== null) root.append(createItem("copyLink", S.menuCopyLink, true), createSeparator());

  if (kind === "input") {
    current.start = place.selectionStart ?? 0;
    current.end = place.selectionEnd ?? 0;
    const selected = current.start !== current.end;
    root.append(
      createItem("cut", S.menuCut, selected),
      createItem("copy", S.menuCopy, selected),
      // クリップボードの応答が届くまで使えない状態で出す（IMP-249 の「Paste の可否」）
      createItem("paste", S.menuPaste, false),
      createItem("selectAll", S.menuSelectAll, true),
    );
  } else {
    root.append(
      createItem("copy", S.menuCopy, current.range !== null),
      createItem("selectAll", S.menuSelectAll, true),
    );
  }

  menu = current;
  root.hidden = false;
  placeAt(root, fromKeyboard ? keyboardPoint(current) : { x: event.clientX, y: event.clientY });

  // 最初の使える項目へ移す（IMP-249）。**移した直後に選択範囲を当て直す**——
  // **WebKit はフォーカスの移動で文書の選択範囲を解除する**（BUG-019）。当て直さないと、
  // 右クリックした時点で選択が画面から消え、Copy が空のままになる。
  const first = root.querySelector(".contextmenu-item:not(:disabled)");
  if (first) first.focus({ preventScroll: true });
  restoreRange(current);

  if (kind === "input") loadPaste(current);
}

// linkTargetOf はリンク先を、Markdown に書かれたとおりの形で返す（FR-063, IMP-249）。リンクでなければ null。
//
// 鍵の合う data-link があればその値、無ければ（生 HTML の <a>、鍵の合わない目印、図の中のリンク）書かれた
// リンク先（util.js の linkHref。SVG の <a> は xlink:href）とする。**a.href（解決済みの URL）を使わない**——
// 絶対パスは利用者のユーザー名を含みうる。**リンク先を持たない <a> はリンクではない**ため、Copy link address
// の行を出さない。
function linkTargetOf(anchor) {
  if (!anchor) return null;

  const written = ownLinkTarget(anchor);
  return written !== null ? written : linkHref(anchor);
}

// selectionIn は選択範囲が空でなく、場所の中に収まっていればその Range を、そうでなければ null を返す。
function selectionIn(place) {
  const selection = getSelection();
  if (!selection || selection.rangeCount === 0 || selection.toString() === "") return null;

  const range = selection.getRangeAt(0);
  return place.contains(range.commonAncestorContainer) ? range : null;
}

// keyboardPoint はキーボードから開いたときの位置を返す（UI-085）。選択範囲の矩形、無ければフォーカス要素の
// 矩形の左下とする。**入力欄では文書の選択範囲を見ない**——入力欄の外に残る選択範囲の近くへ出てしまう。
function keyboardPoint(current) {
  // **控えた Range を使う**（BUG-019）。**選択を引き直さない**——`closeContextMenu()` を通って
  // 開き直したときは、そこで解除されていることがある（WebKit）。項目の可否と位置の根拠を 1 つにする。
  const range = current.kind === "input" ? null : current.range;
  let rect = range ? range.getBoundingClientRect() : null;
  if (!rect || (rect.width === 0 && rect.height === 0)) rect = (current.focus || current.place).getBoundingClientRect();
  return { x: rect.left, y: rect.bottom };
}

// placeAt は点の右下に置き、はみ出すなら左・上へ折り返す（DSP-140）。
//
// **前の位置のまま幅を測ってよい。** 項目は折り返さない（white-space: nowrap）ため、右端の近くにあっても
// 幅は文言より狭くならない。
function placeAt(root, point) {
  root.style.left = `${fold(point.x, root.offsetWidth, window.innerWidth)}px`;
  root.style.top = `${fold(point.y, root.offsetHeight, window.innerHeight)}px`;
}

// fold は 1 軸の位置を決める。点の後ろ（右・下）に OFFSET 離して置き、収まらなければ点の前（左・上）へ
// 折り返し、ウィンドウの端から MARGIN の内側に収める。
function fold(at, size, limit) {
  let start = at + OFFSET;
  if (start + size > limit - MARGIN) start = at - OFFSET - size;

  return Math.max(MARGIN, Math.min(start, limit - MARGIN - size));
}

// loadPaste はクリップボードを読み、テキストがあれば Paste を使える状態にする（IMP-249）。
//
// **読んだ文字列を控え、Paste を選んだときに読み直さない。** メニューはウィンドウがフォーカスを失うと
// 閉じるため、開いている間に他のアプリケーションでクリップボードが変わることはない。
async function loadPaste(current) {
  let result;
  try {
    result = await deps.readClipboard();
  } catch {
    result = { text: "", error: { kind: "paste" } };
  }

  // 応答が届く前に閉じていたら（開き直していたら）何もしない
  if (menu !== current) return;

  const { text, error } = result || {};
  if (error) deps.notify(error);
  if (error || typeof text !== "string" || text === "") return;

  current.pasteText = text;
  // 使える状態へ変えるときにフォーカスを動かさない（既に ↓ で移っている場合がある）
  const item = $("contextmenu").querySelector('[data-action="paste"]');
  if (item) setEnabled(item, true);
}

function onPointerDown(event) {
  rightPressed = event.button === 2;
  if (!menu || $("contextmenu").contains(event.target)) return;

  // **フォーカスは戻さない**（押した先へ移る。UI-085）
  const { focus } = menu;
  hide();

  // セルの編集欄から開いていたら、フォーカスが外れたものとして取り消す（FR-142, UI-085）。押した先がその
  // 編集欄なら取り消さない。**取り消さないと、編集欄は blur の取り消しを見送ったまま、フォーカスを持たずに残る**
  if (focus && focus.matches(CELL_EDITOR) && !focus.contains(event.target)) deps.cancelCellEdit();
}

function onScroll(event) {
  if (!menu) return;

  const target = event.target;
  if (target === $("viewer") || (target instanceof Element && target.matches(".about-licenses"))) closeContextMenu();
}

function onClick(event) {
  const item = event.target instanceof Element ? event.target.closest(".contextmenu-item") : null;
  if (item && !item.disabled) run(item.dataset.action);
}

// onMenuKey はメニューの中のキーを受ける（IMP-249）。**↑ / ↓ / Enter / Space / Tab は閉じる契機ではない。**
// Esc はここで扱わない（IMP-244 の振り分けの (1)）。
//
// **扱ったキーは window へ伝えない。** Enter で項目を実行するとメニューが閉じるため、window の keydown から
// 見ると「メニューは開いていない」になり、Enter が検索の移動（searchNext）へ回る。
function onMenuKey(event) {
  if (!menu || event.isComposing) return;

  const items = [...$("contextmenu").querySelectorAll(".contextmenu-item:not(:disabled)")];
  const index = items.indexOf(document.activeElement);

  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    const step = event.key === "ArrowDown" ? 1 : -1;
    const next = index < 0 ? (step > 0 ? 0 : items.length - 1) : (index + step + items.length) % items.length;
    if (items[next]) items[next].focus({ preventScroll: true });
    // 項目の間を移るたびに当て直す（BUG-019。**このモジュールのフォーカス移動はすべて選択範囲を消しうる**）
    restoreRange(menu);
  } else if (event.key === "Enter" || event.key === " ") {
    if (index >= 0) run(items[index].dataset.action);
  } else if (event.key !== "Tab") {
    return;
  }

  event.preventDefault();
  event.stopPropagation();
}

// run は項目を実行する。**先にメニューを閉じてフォーカスを開く前の位置へ戻し、それから実行する**（UI-085）。
async function run(action) {
  const current = menu;
  if (!current || !hasAction(action)) return;

  closeContextMenu();
  await runAction(action, current, deps);
}

function createItem(action, label, enabled) {
  const item = document.createElement("button");
  item.type = "button";
  item.className = "contextmenu-item";
  item.textContent = label;
  item.setAttribute("role", "menuitem");
  item.dataset.action = action;
  setEnabled(item, enabled);
  return item;
}

// setEnabled は使えない項目を淡色にし、選べないようにする（FR-063。消さずに出す）。
function setEnabled(item, enabled) {
  item.disabled = !enabled;
  if (enabled) item.removeAttribute("aria-disabled");
  else item.setAttribute("aria-disabled", "true");
}

function createSeparator() {
  const separator = document.createElement("div");
  separator.className = "contextmenu-separator";
  separator.setAttribute("role", "separator");
  return separator;
}
