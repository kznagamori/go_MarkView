// harness.js — 描画スモークテストのページ側（BR-054, E2E-109）。
//
// **本番の lazy.js をそのまま呼ぶ。** 描画のコードを写して持つと、
// Mermaid / KaTeX 側の API が変わったときに写しだけが古くなり、
// 「資産を更新しても描画できる」という BR-054 の問いに答えられなくなる。
//
// 結果は DOM へ書かず、Go 側へ POST で返す。描画は非同期であり、
// ヘッドレスブラウザの --dump-dom は「いつ終わったか」を知らないため、
// 終わった側から知らせるほうが待ち時間の推測を持ち込まずに済む。

// **名前空間として読む。** 個別の名前で import すると、未実装の関数が 1 つ
// あるだけでモジュールの読み込みごと失敗し、他の検査まで巻き添えになる。
import * as lazy from "./js/lazy.js";

// **viewer.js は動的に読む。** 静的 import にすると、モジュールの連結に
// 失敗したときにページごと死に、結果が POST されず「終わらない」形の失敗に
// なる。ここで捕まえれば、何が読めなかったかを Go 側へ返せる。
//
// **本番の markBrokenImages をそのまま呼ぶ**（IMP-226）。写しを持つと、
// 実装が変わったときに写しだけが古くなり、検査の意味が失われる。
let markBrokenImages = null;
let viewerError = "";
try {
  ({ markBrokenImages } = await import("./js/viewer.js"));
} catch (error) {
  viewerError = error && error.message ? error.message : String(error);
}

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
    plantuml: [],
    math: { total: 0, katex: 0, failed: [] },
    plantumlRejected: [],
    images: { imgs: [], broken: [], legacy: 0, imported: false, error: viewerError },
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

  // **画像の配線は描画より前に行う**（IMP-220 の手順 6 は 8 より前）。
  // 遅らせると、配線までに読み込みが終わった画像を取りこぼす経路が
  // 通らなくなり、検査が本番と違う順序を見ることになる。
  if (typeof markBrokenImages === "function") {
    report.images.imported = true;
    markBrokenImages(root);
  } else if (!viewerError) {
    viewerError = "viewer.js に markBrokenImages がない（IMP-226 が未実装）";
    report.images.error = viewerError;
  }

  try {
    await lazy.drawMermaid(root);
    await lazy.drawMath(root);

    // BR-054 は PlantUML も検査対象とする（E2E-109）。
    if (typeof lazy.drawPlantUML !== "function") {
      errors.push("lazy.js に drawPlantUML がない（IMP-233 が未実装）");
    } else {
      await lazy.drawPlantUML(root);
    }
  } catch (error) {
    report.fatal = error && error.message ? error.message : String(error);
  }

  collectMermaid(root, report);
  collectPlantUML(root, report);
  collectRejectedPlantUML(root, report);
  collectMath(root, mathSources, report);

  // 画像は非同期に読み込まれる。**結果を集める前に決着させる。**
  await settleImages(root);
  collectImages(root, report);

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

// collectPlantUML は図ごとの描画結果を集める。
//
// **フックの名前は Mermaid と同じ形にそろえる**（IMP-233）。描画結果は
// .plantuml-rendered の中へ、描かなかった理由は .plantuml-error へ入る。
// ここが食い違うと、実装できていても検査が 0 件で落ちる。
function collectPlantUML(root, report) {
  const blocks = [...root.querySelectorAll(".code-block[data-plantuml]")];

  blocks.forEach((block, index) => {
    const svgs = block.querySelectorAll(".plantuml-rendered svg");
    const box = svgs.length > 0 ? svgs[0].getBoundingClientRect() : null;
    const line = block.querySelector(".plantuml-error");
    const source = block.dataset.source || "";

    report.plantuml.push({
      index,
      head: source.split("\n")[0].trim(),
      svg: svgs.length,
      width: box ? Math.round(box.width) : 0,
      height: box ? Math.round(box.height) : 0,
      error: line ? line.textContent : "",
    });
  });
}

// collectRejectedPlantUML は Go 側が描画対象から外したブロックを集める
// （MD-084, IMP-119, DSP-272）。
//
// **これらは data-plantuml を持たない**ため collectPlantUML には現れない。
// 理由が出ていることを確かめるには、別に数える必要がある。
function collectRejectedPlantUML(root, report) {
  const blocks = [...root.querySelectorAll(".code-block[data-puml-error]")];

  report.plantumlRejected = blocks.map((block, index) => {
    const line = block.querySelector(".plantuml-error");
    const source = block.dataset.source || "";

    return {
      index,
      head: source.split("\n")[0].trim(),
      svg: block.querySelectorAll("svg").length,
      error: line ? line.textContent.trim() : "",
    };
  });
}

// settleImages は本文中の img がすべて決着するのを待つ。
//
// **固定時間で待たない**（UT-037 と同じ理由）。読み込みが終わった画像は
// complete が true になる。**置き換えられた画像は DOM から消えるため、
// 走査のたびに数え直す。**
async function settleImages(root, deadlineMs = 5000) {
  const limit = performance.now() + deadlineMs;

  for (;;) {
    const pending = [...root.querySelectorAll("img")].filter((img) => !img.complete);
    if (pending.length === 0 || performance.now() > limit) return;

    await new Promise((resolve) => setTimeout(resolve, 50));
  }
}

// collectImages は読み込みに失敗した画像の扱いを集める（IMP-226, DSP-123）。
//
// **見るのは「枠が出たか」ではなく「代替テキストが本文として読めるか」である。**
// 枠は CSS がこちらで描くため、どのエンジンでも出る。欠けうるのは中身のほうで
// あり、4.32.0 より前はそこをブラウザ既定に委ねていた（BUG-008）。
//
// **エンジン差そのものはここでは見えない**（Chromium 系でしか走らない。
// BR-054, NFR-061）。見ているのは「自前で描いているか」だけである。
function collectImages(root, report) {
  report.images.imgs = [...root.querySelectorAll("img")].map((img) => ({
    alt: img.getAttribute("alt") || "",
    className: img.className,
    complete: img.complete,
    naturalWidth: img.naturalWidth,
  }));

  report.images.broken = [...root.querySelectorAll(".img-broken")].map((el) => ({
    tagName: el.tagName,
    className: el.className,
    text: (el.textContent || "").trim(),
  }));

  // 4.32.0 より前のフック。**残っていたら修正が入っていない**（BUG-008）。
  report.images.legacy = root.querySelectorAll("img.is-broken").length;
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
