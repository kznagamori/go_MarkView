// toolbar.js — ツールバー（IMP-202, DSP-100〜102）。
//
// ボタンの配線と、ツールチップ・aria-label の付与を行う（IMP-295）。

import { S } from "./strings.js";
import { keyLabel } from "./shortcuts.js";
import { state } from "./state.js";
import { $ } from "./util.js";

// initToolbar はツールバーを初期化する（IMP-211）。
//
// deps には各ボタンの処理を渡す。**未実装のボタンには何も渡さない。**
// 押しても無反応になるが、動かない処理を仮に繋ぐより状態がはっきりする。
export function initToolbar(deps) {
  // 戻る・進む（UI-020, FR-051。4.75.0）。押せるかは applyHistory が決める（IMP-323）
  setTip($("btn-back"), S.tipBack, "back");
  setTip($("btn-forward"), S.tipForward, "forward");
  setTip($("btn-open"), S.tipOpen, "open");
  setTip($("btn-reload"), S.tipReload, "reload");
  setTip($("btn-outline"), S.tipOutline, "outline");
  setTip($("btn-filetree"), S.tipFileTree, "filetree");
  setTip($("btn-edit"), S.tipOpenInEditor, "edit");
  // 表記は shortcuts.js の editMode（Ctrl+Shift+M）から付く（IMP-244, IMP-290）
  setTip($("btn-editmode"), S.tipEditMode, "editMode");
  // 修飾の群（UI-020。4.75.0）。取り消し線のキー表記は shortcuts.js の strike から付く（IMP-244, IMP-290）。
  // 文字色はショートカットを持たない。配線と押せるかは format.js（IMP-265）
  setTip($("btn-strike"), S.tipStrike, "strike");
  setTip($("btn-color"), S.tipColor, "");
  setTip($("btn-about"), S.tipAbout, "about");
  // テーマのツールチップは切り替え先で変わるため applyTheme が設定する（IMP-243）

  // 戻る・進むは Alt+← / Alt+→・マウスのサイドボタンと同じ入口（navigate.js の goBack / goForward）を通す
  // （FR-051, IMP-244）
  bind("btn-back", deps.onBack);
  bind("btn-forward", deps.onForward);
  bind("btn-open", deps.onOpen);
  bind("btn-reload", deps.onReload);
  bind("btn-theme", deps.onTheme);
  bind("btn-outline", deps.onOutline);
  bind("btn-filetree", deps.onFileTree);
  bind("btn-edit", deps.onEdit);
  // 編集モードの切り替え（IMP-260 の toggleEditMode）は P18 で渡す。**それまでボタンは disabled のまま**
  // であり（IMP-202）、渡さなくても押せない。
  bind("btn-editmode", deps.onEditMode);
  bind("btn-about", deps.onAbout);
}

// applyHistory は戻れるか・進めるか（HistoryDTO）を写し、戻る・進むのボタンの淡色を決める（IMP-323,
// UI-021, FR-051。4.75.0）。**受け口はこの 1 つとする**——history:changed の購読と起動時（IMP-211）が
// 同じ関数を通す。
//
// **seq が手元より小さい値は写さない**（到着順は決まっていない。EditModeDTO と同じ考え方。IMP-316）。
// **フロントエンドで履歴を数え直さない**（履歴の正は Go 側。IMP-300 の 4）。開けなかった項目を Go 側が
// 取り除いた後の値がそのまま届く（BUG-023）。
export function applyHistory(dto) {
  if (!dto || dto.seq < state.history.seq) return;

  state.history = { canBack: dto.canBack === true, canForward: dto.canForward === true, seq: dto.seq };
  $("btn-back").disabled = !state.history.canBack;
  $("btn-forward").disabled = !state.history.canForward;
}

// syncTargetButtons は、画面の対象で押せるかが決まるボタン（再読み込み・エディタで開く）の活性を決める
// （UI-021, FR-015, FR-090, IMP-202）。状態画面を出し入れするたびに overlay.js が呼ぶ。
//
// - **再読み込みは welcome のときだけ淡色**（4.75.0。v1.1.0 は淡色にしていなかった）。状態画面の間は
//   その対象を読み直せる（FR-015）
// - **エディタで開くは welcome と、画面の対象が画像ファイルのときに淡色**（FR-090。4.75.0）。
//   confirm-large / too-large / render-error を出している間は押せる状態に保つ——大きすぎて開けないファイルや
//   描画に失敗したファイルこそ、エディタで中身を確かめたい（UI-021）。種類は表示中なら state.doc.kind、
//   状態画面の間は state.target.targetKind（IMP-307）で見る
//
// **判定はここだけとし、呼び出し側に真偽値を渡させない。** これは Go 側の `App.target` が空かどうか
// （IMP-190）と同じ条件であり、両者が食い違うと「押せるのに何も起きない」か「押せるはずなのに押せない」に
// なる。引数で受けると、呼ぶ場所ごとに条件を書くことになり、いつか食い違う。
export function syncTargetButtons() {
  const screen = $("state-screen");

  // 状態画面が出ていなければ本文を表示している。出ていても welcome 以外なら対象のファイルがある。
  const welcome = !screen.hidden && screen.dataset.kind === "welcome";
  const kind = state.doc ? state.doc.kind : state.target ? state.target.targetKind : "";

  $("btn-reload").disabled = welcome;
  $("btn-edit").disabled = welcome || kind === "image";
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
