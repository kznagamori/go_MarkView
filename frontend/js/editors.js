// editors.js — エディタ選択ダイアログの中身（UI-103, IMP-252, DSP-172）。
//
// **開閉・フォーカスの復帰・Tab の制御は overlay.js が持つ**（IMP-251,
// IMP-252）。ここは #overlay に入れる DOM を組み立て、選択の状態だけを扱う。
// overlay.js を import しないのは、循環参照を作らないためである。
//
// **Go を呼ばない**（IMP-201）。Browse と Open の処理は handlers として
// 受け取り、overlay.js を経由して main.js が持つ。

import { S } from "./strings.js";
import { $, icon, span } from "./util.js";

// 一覧の末尾に置かれる「Other...」の識別子（IMP-309 の予約語）。
const EDITOR_CUSTOM = "custom";

// buildEditorDialog はダイアログ本体を組み立てる（DSP-172）。
//
// list は EditorListDTO（IMP-309）。handlers は { onCancel, onBrowse, onLaunch }。
// 枠は情報ダイアログと同じものを使い、寸法だけを変える（DSP-172）。
export function buildEditorDialog(list, handlers) {
  const dialog = document.createElement("div");
  dialog.className = "dialog dialog-narrow";
  dialog.setAttribute("role", "dialog");
  dialog.setAttribute("aria-modal", "true");
  dialog.setAttribute("aria-labelledby", "editor-title");

  dialog.appendChild(closeButton(handlers));

  const title = document.createElement("h2");
  title.id = "editor-title";
  title.className = "editor-title";
  title.textContent = S.editorTitle;
  dialog.appendChild(title);

  dialog.appendChild(buildList((list || {}).editors || [], handlers));
  dialog.appendChild(buildActions(handlers));

  return dialog;
}

// syncOpenButton は Open の活性を選択から決める（IMP-252, UI-103）。
//
// **判定はこの 1 つの式だけとする。** 行の種類ごとに条件を書くと Other... が
// 別扱いになり、いつか食い違う。押せるのは「選ばれていて、かつ起動できる
// 実体があるもの」で、実体の有無は Go 側が available で示している（IMP-309）。
// **フロントエンドはパスを見ない**（NFR-035 の 3）。
export function syncOpenButton() {
  const checked = checkedRadio();

  $("editor-open").disabled = !checked || !checked.dataset.usable;
}

// focusInitial は開いた直後のフォーカスを置く（UI-103）。
//
// 初期選択があれば Open に置く。同じエディタを使い続ける利用者が
// **ボタンと Enter の 2 操作で開ける**ようにするためである。無ければ一覧の
// 先頭の選択できる行に置く。
//
// 一覧が空でも**フォーカスを失わない**よう、最後は閉じるボタンへ落とす。
export function focusInitial() {
  focusOpenOr(firstSelectable() || $("editor-x"));
}

// focusAfterBrowse は Browse で描き直した直後のフォーカスを置く。
//
// 選べたら Open へ、選ばれなければ押した Browse 自身へ戻す。
export function focusAfterBrowse() {
  focusOpenOr($("editor-browse"));
}

// checkedEditorId は選ばれている行の識別子を返す。無ければ空文字。
//
// **返すのは識別子だけである**（IMP-309, IMP-300 の 3）。実行ファイルのパスは
// Go 側で生まれて Go 側で消費され、画面には出てこない（NFR-035 の 3）。
export function checkedEditorId() {
  const checked = checkedRadio();

  return checked ? checked.value : "";
}

// buildList は一覧を組み立てる（UI-103, IMP-252）。
//
// **順序は Go 側が決めたものをそのまま使う**（IMP-309）。検出できたものを
// 前へ並べ替えない。並びが環境や起動のたびに変わると、位置で覚えられなくなる。
function buildList(editors, handlers) {
  const box = document.createElement("div");
  box.className = "editor-list";

  for (const editor of editors) {
    box.appendChild(editor.id === EDITOR_CUSTOM ? customRow(editor, handlers) : presetRow(editor));
  }

  return box;
}

// presetRow はプリセット 1 件の行を作る（UI-103, DSP-172）。
//
// **見つからなかったものも行を消さない。** 消すと「なぜ自分のエディタが
// 出ないのか」が分からなくなる。淡色にして (not installed) を添える。
function presetRow(editor) {
  const row = document.createElement("label");
  row.className = editor.available ? "editor-row" : "editor-row is-disabled";

  row.appendChild(radio(editor, editor.available));

  // 名前は Go 側のプリセット表（IMP-172）から届く固有名詞。textContent で
  // 入れる（IMP-220）。
  row.appendChild(span("editor-name", editor.name));

  if (!editor.available) row.appendChild(span("editor-note", S.editorMissing));

  return row;
}

// customRow は「Other...」の行を作る（UI-103, DSP-172）。
//
// **常に選択できる**（UI-103）。実行ファイルを選ぶ前でもラジオを押せなければ、
// Browse へ辿り着く手段が無くなる。起動できるかどうかは Open の活性で表す。
function customRow(editor, handlers) {
  const box = document.createElement("div");
  box.className = "editor-custom";

  const row = document.createElement("label");
  row.className = "editor-row";
  row.appendChild(radio(editor, true));
  row.appendChild(span("editor-name", S.editorOther));
  box.appendChild(row);

  // 選んだ結果は行の下に 1 段（DSP-172）。**実行ファイル名だけを出す。**
  // パスは画面へ出さない（NFR-035 の 3）。Go 側が name にベース名だけを
  // 入れており（IMP-309）、ここで組み立て直さない。
  const detail = document.createElement("div");
  detail.className = "editor-detail";
  detail.appendChild(span("editor-file", editor.available ? editor.name : S.editorNone));
  detail.appendChild(browseButton(handlers));
  box.appendChild(detail);

  return box;
}

// radio は行のラジオを作る（IMP-252）。
//
// selectable は「その行を選べるか」であり、**available（起動できる実体が
// あるか）とは別である。** Other... は実行ファイルを選ぶ前でも選べる
// （UI-103）が、そのままでは起動できない。後者は data-usable として
// Open の活性へ渡す。
function radio(editor, selectable) {
  const input = document.createElement("input");
  input.type = "radio";
  input.name = "editor";
  input.className = "editor-radio";
  input.value = editor.id;
  input.checked = Boolean(editor.selected);
  input.disabled = !selectable;

  if (editor.available) input.dataset.usable = "yes";

  input.addEventListener("change", syncOpenButton);

  return input;
}

// closeButton は右上の [x] を作る（DSP-172）。**Cancel と同じ処理へ繋ぐ。**
// 閉じ方が 2 通りあると、片方だけ保存や起動をしてしまう余地が生まれる
// （FR-091）。
function closeButton(handlers) {
  const button = document.createElement("button");
  button.id = "editor-x";
  button.type = "button";
  button.className = "dialog-x";
  button.title = S.close;
  button.setAttribute("aria-label", S.close);
  button.appendChild(icon("icon-close"));
  button.addEventListener("click", handlers.onCancel);

  return button;
}

function browseButton(handlers) {
  return actionButton("editor-browse", "dialog-button", S.editorBrowse, handlers.onBrowse);
}

function buildActions(handlers) {
  const actions = document.createElement("div");
  actions.className = "dialog-actions editor-actions";

  actions.appendChild(actionButton("editor-cancel", "dialog-button", S.cancel, handlers.onCancel));

  // **主スタイルは 1 つのダイアログにつき高々 1 つ**（DSP-182）。
  const open = actionButton("editor-open", "dialog-button-primary", S.editorOpen, handlers.onLaunch);
  open.disabled = true; // 実際の活性は syncOpenButton が決める
  actions.appendChild(open);

  return actions;
}

function actionButton(id, className, label, handler) {
  const button = document.createElement("button");
  button.id = id;
  button.type = "button";
  button.className = className;
  button.textContent = label;
  button.addEventListener("click", handler);

  return button;
}

// focusOpenOr は Open が押せるならそこへ、押せなければ fallback へ置く。
//
// **「初期選択があるか」は Open の活性と同じ条件である**（syncOpenButton）。
// 別に判定を書かない。
function focusOpenOr(fallback) {
  const open = $("editor-open");
  const target = open.disabled ? fallback : open;

  if (target) target.focus();
}

function firstSelectable() {
  return $("overlay").querySelector(".editor-radio:not(:disabled)");
}

function checkedRadio() {
  return $("overlay").querySelector(".editor-radio:checked");
}
