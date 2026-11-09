// celledit.js — セルの編集欄（FR-142, UI-055, IMP-262, DSP-127, DSP-321）。
//
// 4.75.0 で editmode.js から分けた（IMP-200）。**開いている間の扱い（拒む・保留・確定の待ち・開き直し）は
// blockedit.js が持つ**（IMP-264）。ここはセルの中に編集欄を置き、確定と取り消しを行うだけである。
//
// **フォーカスが外れても取り消さない**（FR-142。4.73.0。v1.1.0 は blur で取り消していた）。**失敗したら入力ごと
// 開き直す**（4.74.0。v1.1.0 は入力を捨てていた）。**セルの文字列の整形（改行・前後の空白・`|` のエスケープ）は
// しない**——Go 側の PlanCell（IMP-106）が行う。2 か所で整形すると、見た目と書き込まれる内容が食い違う。
//
// **viewer.js / overlay.js / main.js / navigate.js / docswitch.js / editmode.js を import しない**。目印の鍵は
// refs.js に聞く（IMP-260）。

import { canSubmit, canTakeFocus, closeBox, isEditBoxOpen, markSubmitting, openBox, refuseIfEditing } from "./blockedit.js";
import { ownRef } from "./refs.js";
import { syncHits } from "./search.js";
import { state } from "./state.js";
import { S } from "./strings.js";
import { $ } from "./util.js";

// セルの中のボタンと、開いている編集欄。ここでのダブルクリックでは編集を始めない（FR-142）
const IGNORE_DOUBLE_CLICK = ".sort-btn, .media-actions, .cell-editor-box";
const NEWLINES = /\r\n|\r|\n/g;

let deps = { api: null, notify() {}, focusViewer() {} };

// editor は開いている編集欄 { box, input, cell, ref }。閉じていれば null。
let editor = null;

// opening はダブルクリックのたびに進める。ソースの応答が、後のダブルクリックより遅れて届いたら開かない。
let opening = 0;

// initCellEdit はセルの編集欄を配線する（IMP-211, IMP-262）。given: { api, notify(error), focusViewer() }。
export function initCellEdit(given) {
  deps = { ...deps, ...given };

  $("markdown").addEventListener("dblclick", onDoubleClick);
  // ウィンドウへ戻ったとき、編集欄がまだ開いていてフォーカスがどこにも無ければ戻す（FR-142）
  window.addEventListener("focus", () => {
    if (editor && editor.box.isConnected && document.activeElement === document.body) editor.input.focus({ preventScroll: true });
  });
}

// markEditableCells は、鍵の合うセル（見出し行を含む）に is-editable を付ける・外す（IMP-262, DSP-126）。
// applyEditMode（IMP-260）から呼ぶ。**鍵の合わないセル（生 HTML の表、補われたセル）には付けない。**
export function markEditableCells(root, on) {
  for (const cell of root.querySelectorAll("th, td")) {
    cell.classList.toggle("is-editable", on && ownRef(cell, "cell") !== null);
  }
}

// onDoubleClick は、編集モードの間に鍵の合うセルでダブルクリックされたら編集欄を開く（IMP-262）。
async function onDoubleClick(event) {
  if (!state.editMode || !(event.target instanceof Element)) return;

  // ボタンの上と、開いている編集欄の中では何もしない。**選択範囲も消さない**（編集欄の中の語の選択を残す）
  if (event.target.closest(IGNORE_DOUBLE_CLICK)) return;

  const cell = event.target.closest("th, td");
  const ref = cell ? ownRef(cell, "cell") : null;
  if (ref === null) return;

  // 別の編集欄（セル・コードブロック）が開いていれば拒む（FR-140。v1.1.0 は取り消してから開いていた）
  if (refuseIfEditing()) return;

  // 既定のダブルクリックによる語の選択を消す
  getSelection().removeAllRanges();

  const token = ++opening;
  const result = await deps.api.getCellSource(ref).catch(() => null);

  // 応答を待つ間に、再描画でセルが消えた・編集モードが終わった・別のセルをダブルクリックした
  if (token !== opening || !cell.isConnected || !state.editMode) return;
  if (!result || result.stale) return;
  if (result.error) {
    // edit-conflict のときは Go 側が読み直しを送っている（IMP-195）
    deps.notify(result.error);
    return;
  }

  const current = buildEditor(cell, ref, result.text);
  // **openBox が true を返してから置く**（4.76.0）。待つ間に描画が来たら開かない
  const ok = await openBox(cell, { cancel: () => cancel(current), focus: () => current.input.focus({ preventScroll: true }), root: () => current.box });
  if (!ok || token !== opening || !state.editMode) {
    if (ok) closeBox();
    return;
  }

  editor = current;
  place(current, result.text.length);
}

// buildEditor は編集欄の器を作る（IMP-262, UI-055）。`.cell-editor-box` に入力欄と、`button.edit-cancel`・
// `button.edit-save` の順のボタンの行（見た目の並びと Tab の順をそろえる。DSP-172 と同じ）。
function buildEditor(cell, ref, text) {
  const box = document.createElement("div");
  box.className = "cell-editor-box";

  const input = document.createElement("input");
  input.type = "text";
  input.className = "cell-editor";
  input.value = text;
  input.addEventListener("keydown", onEditorKey);
  input.addEventListener("paste", onEditorPaste);

  const actions = document.createElement("div");
  actions.className = "edit-actions";
  const current = { box, input, cell, ref, saved: null, pending: null };
  actions.append(button("edit-cancel", S.cancel, () => cancel(current)), button("edit-save", S.save, () => commit(current)));

  box.append(input, actions);
  return current;
}

function button(className, label, onClick) {
  const element = document.createElement("button");
  element.type = "button";
  element.className = className;
  element.textContent = label;
  element.addEventListener("click", onClick);
  return element;
}

// place は編集欄をセルの中に置き、フォーカスとカーソルを置く（IMP-262, DSP-127）。
//
// **ボタンの行が表の左端より左へ出るなら、左端をセルの左端にそろえる**（is-align-left。表は横スクロールの器で、
// 左へはみ出した分は見えない。4.76.0）。**カーソルは明示して末尾に置く**——フォーカスの移動で全体を選ぶエンジンが
// ありうる（NFR-061）。
function place(current, caret) {
  const { box, input, cell } = current;
  cell.classList.add("is-cell-editing");
  box.classList.remove("is-align-left");
  cell.appendChild(box);

  const table = cell.closest("table");
  const actions = box.querySelector(".edit-actions");
  if (table && actions.getBoundingClientRect().left < table.getBoundingClientRect().left) box.classList.add("is-align-left");

  // 押したセルは見えている。フォーカスの移動で本文や表を動かさない（DSP-350）。**情報ウィンドウ・エディタ選択
  // ウィンドウ・拡大画面の表示中は動かさない**（失敗で開き直したとき。フォーカストラップを破らない。FR-140）
  if (canTakeFocus()) input.focus({ preventScroll: true });
  input.setSelectionRange(caret, caret);
}

// onEditorKey は Enter で確定する（IMP-262）。**Esc はここで扱わない**（IMP-244 の振り分けの (3)）。
// **IME の変換中は何もしない**（isComposing に加えて keyCode 229。NFR-061）。
function onEditorKey(event) {
  if (event.isComposing || event.keyCode === 229 || event.key !== "Enter" || !editor) return;

  // 検索の移動（IMP-244 の searchNext）へ届かせない
  event.preventDefault();
  event.stopPropagation();
  commit(editor);
}

// onEditorPaste は貼り付ける文字列の改行を半角空白へ置き換えて入れる（FR-142）。**既定の貼り付けに任せない**
// ——input type="text" は改行を黙って取り除き、語が連結される。右クリックメニューの Paste も同じ（IMP-249）。
function onEditorPaste(event) {
  event.preventDefault();

  const input = event.currentTarget;
  const text = event.clipboardData ? event.clipboardData.getData("text/plain") : "";
  input.setRangeText(text.replace(NEWLINES, " "), input.selectionStart, input.selectionEnd, "end");
}

// commit は確定する（IMP-262 の「確定」。Enter か Save）。
//
// **見た目を先に変える**（NFR-012, DSP-321）。セルの直接の子のうち並べ替えのボタン以外を退避し、入力された文字列を
// そのまま文字として出す。**応答を待つ間も編集欄は開いているとみなす**（blockedit.js が再描画を控える）。
async function commit(current) {
  if (editor !== current || current.pending) return;
  // 控えた外部の更新・削除があれば、警告を出して送らない（編集欄は開いたまま。IMP-264）
  if (!canSubmit()) return;

  const { box, input, cell, ref } = current;
  const value = input.value;
  box.remove();

  current.saved = document.createDocumentFragment();
  for (const node of [...cell.childNodes]) {
    if (!(node instanceof Element && node.classList.contains("sort-btn"))) current.saved.appendChild(node);
  }
  current.pending = document.createElement("span");
  current.pending.className = "cell-pending";
  // **innerHTML を使わない**（IMP-220）
  current.pending.textContent = value;
  cell.insertBefore(current.pending, cell.firstChild);
  // 退避した中身のヒットを落とし、置いた文字を検索の対象にする（IMP-241。4.76.0）
  syncHits(cell);

  // 確定した後は本文ペインへ戻す（UI-055）。**応答を待つ前に移す**
  deps.focusViewer();

  markSubmitting(true);
  // **呼び出しそのものの失敗は stale と同じに扱う**（固まらない。4.76.0）
  const result = (await deps.api.setCell(ref, value).catch(() => null)) || { changed: false, stale: true, error: null };
  markSubmitting(false);

  if (result.changed) {
    // 再描画を待つ（控えていれば、閉じた時点で当たる）
    finish(current);
    return;
  }

  restore(current);
  if (!result.stale && !result.error) {
    // 内容が変わらない確定
    finish(current);
    return;
  }

  // stale と error: **入力ごと開き直し、フォーカスを編集欄へ戻す**（4.74.0。FR-142）。openBox は呼び直さない
  // ——登録は開いたまま続いている
  if (result.error) deps.notify(result.error);
  input.value = value;
  place(current, value.length);
  // stale は通知しないが、控えた外部の更新・削除があればその警告を出す（IMP-262 の表、DSP-321。canSubmit が出す）
  if (!result.error) canSubmit();
}

// restore は確定で退避した子を戻す（DSP-321）。
// 検索を開いていれば、戻した中身を探し直す（中に残る古い <mark> は syncHits が解く。IMP-241。4.76.0）。
function restore(current) {
  if (current.pending && current.pending.isConnected) current.pending.replaceWith(current.saved);
  current.pending = null;
  current.saved = null;
  syncHits(current.cell);
}

// cancel は取り消す（IMP-262。Cancel か、フォーカスのある Esc）。Go は呼ばない。本文ペインへ戻す（UI-055）。
function cancel(current) {
  if (editor !== current || current.pending) return;

  current.box.remove();
  finish(current);
  deps.focusViewer();
}

// finish は編集欄を閉じる。**先に editor を空にしてから closeBox**——閉じると控えた再描画が当たり、本文が
// 差し替わる。
function finish(current) {
  if (editor !== current) return;

  editor = null;
  current.cell.classList.remove("is-cell-editing");
  if (isEditBoxOpen()) closeBox();
}
