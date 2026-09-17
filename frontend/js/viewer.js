// viewer.js — 本文の挿入と後処理の順序（IMP-220, IMP-223）。
//
// **Go を経由しない文字列を innerHTML に渡さない**（IMP-220）。UI 文言の挿入は
// textContent を使う。doc.html は Go 側でサニタイズ済みであり（IMP-116）、
// フロントエンドで追加のサニタイズは行わない。
//
// **本文の飾り付け（Alerts のアイコン・見出しのアンカー・読み込みに失敗した画像。手順 4〜6）は
// decorate.js にある**（IMP-225, IMP-226, IMP-227）。ここは手順の順序と、リンク・スクロール・
// フォーカス・再描画で引き継ぐ状態を持つ。

import { state } from "./state.js";
import { S, warningText } from "./strings.js";
import { hideStateScreen, isAboutOpen, isEditorsOpen } from "./overlay.js";
import { isExpandOpen } from "./expand.js";
import { closeContextMenu } from "./contextmenu.js";
import { applyEditMode, cancelCellEdit, isEditableCell } from "./editmode.js";
import { renderOutline, observeHeadings, syncActive } from "./outline.js";
import { attachCopyButtons } from "./copy.js";
import { drawDiagrams, drawMath, startDrawing } from "./lazy.js";
import { attachImageButtons, captureMedia, numberImages, restoreMedia } from "./media.js";
import { leaveDocument } from "./docswitch.js";
import { isOwnRef } from "./refs.js";
import { closeSearch } from "./search.js";
import { decorateAlerts, decorateHeadings, markBrokenImages } from "./decorate.js";
import { attachSortButtons, captureSort, restoreSort } from "./tablesort.js";
import { updateStatus, showMessage } from "./status.js";
import { $, findInDocument, linkHref, openAncestorDetails } from "./util.js";

// **描画スモークは markBrokenImages を viewer.js から読む**（BR-054, UT-811）。viewer.js を動的に読むのは、
// その連結（viewer.js が import するモジュールのどれか）の失敗を 1 件の失敗として捉えるためであり、
// decorate.js を直接読むと viewer.js の連結の失敗が見えなくなる。
export { markBrokenImages };

let onFollow = null;

// initViewer は本文のイベントを配線する（IMP-211, IMP-223）。
//
// **#markdown に置く。** リンクごとに付けない。**拡大画面（#expand-view）にも置く**——
// 図を本文の外へ移すため、#markdown のリスナは届かない（IMP-253）。expand.js ではなくここに置くのは、
// リンクの既定の動作を止める規則を 1 か所にまとめるためである。
//
// **auxclick も受ける**（AR-060, BUG-016）。中ボタンなど左ボタン以外のクリックは click ではなく auxclick で届き、
// リンクをたどる処理はその既定の動作である（Chromium と WebKitGTK 2.46 以降）。click だけを受けていると、
// WebView2 では Go を通らずにブラウザへ回り、WebKitGTK ではウィンドウの中が遷移して戻れなくなる。
export function initViewer(deps) {
  onFollow = deps.onFollow;
  $("markdown").addEventListener("click", onLinkClick);
  $("expand-view").addEventListener("click", preventLinkDefault);
  for (const id of ["markdown", "expand-view"]) $(id).addEventListener("auxclick", preventLinkDefault);
}

// onLinkClick は本文中のリンクを捕まえる（IMP-223, FR-050, AR-060）。
//
// **href があれば必ず preventDefault する。** WebView 内でのページ遷移を
// 一切発生させない。判断は Go 側に置き、ここではスキームもパスも解釈しない
// （IMP-300, IMP-312）。
function onLinkClick(event) {
  const anchor = event.target.closest("a");
  if (!anchor) return; // 埋め込まれた画像のクリックでは何も起きない（FR-053）

  // 図の中のリンクも同じ経路を通す（FR-050, MD-081）。SVG の <a> は xlink:href に書かれる（BUG-015）
  const href = linkHref(anchor);
  if (!href) return;

  event.preventDefault();

  // **左ボタン以外では何もしない**（AR-060, BUG-016。利用者の判断）。auxclick を実装していない WebKitGTK
  // （2.46 より前）は、中ボタンでも click を出す。既定の動作は上で止めてある
  if (event.button !== 0) return;

  // **編集モードの間は、編集できるセルの中のリンクで遷移しない**（FR-142, FR-050 の例外）。ダブルクリックの
  // 1 回目で文書が切り替わり、編集を始める前に編集モードが終わるためである。鍵は isEditableCell が照合する
  if (isEditableCell(anchor)) return;

  // 同一文書内のアンカーだけはフロントエンドで処理する（IMP-223）。
  if (href.startsWith("#")) {
    scrollToAnchor(href.slice(1));
    return;
  }

  // それ以外は生値のまま Go へ渡す（IMP-312）。target="_blank" も同じ経路。
  if (onFollow) onFollow(href);
}

// preventLinkDefault はリンクの中のクリックの**既定の動作（WebView の中での遷移・新しいウィンドウ）だけを止め、
// 何もしない**（IMP-223, AR-060）。使うのは 2 つ——拡大画面へ移した図の中のリンクの click（UI-104, BUG-015。
// 拡大画面は図を見る画面であり、リンクをたどるときは拡大画面を閉じて本文で押す）と、本文と拡大画面の
// auxclick（中ボタンなど左ボタン以外のクリック。BUG-016）である。リンクでない所のクリックは止めない。
function preventLinkDefault(event) {
  const anchor = event.target instanceof Element ? event.target.closest("a") : null;
  if (anchor && linkHref(anchor)) event.preventDefault();
}

// scrollToAnchor は同一文書内の見出しへ移動する（FR-050）。
//
// **スムーススクロールを使わない**（FR-041）。
//
// **移動先は findInDocument で本文の中だけを探す**（IMP-223, AR-053）。v1.0.0 は
// document.getElementById をフラグメントにそのまま使っており、本文に無い id（`#overlay` など）が
// 画面の要素に当たった（BUG-011）。
export function scrollToAnchor(fragment) {
  const target = findInDocument(fragment);
  if (!target) {
    showMessage(S.errLinkNotFound(`#${fragment}`), "error");
    return;
  }

  // **移動先が折りたたみの中にあれば開いてから移動する**（MD-026, IMP-223）。
  openAncestorDetails(target);
  target.scrollIntoView();
  syncActive();
}

// renderDocument は DocumentDTO を画面へ反映する（IMP-220）。
//
// **処理順序を固定する。** 番号は IMP-220 の手順に対応する。
export function renderDocument(doc) {
  const viewer = $("viewer");
  const markdown = $("markdown");

  // scroll.mode === "keep" のために、差し替える前の位置を控える（IMP-321）。
  const previousTop = viewer.scrollTop;

  // 0a. 引き継ぐ状態を控える、または解除する（DSP-352）。
  //     **ここはまだ前の描画の鍵（state.doc.refKey）で照合する**——0b より前に置く。
  //     **右クリックメニューとセルの編集欄はどちらの場合も閉じる**（IMP-249, IMP-262。メニューが指していた
  //     リンクや選択範囲と、編集していたセルが無くなる）。**控えるより前に閉じる**——開く前のフォーカス
  //     （チェックボックス）へ戻してから、その順番を控える。
  closeContextMenu();
  cancelCellEdit();
  const kept = doc.sameDocument ? captureRedraw(markdown) : null;
  if (!doc.sameDocument) leaveDocument();

  // 0b. 状態を新しい文書へ移す（IMP-250）。以降の照合は新しい鍵で行う（IMP-260）。
  //     **この位置から動かさない**——F5 や更新検知では鍵が変わる（IMP-102）。
  state.doc = doc;
  // 文書を表示している間、状態画面の対象は持たない（IMP-210）。ステータスとツリーは state.doc を見る。
  state.target = null;

  // 0. 検索を閉じる（FR-080, IMP-241）。包んだ <mark> を解いてから
  //    差し替える。ここを飛ばすと、検索状態が前の文書の <mark> を
  //    指したままになる。**同じ文書の再描画でも閉じる**（DSP-352）。
  closeSearch();

  // 1. 一度に挿入する。分割挿入や逐次追加を行わない（AR-052）。
  markdown.innerHTML = doc.html;

  // 2. 状態画面を隠す。
  hideStateScreen();

  // 3. コピーボタンを付与する（IMP-221）。
  attachCopyButtons(markdown);

  // 4. GitHub Alerts のアイコンを付与する（IMP-225）。
  decorateAlerts(markdown);

  // 5. 見出しにアンカーを付与する（IMP-227, MD-020）。
  decorateHeadings(markdown);

  // 5a. 画像に番号を振る（IMP-228）。**6 より前**——6 で読み込みに失敗した画像が置き換わると数えられない。
  numberImages(markdown);

  // 6. 画像の読み込み失敗を捉える配線を行う（IMP-226, DSP-123）。
  //    **遅らせすぎない。** 配線までに読み込みが終わった画像は error を
  //    受け取れないため、markBrokenImages が既に失敗しているものを別途拾う。
  markBrokenImages(markdown);

  // 6a. 同じ文書なら、控えた <details> を同じ順番のものについて開き直す（FR-014, DSP-352）。
  //     **9 より前に行う**——開くと高さが変わり、先にスクロール位置を合わせると位置がずれる。
  if (kept) reopenDetails(markdown, kept.openDetails);

  // 6b. 控えた原寸表示を当て直しを待つ集合に置いてから、読み込みの済んだ画像を包んでボタンを置く
  //     （IMP-228, DSP-352）。図は描画が終わってから当て直す（手順 8 の onDiagramSettled）。
  restoreMedia(kept ? kept.media : null);
  attachImageButtons(markdown);

  // 6c. GFM の表に並べ替えのボタンを付け、控えた並べ替えを当て直す（IMP-229, DSP-352）。**新しい鍵で
  //     照合する**（0b の後）。書き込みの直後（trigger が edit）は行の並びを保つ（FR-130）。
  attachSortButtons(markdown);
  restoreSort(kept ? kept.sort : null, doc.trigger);

  // 6d. 編集モードの状態を画面へ写す（IMP-260）。**新しい鍵で照合する**（0b の後）。チェックボックスの
  //     有効化とセルの印は差し替えた本文に付け直す。6c の並べ替えのボタンより後に置く（セルの子を数えない）。
  applyEditMode(doc);

  // 7. スクロール連動の監視対象を作り直す（IMP-222）。
  observeHeadings(markdown, doc.headings);

  // 8. needsMermaid / needsKaTeX に応じて遅延ロードする（IMP-230, NFR-013）。
  //    **await しない。** 読み込みと描画で本文の表示をブロックしない（NFR-012）。
  //    **描画の世代を 1 つ進め、Mermaid と PlantUML に同じ番号を渡す**（IMP-230）。前の描画の図は、
  //    ここから先は描かれず、DOM にも書き込まれない。
  //    図を 1 つ描き終えるたびに media.js の onDiagramSettled が、両方を出し終えたら
  //    onAllDiagramsSettled が呼ばれる（IMP-230, IMP-228）。PlantUML は条件を付けずに描く（drawDiagrams）。
  drawDiagrams(markdown, doc.needsMermaid, startDrawing());
  if (doc.needsKaTeX) drawMath(markdown);

  // 9. スクロール位置を設定する（13 章 ScrollDTO）。
  applyScroll(viewer, doc.scroll, previousTop);

  // 10. アウトラインとステータスを更新する。
  renderOutline(doc.headings);
  // 9 で位置を飛ばしているため、監視の通知を待たずにここで合わせる（IMP-222）。
  syncActive();
  updateStatus();

  // 変換時の警告はステータスの一時メッセージで伝える（IMP-302, FR-110）。
  // 未知の Kind は無視される（IMP-290）。
  const warning = doc.warnings.map(warningText).find(Boolean);
  showMessage(warning, "warning");

  // 11. 本文ペインへフォーカスを移す（UI-051, IMP-220）。**ただし同じ文書で、控えたチェックボックスと
  //     同じ順番のものがあれば、そこへ移す**（FR-014, UI-051 の例外）。
  if (!focusKeptCheckbox(markdown, kept)) focusViewer();
}

// captureRedraw は、同じ文書の再描画（1.7）で引き継ぐ状態を差し替える前の DOM から控える
// （IMP-220 の手順 0a, DSP-352）。**state.doc を差し替える前に呼ぶ**（前の描画の鍵で照合する）。
//
//   openDetails  開いている <details> の順番（#markdown の中の出現順）
//   checkbox     フォーカスのあるチェックボックスの順番（鍵の合う task の目印を持つものの中で）。無ければ -1
//   sort         表の並べ替えの写し（IMP-229 の captureSort）。無ければ null
//   media        原寸表示の写し（IMP-228 の captureMedia）。無ければ null
//
// **チェックボックスの順番は目印の番号を読まずに数える。** 鍵を読むのは refs.js だけであり（IMP-260）、
// 鍵の合う task の要素を文書の順に並べたときの位置は、目印の番号（Go 側が文書の順に振る。IMP-120）と
// 一致する。
function captureRedraw(markdown) {
  const openDetails = [];
  markdown.querySelectorAll("details").forEach((details, index) => {
    if (details.open) openDetails.push(index);
  });

  const active = document.activeElement;
  const checkbox = markdown.contains(active) ? ownTaskCheckboxes(markdown).indexOf(active) : -1;

  return { openDetails, checkbox, sort: captureSort(), media: captureMedia() };
}

// reopenDetails は控えた順番の <details> を開き直す（IMP-220 の手順 6a, FR-014）。
//
// **順番が文書の中に無くなっていれば何もしない**（折りたたみを消す編集の後など）。
function reopenDetails(markdown, openDetails) {
  const all = markdown.querySelectorAll("details");
  for (const index of openDetails) {
    if (index < all.length) all[index].open = true;
  }
}

// focusKeptCheckbox は控えた順番のチェックボックスへフォーカスを戻し、戻したら true を返す
// （IMP-220 の手順 11, FR-014）。**ダイアログ・拡大画面の表示中は動かさない**（focusViewer と同じ）。
function focusKeptCheckbox(markdown, kept) {
  if (!kept || kept.checkbox < 0 || isFocusHeldElsewhere()) return false;

  const target = ownTaskCheckboxes(markdown)[kept.checkbox];
  if (!target) return false;

  target.focus({ preventScroll: true });
  return true;
}

// ownTaskCheckboxes は鍵の合うタスクのチェックボックスを文書の順に返す（IMP-260）。
function ownTaskCheckboxes(markdown) {
  return [...markdown.querySelectorAll('input[type="checkbox"]')].filter((input) => isOwnRef(input, "task"));
}

// focusViewer は本文ペインへフォーカスを移す（UI-051, IMP-220 の手順 11）。
//
// **キーボードでのスクロールは、スクロールする器がフォーカスを持って初めて
// 効く。** html と body は overflow: hidden であり、実際にスクロールするのは
// #viewer だけである。外にフォーカスがあると PageUp / PageDown / Home / End /
// 方向キーは届いていても動かせる器が無い。
//
// **preventScroll: true とする。** フォーカス移動に伴うスクロールが、直前の
// 手順 9（applyScroll。DSP-350）を打ち消しうる。F5 で位置が維持されず
// （FR-015）、Alt+← で復元されない（FR-051）。
//
// **ダイアログを表示している間は奪わない。** ダイアログは開いたままファイル
// 更新の自動検知（FR-014）を受けうる。背後の本文へフォーカスが移ると、
// フォーカストラップ（IMP-251, IMP-252）が破れる。
//
// **検索バーへの配慮は要らない。** 手順 0 の closeSearch が既に閉じており、
// 入力欄にフォーカスがあれば自分で本文へ戻している（IMP-241）。
//
// **本文ペインへフォーカスを移す箇所は、すべてこの関数を通す**（手順 11、IMP-244 の Esc による編集の
// 取り消し、IMP-253 の拡大画面を閉じたとき、IMP-260 / IMP-262 の編集欄を閉じたとき）。他のモジュール
// へは main.js が deps.focusViewer として渡す（viewer.js を import すると循環しうる。IMP-201）。
export function focusViewer() {
  if (isFocusHeldElsewhere()) return;

  $("viewer").focus({ preventScroll: true });
}

// isFocusHeldElsewhere は、本文へフォーカスを移してはならない画面（ダイアログ・拡大画面）が開いて
// いるかを返す（IMP-220 の手順 11 の条件 2）。
function isFocusHeldElsewhere() {
  return isAboutOpen() || isEditorsOpen() || isExpandOpen();
}

// applyScroll は ScrollDTO の mode に従って位置を決める（IMP-302）。
//
// **スムーススクロールを使わない。** アンカーへは即座に移動する（FR-041）。
function applyScroll(viewer, scroll, previousTop) {
  switch (scroll.mode) {
    case "anchor": {
      // 開いた直後のアンカー復元も、本文のリンクと同じ findInDocument を通す（IMP-223）。
      // scroll.anchor は Go 側が復号したフラグメントで、接頭辞を付けていない（IMP-302）。
      const target = scroll.anchor ? findInDocument(scroll.anchor) : null;
      if (target) {
        openAncestorDetails(target);
        target.scrollIntoView();
        return;
      }
      // 見出しが見つからない場合は先頭へ。位置を動かさないと、リンクを
      // 踏んだのに何も起きていないように見える。
      viewer.scrollTop = 0;
      return;
    }

    case "restore":
      viewer.scrollTop = scroll.top;
      return;

    case "keep":
      viewer.scrollTop = previousTop;
      return;

    default:
      viewer.scrollTop = 0;
  }
}
