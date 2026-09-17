// puml.js — PlantUML の遅延ロードと描画（FR-024, MD-083, MD-084, NFR-013, IMP-230, IMP-233）。
//
// **呼び出し側は lazy.js を通す**（lazy.js が ensurePlantUML / drawPlantUML / redrawPlantUML を
// 再 export する。IMP-230 の export の置き場所）。lazy.js が 400 行の目安を超えたため分けた（IMP-011）。
//
// PlantUML は Mermaid に似ているが、**同じやり方では書けない**（IMP-233）。
//
//   1. render() は Promise を返さない  -> MutationObserver で待つ
//   2. 出力先を要素の id で指定する      -> 図ごとに一意な id を振る
//   3. viz-global.js を先に読む          -> 逆にすると Graphviz が見つからない
//   4. Graphviz 不在で描くと処理系が止まる -> 読めなければ一切描かない
//
// **描くのは鍵の合う図のブロックだけであり、描く原文は ownSource から取る**（IMP-230, IMP-260, BUG-014）。

import { holdHeight, isCurrentDrawing, loadScript, nextDiagramId, releaseHeight, startDrawing } from "./drawing.js";
import { onDiagramSettled } from "./media.js";
import { isOwnRef, ownSource } from "./refs.js";
import { syncHits } from "./search.js";
import { state } from "./state.js";
import { S } from "./strings.js";

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

// 描く図の大きさの上限（px。MD-083, IMP-233）。**render() に必ず明示して渡す。**
//
// 上限は処理系が持つ。1.2026.7 までは 4096 px の固定で、この指定は無視される。**1.2026.8 から既定が
// 8192 px になり、この指定で変えられる**（0 で検査を外す）。省くと、同梱資産の更新（BR-043）だけで
// 描ける図の大きさが黙って変わり、plantuml-limits.md の検証（E2E-240, BUG-010）も崩れる。
const PLANTUML_MAX_SVG_SIZE = 4096;

// plantuml.js の名前空間。ensurePlantUML が成功したときだけ入る。
let plantuml = null;

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
//
// **「描くもの」は鍵の合うブロックで数える**（IMP-233）。生 HTML の偽のブロックだけの文書で
// 資産（5 MiB 超）を読まない（NFR-013, BUG-014）。
//
// gen は描画の世代（drawing.js の startDrawing）。文書の描画（IMP-220 の手順 8）では Mermaid と
// 同じ番号を渡す。省くと自分で世代を進める（描画スモークのように単独で呼ぶ場合）。
//
// **描けなかったブロック（理由を添えたもの）は描かない**（DSP-370, IMP-233）。効くのはテーマの切り替えに
// よる描き直しである。描き直すと、描けないまま原文の <pre> を器に置き換えるため、理由が出るまで本文が
// 動き、原文の中の検索のハイライトが外れる（v1.0.0 から）。原文は CSS だけで新しい配色に追随する。
export async function drawPlantUML(root, gen = startDrawing()) {
  markRejected(root);

  const targets = ownPlantUMLBlocks(root)
    .filter((block) => !block.querySelector(":scope > .plantuml-error"))
    .map((block) => block.querySelector("pre.plantuml-source"))
    .filter((pre) => pre !== null);
  if (targets.length === 0) return;

  const loaded = await ensurePlantUML();
  if (!isCurrentDrawing(gen)) return;

  if (!loaded) {
    // **図を 1 つも描かない場合も、ブロックそれぞれについて知らせる**（IMP-230）。
    for (const pre of targets) {
      const block = pre.closest(".code-block");
      showPlantUMLError(block, S.pumlFailed);
      settleDiagram(block);
    }
    return;
  }

  // **逐次描く**（IMP-233）。完了検知の監視対象を図の数だけ同時に作らない。
  for (const pre of targets) {
    // **世代が変われば残りを描かない**（IMP-230）。
    if (!isCurrentDrawing(gen)) return;
    await drawOnePlantUML(pre, gen);
  }
}

// redrawPlantUML はテーマ切り替え後に引き直す（IMP-243, FR-070）。
//
// 描画済みの SVG を原文（ownSource）から作った <pre> へ戻してから描き直す。
// 原文を属性に持たせてあるのはこのためでもある（IMP-119）。
//
// **資産を読み込んでいる途中でも描く**（lazy.js の redrawMermaid と同じ理由。IMP-233）。読み込み中なら
// 同じ読み込みを待つ——viz-global.js を 2 度実行しない（drawing.js の loadScript）。
//
// **描き直す図のブロックは、描き終えるまで高さを保つ**（DSP-370。drawing.js の holdHeight）。
export async function redrawPlantUML(root, gen = startDrawing()) {
  for (const block of ownPlantUMLBlocks(root)) {
    const rendered = block.querySelector(".plantuml-rendered");
    if (!rendered) continue;

    holdHeight(block);
    rendered.replaceWith(sourceElement(block));
  }

  await drawPlantUML(root, gen);
}

// drawOnePlantUML は 1 枚描いて、完了を DOM で待つ。
//
// **render() は Promise を返さない**（IMP-233 の 1）。await しても即座に戻り、
// SVG はあとから対象要素へ書き込まれる。
async function drawOnePlantUML(pre, gen) {
  const block = pre.closest(".code-block");

  // **文書を切り替えても衝突しない値にする**（IMP-233 の 2）。
  const id = nextDiagramId("plantuml");

  const holder = document.createElement("div");
  holder.className = "plantuml-rendered";
  holder.id = id;
  pre.replaceWith(holder);

  // **原文はここで DOM から外れる**（描き終えるまで 1 秒近くかかりうる）。その間も件数を
  // 実際のハイライトに合わせる（IMP-241, BUG-017）。
  syncHits(block);

  const waiting = waitForPlantUML(holder);

  try {
    // **出力先は要素の id で渡す**（IMP-233 の 2）。要素そのものは渡せない。
    // **独自に HTML を組み立てて挿入しない**（IMP-220, MD-084）。DOM へ書くのは
    // 処理系であり、こちらは対象要素を用意して id を渡すだけにする。
    plantuml.render((ownSource(block) ?? "").split("\n"), id, {
      dark: state.theme === "dark",
      maxSvgSize: PLANTUML_MAX_SVG_SIZE,
    });
  } catch {
    // 処理系が例外を投げた場合の受け皿。
    //
    // **4096 px 超えはここへ来ない**（IMP-233, BUG-010）。制限に達したとき
    // render() は正常に戻り、**出力先の要素へ例外のテキストが書き込まれる**
    // （Diagram too large for browser rendering: <幅>x<高さ> (max 4096 …)。後ろは版で違う）。
    // それは下の waitForPlantUML が "other" と判定する経路で拾う。
    // **表示はどちらの経路でも pumlUnsupported であり、利用者から見た違いは無い。**
    //
    // **例外のテキストを本文へ出さない。** 文言は strings.js が持つ（IMP-290）。
    restorePlantUML(block, holder, S.pumlUnsupported);
    settleDiagram(block);
    return;
  }

  const outcome = await waiting;

  // **待っている間に世代が変わっていれば、結果を DOM へ反映しない**（IMP-230）。描き直し
  // （テーマの切り替え）がこの器をソースへ戻して描き直している最中に、失敗の理由を書き込まない。
  if (!isCurrentDrawing(gen)) return;

  if (outcome === "svg") {
    // **構文エラーは失敗ではない**（FR-024, DSP-272）。PlantUML が行番号付きの
    // エラー図を返しているので、そのまま図として見せる。
    const stale = block.querySelector(".plantuml-error");
    if (stale) stale.remove();
    settleDiagram(block);
    return;
  }

  restorePlantUML(block, holder, outcome === "other" ? S.pumlUnsupported : S.pumlFailed);
  // 描けなかった図も知らせる（IMP-230）。原寸表示と拡大画面の対象にはならない（FR-120）
  settleDiagram(block);
}

// settleDiagram は図 1 つを描き終えた（成功・失敗）ことを知らせる。**描き直しの間に保っていた高さを先に解く**
// （DSP-370。lazy.js の settleDiagram と同じ）。
function settleDiagram(block) {
  releaseHeight(block);
  onDiagramSettled(block);

  // **検索が開いていれば、件数とハイライトを実際の DOM へ合わせる**（IMP-241, BUG-017）。
  // 描けなかった図は原文が新しい <pre> として戻るため、その中を探し直してハイライトを付け直す。
  syncHits(block);
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
//
// **拒んだのが Go 側だと分かるのは鍵が合うときだけである。** 拒んだブロックも plantuml の目印を
// 持つ（IMP-120）。生 HTML で data-puml-error を書いたブロックには理由を書き込まない（IMP-230）。
function markRejected(root) {
  for (const block of root.querySelectorAll('.code-block[data-puml-error="include"]')) {
    if (!isOwnRef(block, "plantuml")) continue;

    showPlantUMLError(block, S.pumlInclude);
    // 描画しなかった図も知らせる（IMP-230）。番号は数える（FR-120）が、対象にはならない
    onDiagramSettled(block);
  }
}

// restorePlantUML は描けなかったブロックをソース表示へ戻し、理由を添える。
function restorePlantUML(block, holder, reason) {
  holder.replaceWith(sourceElement(block));
  showPlantUMLError(block, reason);
}

// sourceElement は原文（ownSource）から元のソース表示を作り直す（IMP-119）。
//
// **textContent で入れる。** Go を経由しない文字列を innerHTML に渡さない（IMP-220）。
function sourceElement(block) {
  const pre = document.createElement("pre");
  pre.className = "plantuml-source";
  pre.textContent = ownSource(block) ?? "";

  return pre;
}

// ownPlantUMLBlocks は、鍵の合う PlantUML のブロックのうち描くものを文書の順に返す（IMP-233）。
//
// **拒んだブロック（data-puml-error）は含めない**——data-plantuml を持たない（IMP-119）。
function ownPlantUMLBlocks(root) {
  return [...root.querySelectorAll(".code-block[data-plantuml]")].filter((block) => isOwnRef(block, "plantuml"));
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
