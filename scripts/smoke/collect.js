// collect.js — 描画スモークテストのページ側で、描画の後の事実を集める関数（BR-054, E2E-109）。
//
// **ここでも判定しない。** 集めた事実は harness.js が Go 側へ POST で返し、判定は scripts/smoke の
// check.go / ids.go / tablesort.go / refs.go が行う。**本番のモジュール（refs.js / tablesort.js）は
// harness.js が動的に読んだものを引数で受け取る**——ここで静的に import すると、読めなかったときに
// ページごと止まり、結果が返らない（harness.js の冒頭）。
//
// harness.js が 400 行の目安（IMP-011）を超えたため分けた。

// collectMermaid は図ごとの描画結果を集める。
//
// **SVG の有無だけでは足りない。** Mermaid は失敗しても大きさのない SVG を
// 残すことがあるため、実際の寸法まで見る。
export function collectMermaid(root, all, report) {
  const blocks = [...root.querySelectorAll(".code-block[data-mermaid]")];

  blocks.forEach((block, index) => {
    const svgs = block.querySelectorAll(".mermaid-rendered svg");
    const box = svgs.length > 0 ? svgs[0].getBoundingClientRect() : null;
    const line = block.querySelector(".mermaid-error");
    const source = block.dataset.source || "";

    report.mermaid.push({
      index,
      block: all.indexOf(block),
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
export function collectPlantUML(root, all, report) {
  const blocks = [...root.querySelectorAll(".code-block[data-plantuml]")];

  blocks.forEach((block, index) => {
    const svgs = block.querySelectorAll(".plantuml-rendered svg");
    const box = svgs.length > 0 ? svgs[0].getBoundingClientRect() : null;
    const line = block.querySelector(".plantuml-error");
    const source = block.dataset.source || "";

    report.plantuml.push({
      index,
      block: all.indexOf(block),
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
export function collectRejectedPlantUML(root, all, report) {
  const blocks = [...root.querySelectorAll(".code-block[data-puml-error]")];

  report.plantumlRejected = blocks.map((block, index) => {
    const line = block.querySelector(".plantuml-error");
    const source = block.dataset.source || "";

    return {
      index,
      block: all.indexOf(block),
      head: source.split("\n")[0].trim(),
      svg: block.querySelectorAll("svg").length,
      error: line ? line.textContent.trim() : "",
    };
  });
}

// diagramBlocks は図のブロックの一覧を文書の順で返す（UT-815）。
//
// **属性の有無で「図のブロックらしいもの」をすべて拾う。** 目印の有無では絞らない
// ——絞るのは refs.js の役目であり、ここで絞ると照合を試さないまま通る。
export function diagramBlocks(root) {
  return [
    ...root.querySelectorAll(
      "div.code-block[data-mermaid], div.code-block[data-plantuml], div.code-block[data-puml-error], div.code-block[data-source]",
    ),
  ];
}

// 名前空間（BR-054 の「要素の名前空間をページから受け取る」）。
const XHTML_NS = "http://www.w3.org/1999/xhtml";
const SVG_NS = "http://www.w3.org/2000/svg";

// collectIds は本文の中の id を持つすべての要素の事実を集める（UT-813）。
//
// **除外をここで済ませない。** SVG の中の要素も、図の器も、そのまま返す。
// 判定（何を処理系の id とみなすか）は Go 側が行う。
export function collectIds(root, report) {
  for (const element of root.querySelectorAll("[id]")) {
    const holder = element.closest(".mermaid-rendered, .plantuml-rendered");
    const svg = element.closest("svg");

    let namespace = element.namespaceURI || "";
    if (namespace === XHTML_NS) namespace = "xhtml";
    else if (namespace === SVG_NS) namespace = "svg";

    let diagram = "";
    if (holder) diagram = holder.classList.contains("mermaid-rendered") ? "mermaid" : "plantuml";

    report.ids.elements.push({
      id: element.getAttribute("id"),
      namespace,
      inSvg: Boolean(holder && svg && holder.contains(svg)),
      container: element === holder,
      diagram,
    });
  }

  // 見出しの文字を文書の順で返す。**どれが Status かは Go 側が順番で決める。**
  report.ids.headings = [...root.querySelectorAll("h1, h2, h3, h4, h5, h6")]
    .filter((heading) => !heading.closest("svg"))
    .map((heading) => (heading.textContent || "").trim());
}

// collectSort は本番の parseNumber / compareCells を、Go が渡した入力で呼ぶ（UT-814）。
//
// tablesort は harness.js が動的に読んだモジュール（読めなければ null）。呼び出しの例外は report.errors へ貯める。
export function collectSort(tablesort, cases, report) {
  if (!tablesort) return;

  if (typeof tablesort.parseNumber !== "function" || typeof tablesort.compareCells !== "function") {
    report.sort.error = "tablesort.js に parseNumber / compareCells がない（IMP-229 が未実装）";
    return;
  }

  report.sort.imported = true;

  for (const item of cases) {
    const fn = item.fn === "parseNumber" ? tablesort.parseNumber : tablesort.compareCells;

    let value;
    try {
      value = fn(...item.args);
    } catch (error) {
      report.errors.push(item.fn + ": " + (error && error.message ? error.message : String(error)));
      value = undefined;
    }

    report.sort.results.push({
      fn: item.fn,
      args: item.args,
      type: typeof value,
      value: typeof value === "number" ? value : null,
    });
  }
}

// REF_KINDS は isOwnRef の種類（IMP-260）。どの要素にも 5 種類すべてで呼ぶ。
const REF_KINDS = ["task", "table", "cell", "mermaid", "plantuml"];

// collectRefs は本番の refs.js の照合結果を、要素の種類ごとに文書の順で集める（UT-815）。
//
// **どれが GFM の要素かはここでは決めない。** 由来は Go 側が出現順で持つ。refs は harness.js が動的に読んだ
// モジュール（読めなければ null）。
export function collectRefs(refs, root, blocks, report) {
  if (!refs) return;

  if (
    typeof refs.isOwnRef !== "function" ||
    typeof refs.ownLinkTarget !== "function" ||
    typeof refs.ownSource !== "function"
  ) {
    report.refs.error = "refs.js に isOwnRef / ownLinkTarget / ownSource がない（IMP-260 が未実装）";
    return;
  }

  report.refs.imported = true;

  const own = (element) =>
    Object.fromEntries(REF_KINDS.map((kind) => [kind, refs.isOwnRef(element, kind) === true]));
  const outsideSvg = (element) => !element.closest("svg");

  report.refs.checkboxes = [...root.querySelectorAll('input[type="checkbox"]')]
    .filter(outsideSvg)
    .map((input) => ({ own: own(input), sortButtons: 0, bodyRows: 0 }));

  report.refs.tables = [...root.querySelectorAll("table")].filter(outsideSvg).map((table) => ({
    own: own(table),
    sortButtons: table.querySelectorAll(".sort-btn").length,
    bodyRows: [...table.tBodies].reduce((count, body) => count + body.rows.length, 0),
  }));

  report.refs.cells = [...root.querySelectorAll("th, td")]
    .filter(outsideSvg)
    .map((cell) => ({ own: own(cell), sortButtons: 0, bodyRows: 0 }));

  report.refs.links = [...root.querySelectorAll("a")].filter(outsideSvg).map((anchor) => {
    const target = refs.ownLinkTarget(anchor);
    return { href: anchor.getAttribute("href") || "", target: typeof target === "string" ? target : null };
  });

  report.refs.blocks = blocks.map((block) => {
    const source = refs.ownSource(block);
    return {
      own: own(block),
      source: typeof source === "string" ? source : null,
      dataSource: block.getAttribute("data-source") || "",
      // 本番の lazy.js が描画した後に見る（IMP-230）。
      rendered: block.querySelector(".mermaid-rendered, .plantuml-rendered") !== null,
    };
  });
}

// settleImages は本文中の img がすべて決着するのを待つ。
//
// **固定時間で待たない**（UT-037 と同じ理由）。読み込みが終わった画像は
// complete が true になる。**置き換えられた画像は DOM から消えるため、
// 走査のたびに数え直す。**
export async function settleImages(root, deadlineMs = 5000) {
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
export function collectImages(root, report) {
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
export function collectMath(root, sources, report) {
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
