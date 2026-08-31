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
  setTip($("btn-about"), S.tipAbout, "about");
  // テーマのツールチップは切り替え先で変わるため applyTheme が設定する（IMP-243）

  bind("btn-open", deps.onOpen);
  bind("btn-reload", deps.onReload);
  bind("btn-theme", deps.onTheme);
  bind("btn-outline", deps.onOutline);
  bind("btn-filetree", deps.onFileTree);
  bind("btn-about", deps.onAbout);
}

// setTip はツールチップと aria-label を与える（UI-024, IMP-295）。
//
// 表示は `Open / 開く (Ctrl+O)` の形。文言は strings.js、キー表記は
// shortcuts.js から採り、どちらも定義を 1 か所に保つ（IMP-290）。
export function setTip(button, text, shortcutId) {
  const key = keyLabel(shortcutId);
  const label = key ? `${text} (${key})` : text;

  button.title = label;
  button.setAttribute("aria-label", label);
}

function bind(id, handler) {
  if (handler) {
    $(id).addEventListener("click", handler);
  }
}
