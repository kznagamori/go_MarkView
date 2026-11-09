// mediabuttons.js — 図と画像の原寸表示・拡大画面のボタンを作る（IMP-228, UI-053, DSP-124）。
//
// media.js が 400 行の目安（IMP-011）を超えたため、ボタンの組み立てだけを分けた。**状態（原寸表示中の集合と
// 対象の一覧）は media.js が持ち、押されたときの処理は media.js が handlers で渡す。** このモジュールは
// 自前のモジュールのうち strings.js と util.js だけを import する（media.js の import の制約を崩さない。IMP-228）。

import { isOwnRef } from "./refs.js";
import { S } from "./strings.js";
import { $, icon } from "./util.js";

// 図の種類（IMP-231, IMP-233）。鍵の合うブロックだけを数える
const DIAGRAMS = ["mermaid", "plantuml"];

// diagramKindOf は鍵の合う図のブロックの種類を返す。合わなければ null（IMP-260）。
export function diagramKindOf(block) {
  return DIAGRAMS.find((kind) => isOwnRef(block, kind)) || null;
}

// diagramNumberOf は図のブロックの番号を返す（FR-120 の「同じ対象」）。
//
// **目印の番号を読まずに数える。** 鍵を読むのは refs.js だけであり（IMP-260）、鍵の合う同じ種類の
// ブロックを文書の順に並べたときの位置は、Go 側が文書の順に振る番号（描画に失敗したもの・取り込み
// 指令で拒んだものを含む。IMP-120）と一致する。
export function diagramNumberOf(block, kind) {
  const scope = $("markdown") || document;
  const blocks = [...scope.querySelectorAll(".code-block")].filter((candidate) => isOwnRef(candidate, kind));
  return blocks.indexOf(block);
}

// diagramKeyOf は図のブロックのボタンの器の鍵（IMP-228 の `mermaid:<n>` / `plantuml:<n>`）を返す。図でなければ null。
// コードブロックの編集のボタン（IMP-264）が、描き終える前の図に器を先に作るときにも同じ鍵を使う。
export function diagramKeyOf(block) {
  const kind = diagramKindOf(block);
  return kind ? `${kind}:${diagramNumberOf(block, kind)}` : null;
}

// diagramButtons は図のブロックのボタンを返す。無ければ作り、コピーボタンの前（左）に置く（DSP-124）。
//
// **描き直し（テーマの切り替え）ではブロックが残るため、ボタンを作り直さない。** フォーカスを失わない。
// handlers: { onActual(key), onExpand(key, button) }。
export function diagramButtons(block, key, handlers) {
  const existing = block.querySelector(":scope > .media-actions");
  if (existing && existing.dataset.mediaKey === key) {
    // **編集のボタンの器が先にあれば、それを使い、原寸表示と拡大画面のボタンを編集のボタンの右に足す**（IMP-264 が
    // 描画を待つ間の図にも編集のボタンを置く。4.76.0）
    if (!existing.querySelector(".media-btn-actual")) existing.append(...createButtons(key, "div", handlers).actions.children);
    return {
      actions: existing,
      actualButton: existing.querySelector(".media-btn-actual"),
      expandButton: existing.querySelector(".media-btn-expand"),
    };
  }
  if (existing) existing.remove();

  const buttons = createButtons(key, "div", handlers);
  block.insertBefore(buttons.actions, block.querySelector(":scope > .copy-btn"));
  return buttons;
}

// createButtons は原寸表示と拡大画面のボタンを作る（UI-053, DSP-124）。器は図なら div、画像なら span
// （段落の中に置くため）。右から「拡大画面」「原寸表示」の順に見え、Tab は左から巡る。
export function createButtons(key, tag, handlers) {
  const actions = document.createElement(tag);
  actions.className = "media-actions";
  actions.dataset.mediaKey = key;

  const actualButton = button("media-btn-actual", S.tipActualSize, "icon-actual-size", () => handlers.onActual(key));
  actualButton.setAttribute("aria-pressed", "false");
  const expandButton = button("media-btn-expand", S.tipExpand, "icon-expand", () => handlers.onExpand(key, expandButton));

  actions.append(actualButton, expandButton);
  return { actions, actualButton, expandButton };
}

// placeImageButtons は、画像が小さくボタンが画像に収まらないとき、ボタンを画像の右上の外側に、上端を画像に
// そろえて出す（DSP-124。4.81.0。画像ファイルの .ico など）。entry は media.js の画像の対象
// （{ outer, target, actions }）。ボタンの表示を決めた後に呼ぶ（隠れたボタンは幅を持たず、収まる側に入る）。
//
// **ボタンを通常の位置に置いて測る**——ボタンの寸法と余白を JavaScript に書き写さない。上端は画像の上端で
// 決める（行の高さより小さい画像は、器の上端より下に描かれる）。
export function placeImageButtons(entry) {
  const { actions, target, outer } = entry;
  actions.classList.remove("is-outside");

  const box = target.getBoundingClientRect();
  const row = actions.getBoundingClientRect();
  if (row.left >= box.left && row.bottom <= box.bottom) return;

  actions.style.setProperty("--media-actions-top", `${box.top - outer.getBoundingClientRect().top}px`);
  actions.classList.add("is-outside");

  // **外側に出す余地が無ければ内側のまま**（画像を含む要素の右端を越える。本文の幅に縮めた、縦に短い横長の
  // 画像で起こる）。外側に出すと本文ペインが横にはみ出す（UI-050）。親がインラインの要素（リンク）なら余地を
  // 測れないため、内側のまま
  if (actions.getBoundingClientRect().right > outer.parentElement.getBoundingClientRect().right + 0.5) {
    actions.classList.remove("is-outside");
  }
}

// button はツールチップと同じ読み上げ名を持つアイコンのボタンを作る（IMP-247, IMP-295）。
//
// **押下を本文へ伝えない**（preventDefault と stopPropagation。UI-053, IMP-223）。画像がリンクに
// 囲まれていても遷移させない。
function button(className, label, symbol, onClick) {
  const element = document.createElement("button");
  element.type = "button";
  element.className = `media-btn ${className}`;
  element.dataset.tip = label;
  element.setAttribute("aria-label", label);
  element.appendChild(icon(symbol));
  element.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
    onClick();
  });
  return element;
}
