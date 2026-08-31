// lazy.js — Mermaid と KaTeX の遅延ロード（NFR-013, AR-021, IMP-230〜232）。
//
// 読み込みは frontend/vendor/ への相対パスで行う。**外部 URL を参照しない**（AR-020）。
// 一度読み込んだら state.lazy に記録し、以降は読み直さない。
//
// 呼び出し側（viewer.js）が doc.needsMermaid / doc.needsKaTeX を見て呼ぶ。
// **この条件分岐が NFR-013 の実体である。**

import { state } from "./state.js";

const MERMAID_JS = "vendor/mermaid/mermaid.min.js";
const KATEX_JS = "vendor/katex/katex.min.js";
const KATEX_CSS = "vendor/katex/katex.min.css";

// Mermaid が描画に使う一時 ID の連番。文書をまたいで重複させない。
let sequence = 0;

// ensureMermaid は未読込なら読み込み、テーマに合わせて初期化する（IMP-230, IMP-231）。
export async function ensureMermaid() {
  if (!state.lazy.mermaid) {
    await loadScript(MERMAID_JS);
    state.lazy.mermaid = true;
  }

  // securityLevel は strict 固定（MD-081）。図の定義からスクリプトや
  // クリックハンドラが実行されないようにする。
  window.mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme: state.theme === "dark" ? "dark" : "default",
  });
}

// ensureKaTeX は未読込なら JS と CSS を読み込む（IMP-230）。
export async function ensureKaTeX() {
  if (state.lazy.katex) return;

  await Promise.all([loadStyle(KATEX_CSS), loadScript(KATEX_JS)]);
  state.lazy.katex = true;
}

// drawMermaid は Mermaid ブロックを描画する（FR-023, IMP-231）。
//
// **ブロック単位で例外を捕捉する。** 1 つの失敗が他のブロックを止めない。
export async function drawMermaid(root) {
  const targets = [...root.querySelectorAll(".code-block[data-mermaid] pre.mermaid-source")];
  if (targets.length === 0) return;

  try {
    await ensureMermaid();
  } catch (error) {
    // 読み込めなければソースをコードブロックのまま残す（FR-023）。
    for (const pre of targets) showBlockError(pre.closest(".code-block"), error);
    return;
  }

  for (const pre of targets) {
    await drawOne(pre);
  }
}

// redrawMermaid はテーマ切り替え後に引き直す（IMP-231, FR-070）。
//
// 描画済みの SVG を data-source から作った <pre> へ戻してから描き直す。
// 原文を属性に持たせてあるのはこのためでもある（IMP-115）。
export async function redrawMermaid(root) {
  if (!state.lazy.mermaid) return;

  for (const block of root.querySelectorAll(".code-block[data-mermaid]")) {
    const rendered = block.querySelector(".mermaid-rendered");
    if (!rendered) continue;

    const pre = document.createElement("pre");
    pre.className = "mermaid-source";
    pre.textContent = block.dataset.source;
    rendered.replaceWith(pre);
  }

  await drawMermaid(root);
}

// drawMath は数式を描画する（MD-060, IMP-232）。
//
// **auto-render を使わない。** Go 側が `.math-inline` / `.math-block` として
// 範囲を確定させており（IMP-113）、デリミタ走査は不要かつ有害である。
export function drawMath(root) {
  const targets = [
    ...root.querySelectorAll(".math-inline:not([data-rendered]), .math-block:not([data-rendered])"),
  ];
  if (targets.length === 0) return Promise.resolve();

  return ensureKaTeX().then(() => {
    for (const element of targets) {
      const source = element.textContent; // Go 側が入れた TeX ソース
      element.dataset.rendered = "1";

      // 要素ごとに呼ぶため、1 つの失敗が他へ波及しない。
      // throwOnError: false により、失敗時は元のソースが赤字で残る（DSP-271）。
      window.katex.render(source, element, {
        displayMode: element.classList.contains("math-block"),
        throwOnError: false,
        trust: false, // NFR-030
      });
    }
  });
}

async function drawOne(pre) {
  const block = pre.closest(".code-block");
  const id = `mermaid-svg-${sequence}`;
  sequence += 1;

  try {
    const { svg, bindFunctions } = await window.mermaid.render(id, block.dataset.source);

    const holder = document.createElement("div");
    holder.className = "mermaid-rendered";
    // **Go を経由しない文字列を innerHTML へ渡す唯一の箇所**（IMP-220 の例外）。
    // Mermaid が securityLevel: 'strict' で自ら無害化したうえで返す SVG であり、
    // これ以外に図を DOM へ入れる手段がない。
    holder.innerHTML = svg;

    pre.replaceWith(holder);
    if (bindFunctions) bindFunctions(holder);

    const stale = block.querySelector(".mermaid-error");
    if (stale) stale.remove();
  } catch (error) {
    showBlockError(block, error);
  } finally {
    // 失敗時に Mermaid が残す作業用の要素を片付ける。
    const leftover = document.getElementById(`d${id}`);
    if (leftover) leftover.remove();
  }
}

// showBlockError はコードブロックの上にエラー内容を添える（FR-023, DSP-270）。
//
// 元のソースはコードブロックのまま残す。文言は Mermaid が返す英語をそのまま
// 出す。利用者向けの定型文ではなく、原因の手掛かりだからである。
function showBlockError(block, error) {
  const stale = block.querySelector(".mermaid-error");
  if (stale) stale.remove();

  const line = document.createElement("p");
  line.className = "mermaid-error";
  line.textContent = error && error.message ? error.message : String(error);
  block.insertBefore(line, block.firstChild);
}

function loadScript(src) {
  return new Promise((resolve, reject) => {
    const element = document.createElement("script");
    element.src = src;
    element.addEventListener("load", () => resolve());
    element.addEventListener("error", () => reject(new Error(`cannot load ${src}`)));
    document.head.appendChild(element);
  });
}

function loadStyle(href) {
  return new Promise((resolve, reject) => {
    const element = document.createElement("link");
    element.rel = "stylesheet";
    element.href = href;
    element.addEventListener("load", () => resolve());
    element.addEventListener("error", () => reject(new Error(`cannot load ${href}`)));
    document.head.appendChild(element);
  });
}
