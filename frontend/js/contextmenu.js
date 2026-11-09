// contextmenu.js — 右クリックメニュー（FR-063, UI-085, AR-060, AR-062, IMP-249, DSP-140）。
//
// **WebView の標準のメニューは、どの場所でも止める**（AR-060）。document の contextmenu で必ず
// preventDefault() し、そのうえで場所が PLACES に当たるときだけ自前のメニューを出す。
//
// **viewer.js / overlay.js / main.js / navigate.js / docswitch.js / editmode.js / format.js / treemenu.js を
// import しない**（IMP-250, IMP-249）。Go の呼び出し・通知・修飾の操作・ツリーの項目は main.js が deps で渡す。
// **目印の鍵を読まない**——書かれたとおりのリンク先は refs.js の ownLinkTarget に聞く（IMP-260）。
//
// **右クリックメニューの Text color のサブメニュー（#contextsubmenu）と、ツールバーの文字色の一覧（#contextmenu を
// 使い回す）もここで開閉する**（UI-085, DSP-141。4.75.0）。項目の要素と位置は menuparts.js が作る。

import { S } from "./strings.js";
import { $ } from "./util.js";
import { isExpandOpen } from "./expand.js";
import { hasAction, restoreRange, runAction } from "./menuactions.js";
import { buildItems, colorItems, keyboardPoint, linkTargetOf, placeAt, placeBelow, placeBeside, selectionIn, setEnabled } from "./menuparts.js";

// PLACES は場所の判定（IMP-249 の表）。**上から順に見て、最初に当たったものを採る**——入力欄は本文ペインの
// 中にもある（セルの編集欄。FR-063）。kind が null の行（検索バーの入力欄以外の部分と本文中のボタン）では
// 出さない。
const PLACES = [
  { selector: "input.search-input, input.cell-editor, textarea.code-editor", kind: "input" },
  { selector: ".about-licenses", kind: "licenses" },
  { selector: "#searchbar, #viewer button, #treefilter", kind: null },
  { selector: "#viewer", kind: "viewer" },
  // ファイルツリーの項目とツリールートの名前の行（FR-036。4.75.0）。項目と実行は treemenu.js（deps.treeMenu）
  { selector: "#tree .tree-item, #tree-root-name", kind: "tree" },
];

// deps を渡さずに初期化しても例外にしない（DOM 検査で単独に読むため）
const NO_DEPS = {
  copyText: async () => null,
  readClipboard: async () => ({ text: "", error: null }),
  notify() {},
  treeMenu: { items: () => [], run: async () => {} },
  format: { enabled: () => false, canFormat: () => false, selectionState: () => ({ strike: false, color: null }), colors: [] },
};
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

// suppressed は、開いている文字色の一覧をそのボタンの pointerdown で閉じたときのボタン。続く click で開き直さない
// （ボタンをもう一度押すと閉じる）。次の pointerdown で外す。
let suppressed = null;

// initContextMenu は右クリックメニューを配線する（IMP-249）。
//
// given: { copyText(text), readClipboard(), notify(error), treeMenu, format }。**v1.1.0 の cancelCellEdit は外した**——
// メニューの外のクリックで閉じても、編集欄を取り消さない（UI-085, FR-142。4.73.0）。
//   copyText は失敗したら ErrorDTO を、成功したら null を返す（api.js の copyToClipboard）。
//   readClipboard は { text, error } を返す（api.js の readClipboard。失敗の error は kind が paste）。
//   treeMenu は { items(place), run(action, place) }（treemenu.js の treeMenuItems / runTreeAction。IMP-270）。
//   format は { enabled(), canFormat(), selectionState(), toggleStrike(), applyColor(name), colors }（format.js。IMP-265）。
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
  const sub = $("contextsubmenu");
  for (const element of [root, sub]) {
    for (const type of ["pointerdown", "mousedown"]) element.addEventListener(type, (event) => event.preventDefault());
    element.addEventListener("click", onClick);
  }
  root.addEventListener("keydown", onMenuKey);
  sub.addEventListener("keydown", onSubKey);
  // マウスを Text color に乗せると開き、本メニューの別の項目へ移ると閉じる（IMP-249）
  root.addEventListener("mouseover", onHover);
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

// closeSubmenu は Text color のサブメニューだけを閉じ、Text color の項目へフォーカスを戻す（← と Esc。IMP-244 の
// (1)、IMP-249）。閉じたら true、開いていなければ false。
export function closeSubmenu() {
  if (!menu || !menu.sub) return false;

  const item = menu.sub;
  hideSubmenu();
  item.focus({ preventScroll: true });
  restoreRange(menu);
  return true;
}

// openColorList はツールバーの文字色のボタンの下に色の一覧を出す（UI-020, UI-085, DSP-141）。#contextmenu を
// 使い回し、右クリックメニューと同時には開かない。**開く前に本文の選択範囲を控える**（BUG-019）。閉じたら
// フォーカスを開く前の要素へ戻す（ボタンではなく、本文の選択を保つため。押したときにフォーカスは移していない）。
export function openColorList(button) {
  if (suppressed === button) {
    suppressed = null;
    return;
  }
  closeContextMenu();
  if (isExpandOpen()) return;

  const active = document.activeElement;
  const selection = getSelection();
  const current = { kind: "colors", place: button, start: 0, end: 0, pasteText: "", link: null, sub: null };
  current.focus = active instanceof HTMLElement && active !== document.body ? active : null;
  current.range = selection && selection.rangeCount > 0 ? selection.getRangeAt(0).cloneRange() : null;

  const root = $("contextmenu");
  root.classList.add("is-color-list");
  root.append(...colorItems(deps.format.colors, deps.format.selectionState().color));
  menu = current;
  root.hidden = false;
  button.setAttribute("aria-expanded", "true");
  placeBelow(root, button);
  focusFirst(root);
}

function hide() {
  const current = menu;
  hideSubmenu();
  menu = null;
  const root = $("contextmenu");
  root.hidden = true;
  root.replaceChildren();
  root.classList.remove("is-color-list");
  if (current && current.kind === "colors") current.place.setAttribute("aria-expanded", "false");
}

function hideSubmenu() {
  const sub = $("contextsubmenu");
  sub.hidden = true;
  sub.replaceChildren();
  if (menu && menu.sub) {
    menu.sub.setAttribute("aria-expanded", "false");
    menu.sub = null;
  }
}

// openSubmenu は Text color の項目の右に 7 色と Default を出し、フォーカスを最初の色へ移す（IMP-249, DSP-141）。
// **淡色のときは開かない**（DSP-141）。印は開いたときに決めた色（open() の selectionState()）で付ける。
function openSubmenu(item) {
  if (!menu || item.disabled || menu.sub === item) return;

  const sub = $("contextsubmenu");
  sub.replaceChildren(...colorItems(deps.format.colors, menu.formatColor));
  sub.hidden = false;
  menu.sub = item;
  item.setAttribute("aria-expanded", "true");
  placeBeside(sub, item, $("contextmenu"));
  focusFirst(sub);
}

// focusFirst は最初の使える項目へ移し、**その直後に選択範囲を当て直す**（BUG-019。WebKit はフォーカスの
// 移動で文書の選択範囲を解除する）。
function focusFirst(root) {
  const first = root.querySelector(".contextmenu-item:not(:disabled)");
  if (first) first.focus({ preventScroll: true });
  restoreRange(menu);
}

function onContextMenu(event) {
  // **場所の判定に defaultPrevented を使わない**（IMP-249）。本番ビルドの Wails のランタイムは、編集できない
  // 要素の上などで先に preventDefault() を呼ぶことがある。
  event.preventDefault();

  const fromKeyboard = !rightPressed;
  rightPressed = false;

  const root = $("contextmenu");
  const target = event.target instanceof Element ? event.target : null;
  if (!target || root.contains(target) || $("contextsubmenu").contains(target)) return;

  closeContextMenu();

  // 拡大画面の表示中は、どこでも出さない（UI-104）
  if (isExpandOpen()) return;

  const found = PLACES.find((entry) => target.closest(entry.selector));
  if (!found || !found.kind) return;

  open(found.kind, target.closest(found.selector), target, event, fromKeyboard);
}

function open(kind, place, target, event, fromKeyboard) {
  const active = document.activeElement;
  const current = { kind, place, start: 0, end: 0, pasteText: "", sub: null };
  current.focus = active instanceof HTMLElement && active !== document.body ? active : null;
  current.link = kind === "viewer" ? linkTargetOf(target.closest("a")) : null;

  // **フォーカスを移す前に、本文の選択範囲を控える**（BUG-019）。**WebKit は要素へフォーカスを移すと
  // 文書の選択範囲を解除する**ため、下の focusFirst で消える。入力欄が start / end を控えているのと
  // 同じ扱いを Range にも与える。控えたものは restoreRange で当て直す。
  const selected = kind === "input" || kind === "tree" ? null : selectionIn(place);
  current.range = selected ? selected.cloneRange() : null;

  if (kind === "input") {
    current.start = place.selectionStart ?? 0;
    current.end = place.selectionEnd ?? 0;
  }

  const root = $("contextmenu");
  const { elements, pending } = buildItems(current, deps);
  root.append(...elements);

  menu = current;
  root.hidden = false;
  placeAt(root, fromKeyboard ? keyboardPoint(current) : { x: event.clientX, y: event.clientY });

  // 最初の使える項目へ移す（IMP-249）。**移した直後に選択範囲を当て直す**——当て直さないと、右クリックした
  // 時点で選択が画面から消え、Copy が空のままになる（BUG-019）
  focusFirst(root);

  if (kind === "input") loadPaste(current);
  for (const { element, ready } of pending) enableWhenReady(current, element, ready);
}

// enableWhenReady は、使えない状態で出した項目を、応答が真なら使える状態にする（Set parent as tree root。IMP-270）。
// **応答が届く前にメニューが閉じていたら何もしない。** 使える状態へ変えるときにフォーカスを動かさない（Paste と同じ）。
async function enableWhenReady(current, element, ready) {
  let ok = false;
  try {
    ok = (await ready()) === true;
  } catch {
    ok = false;
  }
  if (menu === current && ok) setEnabled(element, true);
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
  // **マウスのサイドボタン（3 / 4）では閉じない**（IMP-249 の 4.79.0）。閉じると、続く mouseup の時点でメニューが
  // 閉じているため、shortcuts.js が戻る・進むを行ってしまう（メニューの表示中、サイドボタンは何もしない操作）
  if (event.button === 3 || event.button === 4) return;
  rightPressed = event.button === 2;
  suppressed = null;
  if (!menu || $("contextmenu").contains(event.target) || $("contextsubmenu").contains(event.target)) return;

  // 開いている文字色の一覧を、そのボタンで閉じたら、続く click で開き直さない
  if (menu.kind === "colors" && event.target instanceof Node && menu.place.contains(event.target)) suppressed = menu.place;

  // **フォーカスは戻さない**（押した先へ移る。UI-085）。**編集欄から開いていても取り消さない**（FR-142。4.73.0）
  hide();
}

function onScroll(event) {
  if (!menu) return;

  // 本文ペイン・ライセンス欄・ファイルツリー（.pane-body。UI-032）のスクロールで閉じる
  const target = event.target;
  if (target === $("viewer") || (target instanceof Element && target.matches(".about-licenses, #pane-filetree .pane-body"))) closeContextMenu();
}

function onClick(event) {
  const item = event.target instanceof Element ? event.target.closest(".contextmenu-item") : null;
  if (!item || item.disabled) return;
  if (item.classList.contains("has-submenu")) openSubmenu(item);
  else run(item);
}

// onHover は、マウスを Text color に乗せたらサブメニューを開き、本メニューの別の項目へ移ったら閉じる（IMP-249）。
function onHover(event) {
  const item = event.target instanceof Element ? event.target.closest(".contextmenu-item") : null;
  if (!menu || !item) return;
  if (item.classList.contains("has-submenu")) openSubmenu(item);
  else if (menu.sub) closeSubmenu();
}

// onMenuKey はメニューの中のキーを受ける（IMP-249）。**↑ / ↓ / Enter / Space / Tab は閉じる契機ではない。**
// Esc はここで扱わない（IMP-244 の振り分けの (1)）。
//
// **扱ったキーは window へ伝えない。** Enter で項目を実行するとメニューが閉じるため、window の keydown から
// 見ると「メニューは開いていない」になり、Enter が検索の移動（searchNext）へ回る。
function onMenuKey(event) {
  if (!menu || event.isComposing) return;

  const active = document.activeElement;
  const opensSub = active instanceof Element && active.classList.contains("has-submenu");
  if (opensSub && ["ArrowRight", "Enter", " "].includes(event.key)) openSubmenu(active);
  else if (!navigate($("contextmenu"), event.key)) return;

  event.preventDefault();
  event.stopPropagation();
}

// onSubKey はサブメニューの中のキーを受ける（IMP-249）。← でサブメニューだけを閉じる。Esc は IMP-244 の (1)。
function onSubKey(event) {
  if (!menu || event.isComposing) return;
  if (event.key === "ArrowLeft") closeSubmenu();
  else if (!navigate($("contextsubmenu"), event.key)) return;

  event.preventDefault();
  event.stopPropagation();
}

// navigate は ↑ / ↓ で使える項目の間を巡回し、Enter / Space で実行する。Tab は何もしない（メニューの外へ出さない）。
// 扱ったキーなら true。**↑ / ↓ / Enter / Space / Tab は閉じる契機ではない**（UI-085）。
function navigate(root, key) {
  const items = [...root.querySelectorAll(".contextmenu-item:not(:disabled)")];
  const index = items.indexOf(document.activeElement);

  if (key === "ArrowDown" || key === "ArrowUp") {
    const step = key === "ArrowDown" ? 1 : -1;
    const next = index < 0 ? (step > 0 ? 0 : items.length - 1) : (index + step + items.length) % items.length;
    if (items[next]) items[next].focus({ preventScroll: true });
    // 項目の間を移るたびに当て直す（BUG-019。**このモジュールのフォーカス移動はすべて選択範囲を消しうる**）
    restoreRange(menu);
    return true;
  }
  if (key === "Enter" || key === " ") {
    if (index >= 0) run(items[index]);
    return true;
  }
  return key === "Tab";
}

// run は項目を実行する。**先にメニューを閉じてフォーカスを開く前の位置へ戻し、それから実行する**（UI-085）。
async function run(item) {
  const current = menu;
  if (!current) return;
  const { action, value } = item.dataset;

  // ファイルツリーの項目は treemenu.js が実行する（IMP-270）
  if (current.kind === "tree") {
    closeContextMenu();
    await deps.treeMenu.run(action, current.place);
    return;
  }

  if (!hasAction(action)) return;

  closeContextMenu();
  await runAction(action, current, deps, value);
}
