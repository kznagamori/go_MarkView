// treefilter.js — ファイルツリーのフィルターの枚（FR-037, UI-033, IMP-271, DSP-115）。
//
// ペインの見出しのボタン（#btn-treefilter）で開く重ね表示の枚で、ツリーに並べる種類（図・画像）を選ぶ。
// **拡張子をこのモジュールにも strings.js にも書かない**——Go の ExtensionsDTO（mdfile の定義。IMP-303）から
// 組み立てる（FR-037 の「2 か所に書かない」）。
//
// **Esc はここで扱わない**（IMP-244 の振り分けの (4)。2 か所に書くと順序が入れ替わる）。**外のクリックでは
// 閉じない**（UI-033）。モーダルではないため、Tab も閉じ込めない。自動で閉じる入口（ファイルツリーのペインを
// 隠す・拡大画面・情報ウィンドウ・エディタ選択ウィンドウ）は main.js が closeTreeFilter(false) を呼ぶ。
//
// **filetree.js を import する（読み直し）。filetree.js / main.js / panes.js はこのモジュールを import しない。**

import { reloadTreeKeepingState } from "./filetree.js";
import { state, saveConfig } from "./state.js";
import { S } from "./strings.js";
import { $, icon } from "./util.js";

// ボタンと枚の間、枚の左端とウィンドウの左端の最小の間（DSP-115）。位置を JavaScript が求めるため持つ
const GAP = 4;
const EDGE = 8;

let deps = { focusViewer() {} };

// 図と画像のチェックボックス（Markdown は外せないため持たない）
let boxes = null;

// initTreeFilter は枚の中身を組み立てて配線する（IMP-211, IMP-271）。extensions は ExtensionsDTO、
// given は { focusViewer }（自動で閉じたときに、枚の中のフォーカスを本文ペインへ移す。IMP-220）。
export function initTreeFilter(extensions, given) {
  deps = Object.assign({ focusViewer() {} }, given);

  const join = (list) => (list || []).join("/");
  const sheet = $("treefilter");
  sheet.setAttribute("aria-label", S.filterTitle);

  const head = document.createElement("div");
  head.className = "treefilter-head";
  const title = document.createElement("span");
  title.className = "treefilter-title";
  title.textContent = S.filterTitle;
  const close = document.createElement("button");
  close.type = "button";
  close.className = "pane-btn treefilter-close";
  close.setAttribute("aria-label", S.close);
  close.appendChild(icon("icon-close"));
  close.addEventListener("click", () => closeTreeFilter(true));
  head.append(title, close);

  const markdown = option(S.filterMarkdown(join(extensions && extensions.markdown)), S.tipFilterMarkdown, null);
  const diagrams = option(S.filterDiagrams(join(extensions && extensions.diagrams)), S.tipFilterDiagrams, "diagrams");
  const images = option(S.filterImages(join(extensions && extensions.images)), S.tipFilterImages, "images");
  sheet.append(head, markdown.label, diagrams.label, images.label);
  boxes = { diagrams: diagrams.input, images: images.input };

  syncBoxes();
  updateTip();

  $("btn-treefilter").addEventListener("click", toggleTreeFilter);
  // ウィンドウとファイルツリーのペインの幅が変わったら、位置を合わせ直す（UI-033）
  window.addEventListener("resize", place);
  if (typeof ResizeObserver === "function") new ResizeObserver(place).observe($("pane-filetree"));
}

// toggleTreeFilter はフィルターボタンの入口（UI-033）。開いていれば閉じてボタンへフォーカスを戻す。
export function toggleTreeFilter() {
  if (isTreeFilterOpen()) {
    closeTreeFilter(true);
    return;
  }

  syncBoxes();
  $("treefilter").hidden = false;
  $("btn-treefilter").setAttribute("aria-expanded", "true");
  place();
  // 最初の押せるチェックボックス（Diagrams）へ移す（UI-033）
  boxes.diagrams.focus({ preventScroll: true });
}

export function isTreeFilterOpen() {
  return !$("treefilter").hidden;
}

// isTreeFilterFocused は枚の中にフォーカスがあるか（IMP-244 の Esc の (4)）。
export function isTreeFilterFocused() {
  return isTreeFilterOpen() && $("treefilter").contains(document.activeElement);
}

// closeTreeFilter は枚を閉じる。閉じたら true、開いていなければ false。
//
// restoreFocus: 利用者が閉じたとき（Esc・閉じるボタン・フィルターボタン）は true でボタンへ戻す。自動で
// 閉じるときは false——**枚の中にフォーカスがあれば本文ペインへ移す**（隠したペインの中に残さない。開いた
// ダイアログや拡大画面は、その後で自分へフォーカスを移す。IMP-271）。
export function closeTreeFilter(restoreFocus) {
  const sheet = $("treefilter");
  if (sheet.hidden) return false;

  // 隠すとフォーカスが外れるため、先に控える
  const hadFocus = sheet.contains(document.activeElement);
  sheet.hidden = true;
  $("btn-treefilter").setAttribute("aria-expanded", "false");

  if (restoreFocus) $("btn-treefilter").focus({ preventScroll: true });
  else if (hadFocus) deps.focusViewer();
  return true;
}

// option は選択肢の行（label.treefilter-option）を作る。kind が null なら Markdown（常に表示。外せない）。
function option(text, tip, kind) {
  const label = document.createElement("label");
  label.className = "treefilter-option";
  label.dataset.tip = tip;

  const input = document.createElement("input");
  input.type = "checkbox";
  const span = document.createElement("span");
  span.textContent = text;
  label.append(input, span);

  if (kind === null) {
    input.checked = true;
    input.disabled = true;
  } else {
    input.addEventListener("change", () => onChange(kind, input.checked));
  }
  return { label, input };
}

// onChange は選んだ種類を写し、**設定を保存し終えてから**ツリーを読み直す（FR-037, IMP-271）。Go の ReadDir は
// 保存した設定で絞るため、保存を待たずに読み直すと古い一覧が出る。
async function onChange(kind, checked) {
  if (kind === "diagrams") state.treeShowDiagrams = checked;
  else state.treeShowImages = checked;
  updateTip();

  await saveConfig();
  await reloadTreeKeepingState();
}

// syncBoxes はチェックボックスを状態に合わせる（起動時の値は main.js が init.config から写す。IMP-211）。
function syncBoxes() {
  boxes.diagrams.checked = state.treeShowDiagrams;
  boxes.images.checked = state.treeShowImages;
}

// updateTip はボタンのツールチップと読み上げ名を、表示している種類にする（UI-033, IMP-247, IMP-295）。
// **閉じている間の状態はこれで示し、ボタンの見た目は変えない**（UI-033）。
function updateTip() {
  const kinds = [S.kindMarkdown];
  if (state.treeShowDiagrams) kinds.push(S.kindDiagrams);
  if (state.treeShowImages) kinds.push(S.kindImages);

  const button = $("btn-treefilter");
  const tip = S.tipFilter(kinds);
  button.dataset.tip = tip;
  button.setAttribute("aria-label", tip);
}

// place は枚をボタンの下に置く（DSP-115）。**右端をボタンの右端にそろえ**、左端がウィンドウの左端から 8px より
// 左へ出るなら 8px に寄せる（ペインの右へはみ出す）。幅は CSS が決める（320px。狭いウィンドウでは幅 − 16px）。
function place() {
  const sheet = $("treefilter");
  if (sheet.hidden) return;

  const button = $("btn-treefilter").getBoundingClientRect();
  sheet.style.left = `${Math.max(EDGE, button.right - sheet.offsetWidth)}px`;
  sheet.style.top = `${button.bottom + GAP}px`;
}
