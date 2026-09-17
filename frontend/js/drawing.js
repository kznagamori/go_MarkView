// drawing.js — 図の描画が共有する状態・描き直しの間の高さ・資産の読み込み（IMP-230, IMP-231, IMP-233, DSP-370）。
//
// lazy.js（Mermaid・KaTeX）と puml.js（PlantUML）の両方が使う。**このモジュールは他の自前の
// モジュールを import しない**（葉。循環を作らない）。

// 図の器の id の連番。**Mermaid と PlantUML で共有し、ページを読み込んでから数える**
// （IMP-231, IMP-233 の 2）。文書を切り替えても衝突させない。
let sequence = 0;

// 描画の世代（IMP-230）。描画を始めるたびに 1 つ進める。
let generation = 0;

// nextDiagramId は図の器の id を返す（`mermaid-svg-<n>` / `plantuml-svg-<n>`）。
//
// **user-content- で始めない**（AR-053）。描画スモーク（BR-054）はこの名前で図の器を見分ける。
export function nextDiagramId(kind) {
  const id = `${kind}-svg-${sequence}`;
  sequence += 1;
  return id;
}

// startDrawing は描画の世代を 1 つ進め、その番号を返す（IMP-230）。
//
// **呼ぶのは描画を始める 2 か所だけとする**——文書の描画（IMP-220 の手順 8）と、テーマの切り替えに
// よる描き直し（IMP-231, IMP-233）。Mermaid と PlantUML は同じ番号を受け取る。**それぞれで進めると、
// 同時に走っている相手の描画を自分で止めてしまう。**
export function startDrawing() {
  generation += 1;
  return generation;
}

// isCurrentDrawing は gen が今の世代かを返す（IMP-230）。
//
// **図を 1 つ描き終えるたびに（待ちから戻るたびに）確かめ、違えば残りを描かず、DOM にも触らない。**
// PlantUML は 1 枚に数秒かかりうるため、外部エディタで保存を繰り返すと、前の描画の知らせが新しい
// 描画の後に届く。区別しないと、描き直した図の上に前の描画の失敗の理由を書いたり、DOM から外れた
// 古い要素へ差し替えたりする（FR-122, UC-03）。
export function isCurrentDrawing(gen) {
  return gen === generation;
}

// holdHeight は描き直す図のブロックの高さを、描き終えるまで保つ（DSP-370, IMP-231, IMP-233）。
//
// テーマの切り替えは描画済みの図を原文へ戻してから描き直す。そのままでは、描き終えるまで（Graphviz を要する
// PlantUML は 1 枚 1 秒近く）図より下の本文が原文の高さの分だけ動く。**戻す直前の高さを CSS 変数で渡し、
// is-redrawing の間はその高さに固定する**（components.css）。
export function holdHeight(block) {
  block.style.setProperty("--diagram-height", `${block.getBoundingClientRect().height}px`);
  block.classList.add("is-redrawing");
}

// releaseHeight は holdHeight を解く。**図の知らせ（media.js の onDiagramSettled）より前に呼ぶ**——原寸表示の
// 判定と当て直しは、図の本来の配置で測る（IMP-228）。保っていなければ何もしない。
export function releaseHeight(block) {
  block.classList.remove("is-redrawing");
  block.style.removeProperty("--diagram-height");
}

// 資産の読み込みの約束（IMP-230）。キーは src / href。
//
// **同じ資産の読み込みを二重に始めない。** 読み込みの途中で文書の描画やテーマの切り替えによる描き直しが
// 重なると、それぞれが <script> を挿し、同じ資産が 2 度実行される（viz-global.js の Graphviz を含む）。
// 読み込み中の約束を共有し、読み込めた約束はページの間ずっと返す。**読み込めなかった約束は忘れる**——
// error になった <script> は実行されておらず、次に描くときに読み直してよい。
const loads = new Map();

// loadScript は <script> を挿して読み込みを待つ。
export function loadScript(src) {
  return loadOnce(src, () => {
    const element = document.createElement("script");
    element.src = src;
    return element;
  });
}

// loadStyle は <link rel="stylesheet"> を挿して読み込みを待つ。
export function loadStyle(href) {
  return loadOnce(href, () => {
    const element = document.createElement("link");
    element.rel = "stylesheet";
    element.href = href;
    return element;
  });
}

// loadOnce は key の資産を読み込む要素を 1 度だけ挿し、その約束を返す。
function loadOnce(key, create) {
  const known = loads.get(key);
  if (known) return known;

  const promise = new Promise((resolve, reject) => {
    const element = create();
    element.addEventListener("load", () => resolve());
    element.addEventListener("error", () => {
      // 読めなかった要素は使い回せない。次に読み直すときは新しい要素を挿す
      loads.delete(key);
      element.remove();
      reject(new Error(`cannot load ${key}`));
    });
    document.head.appendChild(element);
  });
  loads.set(key, promise);
  return promise;
}
