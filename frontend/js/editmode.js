// editmode.js — 編集モード（FR-140〜FR-144, UI-055, IMP-260〜IMP-263, DSP-126, DSP-127, DSP-320, DSP-321）。
//
// **判断と書き込みは Go 側が持つ**（IMP-195）。ここにあるのは、指示を送ることと、見た目を先に変えて結果で
// 整えることだけである。**状態の正は Go 側にあり、画面は DocumentDTO と EditModeDTO の値を写す**（IMP-302）。
//
// **viewer.js / overlay.js / main.js / navigate.js / docswitch.js を import しない**（IMP-250。viewer.js と
// docswitch.js がこのモジュールを import する）。本文ペインへのフォーカスは main.js が deps で渡す。
// **目印の鍵を読まない**——照合と値は refs.js に聞く（IMP-260）。**400 行の目安を超えたら、セルの編集欄
// （IMP-262）を celledit.js へ分ける**（IMP-200）。
//
// **セルの文字列の整形（改行・前後の空白・`|` のエスケープ）はしない。** Go 側の PlanCell（IMP-106）が行う。
// 2 か所で整形すると、見た目と書き込まれる内容が食い違う。

import { isOwnRef, ownRef } from "./refs.js";
import { state } from "./state.js";
import { S } from "./strings.js";
import { $ } from "./util.js";

// セルの中のボタンと、開いている編集欄。ここでのダブルクリックでは編集を始めない（FR-142）
const IGNORE_DOUBLE_CLICK = ".sort-btn, .media-actions, .cell-editor";
const NEWLINES = /\r\n|\r|\n/g;

let deps = { api: null, notify() {}, focusViewer() {} };

// editor は開いているセルの編集欄 { input, cell, ref }。閉じていれば null。**常に 1 つだけ**（IMP-262）
let editor = null;

// opening はダブルクリックのたびに進める。ソースの応答が、後のダブルクリックより遅れて届いたら開かない
// （先に押したセルの編集欄が、後から押したセルの編集欄を置き換えないため）。
let opening = 0;

// initEditMode は編集モードを配線する（IMP-260）。given: { api, notify(error), focusViewer() }。
//   api は api.js（setEditMode / setTask / getCellSource / setCell / undoEdit / redoEdit）。
export function initEditMode(given) {
  deps = { ...deps, ...given };

  $("editmode-badge").textContent = S.editModeBadge;
  // チェックボックスの切り替えは #markdown の 1 つのリスナで受ける（イベント委譲。IMP-261）
  $("markdown").addEventListener("change", onCheckboxChange);
  // ラベルをスクロールバーの内側に置くため、ウィンドウ幅が変わったら測り直す（DSP-126）
  window.addEventListener("resize", measureScrollbar);

  // セルの編集欄はダブルクリックで開く（IMP-262）
  $("markdown").addEventListener("dblclick", onDoubleClick);
  // ウィンドウへ戻ったとき、編集欄がまだ開いていてフォーカスがどこにも無ければ戻す（FR-142）
  window.addEventListener("focus", () => {
    if (editor && document.activeElement === document.body) editor.input.focus({ preventScroll: true });
  });
}

// toggleEditMode はボタンと Ctrl+Shift+M の共通の入口（IMP-260, IMP-244）。何もしなければ false。
//
// **呼ぶ前に見た目を変えない**——開始できなかった場合に戻す処理が要り、得るものが無い。**同期で真偽を返す**
// （shortcuts.js は false でなければ既定の動作を止める）。結果は届いた時点で写す。
export function toggleEditMode() {
  if (!state.editable) return false;

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

// isCellEditing は編集欄が開いているかを返す（IMP-244 の Esc の振り分けの (3)）。
export function isCellEditing() {
  return editor !== null;
}

// cancelCellEdit は開いている編集欄を取り消す（IMP-262）。取り消したら true、開いていなければ false。
//
// **何も呼ばず、フォーカスも動かさない。** 本文ペインへ戻すのは呼び出し側が決める（Esc と編集モードの
// 終了は戻す。フォーカスが外れたことによる取り消しは戻さない。UI-055）。
export function cancelCellEdit() {
  if (!editor) return false;

  closeEditor();
  return true;
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
  if (on) measureScrollbar();

  const markdown = $("markdown");
  for (const input of markdown.querySelectorAll('input[type="checkbox"]')) {
    // 鍵の合わない input（生 HTML）には何もしない——disabled と pointer-events: none のまま（DSP-121）
    if (!isOwnRef(input, "task")) continue;
    input.disabled = !on;
    input.classList.toggle("is-editable", on);
  }
  markEditableCells(markdown, on);

  // 編集モードでなくなったら、開いているセルの編集欄を取り消す（FR-142）。**編集欄にフォーカスがあったら
  // 本文ペインへ移す**——編集欄は DOM から取り除かれ、フォーカスが body に落ちると PageUp / PageDown が
  // 効かない（BUG-007 と同じ症状）。フォーカスが無ければ動かさない（UI-055）。
  if (!on && isCellEditing()) {
    const focused = document.activeElement && document.activeElement.matches(".cell-editor");
    cancelCellEdit();
    if (focused) deps.focusViewer();
  }
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

  const checked = input.checked;
  const result = await deps.api.setTask(ref, checked).catch(() => null);
  if (result && result.changed) return;
  if (result && result.error) deps.notify(result.error);
  if (result && !result.stale && !result.error) return;

  // **戻すのは、その input がまだ画面にある場合だけ**——再描画が先に届いていれば input は差し替わっており、
  // 新しい DOM がファイルの状態を示している。戻り値とイベントのどちらが先でも同じ結果になる（IMP-316）
  if (input.isConnected) input.checked = !checked;
}

// --- セルの編集欄（IMP-262） ---

// markEditableCells は、鍵の合うセル（見出し行を含む）に is-editable を付ける・外す（IMP-262, DSP-126）。
//
// **鍵の合わないセル（生 HTML の表、GFM の規則で補われたセル）には付けない。** CSS は鍵を読めないため、
// 見分けられること（UI-055）をクラスで伝える。
function markEditableCells(root, on) {
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

  openEditor(cell, ref, result.text);
}

// openEditor は編集欄をセルの中に置き、フォーカスを移してカーソルを末尾へ置く（IMP-262, DSP-127）。
function openEditor(cell, ref, text) {
  cancelCellEdit();

  const input = document.createElement("input");
  input.type = "text";
  input.className = "cell-editor";
  input.value = text;
  input.addEventListener("keydown", onEditorKey);
  input.addEventListener("paste", onEditorPaste);
  input.addEventListener("blur", onEditorBlur);

  cell.classList.add("is-cell-editing");
  cell.appendChild(input);
  editor = { input, cell, ref };

  // 押したセルは見えている。フォーカスの移動で本文や表を動かさない（DSP-350）
  input.focus({ preventScroll: true });
  input.setSelectionRange(text.length, text.length);
}

// onEditorKey は Enter で確定する（IMP-262）。**Esc はここで扱わない**（IMP-244 の振り分けの (3)）。
//
// **IME の変換中は何もしない**（FR-142）。isComposing に加えて keyCode 229 も見る——変換を確定する Enter を
// isComposing が偽の keydown として届けるエンジンがある（NFR-061。実機で見る）。
function onEditorKey(event) {
  if (event.isComposing || event.keyCode === 229 || event.key !== "Enter") return;

  // 検索の移動（IMP-244 の searchNext）へ届かせない
  event.preventDefault();
  event.stopPropagation();
  commitCellEdit();
}

// onEditorPaste は貼り付ける文字列の改行を半角空白へ置き換えて入れる（FR-142）。
//
// **既定の貼り付けに任せない。** input type="text" は改行を黙って取り除き、語が連結される。
// 右クリックメニューの Paste も同じ置き換えを行う（IMP-249）。
function onEditorPaste(event) {
  event.preventDefault();

  const input = event.currentTarget;
  const text = event.clipboardData ? event.clipboardData.getData("text/plain") : "";
  input.setRangeText(text.replace(NEWLINES, " "), input.selectionStart, input.selectionEnd, "end");
}

// onEditorBlur はフォーカスが外れたら取り消す（FR-142）。**フォーカスは動かさない**（UI-055）。
//
// 次の 2 つは「外れた」に数えない。右クリックメニューを開いた（フォーカスがメニューへ移る）ときと、
// ウィンドウ自体がフォーカスを失ったときである。**数えると、Paste を選ぶ前に編集欄が消える。**
// メニューの外をクリックして閉じたときの取り消しは contextmenu.js が cancelCellEdit を呼ぶ（IMP-249）。
function onEditorBlur(event) {
  if (!editor || event.currentTarget !== editor.input) return;

  const next = event.relatedTarget;
  if (next instanceof Node && $("contextmenu").contains(next)) return;
  if (!document.hasFocus()) return;

  cancelCellEdit();
}

// commitCellEdit は確定する（IMP-262 の「確定」）。
//
// **見た目を先に変える**（NFR-012, DSP-321）。セルの直接の子のうち並べ替えのボタン以外を退避し、入力された
// 文字列をそのまま文字として出す（記法は描かない）。表示は document:changed の再描画で整う（IMP-316）。
async function commitCellEdit() {
  const { input, cell, ref } = editor;
  const value = input.value;
  closeEditor();

  const saved = document.createDocumentFragment();
  for (const node of [...cell.childNodes]) {
    if (!(node instanceof Element && node.classList.contains("sort-btn"))) saved.appendChild(node);
  }
  const pending = document.createElement("span");
  pending.className = "cell-pending";
  // **innerHTML を使わない**（IMP-220）
  pending.textContent = value;
  cell.insertBefore(pending, cell.firstChild);

  // Enter で確定した後は本文ペインへ戻す（UI-055）
  deps.focusViewer();

  const result = await deps.api.setCell(ref, value).catch(() => null);
  if (result && result.changed) return;
  if (result && result.error) deps.notify(result.error);

  // 変わらなかった・古い指示だった・書き込めなかった。**戻すのはセルがまだ画面にある場合だけ**——再描画が
  // 先に届いていれば、画面は既にファイルの状態を示している（DSP-321）
  if (pending.isConnected) pending.replaceWith(saved);
}

// closeEditor は編集欄を取り除く。**先に editor を空にする**——取り除いたときに blur を出すエンジンでも、
// onEditorBlur が取り消しをもう一度行わない。
function closeEditor() {
  const { input, cell } = editor;
  editor = null;
  cell.classList.remove("is-cell-editing");
  input.remove();
}
