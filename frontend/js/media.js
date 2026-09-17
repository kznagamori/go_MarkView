// media.js — 図と画像のボタンと原寸表示（FR-120, FR-121, IMP-228, UI-053, DSP-124）。
//
// **対象は、鍵の合う図のブロック（Mermaid / PlantUML）と、本文の画像である**（FR-120）。生 HTML で
// 書いた図のブロックは描画されず（IMP-230）、ここでも数えない（NFR-030, BUG-014）。
//
// **状態の鍵は種類と順番**（`mermaid:<n>` / `plantuml:<n>` / `image:<n>`。FR-120 の「同じ対象」）。
// 番号は描画の成否を見る前に決まる順番であり、1 つ壊れただけで後ろがすべて別の対象にならない。
//
// **viewer.js / overlay.js / main.js / navigate.js / docswitch.js / lazy.js / puml.js / decorate.js を
// import しない**（それぞれから呼ばれるため、循環する。IMP-250）。拡大画面（IMP-253）へは main.js が
// 渡す deps で知らせる。

import { isOwnRef } from "./refs.js";
import { S } from "./strings.js";
import { $, icon } from "./util.js";

// 画像の番号の属性（IMP-228 の手順 5a）。読み込みに失敗した画像の span へも写る（IMP-226）。
const INDEX_ATTR = "data-media-index";

// 図の種類と、描画した器のクラス（IMP-231, IMP-233）。
const DIAGRAM_KINDS = { mermaid: "mermaid-rendered", plantuml: "plantuml-rendered" };

// 縮小表示中の判定の許容（IMP-228）。小数の丸めで原寸の画像を縮小表示中と見なさない。
const TOLERANCE = 0.5;

// actual は原寸表示中の鍵の集合（Map<鍵, true>。IMP-228, IMP-210）。**設定に保存しない**（UI-111, NFR-042）。
const actual = new Map();

// pending は再描画の後に当て直しを待つ鍵の集合（restoreMedia が置く）。
let pending = new Map();

// entries は準備の済んだ対象（鍵ごと）。{ key, kind, frame, target, actualButton, expandButton }。
// 本文を差し替えると古い要素が残るため、使う前に frame.isConnected を見る。
const entries = new Map();

// 画像の数と、読み込みに失敗した画像の番号（numberImages が数え直す）。
let imageCount = 0;
const brokenImages = new Set();

// 縮小表示中の判定のやり直しを 1 フレームにまとめる（NFR-012）。
let frameRequest = 0;

const NO_DEPS = { onExpand() {}, onTargetSettled() {}, onImagesNumbered() {}, onAllDiagramsSettled() {} };
let deps = NO_DEPS;

// initMedia は依存を受け取り、縮小表示中の判定をやり直す契機を配線する（IMP-211, IMP-228）。
//
// deps: { onExpand, onTargetSettled, onImagesNumbered, onAllDiagramsSettled }（IMP-253 の同名。onExpand は
// openExpand）。**描画スモークは呼ばない**——その場合も、ほかの関数は何もしない依存で動く。
export function initMedia(given) {
  deps = Object.assign({}, NO_DEPS, given);

  const markdown = $("markdown");
  // ウィンドウとペインの幅の変化（FR-120）
  if (typeof ResizeObserver === "function") new ResizeObserver(scheduleUpdate).observe(markdown);
  // 閉じた <details> の中の要素は幅を持たないことがある。toggle は泡立たないため捕捉で受ける
  markdown.addEventListener("toggle", scheduleUpdate, true);
}

// numberImages は本文の画像に番号を振る（renderDocument の手順 5a）。
//
// **読み込みの成否を見る前に振る**（FR-120）。手順 6（IMP-226）で失敗した画像が span へ置き換わると、
// 数えられなくなる。
export function numberImages(root) {
  for (const key of [...entries.keys()]) {
    if (key.startsWith("image:")) entries.delete(key);
  }
  brokenImages.clear();

  const images = [...root.querySelectorAll("img")];
  images.forEach((img, index) => img.setAttribute(INDEX_ATTR, String(index)));
  imageCount = images.length;

  dropMissingImages(pending);
  deps.onImagesNumbered(imageCount);
}

// onImageBroken は IMP-226 が img を span.img-broken へ置き換えた直後に呼ばれる。**失敗した画像は対象に
// しない**（FR-120）。待っていた原寸表示を捨て、拡大画面へ失敗を知らせる。
export function onImageBroken(span) {
  const number = indexOf(span);
  if (number === null) return;

  brokenImages.add(number);
  const key = `image:${number}`;
  pending.delete(key);
  actual.delete(key);
  entries.delete(key);
  deps.onTargetSettled(key, null);
}

// attachImageButtons は読み込みに成功した画像を包み、ボタンを置く（renderDocument の手順 6b）。
//
// **restoreMedia の後に呼ぶ。** 読み込みが済んでいる画像はここで包み、その場で原寸表示を当て直す。
// 成功の判定は load か、配線時の `complete && naturalWidth > 0`（IMP-226 と同じく、済んだものを拾う）。
export function attachImageButtons(root) {
  for (const img of root.querySelectorAll(`img[${INDEX_ATTR}]`)) {
    if (img.complete && img.naturalWidth > 0) {
      wrapImage(img);
    } else if (!img.complete) {
      img.addEventListener("load", () => wrapImage(img), { once: true });
    }
  }
}

// onDiagramSettled は図の描画が 1 つ終わるたびに呼ばれる（成功・失敗・描画しなかった。IMP-230）。
//
// **テーマの切り替えで描き直したときも通り、原寸表示を当て直す**（FR-121, DSP-370）。
export function onDiagramSettled(block) {
  // 前の描画の遅れた知らせ（IMP-230 の描画の世代と二重の防御）
  if (!block || !block.isConnected) return;

  const kind = diagramKind(block);
  if (!kind) return;

  const key = `${kind}:${diagramNumber(block, kind)}`;
  const frame = block.querySelector(`:scope > .${DIAGRAM_KINDS[kind]}`);
  const target = frame ? frame.querySelector(":scope > svg") : null;

  if (!target) {
    // 描画に失敗した図・描画しなかった図は対象にしない（FR-120）
    const stale = block.querySelector(":scope > .media-actions");
    if (stale) stale.remove();
    entries.delete(key);
    pending.delete(key);
    actual.delete(key);
    deps.onTargetSettled(key, null);
    return;
  }

  frame.classList.add("media-frame");
  const buttons = diagramButtons(block, key);
  settle({ key, kind, frame, target, ...buttons });
}

// onAllDiagramsSettled はその文書の図の知らせを出し終えたときに呼ばれ、**待つ集合に残った図の鍵を捨てる**
// （IMP-230, IMP-228）。残すと、次の再描画で図の数が戻ったときに、戻していない原寸表示が当たる。
export function onAllDiagramsSettled() {
  for (const key of [...pending.keys()]) {
    if (!key.startsWith("image:")) pending.delete(key);
  }
  deps.onAllDiagramsSettled();
}

// captureMedia は原寸表示の写しを返す（手順 0a。同じ文書の再描画のときだけ呼ぶ）。無ければ null。
//
// **当て直しを待っている鍵も含める。** 図の描画を待つ間に次の再描画が来た（外部エディタで保存を
// 繰り返した。UC-03）とき、まだ当て直していない原寸表示を落とさないためである。
export function captureMedia() {
  const snapshot = new Map([...actual, ...pending]);
  return snapshot.size > 0 ? snapshot : null;
}

// restoreMedia は写しを当て直しを待つ集合として置き、原寸表示中の集合を空にする（手順 6b）。
//
// **attachImageButtons より前に呼ぶ。** 以後、対象の準備ができた時点（画像は包んだ時点、図は
// onDiagramSettled）で当て直す。画像の数（手順 5a）を超える番号と、既に読み込みに失敗した画像
// （手順 6）の鍵は、ここで捨てる。
export function restoreMedia(snapshot) {
  actual.clear();
  pending = new Map(snapshot || []);
  dropMissingImages(pending);
}

// clearMedia は原寸表示の状態を空にする（文書の切り替えと状態画面への移行。IMP-250 の leaveDocument）。
export function clearMedia() {
  actual.clear();
  pending.clear();
  entries.clear();
  brokenImages.clear();
  imageCount = 0;
}

// naturalSize は対象の本来の大きさ（論理ピクセル。{ width, height }）を返す（IMP-228, IMP-253）。
//
// img は naturalWidth / naturalHeight。svg は width / height 属性が px の数値ならその値、そうでなければ
// viewBox の幅と高さ（Mermaid は `width="100%"` と `max-width`、PlantUML は数値を出す）。**getBBox を
// 使わない**——描画の内容の外接矩形であり、図の大きさではない。
export function naturalSize(target) {
  if (target instanceof HTMLImageElement) return { width: target.naturalWidth, height: target.naturalHeight };

  const box = target.viewBox && target.viewBox.baseVal;
  const length = (name, fallback) => {
    const value = (target.getAttribute(name) || "").trim();
    return /^\d+(\.\d+)?(px)?$/.test(value) ? parseFloat(value) : fallback;
  };
  return { width: length("width", box ? box.width : 0), height: length("height", box ? box.height : 0) };
}

// isReduced は対象が縮小表示中か（本来の幅が本文での表示幅を超えているか）を返す（FR-120）。
function isReduced(target) {
  return naturalSize(target).width > target.getBoundingClientRect().width + TOLERANCE;
}

// wrapImage は読み込みに成功した画像を `span.media-image > (span.media-frame > img) + span.media-actions` に
// 包む（IMP-228）。**ボタンをスクロールする器（media-frame）の中に置かない**——中の絶対配置は内容と一緒に
// 流れる（DSP-124）。
function wrapImage(img) {
  if (!img.isConnected || img.closest(".media-frame")) return;

  const number = indexOf(img);
  if (number === null) return;
  const key = `image:${number}`;

  const outer = document.createElement("span");
  outer.className = "media-image";
  const frame = document.createElement("span");
  frame.className = "media-frame";
  const buttons = createButtons(key, "span");

  img.replaceWith(outer);
  frame.appendChild(img);
  outer.append(frame, buttons.actions);

  settle({ key, kind: "image", frame, target: img, ...buttons });
}

// settle は準備の済んだ対象を登録し、待っていた原寸表示を当て、ボタンの表示を決め、拡大画面へ知らせる。
function settle(entry) {
  entries.set(entry.key, entry);

  if (pending.has(entry.key)) {
    pending.delete(entry.key);
    actual.set(entry.key, true);
  }
  setActual(entry, actual.has(entry.key));
  updateButtons(entry);

  deps.onTargetSettled(entry.key, entry.target);
}

// diagramButtons は図のブロックのボタンを返す。無ければ作り、コピーボタンの前（左）に置く（DSP-124）。
//
// **描き直し（テーマの切り替え）ではブロックが残るため、ボタンを作り直さない。** フォーカスを失わない。
function diagramButtons(block, key) {
  const existing = block.querySelector(":scope > .media-actions");
  if (existing && existing.dataset.mediaKey === key) {
    return {
      actions: existing,
      actualButton: existing.querySelector(".media-btn-actual"),
      expandButton: existing.querySelector(".media-btn-expand"),
    };
  }
  if (existing) existing.remove();

  const buttons = createButtons(key, "div");
  block.insertBefore(buttons.actions, block.querySelector(":scope > .copy-btn"));
  return buttons;
}

// createButtons は原寸表示と拡大画面のボタンを作る（UI-053, DSP-124）。器は図なら div、画像なら span
// （段落の中に置くため）。右から「拡大画面」「原寸表示」の順に見え、Tab は左から巡る。
function createButtons(key, tag) {
  const actions = document.createElement(tag);
  actions.className = "media-actions";
  actions.dataset.mediaKey = key;

  const actualButton = button("media-btn-actual", S.tipActualSize, "icon-actual-size", () => toggleActual(key));
  actualButton.setAttribute("aria-pressed", "false");
  const expandButton = button("media-btn-expand", S.tipExpand, "icon-expand", () => {
    const entry = liveEntry(key);
    if (entry) deps.onExpand(entry.target, key, expandButton);
  });

  actions.append(actualButton, expandButton);
  return { actions, actualButton, expandButton };
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

// toggleActual は対象を縮小表示と原寸表示の間で切り替える（FR-121）。
//
// **切り替えの前後で、押したボタンの画面上の位置を保つ**（DSP-350）。切り替えは要素の高さを変え、
// 後ろの本文を押し下げる。
function toggleActual(key) {
  const entry = liveEntry(key);
  if (!entry) return;

  const before = entry.actualButton.getBoundingClientRect().top;
  const on = !actual.has(key);
  if (on) actual.set(key, true);
  else actual.delete(key);

  setActual(entry, on);
  updateButtons(entry);

  const viewer = $("viewer");
  if (viewer && !entry.actualButton.hidden) {
    viewer.scrollTop += entry.actualButton.getBoundingClientRect().top - before;
  }
}

// setActual は原寸表示の見た目を写す（IMP-228, DSP-124）。**幅と横スクロールは CSS が与える。**
//
// **Mermaid の SVG のインラインの max-width は、原寸表示に入るときに控えて外し、戻すときに戻す。**
// 控えずに消すと、縮小表示へ戻したときに図が本文幅を超える。
function setActual(entry, on) {
  const { frame, target } = entry;

  if (on) {
    if (entry.savedMaxWidth === undefined && target.style && target.style.maxWidth !== "") {
      entry.savedMaxWidth = target.style.maxWidth;
      target.style.removeProperty("max-width");
    }
    frame.style.setProperty("--media-natural-width", `${naturalSize(target).width}px`);
    frame.classList.add("is-actual");
  } else {
    frame.classList.remove("is-actual");
    frame.style.removeProperty("--media-natural-width");
    if (entry.savedMaxWidth !== undefined) {
      target.style.maxWidth = entry.savedMaxWidth;
      entry.savedMaxWidth = undefined;
    }
  }

  entry.actualButton.setAttribute("aria-pressed", String(on));
}

// updateButtons はボタンの表示を決める（FR-120 の表。hidden で切り替える。IMP-202）。
//
// 原寸表示のボタンは「縮小表示中 **または** 原寸表示中」。拡大画面のボタンは、図なら常に、画像なら
// 原寸表示のボタンと同じ条件。
function updateButtons(entry) {
  const shown = actual.has(entry.key) || isReduced(entry.target);
  entry.actualButton.hidden = !shown;
  entry.expandButton.hidden = entry.kind === "image" ? !shown : false;
}

// scheduleUpdate は縮小表示中の判定のやり直しを次のフレームへまとめる（IMP-228, NFR-012）。**拡大画面へ
// 移している対象（器の中に無い）は判定しない**——閉じたときにフォーカスを戻すボタンが隠れうる（IMP-253）。
function scheduleUpdate() {
  if (frameRequest) return;

  frameRequest = requestAnimationFrame(() => {
    frameRequest = 0;
    for (const [key, entry] of [...entries]) {
      if (!entry.frame.isConnected) entries.delete(key);
      else if (entry.frame.contains(entry.target)) updateButtons(entry);
    }
  });
}

// liveEntry は DOM に残っている対象を返す。本文が差し替わって外れていれば null。
function liveEntry(key) {
  const entry = entries.get(key);
  return entry && entry.frame.isConnected ? entry : null;
}

// diagramKind は鍵の合う図のブロックの種類を返す。合わなければ null（IMP-260）。
function diagramKind(block) {
  for (const kind of Object.keys(DIAGRAM_KINDS)) {
    if (isOwnRef(block, kind)) return kind;
  }
  return null;
}

// diagramNumber は図のブロックの番号を返す（FR-120 の「同じ対象」）。
//
// **目印の番号を読まずに数える。** 鍵を読むのは refs.js だけであり（IMP-260）、鍵の合う同じ種類の
// ブロックを文書の順に並べたときの位置は、Go 側が文書の順に振る番号（描画に失敗したもの・取り込み
// 指令で拒んだものを含む。IMP-120）と一致する。
function diagramNumber(block, kind) {
  const scope = $("markdown") || document;
  const blocks = [...scope.querySelectorAll(".code-block")].filter((candidate) => isOwnRef(candidate, kind));
  return blocks.indexOf(block);
}

// indexOf は画像（または置き換えた span）の番号を返す。無ければ null。
function indexOf(element) {
  const value = element.getAttribute(INDEX_ATTR);
  return value !== null && /^\d+$/.test(value) ? Number(value) : null;
}

// dropMissingImages は、画像の数を超える番号と、読み込みに失敗した画像の鍵を捨てる（IMP-228）。
function dropMissingImages(keys) {
  for (const key of [...keys.keys()]) {
    const match = /^image:(\d+)$/.exec(key);
    if (!match) continue;

    const number = Number(match[1]);
    if (number >= imageCount || brokenImages.has(number)) keys.delete(key);
  }
}
