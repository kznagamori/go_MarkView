// codeedit.js — コードブロックと図の編集欄（FR-145, UI-053, UI-055, IMP-264, DSP-124, DSP-128, DSP-321）。
//
// 編集モードの間、鍵の合うコードブロックと図（Mermaid・PlantUML。図のファイルを含む）に編集のボタンを置き、
// ボタンかダブルクリックで、ブロックの中に複数行の編集欄を開く。**開いている間の扱い（拒む・保留・確定の待ち・
// 開き直し）は blockedit.js が持つ**（IMP-264）。
//
// **文字列の整形（改行コード・前置き・囲いの延長）はしない**——Go 側の PlanCode（IMP-122）が行う。**目印の鍵を
// 読まない**——照合と値は refs.js に聞く（IMP-260）。**viewer.js / overlay.js / main.js / navigate.js / docswitch.js /
// editmode.js を import しない。**

import { canSubmit, canTakeFocus, closeBox, isEditBoxOpen, markSubmitting, openBox, refuseIfEditing } from "./blockedit.js";
import { redrawDiagrams } from "./lazy.js";
import { diagramKeyOf } from "./mediabuttons.js";
import { isOwnRef, ownRef } from "./refs.js";
import { syncHits } from "./search.js";
import { state } from "./state.js";
import { S } from "./strings.js";
import { $, icon } from "./util.js";

// 編集できるブロックの種類（IMP-120 の目印）
const KINDS = ["code", "mermaid", "plantuml"];

// 右上のボタンと、開いている編集欄。ここでのダブルクリックでは編集を始めない（FR-145）
const IGNORE_DOUBLE_CLICK = ".copy-btn, .media-actions, .code-editor-box";

// 編集欄の高さの上限は本文ペインの高さの 60 %（DSP-128。利用者の判断）
const MAX_RATIO = 0.6;

let deps = { api: null, notify() {}, focusViewer() {} };

// editor は開いている編集欄 { block, ref, box, textarea, hiddenNodes, theme, pending }。閉じていれば null。
let editor = null;

// opening はダブルクリックとボタンのたびに進める。ソースの応答が、後の操作より遅れて届いたら開かない。
let opening = 0;

// initCodeEdit はコードブロックと図の編集欄を配線する（IMP-211, IMP-264）。given: { api, notify(error), focusViewer() }。
export function initCodeEdit(given) {
  deps = { ...deps, ...given };

  $("markdown").addEventListener("dblclick", onDoubleClick);
  // ウィンドウへ戻ったとき、編集欄がまだ開いていてフォーカスがどこにも無ければ戻す（IMP-262 と同じ）
  window.addEventListener("focus", () => {
    if (editor && editor.box.isConnected && document.activeElement === document.body) editor.textarea.focus({ preventScroll: true });
  });
  // 本文ペインの大きさが変わったら、高さの上限を測り直す（DSP-128）
  window.addEventListener("resize", measureMax);
  if (typeof ResizeObserver === "function") new ResizeObserver(measureMax).observe($("viewer"));
}

// markEditableBlocks は、編集モードの間に編集のボタンを付け、終えたら外す（IMP-264。applyEditMode から呼ぶ）。
//
// 図では、原寸表示・拡大画面のボタンの器（IMP-228。同じ鍵）の先頭に置く。**器が無ければ作る**——図の描画は非同期に
// 終わり、この時点で器が無いことがある。描けなかった図と描かなかった PlantUML も編集できる（4.76.0）。コードブロック
// では器を作って編集のボタンだけを入れる。**前置きが行ごとに違うブロック（data-locked）では押せない**（FR-145）。
export function markEditableBlocks(root, on) {
  for (const block of root.querySelectorAll(".code-block")) {
    const existing = block.querySelector(":scope > .media-actions > .media-btn-edit");
    if (!on || !kindOf(block)) {
      if (existing) removeEditButton(existing);
      continue;
    }
    if (existing) continue;

    const actions = actionsFor(block);
    actions.insertBefore(editButton(block), actions.firstChild);
  }
}

// kindOf は鍵の合う編集できるブロックの種類を返す。合わなければ null（IMP-260）。
function kindOf(block) {
  return KINDS.find((kind) => isOwnRef(block, kind)) || null;
}

// actionsFor はブロックのボタンの器を返す。無ければ作り、コピーボタンの前（左）に置く（DSP-124）。
function actionsFor(block) {
  const existing = block.querySelector(":scope > .media-actions");
  if (existing) return existing;

  const actions = document.createElement("div");
  actions.className = "media-actions";
  // 図は IMP-228 と同じ鍵。後から描き終えた図は、同じ器に原寸表示と拡大画面のボタンを足す
  const key = diagramKeyOf(block);
  if (key) actions.dataset.mediaKey = key;
  block.insertBefore(actions, block.querySelector(":scope > .copy-btn"));
  return actions;
}

function editButton(block) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "media-btn media-btn-edit";
  button.dataset.tip = S.tipEditBlock;
  button.setAttribute("aria-label", S.tipEditBlock);
  button.appendChild(icon("icon-pencil"));
  button.disabled = block.getAttribute("data-locked") === "1";
  // **押下を本文へ伝えない**（図がリンクに囲まれていても遷移させない。UI-053, IMP-223）
  button.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
    startEdit(block);
  });
  return button;
}

// removeEditButton は編集のボタンを外す。図の器（原寸表示・拡大画面）は残し、空になった器は外す。
function removeEditButton(button) {
  const actions = button.parentElement;
  button.remove();
  if (actions && actions.children.length === 0) actions.remove();
}

// onDoubleClick は、編集モードの間に鍵の合うブロックをダブルクリックしたら編集欄を開く（IMP-264）。
function onDoubleClick(event) {
  if (!state.editMode || !(event.target instanceof Element)) return;
  if (event.target.closest(IGNORE_DOUBLE_CLICK)) return;

  const block = event.target.closest(".code-block");
  if (!block || !kindOf(block)) return;

  // 既定のダブルクリックによる語の選択を消す
  getSelection().removeAllRanges();
  // **前置きが行ごとに違うブロックは、code-locked の警告を出して開かない**（FR-145, IMP-315）
  if (block.getAttribute("data-locked") === "1") {
    if (!refuseIfEditing()) deps.notify({ kind: "code-locked" });
    return;
  }
  startEdit(block);
}

// startEdit はソースを取り、編集欄を開く（IMP-264）。
async function startEdit(block) {
  // 別の編集欄（セル・コードブロック）が開いていれば拒む（FR-140）
  if (refuseIfEditing()) return;

  const kind = kindOf(block);
  const ref = kind ? ownRef(block, kind) : null;
  if (ref === null) return;

  const token = ++opening;
  const result = await deps.api.getCodeSource(ref).catch(() => null);

  // 応答を待つ間に、再描画でブロックが消えた・編集モードが終わった・別のブロックを押した
  if (token !== opening || !block.isConnected || !state.editMode) return;
  if (!result || result.stale) return;
  if (result.error) {
    // code-locked は警告、edit-conflict は Go 側が読み直しを送っている（IMP-315, IMP-195）
    deps.notify(result.error);
    return;
  }

  const current = buildEditor(block, ref, result.text);
  // **openBox が true を返してから置く**（4.76.0）
  const ok = await openBox(block, { cancel: () => cancel(current), focus: () => current.textarea.focus({ preventScroll: true }), root: () => current.box });
  if (!ok || token !== opening || !state.editMode) {
    if (ok) closeBox();
    return;
  }

  editor = current;
  place(current, 0);
}

// buildEditor は編集欄の器を作る（IMP-264, DSP-128）。`.code-editor-box` に textarea と、`button.edit-cancel`・
// `button.edit-save` の順のボタンの行。
function buildEditor(block, ref, text) {
  const box = document.createElement("div");
  box.className = "code-editor-box";

  const textarea = document.createElement("textarea");
  textarea.className = "code-editor";
  textarea.spellcheck = false;
  textarea.value = text;
  textarea.addEventListener("keydown", onEditorKey);
  textarea.addEventListener("input", () => fitHeight(textarea));

  const actions = document.createElement("div");
  actions.className = "edit-actions";
  const current = { block, ref, box, textarea, hiddenNodes: [], theme: state.theme, pending: null };
  actions.append(button("edit-cancel", S.cancel, () => cancel(current)), button("edit-save", S.save, () => commit(current)));

  box.append(textarea, actions);
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

// place は元の中身（pre や描いた図）を隠して残し、編集欄を置く（IMP-264, DSP-128）。右上のボタンは
// .is-code-editing で隠す（CSS）。カーソルは先頭。
function place(current, caret) {
  const { block, box, textarea } = current;
  current.hiddenNodes = [...block.children].filter((child) => !child.matches(".copy-btn, .media-actions, .code-editor-box") && !child.hidden);
  for (const child of current.hiddenNodes) child.hidden = true;

  block.classList.add("is-code-editing");
  block.appendChild(box);
  // 検索の走査から外れた元の中身のヒットを落とす（IMP-241。4.76.0）
  syncHits(block);
  measureMax();
  fitHeight(textarea);

  textarea.focus({ preventScroll: true });
  textarea.setSelectionRange(caret, caret);
}

// measureMax は高さの上限（本文ペインの 60 %）を --code-editor-max に入れる（DSP-128）。
function measureMax() {
  if (editor) editor.box.style.setProperty("--code-editor-max", `${Math.round($("viewer").clientHeight * MAX_RATIO)}px`);
}

// fitHeight は高さを内容に合わせる（--code-editor-height。最小 3 行と上限は CSS）。**利用者が手で高さを変えたら
// （resize が style.height を付ける）、以後は入れ直さない**（DSP-128）。
function fitHeight(textarea) {
  if (textarea.style.height) return;
  textarea.style.setProperty("--code-editor-height", "auto");
  textarea.style.setProperty("--code-editor-height", `${textarea.scrollHeight + 4}px`);
}

// onEditorKey は Ctrl+Enter で確定し、Tab / Shift+Tab で字下げを入れる・外す（IMP-264）。**Esc はここで扱わない**
// （IMP-244 の (3)）。**IME の変換中は何もしない**。Enter は改行（既定の動作）で、検索の移動へ回さない。
function onEditorKey(event) {
  if (event.isComposing || event.keyCode === 229 || !editor) return;

  if (event.key === "Enter") {
    event.stopPropagation();
    if (event.ctrlKey) {
      event.preventDefault();
      commit(editor);
    }
    return;
  }
  if (event.key === "Tab" && !event.ctrlKey && !event.altKey) {
    // **Tab で編集欄の外へ出る手段は持たない**（確定か取り消しで閉じる。FR-145）
    event.preventDefault();
    event.stopPropagation();
    indent(event.currentTarget, event.shiftKey);
  }
}

// indent はタブ文字の字下げを入れる・外す（IMP-264）。選択が無ければカーソル位置にタブ文字（Shift+Tab は行の先頭の
// タブ文字を 1 つ除く）。選択が複数行にかかれば、その各行の先頭に足す・除く。**setRangeText で変え**（textarea の
// 中の取り消しが効く形）、input を発火する。
function indent(textarea, outdent) {
  const { value, selectionStart: start, selectionEnd: end } = textarea;
  // 位置 0 では lastIndexOf の起点が -1（0 として扱われる）になり、先頭が改行だと 1 を返すため、0 を明示する
  const lineStart = start === 0 ? 0 : value.lastIndexOf("\n", start - 1) + 1;
  const multiLine = value.slice(start, end).includes("\n");

  if (!outdent && !multiLine) {
    textarea.setRangeText("\t", start, end, "end");
  } else {
    // 1 行の中の Shift+Tab は行の終わりまでを対象にする（カーソルが行頭にあっても、その行の先頭のタブ文字を除く）
    const lineEnd = value.indexOf("\n", end) < 0 ? value.length : value.indexOf("\n", end);
    const blockEnd = !multiLine ? lineEnd : end > start && value[end - 1] === "\n" ? end - 1 : end;
    const lines = value.slice(lineStart, blockEnd).split("\n");
    const changed = lines.map((line) => (outdent ? (line.startsWith("\t") ? line.slice(1) : line) : `\t${line}`)).join("\n");
    textarea.setRangeText(changed, lineStart, blockEnd, "select");
    if (!multiLine) {
      // 選択が 1 行の中の Shift+Tab は、カーソルの位置を保つ（除いた分だけ前へ）
      const shift = changed.length - (blockEnd - lineStart);
      textarea.setSelectionRange(Math.max(lineStart, start + shift), Math.max(lineStart, end + shift));
    }
  }
  textarea.dispatchEvent(new Event("input", { bubbles: true }));
}

// commit は確定する（IMP-264。Ctrl+Enter か Save）。
//
// **見た目を先に変える**（NFR-012, DSP-321）。元の中身の代わりに pre.code-pending を置き、入力を textContent で
// そのまま出す（ハイライトと図は再描画で戻る）。**応答を待つ間も編集欄は開いているとみなす**（blockedit.js）。
async function commit(current) {
  if (editor !== current || current.pending) return;
  if (!canSubmit()) return;

  const { block, box, textarea, ref } = current;
  const text = textarea.value;
  box.remove();

  current.pending = document.createElement("pre");
  current.pending.className = "code-pending";
  // **innerHTML を使わない**（IMP-220）
  current.pending.textContent = text;
  block.insertBefore(current.pending, block.querySelector(":scope > .copy-btn, :scope > .media-actions"));
  // 置いた中身を検索の対象にする（IMP-241。4.76.0）
  syncHits(block);

  deps.focusViewer();

  markSubmitting(true);
  // **呼び出しそのものの失敗は stale と同じに扱う**（固まらない。4.76.0）
  const result = (await deps.api.setCode(ref, text).catch(() => null)) || { changed: false, stale: true, error: null };
  markSubmitting(false);

  if (result.changed) {
    finish(current);
    return;
  }

  removePending(current);
  if (!result.stale && !result.error) {
    showOriginal(current);
    finish(current);
    redrawIfThemeChanged(current);
    return;
  }

  // stale と error: 元の中身へ戻し、**入力ごと開き直して、フォーカスを編集欄へ戻す**（FR-145。4.74.0）
  if (result.error) deps.notify(result.error);
  textarea.value = text;
  block.appendChild(box);
  // 外した pre.code-pending のヒットを落とす（IMP-241）
  syncHits(block);
  // ダイアログ・拡大画面の表示中はフォーカスを動かさない（FR-140）。stale は通知しないが、控えた外部の更新・削除が
  // あればその警告を出す（DSP-321。canSubmit が出す）
  if (canTakeFocus()) textarea.focus({ preventScroll: true });
  if (!result.error) canSubmit();
}

function removePending(current) {
  if (current.pending) current.pending.remove();
  current.pending = null;
}

// showOriginal は隠していた元の中身を戻す（編集欄を開いた後に描き終えた図を含む）。
// 検索を開いていれば、戻した中身を探し直す（IMP-241。4.76.0。中に残る古い <mark> は syncHits が解く）。
function showOriginal(current) {
  for (const child of current.hiddenNodes) if (child.isConnected) child.hidden = false;
  // 編集欄を開いた後に描き終えた図・描けずに戻した原文と理由の行（lazy.js / puml.js が hidden を引き継ぐ）も見せる
  for (const child of current.block.querySelectorAll(":scope > .mermaid-rendered, :scope > .plantuml-rendered, :scope > .plantuml-source, :scope > .plantuml-error, :scope > .mermaid-error")) child.hidden = false;
  current.hiddenNodes = [];
  syncHits(current.block);
}

// redrawIfThemeChanged は、編集欄を開いている間にテーマが変わっていれば、図をいまのテーマで描き直す（テーマの描き
// 直しは編集中のブロックを飛ばしている。IMP-264, DSP-370）。**is-code-editing を外した後に呼ぶ**——付いたままだと
// 描き直しがまた飛ばす。
function redrawIfThemeChanged(current) {
  if (current.theme !== state.theme && kindOf(current.block) !== "code" && current.block.isConnected) redrawDiagrams($("markdown"));
}

// cancel は取り消す（IMP-264。Cancel か、フォーカスのある Esc）。Go は呼ばない。本文ペインへ戻す。
function cancel(current) {
  if (editor !== current || current.pending) return;

  current.box.remove();
  showOriginal(current);
  finish(current);
  redrawIfThemeChanged(current);
  deps.focusViewer();
}

// finish は編集欄を閉じる。**先に editor を空にしてから closeBox**——閉じると控えた再描画が当たり、本文が差し替わる。
function finish(current) {
  if (editor !== current) return;

  editor = null;
  current.block.classList.remove("is-code-editing");
  if (isEditBoxOpen()) closeBox();
}
