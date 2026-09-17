// treenodes.js — ファイルツリーの項目の組み立てと、項目の中を読む関数（FR-032, DSP-112, DSP-330, IMP-248）。
//
// **DOM だけを扱う。** Go を呼ばず、state も読まない。filetree.js（読み込み・展開・強調・操作）が
// 400 行の目安（IMP-011）を超えたため分けた。**filetree.js を import しない**（循環参照を作らない）。
//
// 項目は `li.tree-item[data-path][data-dir] > div.tree-row` と、ディレクトリなら子の群の
// `ul.tree-children`。**名前はパスから作らず、Go 側が返した name を表示する**（IMP-300）。

import { S } from "./strings.js";
import { icon } from "./util.js";

// fill は ReadDir の結果（FileNodeDTO の並び。IMP-304）から項目を作って container へ足す（FR-032, DSP-112）。
export function fill(container, nodes, depth) {
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

// setDirIcons はディレクトリの項目のアイコンを、展開しているかに合わせる（DSP-330）。
export function setDirIcons(item, expanded) {
  const uses = rowOf(item).querySelectorAll("use");
  uses[0].setAttribute("href", expanded ? "#icon-chevron-down" : "#icon-chevron-right");
  uses[1].setAttribute("href", expanded ? "#icon-filetree" : "#icon-dir");
}

// childByName は container の直下から、名前が wanted の項目を返す。無ければ null（DSP-331）。
export function childByName(container, wanted) {
  for (const item of container.children) {
    if (!item.dataset.path) continue;
    if (nameOf(item) === wanted) return item;
  }

  return null;
}

// childGroup はディレクトリの項目の子の群（ul）を返す。ファイルの項目では null。
export function childGroup(item) {
  return item.querySelector(":scope > ul");
}

function rowOf(item) {
  return item.querySelector(":scope > .tree-row");
}

function nameOf(item) {
  return rowOf(item).querySelector(".tree-name").textContent;
}

// depthOf は項目の深さ（ツリールートの直下が 0）を返す。
export function depthOf(item) {
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
