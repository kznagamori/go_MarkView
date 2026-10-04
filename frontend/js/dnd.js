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

// ウィンドウ内で始まったドラッグか（BUG-020）。
//
// **エンジンによって渡ってくる型が違う**。Chromium は OS からのファイルのドラッグに
// "Files" を入れるが、**WebKitGTK は text/uri-list しか渡さない**。その text/uri-list は
// 本文のリンクをドラッグしたときにも付くため、型だけでは見分けられない。
// dragstart が出たかどうかで、ウィンドウ内で始まったものを除く。
let internal = false;

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

  // ウィンドウ内で始まったドラッグを覚える（BUG-020）。
  //
  // **中止された dragstart では旗を立てない。** 既定の動作を止められた dragstart には
  // **dragend が続かない**ため（拡大画面の台紙が止めている。IMP-253）、立てたままにすると
  // 旗が固定され、**以後ドロップの案内が二度と出なくなる。**
  // 台紙のリスナは対象の段階で動き、ここ（泡立ちの段階）へ来たときには既に止まっている。
  window.addEventListener("dragstart", (event) => (internal = !event.defaultPrevented));
  window.addEventListener("dragend", () => (internal = false));

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
  // **WebView がドロップされたファイルへ遷移するのを防ぐ**（AR-060）。
  // Wails のランタイムも同じことを行うが、ウィンドウ内でのページ遷移を
  // 起こさないという規約を、外部のランタイムの実装に委ねない。
  //
  // **ファイルの判定に掛けない**（BUG-020）。判定は案内を出すためのもので
  // あり、遷移を止める条件ではない。WebKitGTK は "Files" を渡さないため、
  // 掛けると素通りして**画面がファイルの中身に置き換わり、戻せなくなる**。
  // ウィンドウ内のドラッグでも、リンクを落とせば遷移しうる。
  event.preventDefault();
}

function onLeave(event) {
  if (!hasFiles(event)) return;

  depth = Math.max(0, depth - 1);
  if (depth === 0) hide();
}

function onDrop(event) {
  // onOver と同じ理由で、条件を付けずに止める（AR-060, BUG-020）
  event.preventDefault();

  const files = hasFiles(event);
  internal = false;
  if (!files) return;

  // ドロップで確定するため、深さの計算を待たずに閉じる（UI-070）。
  depth = 0;
  hide();
}

// hasFiles は OS からのファイルのドラッグかを判定する。**案内の出し分けにだけ使う**
// （遷移を止める条件にしない。BUG-020）。
//
// 本文中のテキストやリンクを動かした場合など、ウィンドウ内で完結する
// ドラッグではオーバーレイを出さない。
function hasFiles(event) {
  if (internal || !event.dataTransfer) return false;

  // **WebKitGTK は "Files" を入れない**（BUG-020）。text/uri-list も受ける。
  const types = [...event.dataTransfer.types];
  return types.includes("Files") || types.includes("text/uri-list");
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
