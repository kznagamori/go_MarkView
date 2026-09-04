// dnd.js — ドラッグ＆ドロップ（FR-011, UI-070, IMP-245, DSP-190）。
//
// **パスを受け取るのは Go 側である**（IMP-313）。Wails の OnFileDrop が
// 絶対パスを渡し、対象の選択（複数ファイル・ディレクトリ・Markdown 以外）は
// すべて Go 側で判定する（FR-011）。HTML5 の DataTransfer からは実体の
// パスが得られないため、そちらに依存しない。
//
// ここで行うのは**ドロップ可能であることの表示だけ**とする。

import { OnFileDrop } from "../wailsjs/runtime/runtime.js";
import { S } from "./strings.js";
import { $, clear, icon } from "./util.js";

// ドラッグ中の入れ子の深さ。**カウンタ方式にする**（IMP-245）。
//
// dragleave はウィンドウの外へ出たときだけでなく、ウィンドウ内の要素間を
// 移動しただけでも発生する。単純に「dragleave で隠す」とすると、本文の上を
// 動かしている最中にオーバーレイが点滅する。
let depth = 0;

// initDnd はドロップの受け口とドラッグ中の表示を配線する（IMP-211, IMP-245）。
export function initDnd() {
  buildOverlay();

  // **Wails のランタイム側の drop リスナを取り付ける**（IMP-245）。
  //
  // Windows ではこれが唯一の手段であり、**呼ばなければドロップは一切届かない。**
  // 起動オプションの EnableFileDrop（main.go）だけでは足りない。
  // Linux は GTK の signal で受けるため呼ばなくても届くが、経路を OS で
  // 分けない。
  //
  // **コールバックは空でよい。** パスの判定は Go 側が行い（IMP-313）、
  // 結果はイベント（document:opened / tree:root-changed / error）で届く
  // （IMP-320, IMP-322）。ここに処理を書くと経路が 2 つになる。
  //
  // 第 2 引数（useDropTarget）は true。ただし**これが検査するのは JS 側の
  // コールバックだけ**であり、Go 側のコールバックは検査を通らずに呼ばれる。
  OnFileDrop(() => {}, true);

  window.addEventListener("dragenter", onEnter);
  window.addEventListener("dragover", onOver);
  window.addEventListener("dragleave", onLeave);
  window.addEventListener("drop", onDrop);
}

function onEnter(event) {
  if (!hasFiles(event)) return;

  depth += 1;
  show();
}

function onOver(event) {
  if (!hasFiles(event)) return;

  // **WebView がドロップされたファイルへ遷移するのを防ぐ**（AR-060）。
  // Wails のランタイムも同じことを行うが、ウィンドウ内でのページ遷移を
  // 起こさないという規約を、外部のランタイムの実装に委ねない。
  event.preventDefault();
}

function onLeave(event) {
  if (!hasFiles(event)) return;

  depth = Math.max(0, depth - 1);
  if (depth === 0) hide();
}

function onDrop(event) {
  if (!hasFiles(event)) return;

  event.preventDefault();

  // ドロップで確定するため、深さの計算を待たずに閉じる（UI-070）。
  depth = 0;
  hide();
}

// hasFiles は OS からのファイルのドラッグかを判定する。
//
// 本文中のテキストを選択して動かした場合など、ウィンドウ内で完結する
// ドラッグではオーバーレイを出さない。
function hasFiles(event) {
  return Boolean(event.dataTransfer) && [...event.dataTransfer.types].includes("Files");
}

function show() {
  $("dropzone").hidden = false;
}

function hide() {
  $("dropzone").hidden = true;
}

// buildOverlay はオーバーレイの中身を組み立てる（DSP-190）。
//
// 文言は strings.js から採る（IMP-290）。
function buildOverlay() {
  const zone = $("dropzone");
  clear(zone);

  zone.appendChild(icon("icon-open", "icon dropzone-icon"));

  const hint = document.createElement("p");
  hint.className = "dropzone-hint";
  hint.textContent = S.dropHint;
  zone.appendChild(hint);
}
