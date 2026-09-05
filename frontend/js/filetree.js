// filetree.js — ファイルツリーペイン（UI-030, FR-030〜035, DSP-112, DSP-330, DSP-331）。
//
// 展開は遅延させ、開いたディレクトリの分だけ ReadDir を呼ぶ（FR-032）。
// 起動時に階層全体を走査しない。
//
// **パスの解釈をここで行わない**（IMP-300）。どのノードが表示中の文書かは
// 絶対パスの一致で判定し、経路の追跡は Go 側が算出した displayPath を
// 区切りで分けた「名前の並び」で行う。結合・正規化・大文字小文字の規則を
// フロントエンドに持ち込まない。

import * as api from "./api.js";
import { S, errorText } from "./strings.js";
import { state } from "./state.js";
import { showMessage } from "./status.js";
import { $, clear, icon, baseName } from "./util.js";

let openFile = null;

// initFileTree はツリーを初期化する（IMP-211）。
//
// クリックもキーも #tree の 1 か所で受ける。ノードごとに購読すると、展開の
// たびに登録と解除が要る（IMP-322 と同じ考え方）。
export function initFileTree(deps) {
  openFile = deps.onOpen;

  const tree = $("tree");
  tree.addEventListener("click", onClick);
  // キーボード操作（UI-031, IMP-248）。
  tree.addEventListener("keydown", onKeyDown);
  // **roving tabindex の付け替えはここ 1 か所で行う**（IMP-248）。
  // クリック・Tab・キーでの移動のどれで来ても同じ規則になる。
  tree.addEventListener("focusin", onFocusIn);
}

// loadTreeRoot はツリールートを読み直す（FR-030, FR-032）。
//
// **ルート直下だけを読んで展開状態で表示する。** 以降は展開のたびに読む。
export async function loadTreeRoot(root) {
  const label = $("tree-root-name");
  const list = $("tree");

  clear(list);

  if (!root) {
    label.textContent = "";
    label.title = "";
    return;
  }

  // 見出しの下にツリールートのディレクトリ名、ツールチップに絶対パス（UI-030）。
  label.textContent = baseName(root);
  label.title = root;

  // 空文字を渡すとツリールートを読む（IMP-310）。ここでパスを組み立てない。
  const { nodes, error } = await api.readDir("");
  if (error) {
    showMessage(errorText(error), "error");
    return;
  }

  fill(list, nodes, 0);
  await revealCurrent();
  refreshFocusTarget();
}

// revealCurrent は表示中の文書までの経路を展開し、可視にする（DSP-331）。
//
// ツリールートの外を表示している場合は、選択もせず展開もしない（FR-052, UI-030）。
export async function revealCurrent() {
  markSelected();

  const doc = state.doc;
  if (!doc || doc.outsideTree || !doc.displayPath) return;

  const segments = doc.displayPath.split(/[\\/]/).filter(Boolean);
  let container = $("tree");

  for (let i = 0; i < segments.length - 1; i += 1) {
    const item = childByName(container, segments[i]);
    if (!item) return;

    if (item.getAttribute("aria-expanded") !== "true") {
      const ok = await expand(item);
      if (!ok) return;
    }
    container = childGroup(item);
  }

  markSelected();

  const target = childByName(container, segments[segments.length - 1]);
  if (target) target.scrollIntoView({ block: "nearest" });
}

async function onClick(event) {
  const item = event.target.closest("li[data-path]");
  if (!item) return; // 「… and N more」はクリック不可（DSP-112）

  await activate(item);
}

// activate はクリックと Enter の共通の入口（IMP-248）。
//
// **同じ操作を 2 通りに分けない。** 分けると、片方だけが壊れている状態を
// 検証で通してしまう（DSP-360 の IMPORTANT と同じ理由）。
async function activate(item) {
  if (item.dataset.dir === "true") {
    await toggle(item);
    return;
  }

  // ファイルを選んでもツリールートは変わらない（FR-030）。判断は Go 側。
  //
  // **開いた後、フォーカスは本文ペインへ移る**（UI-051, IMP-220 の手順 11）。
  // ここでツリーに残すと PageUp / PageDown が効かず、利用者から見れば
  // BUG-007 と同じ症状になる。経路ごとに分けない。
  if (openFile) await openFile(item.dataset.path);
}

// onKeyDown はツリーのキーボード操作を扱う（UI-031, IMP-248）。
async function onKeyDown(event) {
  // **修飾キー付きは扱わない。** Alt+← / Alt+→ は履歴の移動である
  // （UI-090, IMP-244）。ここで奪うと戻れなくなる。
  if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;

  const item = event.target.closest("li[data-path]");
  if (!item) return;

  switch (event.key) {
    case "ArrowDown":
      event.preventDefault();
      moveFocus(item, 1);

      return;

    case "ArrowUp":
      event.preventDefault();
      moveFocus(item, -1);

      return;

    case "ArrowRight":
      // **展開するだけ。子へは移らない**（UI-031）。
      event.preventDefault();
      if (isDir(item) && item.getAttribute("aria-expanded") !== "true") {
        await expand(item);
      }

      return;

    case "ArrowLeft":
      // **折りたたむだけ。親へは移らない**（UI-031, IMP-248 の NOTE）。
      event.preventDefault();
      if (isDir(item) && item.getAttribute("aria-expanded") === "true") {
        collapse(item);
      }

      return;

    case "Enter":
      event.preventDefault();
      await activate(item);

      return;

    default:
      // 他のキーは素通りさせる。
  }
}

// onFocusIn は着地点を、いまフォーカスした項目へ移す（IMP-248）。
function onFocusIn(event) {
  const item = event.target.closest("li[data-path]");
  if (item) setRovingTarget(item);
}

function isDir(item) {
  return item.dataset.dir === "true";
}

// moveFocus は見えている項目の間を 1 つ動く（IMP-248）。
function moveFocus(item, step) {
  const items = visibleItems();
  const next = items[items.indexOf(item) + step];
  if (!next) return;

  next.focus({ preventScroll: true });
  // **block を省かない。** 既定値では本文ペインまで動きうる。
  next.scrollIntoView({ block: "nearest" });
}

// visibleItems は見えている項目を上から順に返す（IMP-248）。
//
// **折りたたまれた中へは降りない。** 群は hidden 属性で切り替えており
// （IMP-202）、hidden ならその配下は見えていない。
// 切り詰めの行（li.tree-more）は data-path を持たないため入らない。
function visibleItems() {
  const out = [];

  walkVisible($("tree"), out);

  return out;
}

function walkVisible(container, out) {
  for (const item of container.children) {
    if (!item.dataset.path) continue;

    out.push(item);

    const group = childGroup(item);
    if (group && !group.hidden) walkVisible(group, out);
  }
}

// refreshFocusTarget は Tab の着地点を 1 つに保つ（IMP-248 の roving tabindex）。
//
// **ツリーを組み直すたびに呼ぶ。** 着地点の要素が組み直しで消えると、
// Tab がツリーを素通りする。
function refreshFocusTarget() {
  const items = visibleItems();
  if (items.length === 0) return;

  const focused = items.find((item) => item === document.activeElement);
  const selected = items.find((item) => item.classList.contains("selected"));

  setRovingTarget(focused || selected || items[0]);
}

// setRovingTarget は tabindex="0" を 1 つだけにする（IMP-248）。
function setRovingTarget(wanted) {
  for (const item of $("tree").querySelectorAll("li[data-path]")) {
    item.tabIndex = item === wanted ? 0 : -1;
  }
}

async function toggle(item) {
  if (item.getAttribute("aria-expanded") === "true") {
    collapse(item);
    return;
  }
  await expand(item);
}

// expand は子を**毎回読み直して**展開する。
//
// 折りたたんで開き直す操作が、そのまま手動更新を兼ねる（FR-035, DSP-330）。
async function expand(item) {
  const { nodes, error } = await api.readDir(item.dataset.path);
  if (error) {
    showMessage(errorText(error), "error");
    return false;
  }

  const group = childGroup(item);
  clear(group);
  fill(group, nodes, depthOf(item) + 1);
  group.hidden = false;

  item.setAttribute("aria-expanded", "true");
  setDirIcons(item, true);
  markSelected();
  refreshFocusTarget();

  return true;
}

function collapse(item) {
  childGroup(item).hidden = true;
  item.setAttribute("aria-expanded", "false");
  setDirIcons(item, false);
  // 畳んだ中に着地点が居ることがある（IMP-248）。
  refreshFocusTarget();
}

function fill(container, nodes, depth) {
  for (const node of nodes) {
    container.appendChild(createItem(node, depth));
  }

  // 切り詰めは一覧単位。先頭の要素が件数を持つ（IMP-304）。
  if (nodes.length > 0 && nodes[0].omitted > 0) {
    container.appendChild(createMore(nodes[0].omitted, depth));
  }
}

function createItem(node, depth) {
  const item = document.createElement("li");
  item.className = "tree-item";
  item.setAttribute("role", "treeitem");
  // roving tabindex（IMP-248）。0 を持つのは refreshFocusTarget が選ぶ 1 つだけ。
  item.tabIndex = -1;
  item.dataset.path = node.path;
  item.dataset.dir = String(node.isDir);

  const row = document.createElement("div");
  row.className = "tree-row";
  row.style.setProperty("--depth", String(depth));

  if (node.isDir) {
    item.setAttribute("aria-expanded", "false");
    row.appendChild(icon("icon-chevron-right", "tree-arrow"));
    row.appendChild(icon("icon-dir", "icon"));
  } else {
    row.appendChild(spacer());
    row.appendChild(icon("icon-file", "icon"));
  }

  row.appendChild(name(node.name));
  item.appendChild(row);

  if (node.isDir) {
    const group = document.createElement("ul");
    group.className = "tree-children";
    group.setAttribute("role", "group");
    group.hidden = true;
    item.appendChild(group);
  }

  return item;
}

// createMore は省略された件数を末尾に出す（FR-032, DSP-112）。
//
// data-path を持たせない。クリックしても何も起きない。
function createMore(count, depth) {
  const item = document.createElement("li");
  item.className = "tree-more";
  item.setAttribute("role", "none");

  const row = document.createElement("div");
  row.className = "tree-row";
  row.style.setProperty("--depth", String(depth));
  row.appendChild(spacer());
  row.appendChild(name(S.treeMore(count)));
  item.appendChild(row);

  return item;
}

// markSelected は表示中の文書のノードだけを選択状態にする（DSP-330）。
function markSelected() {
  const current = state.doc && !state.doc.outsideTree ? state.doc.path : "";

  for (const item of $("tree").querySelectorAll("li[data-path]")) {
    const selected = item.dataset.path === current;
    item.classList.toggle("selected", selected);

    if (selected) {
      item.setAttribute("aria-selected", "true");
    } else {
      item.removeAttribute("aria-selected");
    }
  }
}

function setDirIcons(item, expanded) {
  const uses = rowOf(item).querySelectorAll("use");
  uses[0].setAttribute("href", expanded ? "#icon-chevron-down" : "#icon-chevron-right");
  uses[1].setAttribute("href", expanded ? "#icon-filetree" : "#icon-dir");
}

function childByName(container, wanted) {
  for (const item of container.children) {
    if (!item.dataset.path) continue;
    if (nameOf(item) === wanted) return item;
  }

  return null;
}

function childGroup(item) {
  return item.querySelector(":scope > ul");
}

function rowOf(item) {
  return item.querySelector(":scope > .tree-row");
}

function nameOf(item) {
  return rowOf(item).querySelector(".tree-name").textContent;
}

function depthOf(item) {
  return Number(rowOf(item).style.getPropertyValue("--depth") || 0);
}

function name(text) {
  const element = document.createElement("span");
  element.className = "tree-name";
  element.textContent = text;

  return element;
}

function spacer() {
  const element = document.createElement("span");
  element.className = "tree-arrow";

  return element;
}

