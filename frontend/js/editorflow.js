// editorflow.js — エディタで開く流れ（FR-090, FR-091, FR-036, UI-103, IMP-252, IMP-270, IMP-331）。
//
// **Go を呼ぶのはここで、overlay.js / editors.js は受け取った値を描くだけにする**（IMP-201, IMP-252）。main.js が
// 400 行の目安（IMP-011）を超えたため、エディタ選択ウィンドウを開く・選ぶ・起動する流れを分けた（4.92.0）。
// 入口は 2 つ——ツールバー・Ctrl+E（showEditorDialog）と、ファイルツリーの Open in editor...（openEditorsForTree）。
// **どちらも同じ showEditors で描く**（IMP-252）。
//
// **main.js / viewer.js を import しない**（main.js がこのモジュールを配線する）。一覧と起動の失敗は navigate.js の
// showError で出す（main.js にあったときと同じ。状態画面の種別でなければステータス領域へ）。

import * as api from "./api.js";
import { S } from "./strings.js";
import { canEdit } from "./toolbar.js";
import { isExpandOpen } from "./expand.js";
import { showEditors } from "./overlay.js";
import { closeTreeFilter } from "./treefilter.js";
import { showError } from "./navigate.js";
import { showMessage } from "./status.js";

// showEditorDialog はエディタ選択ウィンドウを開く（FR-091, UI-103）。ツールバーのボタンと Ctrl+E の入口。
//
// **押せるかどうかの判定もここ 1 か所に置く。** ボタンは淡色で防げるが（UI-021）、Ctrl+E はそれだけでは止まらない。
// **押すたびに一覧を取り直す**（IMP-310, NFR-013）。開くのは、Go が開いた時点で控えた対象（editorTarget）である。
export async function showEditorDialog() {
  if (!canEdit()) return;

  present(await api.listEditors(), null);
}

// openEditorsForTree はファイルツリーの Open in editor... の入口（FR-036, IMP-270）。Go がツリーの検査（IMP-135）を
// 通したファイルを控える。opener は閉じたときにフォーカスを戻す要素（ツリーの項目。IMP-252）。
export async function openEditorsForTree(path, opener) {
  present(await api.listEditorsForTree(path), opener);
}

// present は一覧を描く（IMP-252）。
//
// **一覧すら作れなかったときはウィンドウを出さない**（理由はステータス領域へ。IMP-315）。**一覧が空なら出さない**
// ——対象が文字のファイルでない（ツールバーから画像を開いたとき。IMP-310）か、ツリーの検査を通らなかった（IMP-196）。
// **応答を待つ間に拡大画面が開いていれば出さない**（IMP-252。拡大画面の下に隠れる）。フィルターの枚とは同時に
// 開かない（IMP-271）。
function present(list, opener) {
  if (list && list.error) {
    showError(list.error);
    return;
  }
  if (!list || !Array.isArray(list.editors) || list.editors.length === 0) return;
  if (isExpandOpen()) return;

  closeTreeFilter(false);
  showEditors(list, opener);
}

// browseEditor は実行ファイルを選ぶダイアログを開き、新しい一覧を返す（FR-091, IMP-310）。**描き直すのは
// overlay.js の役目**である（IMP-252）。作れなかったときは null を返し、いま出ている一覧を保つ。
export async function browseEditor() {
  const list = await api.browseEditor();

  if (list && list.error) {
    showError(list.error);
    return null;
  }

  return list;
}

// openInEditor は選ばれたエディタで開き、結果をステータス領域へ出す（FR-090, DSP-151, IMP-331）。
//
// **成功したときも出す。** エディタが背面のウィンドウで開くことがあり、何も出ないと「押しても無反応」に
// 見える（FR-090, UI-060）。
export async function openInEditor(id) {
  const result = await api.openInEditor(id);
  if (!result) return;

  if (result.error) {
    showError(result.error);
    return;
  }

  showMessage(S.statusEditor(result.name), "info");
}

// closeEditorDialog は、選択ウィンドウを Open 以外（Cancel・Esc・暗幕のクリック）で閉じたことを Go へ知らせる
// （IMP-252, IMP-310）。Go は控えた対象と確定前の候補を捨てる。Open で閉じたときは呼ばない（OpenInEditor が捨てる）。
export function closeEditorDialog() {
  api.closeEditor();
}
