// harness.js — 描画スモークテストのページ側（BR-054, E2E-109）。
//
// **本番の lazy.js をそのまま呼ぶ。** 描画のコードを写して持つと、
// Mermaid / KaTeX 側の API が変わったときに写しだけが古くなり、
// 「資産を更新しても描画できる」という BR-054 の問いに答えられなくなる。
//
// v1.1.0 からは、本番の tablesort.js と refs.js も呼ぶ（BR-054 の「表の並べ替えの
// 比較」「目印の照合」）。本文の id の事実も集める（「文書の id の名前空間」）。
// **ここでは判定しない。** どの要素が GFM の由来かも、何を処理系の id とみなすかも、
// Go 側（scripts/smoke の ids.go / tablesort.go / refs.go）が決める。
//
// 結果は DOM へ書かず、Go 側へ POST で返す。描画は非同期であり、
// ヘッドレスブラウザの --dump-dom は「いつ終わったか」を知らないため、
// 終わった側から知らせるほうが待ち時間の推測を持ち込まずに済む。

// **名前空間として読む。** 個別の名前で import すると、未実装の関数が 1 つ
// あるだけでモジュールの読み込みごと失敗し、他の検査まで巻き添えになる。
import * as lazy from "./js/lazy.js";

// **本番の state.js を使う。** refs.js は state.doc.refKey で鍵を照合する
// （IMP-260）。同じ URL のモジュールは 1 つしか作られないため、ここで入れた値を
// lazy.js / refs.js / tablesort.js が同じく読む。
import { state } from "./js/state.js";

// **集める関数は collect.js にある**（harness.js が 400 行の目安を超えたため分けた。IMP-011）。
// 検証用のページの一部であり本番のモジュールではないため、静的に読む。
import {
  collectIds,
  collectImages,
  collectMath,
  collectMermaid,
  collectPlantUML,
  collectRefs,
  collectRejectedPlantUML,
  collectSort,
  diagramBlocks,
  settleImages,
} from "./_smoke_collect.js";

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

// **tablesort.js と refs.js も動的に読む**（BR-054）。理由は viewer.js と同じ。
// 読めなかったら、その検査だけを失敗 1 件にして他の検査は続ける（UT-814, UT-815）。
let tablesort = null;
let tablesortError = "";
try {
  tablesort = await import("./js/tablesort.js");
} catch (error) {
  tablesortError = error && error.message ? error.message : String(error);
}

let refs = null;
let refsError = "";
try {
  refs = await import("./js/refs.js");
} catch (error) {
  refsError = error && error.message ? error.message : String(error);
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

  // 目印の鍵と並べ替えの入力（BR-054）。**期待値は受け取らない。** 判定は Go 側。
  const config = await (await fetch("_config")).json();

  // renderDocument（IMP-220）と同じく、描画の前に文書を state へ置く。
  // 鍵の合う図だけを描く lazy.js（IMP-230）も、この値を読む。
  state.doc = { refKey: config.refKey };

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
    ids: { elements: [], headings: [], plantumlDone: false },
    sort: { imported: false, error: tablesortError, results: [] },
    refs: {
      imported: false,
      error: refsError,
      sortImported: false,
      checkboxes: [],
      tables: [],
      cells: [],
      links: [],
      blocks: [],
    },
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
      // **id はこの後に集める。** plantuml.js はログを出すたびに #status を
      // 書き換えるため、描画の前に見ると衝突していても通る（BR-054, UT-813）。
      report.ids.plantumlDone = true;
    }
  } catch (error) {
    report.fatal = error && error.message ? error.message : String(error);
  }

  // 並べ替えのボタンを付ける（renderDocument の手順 6c）。**本番の関数を呼ぶ。**
  if (tablesort && typeof tablesort.attachSortButtons === "function") {
    try {
      if (typeof tablesort.initTableSort === "function") tablesort.initTableSort({ closeSearch() {} });
      tablesort.attachSortButtons(root);
      report.refs.sortImported = true;
    } catch (error) {
      errors.push("attachSortButtons: " + (error && error.message ? error.message : String(error)));
    }
  }

  const blocks = diagramBlocks(root);

  collectMermaid(root, blocks, report);
  collectPlantUML(root, blocks, report);
  collectRejectedPlantUML(root, blocks, report);
  collectMath(root, mathSources, report);
  collectIds(root, report);
  collectSort(tablesort, config.sortCases || [], report);
  collectRefs(refs, root, blocks, report);

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

function describe(value) {
  if (value instanceof Error) return value.message;
  if (typeof value === "string") return value;

  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}
