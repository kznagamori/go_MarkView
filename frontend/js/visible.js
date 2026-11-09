// visible.js — 見える文字の数え方（IMP-266。FR-146, BR-054）。
//
// **Go 側の IMP-123（renderer.Visible / VisibleTexts）と同じ数え方にする。** どちらかを変えたら、
// もう一方と描画スモーク（BR-054。scripts/smoke の UT-817）を同じ変更で直す。
// **数え方を合わせるために、Go 側の結果を画面へ送らない**——両側が独立に数えて照合する（FR-146）。
//
// **自前の他のモジュールを import しない**（IMP-200）。描画スモークがこのファイルだけを直接読む。
// 画面が足す要素のクラス名（下の SKIP_CLASSES）は、そのためにここへ書き写している。
//
// **鍵を照合しない。** 対象のブロックを決めるのは呼び出し側（IMP-265 の refs.js の照合）である。
// 生 HTML で下の表の対象と同じ形を作ったもの（行の中の <input type="checkbox"> など）も表のとおりに
// 数え、Go 側と食い違ったブロックは修飾できないだけで済む（設計どおり。IMP-266）。
//
// 3 つの関数は、1 回の走査（scan）が作る「区切り」の並びを共有する。数え方を 3 か所に書くと、
// 照合（visibleText）と位置の変換（offsetOf / rangeAt）が食い違い、選んだ文字と違う文字を修飾する。

// PART は 1 文字と数える部品を表す文字（U+FFFC。IMP-123 と同じ文字。4.77.0）。
const PART = "\uFFFC";

// SKIP_CLASSES は画面が足した要素（要素ごと数えない。IMP-266）。見出しのアンカー（IMP-227）・
// 並べ替えのボタン（IMP-229）・図と画像のボタン（IMP-228）・コピーボタン（IMP-221）。
const SKIP_CLASSES = ["heading-anchor", "sort-btn", "media-actions", "copy-btn"];

// PART_CLASSES は要素全体で 1 文字と数える部品のクラス。数式（KaTeX が描いた中身を含む）と、
// 読み込みに失敗した画像（中の代替テキストを数えない。IMP-226）。
const PART_CLASSES = ["math-inline", "img-broken"];

// LI_BLOCK_TAGS はリストの項目の中で数えるのをやめるブロック要素（IMP-266 の li の行。IMP-123 の
// 「最初の子の TextBlock の中だけ」）。サニタイズ（IMP-116）が通すブロック要素を並べた。
const LI_BLOCK_TAGS = new Set([
  "P", "DIV", "BLOCKQUOTE", "PRE", "UL", "OL", "DL", "TABLE", "HR", "DETAILS", "FIGURE",
  "H1", "H2", "H3", "H4", "H5", "H6",
]);

const TEXT_NODE = 3;
const ELEMENT_NODE = 1;

// visibleText はブロックの見える文字（文字列）を返す。ブロックが無ければ空文字。
export function visibleText(block) {
  if (!block) return "";
  return scan(block).pieces.map((piece) => piece.chars.join("")).join("");
}

// offsetOf は DOM の位置（Range の端と同じ (node, offset)）を、見える文字の中の符号位置へ移す。
// ブロックの外・リストの項目の数えない部分（入れ子のリストなど）なら -1。
//
// edge は選択の端の向き（"start" | "end"。4.92.0）。**部品（数式・画像・脚注の参照）の中の位置は、始まりの端なら
// 部品の手前、終わりの端なら部品の後ろとする**——どちらの端でも部品を選択に含める（部品は 1 文字であり、中の位置に
// 当たる符号位置が無い。Go 側の IMP-124 の 3 の「部品の一部にかかっていたら全体へ広げる」と同じ向き）。数えない要素
// （見出しのアンカーなど）の中の位置は、その要素の手前の文字までを数えた位置になる。
export function offsetOf(block, node, offset, edge = "start") {
  if (!block || !node || !(block === node || block.contains(node))) return -1;

  let point;
  try {
    point = block.ownerDocument.createRange();
    point.setStart(node, offset);
  } catch {
    return -1; // offset が node の長さを超えるなど
  }

  const { pieces, stop } = scan(block);
  // 数えるのをやめたブロック要素の手前（その位置ちょうどは見える文字の終わり）より後ろは範囲の外。
  if (stop && point.comparePoint(stop.parentNode, indexOf(stop)) < 0) return -1;

  let count = 0;
  for (const piece of pieces) {
    if (piece.kind === "text") {
      if (piece.node === node) {
        const end = Math.min(Math.max(offset, piece.from), piece.to);
        count += codePoints(piece.node.data.slice(piece.from, end));
        break;
      }
      // 区切りの終わりが位置より前なら、区切りの全体が手前にある。
      if (point.comparePoint(piece.node, piece.to) < 0) {
        count += piece.chars.length;
        continue;
      }
      break;
    }

    // 部品: 終わり（要素の直後）が位置と同じか手前なら数える。中・手前なら、そこで止める。
    if (point.comparePoint(piece.node.parentNode, indexOf(piece.node) + 1) <= 0) {
      count += 1;
      continue;
    }
    // 位置が部品の中（始まり（要素の直前）が位置より手前）で、終わりの端なら、部品の後ろとする（部品を含める）
    if (edge === "end" && point.comparePoint(piece.node.parentNode, indexOf(piece.node)) < 0) count += 1;
    break;
  }
  return count;
}

// rangeAt は見える文字の符号位置 [start, end) を DOM の Range へ移す（再描画の後の当て直し。IMP-265）。
// 位置が整数でない・負・逆順・見える文字の長さを超えるなら null（呼び出し側は当て直さない）。
export function rangeAt(block, start, end) {
  if (!block || !Number.isInteger(start) || !Number.isInteger(end)) return null;
  if (start < 0 || end < start) return null;

  const { pieces, length } = scan(block);
  if (end > length) return null;

  // 区切りの境目の位置は、始まりなら後ろの区切りの頭に、終わりなら前の区切りの終わりに置く
  // （選んだ範囲の外の要素を Range に含めない）。空の範囲は両端を同じ点にする。
  const first = pointAt(block, pieces, start, false);
  const last = start === end ? first : pointAt(block, pieces, end, true);

  const range = block.ownerDocument.createRange();
  range.setStart(first.node, first.offset);
  range.setEnd(last.node, last.offset);
  return range;
}

// scan はブロックを走査し、見える文字の区切りの並びを返す。
//
//   pieces: { kind: "text", node, from, to, chars, start } … テキストノードの [from, to)（UTF-16 の位置）
//           { kind: "part", node, chars: [PART], start }    … 1 文字と数える部品の要素
//   length: 見える文字の数（符号位置）
//   stop:   リストの項目で数えるのをやめたブロック要素（無ければ null）
function scan(block) {
  const pieces = [];
  let length = 0;
  let stop = null;
  // チェックボックスの直後のテキストの先頭の半角空白 1 つを数えない（変換器が書く。構文木には無い。
  // IMP-120）。検索のハイライトが空白ごと包むこともあるため、兄弟に限らず、次に現れる文字で見る。
  let afterCheckbox = false;
  const isItem = block.tagName === "LI";

  const addText = (node, from) => {
    const to = node.data.length;
    if (from >= to) return;
    const chars = [...node.data.slice(from, to)]; // 符号位置で数える（サロゲートペアは 1 つ）
    pieces.push({ kind: "text", node, from, to, chars, start: length });
    length += chars.length;
  };

  const addPart = (node) => {
    pieces.push({ kind: "part", node, chars: [PART], start: length });
    length += 1;
  };

  const walk = (parent, top) => {
    for (const child of parent.childNodes) {
      if (stop) return;

      if (child.nodeType === TEXT_NODE) {
        if (child.data === "") continue;
        const from = afterCheckbox && child.data.startsWith(" ") ? 1 : 0;
        afterCheckbox = false;
        addText(child, from);
        continue;
      }
      if (child.nodeType !== ELEMENT_NODE) continue;

      if (isItem && top && LI_BLOCK_TAGS.has(child.tagName)) {
        stop = child;
        length -= trimBeforeBlock(pieces, child);
        return;
      }
      if (hasClass(child, SKIP_CLASSES)) {
        afterCheckbox = false;
        continue;
      }
      if (isPart(child)) {
        afterCheckbox = false;
        addPart(child);
        continue;
      }
      if (child.tagName === "INPUT") {
        // 入力欄は 0 文字。後ろの空白を落とすのはチェックボックスだけ。
        afterCheckbox = (child.getAttribute("type") || "").toLowerCase() === "checkbox";
        continue;
      }
      // 包むだけの要素（強調・リンク・検索の mark・修飾の仮の span・.cell-pending など）は中を数える。
      // br は中が無く 0 文字になる（変換器が <br> の後ろに書く改行が、次のテキストノードの 1 文字になる。
      // IMP-123）。
      walk(child, false);
    }
  };

  walk(block, true);
  return { pieces, length, stop };
}

// trimBeforeBlock は、リストの項目で数えるのをやめるブロック要素の手前を整え、減らした文字数を返す。
// **文字の後ろの改行 1 つを数えない**（変換器が書く。構文木には無い。IMP-123 の「入れ子のブロックを
// 持つリストの項目」。IMP-266）。変換器が書くのは改行 1 つだけであり、文字の後ろがインラインの要素の
// ときは改行だけのテキストノードになる（`<li><em>a</em>` + 改行 + `<ul>`）。IMP-266 の「空白だけの
// テキスト」は、それが同じ規則で消えることを指す。
function trimBeforeBlock(pieces, blockElement) {
  const previous = blockElement.previousSibling;
  const last = pieces[pieces.length - 1];
  if (!previous || previous.nodeType !== TEXT_NODE || !last || last.node !== previous) return 0;

  if (last.chars[last.chars.length - 1] === "\n") {
    last.chars.pop();
    last.to -= 1;
    if (last.chars.length === 0) pieces.pop();
    return 1;
  }
  return 0;
}

// isPart は要素全体で 1 文字と数える部品か（画像・数式・読み込みに失敗した画像・脚注の参照）。
function isPart(element) {
  if (element.tagName === "IMG") return true;
  if (hasClass(element, PART_CLASSES)) return true;
  // 脚注の参照は a.footnote-ref を含む sup の全体（中の番号を数えない）。
  return element.tagName === "SUP" && element.querySelector("a.footnote-ref") !== null;
}

function hasClass(element, names) {
  return names.some((name) => element.classList.contains(name));
}

// pointAt は見える文字の位置 k を DOM の点 { node, offset } へ移す。atEnd が真なら、境目では前の
// 区切りの終わりを選ぶ（偽なら後ろの区切りの頭）。
function pointAt(block, pieces, k, atEnd) {
  if (pieces.length === 0) return { node: block, offset: 0 };

  for (const piece of pieces) {
    const begin = piece.start;
    const finish = begin + piece.chars.length;
    const inside = atEnd ? k > begin && k <= finish : k >= begin && k < finish;
    if (!inside) continue;

    if (piece.kind === "text") {
      return { node: piece.node, offset: piece.from + piece.chars.slice(0, k - begin).join("").length };
    }
    return k === begin ? before(piece.node) : after(piece.node);
  }

  // 境目のうち、上で選べなかった端（終わりを指す始まり・頭を指す終わり）。
  const edge = k === 0 ? pieces[0] : pieces[pieces.length - 1];
  const atHead = k === 0;
  if (edge.kind === "text") return { node: edge.node, offset: atHead ? edge.from : edge.to };
  return atHead ? before(edge.node) : after(edge.node);
}

function before(node) {
  return { node: node.parentNode, offset: indexOf(node) };
}

function after(node) {
  return { node: node.parentNode, offset: indexOf(node) + 1 };
}

function indexOf(node) {
  return Array.prototype.indexOf.call(node.parentNode.childNodes, node);
}

function codePoints(text) {
  return [...text].length;
}
