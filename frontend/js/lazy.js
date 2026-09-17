// lazy.js — Mermaid・KaTeX・PlantUML の遅延ロード（NFR-013, AR-021, IMP-230〜233）。
//
// 読み込みは frontend/vendor/ への相対パスで行う。**外部 URL を参照しない**（AR-020）。
// 一度読み込んだら state.lazy に記録し、以降は読み直さない。**読み込み中の資産は、その読み込みを待つ**
// （drawing.js の loadScript / loadStyle。同じ資産を 2 度実行しない）。
//
// 呼び出し側（viewer.js）が doc.needsMermaid / doc.needsKaTeX を見て呼ぶ。
// **この条件分岐が NFR-013 の実体である。**
//
// **PlantUML は puml.js に分けた**（400 行の目安。IMP-011）。**呼び出し側はこのモジュールだけを
// import する**——PlantUML の 3 つの関数もここから再 export する（IMP-230 の export の置き場所）。
// 図の器の連番・描画の世代・資産の読み込みは、両方が使う drawing.js にある。
//
// **描くのは鍵の合う図のブロックだけであり、描く原文は ownSource から取る**（IMP-230, IMP-260）。
// `class="code-block"` と `data-mermaid` / `data-plantuml` / `data-source` は生 HTML でも書ける。
// 属性で選ぶと、Go 側の取り込み指令の検査（MD-084, IMP-119）を通っていない PlantUML が処理系へ
// 渡り、PlantUML を含まない文書でも資産を読む（NFR-013, NFR-030, BUG-014。v1.0.0 はこの形だった）。
// **鍵の合わないブロックは HTML のまま残し、理由も書き込まない。**

import { holdHeight, isCurrentDrawing, loadScript, loadStyle, nextDiagramId, releaseHeight, startDrawing } from "./drawing.js";
import { onAllDiagramsSettled, onDiagramSettled } from "./media.js";
import { drawPlantUML, redrawPlantUML } from "./puml.js";
import { isOwnRef, ownSource } from "./refs.js";
import { syncHits } from "./search.js";
import { state } from "./state.js";

export { ensurePlantUML, drawPlantUML, redrawPlantUML } from "./puml.js";
export { startDrawing } from "./drawing.js";

const MERMAID_JS = "vendor/mermaid/mermaid.min.js";
const KATEX_JS = "vendor/katex/katex.min.js";
const KATEX_CSS = "vendor/katex/katex.min.css";

// drawDiagrams は文書の図（Mermaid と PlantUML）を描く（IMP-220 の手順 8, IMP-230）。
//
// **両方の知らせを出し終えたら、media.js の onAllDiagramsSettled を 1 度だけ呼ぶ**（IMP-230, IMP-228）。
// 図を含まない文書でも呼ぶ——拡大画面が画像を開いていた場合の判断に要る（IMP-253）。**世代が変わって
// いれば呼ばない**（その描画の知らせは途中で止まっている）。
//
// **PlantUML だけは条件を付けずに描く。** Go 側が取り込み指令を含むブロックを描画対象から外しており
// （IMP-119）、そのブロックは needsPlantUML を立てないが、**理由は表示しなければならない**（DSP-272）。
// drawPlantUML は描くものが無ければ資産を読まずに戻るため、NFR-013 は保たれる。
export async function drawDiagrams(root, needsMermaid, gen = startDrawing()) {
  await Promise.allSettled([needsMermaid ? drawMermaid(root, gen) : null, drawPlantUML(root, gen)]);
  if (isCurrentDrawing(gen)) onAllDiagramsSettled();
}

// redrawDiagrams はテーマの切り替えで図を描き直す（IMP-243, IMP-231, IMP-233）。知らせは drawDiagrams と同じ。
//
// **描き直した図にも原寸表示を当て直す**（onDiagramSettled。FR-121, DSP-370）。
export async function redrawDiagrams(root, gen = startDrawing()) {
  await Promise.allSettled([redrawMermaid(root, gen), redrawPlantUML(root, gen)]);
  if (isCurrentDrawing(gen)) onAllDiagramsSettled();
}

// ensureMermaid は未読込なら読み込み、テーマに合わせて初期化する（IMP-230, IMP-231）。
export async function ensureMermaid() {
  if (!state.lazy.mermaid) {
    await loadScript(MERMAID_JS);
    state.lazy.mermaid = true;
  }

  // securityLevel は strict 固定（MD-081）。図の定義からスクリプトや
  // クリックハンドラが実行されないようにする。
  //
  // **dompurifyConfig の SANITIZE_NAMED_PROPS を落とさない**（IMP-231, AR-053）。図のラベルには
  // 書き手が HTML を書け（`A["<span id='tooltip'>x</span>"]`）、strict でもその id は SVG の中に
  // 残り、画面の要素（#tooltip）や同梱資産の決め打ち（#status）を乗っ取る（BUG-011）。
  // この設定で DOMPurify が id / name を user-content- 付きへ書き換える（Mermaid 11.17.2 で実測）。
  // 資産を更新したら描画スモーク（BR-054）でこれが効いていることを確かめる。
  window.mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme: state.theme === "dark" ? "dark" : "default",
    dompurifyConfig: { SANITIZE_NAMED_PROPS: true },
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
//
// gen は描画の世代（drawing.js の startDrawing。IMP-230）。文書の描画（IMP-220 の手順 8）では
// PlantUML と同じ番号を渡す。省くと自分で世代を進める（描画スモークのように単独で呼ぶ場合）。
//
// **描けなかったブロック（理由を添えたもの）は描かない**（DSP-370, IMP-231）。文書の描画では理由はまだ
// 無く、効くのはテーマの切り替えによる描き直しである。原文のコードブロックは CSS だけで新しい配色に
// 追随する。読み込めなかった資産の読み直しは、次の文書の描画（F5 を含む）で行う。
export async function drawMermaid(root, gen = startDrawing()) {
  const targets = ownMermaidBlocks(root)
    .filter((block) => !block.querySelector(":scope > .mermaid-error"))
    .map((block) => block.querySelector("pre.mermaid-source"))
    .filter((pre) => pre !== null);
  if (targets.length === 0) return;

  try {
    await ensureMermaid();
  } catch (error) {
    if (!isCurrentDrawing(gen)) return;
    // 読み込めなければソースをコードブロックのまま残す（FR-023）。**図を 1 つも描かない場合も、
    // ブロックそれぞれについて知らせる**（IMP-230。拡大画面は「その図はもう来ない」ことを知って閉じる）。
    for (const pre of targets) {
      const block = pre.closest(".code-block");
      showBlockError(block, error);
      settleDiagram(block);
    }
    return;
  }

  for (const pre of targets) {
    // **世代が変われば残りを描かない**（IMP-230）。
    if (!isCurrentDrawing(gen)) return;
    await drawOne(pre, gen);
  }
}

// redrawMermaid はテーマ切り替え後に引き直す（IMP-231, FR-070）。
//
// 描画済みの SVG を原文（ownSource。Go 側が data-source に持たせた値）から作った <pre> へ
// 戻してから描き直す。原文を属性に持たせてあるのはこのためでもある（IMP-115）。
//
// **資産を読み込んでいる途中でも描く。** 初めての図の資産を読み込む間にテーマを切り替えると、読み込みを
// 待っていた文書の描画は世代が変わって描かずに戻る（IMP-230）。ここで「読み込み済みでなければ戻る」と、
// 図が原文のまま残る（v1.0.0 には世代が無く起きなかった）。読み込み中なら同じ読み込みを待ち
// （drawing.js の loadScript）、鍵の合うブロックが無ければ資産を読まずに戻る（drawMermaid。NFR-013）。
//
// **描き直す図のブロックは、描き終えるまで高さを保つ**（DSP-370。drawing.js の holdHeight）。
export async function redrawMermaid(root, gen = startDrawing()) {
  for (const block of ownMermaidBlocks(root)) {
    const rendered = block.querySelector(".mermaid-rendered");
    if (!rendered) continue;

    holdHeight(block);
    const pre = document.createElement("pre");
    pre.className = "mermaid-source";
    pre.textContent = ownSource(block) ?? "";
    rendered.replaceWith(pre);
  }

  await drawMermaid(root, gen);
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

    // **数式の要素の中身は描画結果へ置き換わる**（IMP-232）。描く前に包んだハイライトは
    // DOM から外れるため、検索の状態を合わせる（IMP-241, BUG-017）。
    for (const element of targets) syncHits(element);
  });
}

async function drawOne(pre, gen) {
  const block = pre.closest(".code-block");
  const id = nextDiagramId("mermaid");

  try {
    // 呼び出し元が鍵の合うブロックに絞っているため、ownSource は文字列を返す。
    const { svg, bindFunctions } = await window.mermaid.render(id, ownSource(block) ?? "");

    // **描いている間に世代が変わっていれば、DOM へ反映しない**（IMP-230）。
    if (!isCurrentDrawing(gen)) return;

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
    // 描き終えた図を知らせる（IMP-230, IMP-228）。**成功・失敗のどちらでも**、世代が今のときだけ。
    settleDiagram(block);
  } catch (error) {
    if (isCurrentDrawing(gen)) {
      showBlockError(block, error);
      settleDiagram(block);
    }
  } finally {
    // 失敗時に Mermaid が body に残す作業用の要素を片付ける。画面の要素であり、文書の id
    // （user-content- で始まる。AR-053）とは重ならない。
    const leftover = document.getElementById(`d${id}`);
    if (leftover) leftover.remove();
  }
}

// settleDiagram は図 1 つを描き終えた（成功・失敗）ことを知らせる（IMP-230, IMP-228）。**描き直しの間に保っていた
// 高さを先に解く**（DSP-370）——原寸表示の判定は図の本来の配置で測る。
function settleDiagram(block) {
  releaseHeight(block);
  onDiagramSettled(block);

  // **検索が開いていれば、件数とハイライトを実際の DOM へ合わせる**（IMP-241, BUG-017）。
  // 描き終えた図の原文は DOM から外れており、そのままでは件数だけが残る。
  syncHits(block);
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

// ownMermaidBlocks は鍵の合う Mermaid のブロックを文書の順に返す（IMP-230, IMP-231）。
function ownMermaidBlocks(root) {
  return [...root.querySelectorAll(".code-block[data-mermaid]")].filter((block) => isOwnRef(block, "mermaid"));
}
