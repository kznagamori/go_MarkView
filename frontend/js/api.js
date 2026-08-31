// api.js — Go バインディングの薄いラッパ（13 章）。
//
// Wails が生成する wailsjs/ を import するのはこのファイルだけとし、
// 他のモジュールはここを経由して Go を呼ぶ。
//
// **任意のパスを開く汎用 API を作らない**（IMP-300）。ファイルを開く経路は
// ダイアログ・ドロップ・引数・ツリー・リンク・履歴の 6 つに限る（IMP-192）。
// ドロップと引数は Go 側が受けるため、ここには現れない。

import * as App from "../wailsjs/go/main/App.js";
import { EventsOn } from "../wailsjs/runtime/runtime.js";

// Go から届くイベントの名前（IMP-320）。
export const EVENT = {
  documentOpened: "document:opened", // ドロップ・引数（FR-011）
  documentChanged: "document:changed", // 更新の検知（FR-014）
  documentRemoved: "document:removed", // 削除の検知（FR-014）
  treeRootChanged: "tree:root-changed", // ツリールートの移動（FR-030）
  error: "error", // 非同期処理のエラー（FR-110）
};

// --- 状態の取得 -------------------------------------------------------------

// getInitialState は起動直後の状態をまとめて取る（IMP-303）。
export function getInitialState() {
  return App.GetInitialState();
}

// getTreeRoot はツリールートの絶対パスを返す。未確定なら空文字（FR-030）。
export function getTreeRoot() {
  return App.GetTreeRoot();
}

// getAbout はアプリケーション情報を取る（IMP-306, FR-100, FR-101）。
export function getAbout() {
  return App.GetAbout();
}

// --- 文書を開く -------------------------------------------------------------
//
// いずれも OpenResultDTO を返す（IMP-308）。
//   { document, error: null } 成功
//   { document: null, error } 失敗。error.kind で表示先が決まる（IMP-315）
//   { document: null, error: null } 何も起きなかった。表示を変えない

// openFileDialog は OS 標準のファイル選択ダイアログを開く（FR-010）。
export function openFileDialog() {
  return App.OpenFileDialog();
}

// openFromTree はツリーで選ばれたファイルを開く（FR-033）。
// ツリールートは変わらない（FR-030）。
export function openFromTree(path) {
  return App.OpenFromTree(path);
}

// openConfirmed は確認画面の「Open anyway」を実行する（FR-016, IMP-314）。
// 直前に確認画面を出したパス以外は Go 側が拒否する。
export function openConfirmed(path) {
  return App.OpenConfirmed(path);
}

// reload は表示中のファイルを読み直す（FR-015）。
export function reload() {
  return App.Reload();
}

// historyBack / historyForward は表示履歴を移動する（FR-051）。
export function historyBack() {
  return App.HistoryBack();
}

export function historyForward() {
  return App.HistoryForward();
}

// followLink は本文中のリンクを処理する（FR-050, FR-053）。
//
// LinkResultDTO を返す（IMP-305）。kind が external のとき、Go 側は既に
// OS へ委譲済みであり、フロントエンドは何もしない。
export function followLink(href) {
  return App.FollowLink(href);
}

// --- ツリー -----------------------------------------------------------------

// readDir はディレクトリの直下を読む（FR-032, FR-035）。
// 空文字を渡すとツリールートを読む。再帰しない。
//
// **Go 側は error を返す**（IMP-310）。ここで OpenResultDTO と同じ形へ
// 揃え、呼び出し側が 2 通りの失敗の受け方を持たなくて済むようにする。
export async function readDir(path) {
  try {
    return { nodes: await App.ReadDir(path), error: null };
  } catch (e) {
    return { nodes: [], error: toErrorDTO(e, "") };
  }
}

// --- 記録と設定 -------------------------------------------------------------

// setScrollTop は現在のスクロール位置を履歴へ記録する（IMP-311）。
// **文書を離れる直前に 1 回だけ呼ぶ。** スクロールのたびには呼ばない。
export function setScrollTop(top) {
  return App.SetScrollTop(top);
}

// updateConfig は設定を更新する（UI-110, UI-114）。
// ウィンドウの大きさと最大化状態は含まない（IMP-194）。
export function updateConfig(patch) {
  return App.UpdateConfig(patch);
}

// quit はアプリケーションを終了する（UI-090）。
//
// Ctrl+Q の受け口。Alt+F4 と閉じるボタンは OS 側で処理されるため、
// ここを通らない。設定の保存はいずれの経路でも Go 側の終了処理が行う
// （IMP-194）。
export function quit() {
  return App.Quit();
}

// --- クリップボード ---------------------------------------------------------

// copyToClipboard はテキストをクリップボードへ書く（FR-061, AR-062）。
// 成功したら null、失敗したら ErrorDTO を返す。
export async function copyToClipboard(text) {
  try {
    await App.CopyToClipboard(text);
    return null;
  } catch (e) {
    return toErrorDTO(e, "clipboard");
  }
}

// --- イベント ---------------------------------------------------------------

// on はイベントを購読する。**起動時に 1 回だけ呼ぶ**（IMP-322）。
// 動的な購読・解除を繰り返さない。
export function on(name, handler) {
  EventsOn(name, handler);
}

// --- 内部 -------------------------------------------------------------------

// toErrorDTO は拒否された Promise を ErrorDTO の形（IMP-307）へ揃える。
//
// Wails v2 は Go の error をメッセージ文字列としてしか渡せないため、
// kind を推し量れない。**呼び出し側が確実に言えるときだけ kind を渡す。**
// 空のままなら strings.js が message をそのまま出す（IMP-315 のフォールバック）。
function toErrorDTO(e, kind) {
  return {
    kind,
    message: e && e.message ? e.message : String(e),
    path: "",
    size: 0,
    limit: 0,
  };
}
