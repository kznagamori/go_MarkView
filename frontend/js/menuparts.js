// menuparts.js — 右クリックメニューの項目の組み立てと置き場所（FR-063, UI-085, IMP-249, DSP-140, DSP-141）。
//
// **開閉・フォーカス・場所の判定は contextmenu.js が持つ。** ここが持つのは、場所ごとの項目・区切り・印・色の見本の
// 要素を作ることと、メニューの位置（リンク先と選択範囲の求め方を含む）を決めることだけである（IMP-011 の 400 行の
// 目安で分けた。4.93.0）。**状態を持たない。** 自前のモジュールは refs.js・strings.js・util.js だけを import する。
// **目印の鍵を読まない**——書かれたとおりのリンク先は refs.js の ownLinkTarget に聞く（IMP-260）。

import { ownLinkTarget } from "./refs.js";
import { S } from "./strings.js";
import { icon, linkHref } from "./util.js";

// 右クリックした点からの距離と、ウィンドウの端から空ける幅（DSP-140）、サブメニューと本メニューの間（DSP-141）、
// 文字色の一覧とボタンの間（DSP-141）
const [OFFSET, MARGIN, SUB_GAP, BELOW_GAP] = [2, 8, 2, 4];

// createItem は項目を作る（IMP-249）。value は実行に渡す値（文字色の名前）。
export function createItem(action, label, enabled, value) {
  const item = document.createElement("button");
  item.type = "button";
  item.className = "contextmenu-item";
  item.textContent = label;
  item.setAttribute("role", "menuitem");
  item.dataset.action = action;
  if (value !== undefined) item.dataset.value = value;
  setEnabled(item, enabled);
  return item;
}

// setEnabled は使えない項目を淡色にし、選べないようにする（FR-063。消さずに出す）。
export function setEnabled(item, enabled) {
  item.disabled = !enabled;
  if (enabled) item.removeAttribute("aria-disabled");
  else item.setAttribute("aria-disabled", "true");
}

export function createSeparator() {
  const separator = document.createElement("div");
  separator.className = "contextmenu-separator";
  separator.setAttribute("role", "separator");
  return separator;
}

// checkable は項目を印の付く項目にする（UI-085, DSP-141）。role は menuitemcheckbox（Strikethrough）か
// menuitemradio（色）。印は icon-check を左の 16px の列に置く。**印の列は、修飾の項目を含むメニューの
// すべての項目が空ける**（CSS の :has。印の無い項目も文字の頭をそろえる）。
export function checkable(item, role, checked) {
  item.setAttribute("role", role);
  item.setAttribute("aria-checked", String(checked));
  if (checked) item.prepend(icon("icon-check", "icon contextmenu-check"));
  return item;
}

// submenuItem は Text color の項目（サブメニューを持つ。右端に icon-chevron-right。IMP-249, DSP-141）。
export function submenuItem(enabled) {
  const item = createItem("submenu", S.menuTextColor, enabled);
  item.classList.add("has-submenu");
  item.setAttribute("aria-haspopup", "menu");
  item.setAttribute("aria-expanded", "false");
  item.append(icon("icon-chevron-right", "icon contextmenu-chevron"));
  return item;
}

// colorItems は色の一覧の項目を作る（7 色 → 区切り → Default。FR-147, DSP-141）。checked は印を付ける色の名前。
// 各色の項目には色の見本（span.color-swatch.md-color-<名前>。いまのテーマの色）を置く。**Default は見本を
// 見せずに列を空ける**（文字の頭を色の名前にそろえる）。
export function colorItems(colors, checked) {
  const list = colors.map(({ name, label }) => {
    const item = checkable(createItem("color", label, true, name), "menuitemradio", name === checked);
    item.insertBefore(swatch(`md-color-${name}`), item.lastChild);
    return item;
  });
  const fallback = checkable(createItem("color", S.colorDefault, true, "default"), "menuitemradio", false);
  fallback.insertBefore(swatch("is-blank"), fallback.lastChild);
  return [...list, createSeparator(), fallback];
}

function swatch(modifier) {
  const element = document.createElement("span");
  element.className = `color-swatch ${modifier}`;
  element.setAttribute("aria-hidden", "true");
  return element;
}

// buildItems は場所ごとの項目を作る（IMP-249 の場所の表の項目。FR-063, FR-036, UI-085）。current.formatColor に
// 選択の全体の色（サブメニューの印）を控える。pending は、使えない状態で出して応答を待つ項目（IMP-270）。
//
// **修飾の項目は、行ごと出すかを編集モードで、使えるかを canFormat() で、印を selectionState() で決める**
// （4.75.0）。**フォーカスを移す前に呼ぶ**（開いた後は WebKit で選択が一度解除される）。
export function buildItems(current, deps) {
  const elements = [];
  const pending = [];
  if (current.link !== null) elements.push(createItem("copyLink", S.menuCopyLink, true), createSeparator());

  if (current.kind === "viewer" && deps.format.enabled()) {
    const usable = deps.format.canFormat();
    const marks = usable ? deps.format.selectionState() : { strike: false, color: null };
    current.formatColor = marks.color;
    elements.push(checkable(createItem("strike", S.menuStrikethrough, usable), "menuitemcheckbox", marks.strike), submenuItem(usable), createSeparator());
  }

  if (current.kind === "tree") {
    // ファイルツリー（IMP-270）。**右クリックだけでは項目を選ばず、開かず、強調も動かさない**（FR-036）
    for (const entry of deps.treeMenu.items(current.place)) {
      if (entry === null) {
        elements.push(createSeparator());
        continue;
      }
      const element = createItem(entry.action, entry.label, entry.enabled);
      elements.push(element);
      if (entry.ready) pending.push({ element, ready: entry.ready });
    }
  } else if (current.kind === "input") {
    const selected = current.start !== current.end;
    elements.push(
      createItem("cut", S.menuCut, selected),
      createItem("copy", S.menuCopy, selected),
      // クリップボードの応答が届くまで使えない状態で出す（IMP-249 の「Paste の可否」）
      createItem("paste", S.menuPaste, false),
      createItem("selectAll", S.menuSelectAll, true),
    );
  } else {
    elements.push(createItem("copy", S.menuCopy, current.range !== null), createItem("selectAll", S.menuSelectAll, true));
  }
  return { elements, pending };
}

// linkTargetOf はリンク先を、Markdown に書かれたとおりの形で返す（FR-063, IMP-249）。リンクでなければ null。
//
// 鍵の合う data-link があればその値、無ければ（生 HTML の <a>、鍵の合わない目印、図の中のリンク）書かれた
// リンク先（util.js の linkHref。SVG の <a> は xlink:href）とする。**a.href（解決済みの URL）を使わない**——
// 絶対パスは利用者のユーザー名を含みうる。**リンク先を持たない <a> はリンクではない**ため、Copy link address
// の行を出さない。
export function linkTargetOf(anchor) {
  if (!anchor) return null;

  const written = ownLinkTarget(anchor);
  return written !== null ? written : linkHref(anchor);
}

// selectionIn は選択範囲が空でなく、場所の中に収まっていればその Range を、そうでなければ null を返す。
export function selectionIn(place) {
  const selection = getSelection();
  if (!selection || selection.rangeCount === 0 || selection.toString() === "") return null;

  const range = selection.getRangeAt(0);
  return place.contains(range.commonAncestorContainer) ? range : null;
}

// keyboardPoint はキーボードから開いたときの位置を返す（UI-085）。選択範囲の矩形、無ければフォーカス要素の
// 矩形の左下とする。**入力欄では文書の選択範囲を見ない**——入力欄の外に残る選択範囲の近くへ出てしまう。
export function keyboardPoint(current) {
  // **控えた Range を使う**（BUG-019）。**選択を引き直さない**——`closeContextMenu()` を通って
  // 開き直したときは、そこで解除されていることがある（WebKit）。項目の可否と位置の根拠を 1 つにする。
  const range = current.kind === "input" ? null : current.range;
  let rect = range ? range.getBoundingClientRect() : null;
  if (!rect || (rect.width === 0 && rect.height === 0)) rect = anchorOf(current).getBoundingClientRect();
  return { x: rect.left, y: rect.bottom };
}

// anchorOf はキーボードから開いたときに、メニューを下に出す要素を返す。**ツリーの項目は li ではなく行
// （.tree-row）**——展開中の li は配下の木全体を含む（IMP-270）。
function anchorOf(current) {
  const element = current.focus || current.place;
  if (current.kind !== "tree") return element;

  const row = current.place.querySelector(":scope > .tree-row");
  return row || current.place;
}

// placeAt は点の右下に置き、はみ出すなら左・上へ折り返す（DSP-140）。
//
// **前の位置のまま幅を測ってよい。** 項目は折り返さない（white-space: nowrap）ため、右端の近くにあっても
// 幅は文言より狭くならない。
export function placeAt(root, point) {
  root.style.left = `${fold(point.x, root.offsetWidth, window.innerWidth)}px`;
  root.style.top = `${fold(point.y, root.offsetHeight, window.innerHeight)}px`;
}

// placeBeside はサブメニューを項目の右に、上端をその項目の上端にそろえて置く（本メニューとの間 2px）。
// **ウィンドウの右端で収まらなければ本メニューの左に出す。** 下端で収まらなければ上へずらす（DSP-141）。
export function placeBeside(sub, item, main) {
  const menu = main.getBoundingClientRect();
  const width = sub.offsetWidth;
  let left = menu.right + SUB_GAP;
  if (left + width > window.innerWidth - MARGIN) left = menu.left - SUB_GAP - width;

  sub.style.left = `${Math.max(MARGIN, left)}px`;
  sub.style.top = `${clamp(item.getBoundingClientRect().top, sub.offsetHeight, window.innerHeight)}px`;
}

// placeBelow は文字色の一覧をボタンの下 4px に、左端をボタンの左端にそろえて置く（右端で収まらなければ右端を
// そろえる。DSP-141）。
export function placeBelow(root, button) {
  const rect = button.getBoundingClientRect();
  const width = root.offsetWidth;
  let left = rect.left;
  if (left + width > window.innerWidth - MARGIN) left = rect.right - width;

  root.style.left = `${Math.max(MARGIN, left)}px`;
  root.style.top = `${clamp(rect.bottom + BELOW_GAP, root.offsetHeight, window.innerHeight)}px`;
}

// fold は 1 軸の位置を決める。点の後ろ（右・下）に OFFSET 離して置き、収まらなければ点の前（左・上）へ
// 折り返し、ウィンドウの端から MARGIN の内側に収める。
function fold(at, size, limit) {
  let start = at + OFFSET;
  if (start + size > limit - MARGIN) start = at - OFFSET - size;

  return Math.max(MARGIN, Math.min(start, limit - MARGIN - size));
}

// clamp は始まりの位置を、ウィンドウの端から MARGIN の内側に収める（下端で収まらなければ上へずらす）。
function clamp(start, size, limit) {
  return Math.max(MARGIN, Math.min(start, limit - MARGIN - size));
}
