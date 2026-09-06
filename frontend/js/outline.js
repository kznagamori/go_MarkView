// outline.js — アウトラインペイン（FR-040〜043, IMP-222, IMP-224, DSP-113, DSP-340）。
//
// 見出しは Go 側が返す DocumentDTO.headings から作る（IMP-117）。
// **HTML を走査して見出しを抽出しない。** 抽出規則を 2 か所に持たないため。

import { S } from "./strings.js";
import { $, clear, openAncestorDetails } from "./util.js";

// 「ペイン上端付近」を表す帯。下端を 85 % 削り、上 15 % だけを判定に使う（IMP-222）。
const BAND = "0px 0px -85% 0px";
const BAND_RATIO = 0.15;

let observer = null;
let order = []; // 文書順の見出し ID
let active = ""; // いま強調している見出し ID

// initOutline はアウトラインを初期化する（IMP-211）。
//
// クリックは #outline の 1 か所で受ける（IMP-322 と同じ考え方）。
export function initOutline() {
  $("outline").addEventListener("click", onClick);
}

// renderOutline は見出し一覧を組み立てる（IMP-224）。
export function renderOutline(headings) {
  const list = $("outline");
  clear(list);
  active = "";

  if (!headings || headings.length === 0) {
    list.appendChild(emptyItem());
    return;
  }

  const depths = relativeDepths(headings);

  headings.forEach((heading, index) => {
    list.appendChild(createItem(heading, depths[index]));
  });
}

// observeHeadings はスクロール連動の監視対象を作り直す（IMP-222, FR-042）。
//
// **スクロールイベントで全見出しの座標を計算する方式を採らない**（AR-051）。
export function observeHeadings(root, headings) {
  if (observer) observer.disconnect();

  order = [];

  observer = new IntersectionObserver(syncActive, {
    root: $("viewer"),
    rootMargin: BAND,
    threshold: 0,
  });

  for (const heading of headings || []) {
    if (!heading.id) continue; // アンカーを持たない見出しは移動先にできない

    const element = root.querySelector(`[id="${cssEscape(heading.id)}"]`);
    if (!element) continue;

    order.push(heading.id);
    observer.observe(element);
  }
}

// syncActive は現在位置を判定して強調を移す（FR-042, DSP-340）。
//
// **監視は「いつ計算するか」だけに使い、判定はその時点の実座標で行う。**
// IntersectionObserver は交差比率が変わったときにしか発火せず、帯の上から
// 帯の下へ一気に飛んだ場合（アウトラインのクリック、アンカー移動、位置の復元）は
// 0 → 0 の変化となって通知が来ない。entry の履歴を積み上げる方式だと、
// そこで古い状態が残る。
//
// 座標を読むのはコールバックの中だけであり、スクロールのたびではない（AR-051）。
export function syncActive() {
  const viewer = $("viewer");
  const bandBottom = viewer.getBoundingClientRect().top + viewer.clientHeight * BAND_RATIO;

  // 判定は「本文ペインの上端より上にある最後の見出し」（FR-042）。
  let found = "";
  for (const id of order) {
    const element = document.getElementById(id);
    if (!element) continue;
    if (element.getBoundingClientRect().top > bandBottom) break;

    found = id;
  }

  setActive(found);
}

// setActive は強調を移す。**変わったときだけ DOM を触る**（IMP-222）。
function setActive(id) {
  if (id === active) return;

  const list = $("outline");
  const previous = list.querySelector(".outline-item.current");
  if (previous) previous.classList.remove("current");

  active = id;
  if (!id) return;

  const item = itemFor(list, id);
  if (!item) return;

  item.classList.add("current");
  ensureVisible(item);
}

// ensureVisible はアウトラインペインの外に出た項目だけを引き寄せる（IMP-222）。
function ensureVisible(item) {
  const pane = $("pane-outline");
  const itemBox = item.getBoundingClientRect();
  const paneBox = pane.getBoundingClientRect();

  if (itemBox.top < paneBox.top || itemBox.bottom > paneBox.bottom) {
    item.scrollIntoView({ block: "nearest" });
  }
}

// onClick は本文の該当見出しへ移動する（FR-041）。
//
// **スムーススクロールを使わない。** 即座に移動する。
function onClick(event) {
  const item = event.target.closest("li[data-id]");
  if (!item) return;

  const target = document.getElementById(item.dataset.id);
  if (!target) return;

  // **アウトラインは折りたたみの中の見出しも項目として並べる**（IMP-224）。
  openAncestorDetails(target);
  target.scrollIntoView();
  syncActive();
}

// relativeDepths は見出しの相対的な深さを返す（FR-040）。
//
// **レベルが飛んでいても段は飛ばさない。** `#` の次が `###` でも 1 段下げる。
// ソース上の階層をそのまま反映しつつ、見た目が不自然に深くなるのを避ける。
function relativeDepths(headings) {
  const stack = [];

  return headings.map((heading) => {
    while (stack.length > 0 && stack[stack.length - 1] >= heading.level) {
      stack.pop();
    }
    const depth = stack.length;
    stack.push(heading.level);

    return depth;
  });
}

function createItem(heading, depth) {
  const item = document.createElement("li");
  item.className = "outline-item";
  item.dataset.level = String(heading.level);
  item.style.setProperty("--depth", String(depth));

  if (heading.id) item.dataset.id = heading.id;

  const text = document.createElement("span");
  text.className = "outline-text";
  text.textContent = heading.text;
  // 長い見出しは 1 行で省略し、全文はツールチップで示す（DSP-113）。
  text.title = heading.text;
  item.appendChild(text);

  return item;
}

function emptyItem() {
  const item = document.createElement("li");
  item.className = "outline-empty";
  item.textContent = S.noHeadings;

  return item;
}

function itemFor(list, id) {
  for (const item of list.children) {
    if (item.dataset.id === id) return item;
  }

  return null;
}

// cssEscape は属性セレクタに入れる値をエスケープする。
//
// 見出し ID は GitHub 互換のスラッグであり（MD-021）、引用符は現れないが、
// セレクタへ素の文字列を差し込む形は残さない。
function cssEscape(value) {
  return value.replace(/["\\]/g, "\\$&");
}
