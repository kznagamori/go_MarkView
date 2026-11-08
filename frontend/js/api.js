// api.js — Go バインディングの薄いラッパ（13 章）。
//
// **Go を呼ぶために** wailsjs/ を import するのはこのファイルだけとし、
// 他のモジュールはここを経由する。唯一の例外は dnd.js で、あちらは Go を
// 呼ぶのではなく **Wails のランタイム側の drop リスナを取り付ける**ために
// OnFileDrop を取り込む（IMP-245）。
//
// **任意のパスを開く汎用 API を作らない**（IMP-300）。ファイルを開く経路は
// ダイアログ・ドロップ・引数・ツリー・リンク・履歴の 6 つに限る（IMP-192）。
// ドロップと引数は Go 側が受けるため、ここには現れない。

import * as App from "../wailsjs/go/desktop/App.js";
import { EventsOn } from "../wailsjs/runtime/runtime.js";

// Go から届くイベントの名前（IMP-320）。
export const EVENT = {
  documentOpened: "document:opened", // ドロップ・引数（FR-011）
  documentChanged: "document:changed", // 更新の検知（FR-014）
  documentRemoved: "document:removed", // 削除の検知（FR-014）
  treeRootChanged: "tree:root-changed", // ツリールートの移動（FR-030）
  historyChanged: "history:changed", // 戻れるか・進めるか（HistoryDTO。FR-051, IMP-323。4.75.0）
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

// --- エディタ ---------------------------------------------------------------
//
// **いずれもパスを引数に取らない**（IMP-300 の 3）。開く対象は Go 側の
// App.target（IMP-190）であり、エディタの実行ファイルのパスも Go 側だけが
// 持つ（NFR-035 の 3）。フロントエンドを通るのは識別子だけである。

// listEditors は選択ウィンドウの一覧を取る（IMP-310, FR-091）。
//
// **押すたびに呼ぶ。** 実行中にエディタがインストール・アンインストール
// されうるため、起動時に先読みしない（NFR-013, IMP-331）。
export function listEditors() {
  return App.ListEditors();
}

// browseEditor は実行ファイルを選ぶダイアログを開き、一覧を返し直す
// （IMP-310）。**選んだだけでは何も保存しない**（UI-116）。
export function browseEditor() {
  return App.BrowseEditor();
}

// openInEditor は選ばれたエディタで対象のファイルを開く（FR-090, IMP-310）。
//
// id はプリセットの ID、または "custom"。EditorResultDTO を返す（IMP-309）。
// **開くのは、選択ウィンドウを開いた時点で Go 側が控えたファイル（editorTarget）である**（IMP-310）。
export function openInEditor(id) {
  return App.OpenInEditor(id);
}

// listEditorsForTree はツリーで選んだファイルについて選択ウィンドウの一覧を取る（FR-036, FR-091）。
// Go 側が検査（IMP-135）を通ったファイルを控える。文字のファイルでなければ一覧は空。
export function listEditorsForTree(path) {
  return App.ListEditorsForTree(path);
}

// closeEditor は選択ウィンドウを開いたまま閉じたことを知らせる（Cancel / Esc / 外のクリック。FR-091）。
// Go 側は控えと確定前の候補を捨てる。通知しない。
export function closeEditor() {
  return App.CloseEditor();
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

// ツリーの操作（FR-036, UI-032。4.75.0）。いずれも TreeActionDTO を返す（IMP-317）。
// **パスは Go 側が検査（IMP-135）を通してから使う**（IMP-196）。ツリールートを変えた結果は戻り値で届き、
// tree:root-changed は送られない（IMP-317）。

// setTreeRoot はディレクトリをツリールートにする。
export function setTreeRoot(path) {
  return App.SetTreeRoot(path);
}

// setTreeRootUp はツリールートの親をツリールートにする（パスを渡さない）。
export function setTreeRootUp() {
  return App.SetTreeRootUp();
}

// canSetTreeRootUp はツリールートが根でないか（親へ上がれるか）を返す（UI-032）。
// フロントエンドはパスから根かどうかを判断しない（IMP-300 の 2）。
export function canSetTreeRootUp() {
  return App.CanSetTreeRootUp();
}

// openInFileManager はディレクトリをファイルマネージャで開く（NFR-035）。
export function openInFileManager(path) {
  return App.OpenInFileManager(path);
}

// openContainingFolder はファイルのあるディレクトリをファイルマネージャで開く（NFR-035）。
export function openContainingFolder(path) {
  return App.OpenContainingFolder(path);
}

// copyTreePath はパスをクリップボードへ書く。成功は知らせない。
export function copyTreePath(path) {
  return App.CopyTreePath(path);
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

// readClipboard はクリップボードのテキストを読む（FR-063, AR-062, IMP-310）。
// `{ text, error }` を返す。**テキスト以外しか入っていなければ空文字で、失敗ではない**（Go 側が判定する）。
// 失敗したら error は kind が paste の ErrorDTO（IMP-315）。
export async function readClipboard() {
  try {
    return { text: await App.ReadClipboard(), error: null };
  } catch (e) {
    return { text: "", error: toErrorDTO(e, "paste") };
  }
}

// --- 編集モード -------------------------------------------------------------
//
// 戻り値は EditModeDTO / EditResultDTO / CellSourceDTO（IMP-316）。**失敗も DTO で届く**（Go の error を
// 返さない。IMP-310）。**表示の更新は戻り値ではなく document:changed で届く**（AR-061）。

// setEditMode は編集モードを始める・終える（FR-140）。
export function setEditMode(on) {
  return App.SetEditMode(on);
}

// setTask はチェックボックスを書き換える（FR-141）。ref は data-ref の値そのもの（IMP-316）。
export function setTask(ref, checked) {
  return App.SetTask(ref, checked);
}

// getCellSource はセルの編集欄に入れるソースを取る（FR-142）。
export function getCellSource(ref) {
  return App.GetCellSource(ref);
}

// setCell はセルを書き換える（FR-142）。
export function setCell(ref, text) {
  return App.SetCell(ref, text);
}

// undoEdit は直前の書き換えを取り消す（FR-144）。
export function undoEdit() {
  return App.UndoEdit();
}

// redoEdit は取り消した書き換えをやり直す（FR-144）。
export function redoEdit() {
  return App.RedoEdit();
}

// getCodeSource はコードブロックと図の編集欄に入れるソースを取る（FR-145。CodeSourceDTO。4.75.0）。
export function getCodeSource(ref) {
  return App.GetCodeSource(ref);
}

// setCode はコードブロックと図を書き換える（FR-145）。
export function setCode(ref, text) {
  return App.SetCode(ref, text);
}

// applyFormat は選んだ文字に取り消し線・文字色を付ける・外す（FR-146, FR-147。4.75.0）。
// start / end は見える文字（visible.js）の中の符号位置、visible はそのブロックの見える文字の全体、
// style は FormatStyleDTO（{ strike: true } か { color: name }。IMP-316）。
export function applyFormat(ref, start, end, visible, style) {
  return App.ApplyFormat(ref, start, end, visible, style);
}

// setEditBoxOpen は編集欄が開いたか・閉じたかを知らせる（FR-140。4.75.0）。**ドロップを拒むためにだけ
// ある**（IMP-313）。**前の呼び出しの応答を待ってから次を送る**（到着順が入れ替わりうる。IMP-310）。
export function setEditBoxOpen(open) {
  return App.SetEditBoxOpen(open);
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
