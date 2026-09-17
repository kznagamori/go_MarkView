// navigate.js — 文書を開く経路と、結果の反映（IMP-250, IMP-308, IMP-305, IMP-311, IMP-315）。
//
// ダイアログ・ツリー・リンク・履歴・再読み込み・確認画面の Open anyway から Go を呼び、戻り値
// （OpenResultDTO / LinkResultDTO）とイベントの DocumentDTO / ErrorDTO を画面へ落とす。**状態画面を
// 出すのはこのモジュールの enterStateScreen だけとする**（IMP-250）。
//
// main.js（起動・購読・ショートカット・ダイアログの配線）が 400 行の目安（IMP-011）を超えたため分けた。
// **このモジュールは main.js を import しない**（main.js が配線のためにここを import する）。

import * as api from "./api.js";
import { leaveDocument } from "./docswitch.js";
import { loadTreeRoot, revealCurrent } from "./filetree.js";
import { showStateScreen } from "./overlay.js";
import { state } from "./state.js";
import { updateStatus, showMessage } from "./status.js";
import { errorText } from "./strings.js";
import { $ } from "./util.js";
import { renderDocument, scrollToAnchor } from "./viewer.js";

// ErrorDTO.Kind と状態画面の対応（IMP-315 の「表示先」）。
// ここにない Kind はステータス領域に 1 行で出す。
const STATE_SCREENS = {
  "needs-confirm": "confirm-large",
  "too-large": "too-large",
  "render-error": "render-error",
};

// isStateScreenError は、error が状態画面へ出す種別かを返す（IMP-315）。
export function isStateScreenError(error) {
  return Boolean(error) && Object.prototype.hasOwnProperty.call(STATE_SCREENS, error.kind);
}

// showDocument は本文を描画し、ツリーの選択と展開を合わせる（DSP-331）。
//
// **描画とツリーの追随を 1 か所にまとめる。** 経路ごとに呼び分けると、
// どれか 1 つで追随を書き忘れたときに気付けない。
export function showDocument(doc) {
  renderDocument(doc);
  revealCurrent();
}

// showError は ErrorDTO を表示先へ振り分ける（IMP-315）。
export function showError(error) {
  if (!error) return;

  const screen = STATE_SCREENS[error.kind];
  if (screen) {
    enterStateScreen(screen, error);
    return;
  }

  showMessage(errorText(error), "error");
}

// enterStateScreen は状態画面へ移る（IMP-250, DSP-302, FR-016）。
//
// **状態画面を出すのはここだけとする**（起動時・OpenResultDTO・LinkResultDTO・error イベント。
// IMP-250, IMP-320）。error イベントで届いた状態画面の種別も同じく扱う——Go 側は既に状態画面へ
// 移っており（IMP-192）、ステータスに出すだけでは画面が前の文書のまま残る。
//
// 順序は IMP-250 のとおり: leaveDocument → state.doc を空にして state.target を入れる → 状態画面を
// 出す → ステータスとツリーの強調を合わせる。**状態画面の間のステータスの左とツリーの強調は、画面の
// 対象（state.target）から決める**（DSP-302）。v1.0.0 は state.doc を残しており、前の文書のパスと
// 強調が残っていた（BUG-012）。
//
// **先頭で docswitch.js の leaveDocument を呼ぶ**——状態画面へ移ることは文書の切り替えであり
// （1.7, DSP-352）、renderDocument の手順 0a と同じ後始末を通す（IMP-250）。
export function enterStateScreen(kind, error) {
  leaveDocument();
  state.doc = null;
  state.target = screenTarget(kind, error);

  showStateScreen(kind, withConfirm(error));
  updateStatus();
  revealCurrent();
}

// openViaDialog はファイル選択ダイアログから開く（FR-010）。
export async function openViaDialog() {
  await recordScroll();
  handleResult(await api.openFileDialog());
}

// reloadCurrent は画面の対象を読み直す（FR-015）。
export async function reloadCurrent() {
  await recordScroll();
  handleResult(await api.reload());

  // **ツリーも読み直す**（FR-035 の 2 番目の契機。IMP-240）。
  //
  // Reload() は画面の対象を開き直すだけでツリーに触れない（IMP-310）ため、
  // ここで続けて呼ぶ。**文書の再読み込みが失敗しても行う**——FR-035 が契機と
  // 定めているのは「再読み込み操作を行ったとき」であり、その成否ではない。
  await loadTreeRoot(state.treeRoot);
}

// goBack / goForward は表示履歴をたどる（FR-051, UI-090）。
//
// ツールバーにボタンはなく（UI-020）、Alt+← / Alt+→ が唯一の経路である。
// 端に居るときは document も error も null が返り、表示は変わらない
// （IMP-308）。
export async function goBack() {
  await recordScroll();
  handleResult(await api.historyBack());
}

export async function goForward() {
  await recordScroll();
  handleResult(await api.historyForward());
}

// openFromTree はツリーで選ばれたファイルを開く（FR-033）。
// **ツリールートは変わらない**（FR-030）。判断は Go 側にある。
export async function openFromTree(path) {
  await recordScroll();
  handleResult(await api.openFromTree(path));
}

// followLink は本文中のリンクをたどる（FR-050, IMP-330）。
//
// 判断は Go 側にある。ここは結果を描画へ落とすだけとする。
export async function followLink(href) {
  await recordScroll();
  handleLink(await api.followLink(href));
}

// handleResult は OpenResultDTO を描画へ落とす（IMP-308）。
//
// document も error も null のときは何も起きなかったことを意味する。
// 表示を変えない（履歴の端、ダイアログの取り消し、確認待ちでない Open anyway など）。
function handleResult(result) {
  if (!result) return;

  if (result.error) {
    showError(result.error);
    return;
  }
  if (result.document) {
    showDocument(result.document);
  }
}

// handleLink は LinkResultDTO を種類ごとに振り分ける（IMP-305）。
function handleLink(result) {
  if (!result) return;

  switch (result.kind) {
    case "document":
      showDocument(result.document);
      return;

    case "anchor":
      scrollToAnchor(result.anchor);
      return;

    case "external":
      // Go 側が既に OS へ委譲済み。表示は変えない（FR-053）。
      return;

    default:
      // **状態画面にするのは、状態画面の種別（リンク先の Markdown が大きい・変換に失敗した）だけ**
      // （IMP-223, IMP-315）。OS への委譲に失敗した open-failed（許可されていないスキーム・既定の
      // アプリケーションが無い）は、ほかの種別と同じくステータスに出して本文を残す（FR-053）。
      // v1.0.0 は Go 側が委譲の失敗を render-error に写しており、状態画面が出て本文が消えた（BUG-013）。
      showError(result.error);
  }
}

// screenTarget は状態画面の対象を ErrorDTO から作る（IMP-210, IMP-307）。welcome では null。
//
// **表示用のパスは Go 側が決めた displayPath / outsideTree をそのまま使う。** ここで相対化しない
// （IMP-300 の 2）。
function screenTarget(kind, error) {
  if (kind === "welcome" || !error) return null;

  return {
    path: error.path || "",
    displayPath: error.displayPath || "",
    outsideTree: error.outsideTree === true,
  };
}

// withConfirm は確認画面のボタンに処理を結び付ける（FR-016, IMP-314）。
function withConfirm(error) {
  if (!error) return null;

  return Object.assign({}, error, { onConfirm: openConfirmed });
}

async function openConfirmed(path) {
  handleResult(await api.openConfirmed(path));
}

// recordScroll は現在のスクロール位置を履歴へ記録する（IMP-311）。
//
// **文書を離れる直前に 1 回だけ呼ぶ。** スクロールのたびには呼ばない。
// **docswitch.js の leaveDocument（画面の状態の後始末。IMP-250）と呼び分ける**——v1.0.0 はこの関数を
// main.js の leaveDocument と呼んでおり、同じ名前が 2 つあると呼ぶべきほうを取り違える。
async function recordScroll() {
  if (!state.doc) return;

  await api.setScrollTop($("viewer").scrollTop);
}
