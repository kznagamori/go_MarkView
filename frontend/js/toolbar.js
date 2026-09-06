// toolbar.js — ツールバー（IMP-202, DSP-100〜102）。
//
// ボタンの配線と、ツールチップ・aria-label の付与を行う（IMP-295）。

import { S } from "./strings.js";
import { keyLabel } from "./shortcuts.js";
import { $ } from "./util.js";

// initToolbar はツールバーを初期化する（IMP-211）。
//
// deps には各ボタンの処理を渡す。**未実装のボタンには何も渡さない。**
// 押しても無反応になるが、動かない処理を仮に繋ぐより状態がはっきりする。
export function initToolbar(deps) {
  setTip($("btn-open"), S.tipOpen, "open");
  setTip($("btn-reload"), S.tipReload, "reload");
  setTip($("btn-outline"), S.tipOutline, "outline");
  setTip($("btn-filetree"), S.tipFileTree, "filetree");
  setTip($("btn-edit"), S.tipEdit, "edit");
  setTip($("btn-about"), S.tipAbout, "about");
  // テーマのツールチップは切り替え先で変わるため applyTheme が設定する（IMP-243）

  bind("btn-open", deps.onOpen);
  bind("btn-reload", deps.onReload);
  bind("btn-theme", deps.onTheme);
  bind("btn-outline", deps.onOutline);
  bind("btn-filetree", deps.onFileTree);
  bind("btn-edit", deps.onEdit);
  bind("btn-about", deps.onAbout);
}

// syncEditButton は「エディタで開く」の活性を画面の状態から決める
// （UI-021, FR-090）。状態画面を出し入れするたびに overlay.js が呼ぶ。
//
// **淡色にするのは welcome のときだけである。** confirm-large / too-large /
// render-error を出している間は押せる状態に保つ。大きすぎて開けないファイルや
// 描画に失敗したファイルこそ、エディタで中身を確かめたい（UI-021）。
//
// **判定はこの 1 つの式だけとし、呼び出し側に真偽値を渡させない。** これは
// Go 側の `App.target` が空かどうか（IMP-190）と同じ条件であり、両者が
// 食い違うと「押せるのに何も起きない」か「押せるはずなのに押せない」になる。
// 引数で受けると、呼ぶ場所ごとに条件を書くことになり、いつか食い違う。
export function syncEditButton() {
  const screen = $("state-screen");

  // 状態画面が出ていなければ本文を表示している。出ていても welcome 以外なら
  // 対象のファイルがある。
  $("btn-edit").disabled = !screen.hidden && screen.dataset.kind === "welcome";
}

// canEdit は「エディタで開く」を実行できるかを返す（UI-021, UI-090）。
//
// **ボタン自身の状態を読む。** ショートカット（Ctrl+E）とボタンで別々に
// 判定すると、片方だけ直したときに食い違う。
export function canEdit() {
  return !$("btn-edit").disabled;
}

// setTip はツールチップと aria-label を与える（UI-024, IMP-295）。
//
// 表示は `Open / 開く (Ctrl+O)` の形。文言は strings.js、キー表記は
// shortcuts.js から採り、どちらも定義を 1 か所に保つ（IMP-290）。
//
// **title ではなく data-tip に置く**（IMP-247）。DSP-102 が定める見た目と
// 遅延はブラウザ既定のツールチップでは満たせないため、tooltip.js が
// 自前で描く。title を残すと両方が出る。
export function setTip(button, text, shortcutId) {
  const key = keyLabel(shortcutId);
  const label = key ? `${text} (${key})` : text;

  button.dataset.tip = label;
  button.setAttribute("aria-label", label);
}

function bind(id, handler) {
  if (handler) {
    $(id).addEventListener("click", handler);
  }
}
