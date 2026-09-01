// harness.js — 描画スモークテストのページ側（BR-054, E2E-109）。
//
// **本番の lazy.js をそのまま呼ぶ。** 描画のコードを写して持つと、
// Mermaid / KaTeX 側の API が変わったときに写しだけが古くなり、
// 「資産を更新しても描画できる」という BR-054 の問いに答えられなくなる。
//
// 結果は DOM へ書かず、Go 側へ POST で返す。描画は非同期であり、
// ヘッドレスブラウザの --dump-dom は「いつ終わったか」を知らないため、
// 終わった側から知らせるほうが待ち時間の推測を持ち込まずに済む。

import { drawMermaid, drawMath } from "./js/lazy.js";

const started = performance.now();

// 描画中に起きた異常を貯める（BR-054 の「JavaScript のエラーが発生しない」）。
const errors = [];
const consoleErrors = [];

// **キャプチャフェーズで拾う。** 資産の読み込み失敗（script / link の
// error）はバブルしないため、これを付けないと取りこぼす。
window.addEventListener(
  "error",
  (event) => {
    const target = event.target;
    if (target && target !== window && target.tagName) {
      errors.push("resource: " + target.tagName + " " + (target.src || target.href || ""));
      return;
    }
    errors.push("error: " + (event.message || String(event.error)));
  },
  true,
);

window.addEventListener("unhandledrejection", (event) => {
  const reason = event.reason;
  errors.push("rejection: " + (reason && reason.message ? reason.message : String(reason)));
});

// Mermaid は解析に失敗しても例外を投げずに console.error へ流すことがある。
// 握りつぶさず記録する。合否の判断は Go 側で行う。
const passThrough = console.error.bind(console);
console.error = (...args) => {
  consoleErrors.push(args.map(describe).join(" "));
  passThrough(...args);
};

main();

async function main() {
  const root = document.getElementById("markdown");
  const report = {
    mermaid: [],
    math: { total: 0, katex: 0, failed: [] },
    errors,
    console: consoleErrors,
    elapsedMs: 0,
    userAgent: navigator.userAgent,
    fatal: "",
  };

  // KaTeX は描画すると要素の中身を置き換えるため、原文を先に控える。
  const mathSources = [...root.querySelectorAll(".math-inline, .math-block")].map(
    (element) => element.textContent,
  );

  try {
    await drawMermaid(root);
    await drawMath(root);
  } catch (error) {
    report.fatal = error && error.message ? error.message : String(error);
  }

  collectMermaid(root, report);
  collectMath(root, mathSources, report);

  report.elapsedMs = Math.round(performance.now() - started);

  await fetch("_result", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(report),
  });
}

// collectMermaid は図ごとの描画結果を集める。
//
// **SVG の有無だけでは足りない。** Mermaid は失敗しても大きさのない SVG を
// 残すことがあるため、実際の寸法まで見る。
function collectMermaid(root, report) {
  const blocks = [...root.querySelectorAll(".code-block[data-mermaid]")];

  blocks.forEach((block, index) => {
    const svgs = block.querySelectorAll(".mermaid-rendered svg");
    const box = svgs.length > 0 ? svgs[0].getBoundingClientRect() : null;
    const line = block.querySelector(".mermaid-error");
    const source = block.dataset.source || "";

    report.mermaid.push({
      index,
      head: source.split("\n")[0].trim(),
      svg: svgs.length,
      width: box ? Math.round(box.width) : 0,
      height: box ? Math.round(box.height) : 0,
      error: line ? line.textContent : "",
    });
  });
}

// KaTeX が「解釈できなかった」ことを示す印（IMP-232 の throwOnError: false）。
//
// 失敗の現れ方は 2 通りあり、**片方だけを見ると取りこぼす。**
//
//   .katex-error       式全体の構文解析に失敗したとき。原文がそのまま残る
//   errorColor の着色  未知のコマンドだけを赤くして、残りは描き切るとき
//
// 後者は `.katex` が普通に生成されるため、要素の有無では区別できない。
// 実際 `\nosuchcommand{x}` は `<span class="katex">` の中に
// `<mstyle mathcolor="...">` として現れ、数だけを数えると合格してしまう
// （2026-09-02 に実機で確認）。
//
// **色が付いていること自体を印として使う。** この検証用文書には色を指定する
// 記法を書かない約束にしてあり（testdata/smoke.md の注記）、出力に色が
// 現れたらそれは KaTeX が付けたものである。
const KATEX_FAILURE = '.katex-error, [mathcolor], [style*="color:"]';

// collectMath は数式の描画結果を集める。
function collectMath(root, sources, report) {
  const targets = [...root.querySelectorAll(".math-inline, .math-block")];

  let katex = 0;
  const failed = [];

  targets.forEach((element, index) => {
    if (element.querySelector(".katex")) katex += 1;
    if (element.querySelector(KATEX_FAILURE)) {
      failed.push((sources[index] || "").slice(0, 80));
    }
  });

  report.math = { total: targets.length, katex, failed };
}

function describe(value) {
  if (value instanceof Error) return value.message;
  if (typeof value === "string") return value;

  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}
