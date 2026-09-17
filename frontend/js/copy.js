// copy.js — コードブロックのコピーボタン（FR-060, FR-061, IMP-221, DSP-251, DSP-252）。
//
// **クリップボードへの書き込みは Go 側の API を経由する**（AR-062）。
// navigator.clipboard は権限や実行文脈によって失敗しうるため、既定経路にしない。

import * as api from "./api.js";
import { ownSource } from "./refs.js";
import { S, errorText } from "./strings.js";
import { showMessage } from "./status.js";
import { icon } from "./util.js";

// 成功表示を戻すまでの時間（DSP-252）。
const RESET_MS = 1500;

// ボタンごとの復帰タイマ。要素に生えたプロパティで持たない。
const timers = new WeakMap();

// attachCopyButtons は各コードブロックにボタンを付ける（IMP-221）。
//
// ボタンは常に置く。**`display: none` にして隠さない**（FR-060）。
export function attachCopyButtons(root) {
  for (const block of root.querySelectorAll(".code-block")) {
    block.appendChild(createButton(block));
  }
}

function createButton(block) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "copy-btn";
  button.setAttribute("aria-label", S.copy);
  button.title = S.copy;
  button.appendChild(icon("icon-copy"));

  button.addEventListener("click", () => copy(block, button));

  return button;
}

async function copy(block, button) {
  const error = await api.copyToClipboard(sourceOf(block));

  // 失敗してもアイコンは変えず、ステータス領域に出す（DSP-252, FR-110）。
  if (error) {
    showMessage(errorText(error), "error");
    return;
  }

  succeed(button);
}

// sourceOf はコピー対象の原文を取り出す（IMP-221）。
//
// **鍵の合う図のブロックでは、Go 側が持たせた原文（ownSource）を先に見る。** Mermaid は描画後に
// <pre> が SVG へ置き換わり、原文が DOM から失われるため（IMP-115, IMP-119）。
//
// **data-source を直接読まない。** 生 HTML で
// `<div class="code-block" data-source="curl … | sh"><pre><code>npm install</code></pre></div>` と
// 書くと、画面に見えているコードと違う文字列をコピーさせられる（NFR-030, BUG-014。v1.0.0 はこの
// 形だった）。鍵の合わないブロックでは、見えている pre code の文字をコピーする。
function sourceOf(block) {
  const source = ownSource(block);
  if (source !== null) {
    return trimTrailingNewline(source);
  }

  const code = block.querySelector("pre code");

  return trimTrailingNewline(code ? code.textContent : "");
}

// trimTrailingNewline は末尾の改行を 1 つだけ落とす（FR-061）。
//
// 改行は Go 側で LF に正規化済みである（IMP-103）。
function trimTrailingNewline(text) {
  return text.endsWith("\n") ? text.slice(0, -1) : text;
}

// succeed はアイコンを差し替え、1.5 秒後に戻す（DSP-252）。
function succeed(button) {
  button.querySelector("use").setAttribute("href", "#icon-check");
  button.classList.add("copied");

  clearTimeout(timers.get(button));
  timers.set(
    button,
    setTimeout(() => {
      button.querySelector("use").setAttribute("href", "#icon-copy");
      button.classList.remove("copied");
      timers.delete(button);
    }, RESET_MS),
  );
}
