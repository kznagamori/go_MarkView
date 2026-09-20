// menuactions.js — 右クリックメニューの項目の実行（FR-063, IMP-249, AR-062）。
//
// **場所の判定・メニューの組み立て・開閉は `contextmenu.js` が持つ。** ここが持つのは
// 「押されたときに何をするか」だけである（IMP-011 の 400 行の目安で分けた）。
//
// **クリップボードへの格納は `deps`（`api.js`）を通す**——このモジュールは Go を直接呼ばない。

import { $ } from "./util.js";

const CELL_EDITOR = "input.cell-editor";

const ACTIONS = {
  copyLink: (current, deps) => store(current.link, deps),
  copy: (current, deps) => (current.kind === "input" ? inputCommand(current, "copy", deps) : copySelection(current, deps)),
  cut: (current, deps) => inputCommand(current, "cut", deps),
  paste: (current) => paste(current),
  selectAll: (current) => selectAll(current),
};

// hasAction は実行できる項目かを返す。
export function hasAction(action) {
  return Object.hasOwn(ACTIONS, action);
}

// runAction は項目を実行する（contextmenu.js の run から呼ぶ）。
export function runAction(action, current, deps) {
  return ACTIONS[action](current, deps);
}

// restoreRange は控えた選択範囲を当て直す（BUG-019）。控えが無ければ何もしない。
//
// **フォーカスを移すたびに呼ぶ。** WebKit は要素へフォーカスを移すと文書の選択範囲を解除するため、
// メニューを開いたとき（項目へ移す）と、実行の直前（closeContextMenu が開く前の要素へ戻す）の
// 2 か所で消えうる。**再描画などで控えた位置が DOM から外れていたら当て直さない。**
export function restoreRange(current) {
  const range = current && current.range;
  if (!range || !range.startContainer.isConnected || !range.endContainer.isConnected) return;

  const selection = getSelection();
  if (!selection) return;

  // **当て直せないことは失敗ではない**（当て直さないだけ）。要素が残っていても、文字が短くなっていれば
  // addRange は例外を投げる（本文の書き換え・検索の包み直し）。open() の途中で投げると、メニューを
  // 出したまま残りの処理が飛ぶ。
  try {
    selection.removeAllRanges();
    selection.addRange(range);
  } catch {
    // 何もしない
  }
}

// copySelection は本文・ライセンス欄の選択範囲をコピーする。
//
// **execCommand('copy') を使う**——WebView 自身のコピー処理であり、Ctrl+C と同じ内容（書式付き）が入る
// （FR-063, AR-062）。**Go 側の CopyToClipboard で実装しない**（プレーンテキストしか扱わない）。
// false が返ったときだけ、書式を失っても文字列を格納する（本来の経路ではない。IMP-249）。
function copySelection(current, deps) {
  // **実行の直前にもう一度当て直す**（BUG-019）。run() は先に closeContextMenu() で開く前の
  // フォーカス要素へ戻すため、WebKit ではそこでもう一度選択が解除される。
  restoreRange(current);

  // **選択が無ければ execCommand も呼ばない**（BUG-019）。当て直せなかったとき（控えた位置が
  // 消えていた）にここへ来る。**空のままコピーを実行すると、エンジンによっては成功を返し、
  // 利用者のクリップボードの中身が消える。**
  const text = String(getSelection());
  if (text === "") return;

  if (command("copy")) return;

  // 控えの経路。書式は失うが、文字列だけでも格納する（IMP-249）
  return store(text, deps);
}

// inputCommand は入力欄の選択範囲をコピー・切り取りする。控えた選択範囲を当て直してから実行する。
async function inputCommand(current, name, deps) {
  const input = current.place;
  input.focus({ preventScroll: true });
  input.setSelectionRange(current.start, current.end);
  if (command(name)) return;

  // execCommand が効かなかったときは Go 側で文字列だけを格納する（copySelection と同じ扱い）
  const stored = await store(input.value.slice(current.start, current.end), deps);
  if (stored && name === "cut") replaceRange(input, "", current.start, current.end);
}

// paste は控えた文字列を、入力欄の控えた範囲へ入れる。**書式は持ち込まない**（FR-063）。
// **セルの編集欄では改行を半角空白へ置き換える**（FR-142。編集欄の paste と同じ置き換え。IMP-262）。
function paste(current) {
  const input = current.place;
  const text = input.matches(CELL_EDITOR) ? current.pasteText.replace(/\r\n|\r|\n/g, " ") : current.pasteText;
  input.focus({ preventScroll: true });
  replaceRange(input, text, current.start, current.end);
}

// replaceRange は入力欄の範囲を置き換え、input を発火する（検索はインクリメンタルなため。IMP-241）。
function replaceRange(input, text, start, end) {
  input.setRangeText(text, start, end, "end");
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

// selectAll はその場所の全体を選ぶ（FR-063）。本文ペインは、文書を表示していれば #markdown、状態画面なら
// #state-screen とする（検索バーや本文中のボタンを含めない）。
function selectAll(current) {
  if (current.kind === "input") {
    current.place.focus({ preventScroll: true });
    current.place.select();
    return;
  }

  let target = current.place;
  if (current.kind === "viewer") target = $("state-screen").hidden ? $("markdown") : $("state-screen");
  getSelection().selectAllChildren(target);
}

// store は Go 側のクリップボードへ文字列を格納する。格納できたら true。
async function store(text, deps) {
  let error;
  try {
    error = await deps.copyText(text);
  } catch {
    error = { kind: "clipboard" };
  }
  if (error) deps.notify(error);
  return !error;
}

// command は execCommand を呼ぶ。例外は false として扱う。
function command(name) {
  try {
    return document.execCommand(name);
  } catch {
    return false;
  }
}
