// editmode.js — 編集モード（FR-140〜FR-144, UI-055, IMP-260〜IMP-263, DSP-126, DSP-127, DSP-320, DSP-321）。
//
// **判断と書き込みは Go 側が持つ**（IMP-195）。ここにあるのは、指示を送ることと、見た目を先に変えて結果で
// 整えることだけである。**状態の正は Go 側にあり、画面は DocumentDTO と EditModeDTO の値を写す**（IMP-302）。
//
// **viewer.js / overlay.js / main.js / navigate.js / docswitch.js を import しない**（IMP-250。viewer.js と
// docswitch.js がこのモジュールを import する）。本文ペインへのフォーカスは main.js が deps で渡す。
// **目印の鍵を読まない**——照合と値は refs.js に聞く（IMP-260）。**セルの編集欄（IMP-262）は celledit.js、
// 編集欄が開いている間の扱い（拒む・保留）は blockedit.js にある**（4.75.0。IMP-200, IMP-264）。

import { isEditBoxFocused, isEditBoxOpen, refuseIfEditing } from "./blockedit.js";
import { markEditableCells } from "./celledit.js";
import { markEditableBlocks } from "./codeedit.js";
import { updateFormatButtons } from "./format.js";
import { isOwnRef, ownRef } from "./refs.js";
import { state } from "./state.js";
import { S } from "./strings.js";
import { $ } from "./util.js";

let deps = { api: null, notify() {}, focusViewer() {} };

// initEditMode は編集モードを配線する（IMP-260）。given: { api, notify(error), focusViewer() }。
//   api は api.js（setEditMode / setTask / getCellSource / setCell / undoEdit / redoEdit）。
export function initEditMode(given) {
  deps = { ...deps, ...given };

  $("editmode-badge").textContent = S.editModeBadge;
  // チェックボックスの切り替えは #markdown の 1 つのリスナで受ける（イベント委譲。IMP-261）
  $("markdown").addEventListener("change", onCheckboxChange);
  // ラベルをスクロールバーの内側に置くため、ウィンドウ幅が変わったら測り直す（DSP-126）
  window.addEventListener("resize", measureScrollbar);
}

// toggleEditMode はボタンと Ctrl+Shift+M の共通の入口（IMP-260, IMP-244）。何もしなければ false。
//
// **呼ぶ前に見た目を変えない**——開始できなかった場合に戻す処理が要り、得るものが無い。**同期で真偽を返す**
// （shortcuts.js は false でなければ既定の動作を止める）。結果は届いた時点で写す。
export function toggleEditMode() {
  if (!state.editable) return false;
  // **編集欄が開いていれば、終えることを拒む**（FR-140。開くときは編集欄が開いていない）
  if (state.editMode && refuseIfEditing()) return true;

  deps.api
    .setEditMode(!state.editMode)
    .then(applyModeResult)
    .catch(() => {});
  return true;
}

// applyEditMode は DocumentDTO の編集モードの状態を写す（IMP-220 の手順 6d, IMP-260）。
//
// **Go 側の値を正とし、フロントエンドが持っていた値で上書きしない**（IMP-302）。ただし版（editSeq）が
// 手元より小さければ、到着順が入れ替わった古い値として写さない。**本文は差し替わっているため、画面への
// 反映（チェックボックスとセルの印）は版に関わらず行う。**
export function applyEditMode(doc) {
  if (doc.editSeq >= state.editSeq) {
    state.editSeq = doc.editSeq;
    state.editable = doc.editable === true;
    state.editMode = state.editable && doc.editMode === true;
  }
  render();
}

// leaveEditMode は編集モードを終えた状態を写す（IMP-260）。docswitch.js の leaveDocument（文書の切り替えと
// 状態画面への移行。IMP-250）と、document:removed の受け口（main.js）から呼ぶ。
//
// Go 側は既に終えており、次に読み込めるまで開始させない（IMP-195, FR-140 の表）。**editSeq は変えない**
// ——次に届く DocumentDTO / EditModeDTO の版と比べる基準であり、戻すと遅れて届いた古い値を写してしまう。
export function leaveEditMode() {
  state.editable = false;
  state.editMode = false;
  render();
}

// isEditableCell は、編集モード中で element が鍵の合うセルの中にあるかを返す（IMP-223 のリンクの捕捉）。
export function isEditableCell(element) {
  if (!state.editMode || !(element instanceof Element)) return false;

  const cell = element.closest("th, td");
  return Boolean(cell) && isOwnRef(cell, "cell");
}

// isEditableBlock は、編集モード中で element が鍵の合う図のブロック（Mermaid・PlantUML）の中にあるかを返す
// （IMP-223 のリンクの捕捉。FR-145）。ダブルクリックの 1 回目で、図の中のリンクから文書が切り替わらないようにする。
export function isEditableBlock(element) {
  if (!state.editMode || !(element instanceof Element)) return false;

  const block = element.closest(".code-block");
  return Boolean(block) && (isOwnRef(block, "mermaid") || isOwnRef(block, "plantuml"));
}

// undoEdit / redoEdit は直前の書き込みを取り消す・やり直す（IMP-263, FR-144）。編集モードでなければ false
// （IMP-244。既定の動作を止めない）。
//
// **見た目を先に変えない**——どこが変わるかをフロントエンドは知らない（Patch は Go 側にある。IMP-108）。
// 表示は document:changed で届く。取り消すものが無い（changed が偽）・古い指示（stale）は通知しない。
export function undoEdit() {
  return sendHistory(() => deps.api.undoEdit());
}

export function redoEdit() {
  return sendHistory(() => deps.api.redoEdit());
}

function sendHistory(call) {
  if (!state.editMode) return false;
  // **編集欄が開いている間（開きかけを含む）は、ファイルの取り消し・やり直しを受け付けない**（FR-144, FR-140）。
  // 中にフォーカスがあれば入力欄の扱い（IMP-244。入力欄は shortcuts.js が素通しし、Save / Cancel では何もしない）、
  // 外なら拒む
  if (isEditBoxFocused()) return false;
  if (refuseIfEditing()) return true;

  call()
    .then((result) => {
      if (result && result.error) deps.notify(result.error);
    })
    .catch(() => {});
  return true;
}

// applyModeResult は SetEditMode の結果（EditModeDTO）を写す（IMP-260）。
//
// - 版が手元より小さければ写さない（到着順が入れ替わった古い値。IMP-302）。
// - **state.editable が偽のときは、on が真の値を写さない。** 状態画面への移行と document:removed は版を
//   運ばないため、その前に送った SetEditMode(true) の結果（版は手元より大きい）が後から届くと、状態画面や
//   削除の後に枠とラベルとボタンの ON が出る（FR-140, DSP-320）。
function applyModeResult(result) {
  if (!result || result.seq < state.editSeq) return;
  if (result.on && !state.editable) return;

  state.editSeq = result.seq;
  state.editMode = result.on === true;
  render();
}

// render は state の編集モードを画面へ写す（IMP-260, DSP-126, DSP-320）。
//
// **ボタン・枠・ラベル・チェックボックス・セルの印を同じ関数で同時に切り替える**（DSP-320）。ボタンだけが
// ON で枠が出ていない時間を作らない。**どの経路で終わっても（ボタン・Ctrl+Shift+M・削除・読み直し・
// 状態画面）この 1 か所を通る。**
function render() {
  const on = state.editable && state.editMode;

  const button = $("btn-editmode");
  button.disabled = !state.editable;
  button.setAttribute("aria-pressed", String(on));

  $("viewer-frame").classList.toggle("is-editing", on);
  $("editmode-badge").hidden = !on;
  // ツールバーの修飾の群は編集モードの間だけ出す（UI-020, IMP-265, IMP-202）。押せるかは選択で決まる
  $("tb-format").hidden = !on;
  updateFormatButtons();
  if (on) measureScrollbar();

  const markdown = $("markdown");
  for (const input of markdown.querySelectorAll('input[type="checkbox"]')) {
    // 鍵の合わない input（生 HTML）には何もしない——disabled と pointer-events: none のまま（DSP-121）
    if (!isOwnRef(input, "task")) continue;
    input.disabled = !on;
    input.classList.toggle("is-editable", on);
  }
  // セルの印（IMP-262）。**編集欄が開いている間は、編集モードを終える表示へ移らない**——終えることは拒み、自動で
  // 終える知らせ（削除・不正なバイト列）は blockedit.js が控えて、閉じた時点で当てる（IMP-260, IMP-264）
  markEditableCells(markdown, on);
  // コードブロックと図の編集のボタン（IMP-264）
  markEditableBlocks(markdown, on);
}

// measureScrollbar は #viewer のスクロールバーの幅を --viewer-scrollbar-width として #viewer-frame に与える
// （DSP-126）。**CSS だけでスクロールバーの幅を得る手段は無い**（IMP-202）。
function measureScrollbar() {
  const viewer = $("viewer");
  $("viewer-frame").style.setProperty("--viewer-scrollbar-width", `${viewer.offsetWidth - viewer.clientWidth}px`);
}

// onCheckboxChange はタスクのチェックボックスの切り替えを書き込む（IMP-261, FR-141）。
//
// **見た目はブラウザが既に反転している**（change の時点）。これが FR-141 の「即座に反転」（NFR-012）に当たる。
// 結果に応じて整える: changed は何もしない（再描画で同じ状態が届く）。changed が偽で stale も error も無い
// （既にその状態だった）ときもそのまま。stale は戻す（通知しない。FR-143）。error は戻して通知する。
async function onCheckboxChange(event) {
  const input = event.target;
  if (!state.editMode || !(input instanceof HTMLInputElement) || input.type !== "checkbox") return;

  const ref = ownRef(input, "task");
  if (ref === null) return;

  // 編集欄が開いていれば（開きかけを含む）、ブラウザが反転した見た目を戻して拒む（FR-140, FR-141。Go を呼ばない）
  if (refuseIfEditing()) {
    input.checked = !input.checked;
    return;
  }

  const checked = input.checked;
  const result = await deps.api.setTask(ref, checked).catch(() => null);
  if (result && result.changed) return;
  if (result && result.error) deps.notify(result.error);
  if (result && !result.stale && !result.error) return;

  // **戻すのは、その input がまだ画面にある場合だけ**——再描画が先に届いていれば input は差し替わっており、
  // 新しい DOM がファイルの状態を示している。戻り値とイベントのどちらが先でも同じ結果になる（IMP-316）
  if (input.isConnected) input.checked = !checked;
}
