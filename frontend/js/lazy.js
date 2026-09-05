// lazy.js — Mermaid・KaTeX・PlantUML の遅延ロード（NFR-013, AR-021, IMP-230〜233）。
//
// 読み込みは frontend/vendor/ への相対パスで行う。**外部 URL を参照しない**（AR-020）。
// 一度読み込んだら state.lazy に記録し、以降は読み直さない。
//
// 呼び出し側（viewer.js）が doc.needsMermaid / doc.needsKaTeX を見て呼ぶ。
// **この条件分岐が NFR-013 の実体である。**
//
// PlantUML は Mermaid に似ているが、**同じやり方では書けない**（IMP-233）。
//
//   1. render() は Promise を返さない  -> MutationObserver で待つ
//   2. 出力先を要素の id で指定する      -> 図ごとに一意な id を振る
//   3. viz-global.js を先に読む          -> 逆にすると Graphviz が見つからない
//   4. Graphviz 不在で描くと処理系が止まる -> 読めなければ一切描かない

import { state } from "./state.js";
import { S } from "./strings.js";

const MERMAID_JS = "vendor/mermaid/mermaid.min.js";
const KATEX_JS = "vendor/katex/katex.min.js";
const KATEX_CSS = "vendor/katex/katex.min.css";

// **viz-global.js を先に、plantuml.js を後に**（IMP-230, IMP-233）。
const PLANTUML_VIZ = "vendor/plantuml/viz-global.js";
const PLANTUML_JS = "vendor/plantuml/plantuml.js";

// 1 枚あたりの打ち切り。Graphviz を要する図は 1 枚 400〜700 ms かかりうる
// （NFR-011）。**待たないという選択肢は無い**——render() が完了を教えないため、
// 打ち切りを設けないと描けなかったブロックが永久に空のままになる。
//
// **描画は逐次である**（IMP-233）。打ち切りが長いと、描けない図 1 枚が
// あとに続く図をその分だけ待たせる。NFR-011 の目標（Graphviz 系 1 枚 1.0 秒）
// に対して十分な余裕を取りつつ、止まったときの待ち時間を抑える。
const PLANTUML_TIMEOUT_MS = 10000;

// SVG 以外が書き込まれたあと、SVG を待つ猶予。
//
// **処理系が「描けない」と答えたときに、打ち切りまで待たないためにある。**
// ditaa を投げると内容は即座に返るが SVG にはならず、これが無いと 1 枚につき
// 打ち切りいっぱいを空費する（実測: 30 秒。2026-09-03）。
// 猶予を挟むのは、処理系が入れ物を先に置いてから SVG を入れる場合に
// 早合点しないためである。
const PLANTUML_SETTLE_MS = 400;

// plantuml.js の名前空間。ensurePlantUML が成功したときだけ入る。
let plantuml = null;

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
      //
      // **errorColor を渡さないと KaTeX 既定の #cc0000 が
      // インラインスタイルで入り、CSS から上書きできない。** トークンを
      // 参照する式を渡し、テーマに追従させる（DSP-271）。
      window.katex.render(source, element, {
        displayMode: element.classList.contains("math-block"),
        throwOnError: false,
        errorColor: "var(--danger-fg)",
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

// ensurePlantUML は未読込なら viz-global.js -> plantuml.js の順で読む（IMP-230）。
//
// **読めなかったら false を返し、呼び出し側は一切描かない**（IMP-233 の 4）。
// Graphviz 不在のまま class 図を投げると**処理系ごと止まり、以降のすべての
// 描画が返ってこなくなる。** 例外を投げずに真偽値で返すのは、この判断を
// 呼び出し側から見落としにくくするためである。
export async function ensurePlantUML() {
  if (state.lazy.plantuml) return true;

  try {
    await loadScript(PLANTUML_VIZ);
  } catch {
    return false;
  }

  // 読み込み自体は成功しても、グローバルが置かれていなければ同じことである。
  if (typeof window.Viz === "undefined") return false;

  try {
    // 動的 import は**このモジュールからの相対**で解決されるため、
    // 他の資産と同じく文書からの相対で指定し直す。
    plantuml = await import(new URL(PLANTUML_JS, document.baseURI).href);
  } catch {
    return false;
  }

  if (typeof plantuml.render !== "function") return false;

  state.lazy.plantuml = true;
  return true;
}

// drawPlantUML は PlantUML ブロックを描画する（FR-024, IMP-233）。
//
// **条件を付けずに呼んでよい。** 描くものが無ければ資産を読まずに戻るため、
// NFR-013 は保たれる。取り込み指令で拒まれたブロック（IMP-119）は
// needsPlantUML を立てないが、**理由は出さなければならない**（DSP-272）。
export async function drawPlantUML(root) {
  markRejected(root);

  const targets = [...root.querySelectorAll(".code-block[data-plantuml] pre.plantuml-source")];
  if (targets.length === 0) return;

  if (!(await ensurePlantUML())) {
    for (const pre of targets) showPlantUMLError(pre.closest(".code-block"), S.pumlFailed);
    return;
  }

  // **逐次描く**（IMP-233）。完了検知の監視対象を図の数だけ同時に作らない。
  for (const pre of targets) {
    await drawOnePlantUML(pre);
  }
}

// redrawPlantUML はテーマ切り替え後に引き直す（IMP-243, FR-070）。
//
// 描画済みの SVG を data-source から作った <pre> へ戻してから描き直す。
// 原文を属性に持たせてあるのはこのためでもある（IMP-119）。
export async function redrawPlantUML(root) {
  if (!state.lazy.plantuml) return;

  for (const block of root.querySelectorAll(".code-block[data-plantuml]")) {
    const rendered = block.querySelector(".plantuml-rendered");
    if (!rendered) continue;

    rendered.replaceWith(sourceElement(block));
  }

  await drawPlantUML(root);
}

// drawOnePlantUML は 1 枚描いて、完了を DOM で待つ。
//
// **render() は Promise を返さない**（IMP-233 の 1）。await しても即座に戻り、
// SVG はあとから対象要素へ書き込まれる。
async function drawOnePlantUML(pre) {
  const block = pre.closest(".code-block");

  // **文書を切り替えても衝突しない値にする**（IMP-233 の 2）。
  // Mermaid と同じ連番を使い、両者の間でも重ならないようにしている。
  const id = `plantuml-svg-${sequence}`;
  sequence += 1;

  const holder = document.createElement("div");
  holder.className = "plantuml-rendered";
  holder.id = id;
  pre.replaceWith(holder);

  const waiting = waitForPlantUML(holder);

  try {
    // **出力先は要素の id で渡す**（IMP-233 の 2）。要素そのものは渡せない。
    // **独自に HTML を組み立てて挿入しない**（IMP-220, MD-084）。DOM へ書くのは
    // 処理系であり、こちらは対象要素を用意して id を渡すだけにする。
    plantuml.render(block.dataset.source.split("\n"), id, { dark: state.theme === "dark" });
  } catch {
    // 処理系が例外を投げた場合の受け皿。
    //
    // **4096 px 超えはここへ来ない**（IMP-233, BUG-010）。制限に達したとき
    // render() は正常に戻り、**出力先の要素へ例外のテキストが書き込まれる**
    // （Diagram too large for browser rendering: <幅>x<高さ> (max 4096)）。
    // それは下の waitForPlantUML が "other" と判定する経路で拾う。
    // **表示はどちらの経路でも pumlUnsupported であり、利用者から見た違いは無い。**
    //
    // **例外のテキストを本文へ出さない。** 文言は strings.js が持つ（IMP-290）。
    restorePlantUML(block, holder, S.pumlUnsupported);
    return;
  }

  const outcome = await waiting;
  if (outcome === "svg") {
    // **構文エラーは失敗ではない**（FR-024, DSP-272）。PlantUML が行番号付きの
    // エラー図を返しているので、そのまま図として見せる。
    const stale = block.querySelector(".plantuml-error");
    if (stale) stale.remove();
    return;
  }

  restorePlantUML(block, holder, outcome === "other" ? S.pumlUnsupported : S.pumlFailed);
}

// waitForPlantUML は対象要素に SVG が現れるのを待つ（IMP-233 の 1）。
//
// 戻り値は 3 通り。**判定は「SVG が得られたか」だけで行う**（DSP-272）。
// 図種別ごとの判定をフロントエンドに持たない。
//
//   "svg"      図が得られた
//   "other"    処理系が何か書いたが SVG ではない（未対応の図種別など）
//   "timeout"  何も起きなかった（資産の不調）
function waitForPlantUML(holder) {
  return new Promise((resolve) => {
    let deadline = null;
    let settle = null;

    const observer = new MutationObserver(() => {
      if (holder.querySelector("svg")) {
        done("svg");
        return;
      }

      // SVG 以外が入った。処理系は答えているので、猶予だけ待って打ち切る。
      if (holder.childNodes.length > 0 && settle === null) {
        settle = setTimeout(() => {
          done(holder.querySelector("svg") ? "svg" : "other");
        }, PLANTUML_SETTLE_MS);
      }
    });

    function done(outcome) {
      observer.disconnect();
      clearTimeout(deadline);
      clearTimeout(settle);
      resolve(outcome);
    }

    // 何も起きないまま尽きた場合。資産の不調やタイムアウトに当たる（DSP-272）。
    deadline = setTimeout(() => done("timeout"), PLANTUML_TIMEOUT_MS);

    observer.observe(holder, { childList: true, subtree: true });
  });
}

// markRejected は Go 側が拒んだブロックに理由を添える（IMP-119, DSP-272）。
//
// **資産を読まない。** 拒まれたブロックしか無い文書で 5 MiB を読むのは無駄で
// あり、IMP-119 が needsPlantUML を立てないのもそのためである（NFR-013）。
function markRejected(root) {
  for (const block of root.querySelectorAll('.code-block[data-puml-error="include"]')) {
    showPlantUMLError(block, S.pumlInclude);
  }
}

// restorePlantUML は描けなかったブロックをソース表示へ戻し、理由を添える。
function restorePlantUML(block, holder, reason) {
  holder.replaceWith(sourceElement(block));
  showPlantUMLError(block, reason);
}

// sourceElement は data-source から元のソース表示を作り直す（IMP-119）。
//
// **textContent で入れる。** Go を経由しない文字列を innerHTML に渡さない（IMP-220）。
function sourceElement(block) {
  const pre = document.createElement("pre");
  pre.className = "plantuml-source";
  pre.textContent = block.dataset.source;

  return pre;
}

// showPlantUMLError は理由をブロックの上に添える（FR-110, DSP-272）。
//
// **図が出ていないときにだけ使う。** 構文エラーは PlantUML 自身がエラー図を
// 返すため、こちらでは何も書かない（FR-024）。
function showPlantUMLError(block, reason) {
  const stale = block.querySelector(".plantuml-error");
  if (stale) stale.remove();

  const line = document.createElement("p");
  line.className = "plantuml-error";
  line.textContent = reason;
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
