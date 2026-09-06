// status.js — ステータス領域（UI-060, DSP-150, DSP-151）。
//
// 通知先はステータス領域・本文中・状態画面の 3 か所に限る。
// モーダルダイアログを使わない（FR-110, UI-052）。

import { S } from "./strings.js";
import { state } from "./state.js";
import { $, clear } from "./util.js";

// 一時メッセージの表示時間（DSP-151）。
const MESSAGE_MS = 5000;

let messageTimer = 0;

// updateStatus は表示中の文書からステータス領域を組み立てる（DSP-150）。
//
// 文言は textContent で入れる。Go を経由しない文字列を innerHTML へ渡さない
// （IMP-220）。
export function updateStatus() {
  const path = $("status-path");
  const meta = $("status-meta");

  clear(meta);

  if (!state.doc) {
    clear(path);
    return;
  }

  // ツリー外の文書は絶対パスの末尾に印を付ける（FR-052, DSP-150）。
  //
  // **印だけ淡い色にするため要素を分ける。** 1 つの textContent に
  // 混ぜると、パスと同じ --fg-muted になってしまう。
  clear(path);

  const text = document.createElement("span");
  text.className = "status-path-text";
  text.textContent = state.doc.displayPath;
  path.appendChild(text);

  if (state.doc.outsideTree) {
    const mark = document.createElement("span");
    mark.className = "status-outside";
    mark.textContent = ` ${S.outsideTree}`;
    path.appendChild(mark);
  }

  path.title = state.doc.path;

  // 倍率は 100 % 以外のときだけ出す（FR-081）。等倍が既定であり、常に
  // 表示すると変更されていることが目に留まらなくなる。
  const items = [state.doc.encoding, S.statusLines(state.doc.lineCount)];
  if (state.zoom !== 100) items.push(S.statusZoom(state.zoom));

  for (const text of items) {
    const item = document.createElement("span");
    item.className = "status-item";
    item.textContent = text;
    meta.appendChild(item);
  }
}

// showMessage は一時メッセージを出す（DSP-151, FR-110）。
//
// level は "info" | "warning" | "error"。**積み上げない。** 表示中に新しい
// メッセージが来たら置き換えてタイマを引き直す。
export function showMessage(text, level) {
  if (!text) return;

  const element = $("status-message");
  element.textContent = text;
  // 溢れる場合は末尾を省略し、全文はツールチップで示す（DSP-151）。
  element.title = text;
  element.dataset.level = level || "info";
  element.hidden = false;

  clearTimeout(messageTimer);
  messageTimer = setTimeout(() => {
    element.hidden = true;
  }, MESSAGE_MS);
}
