// docswitch.js — 文書の切り替えと状態画面への移行の後始末（IMP-250, IMP-220 の手順 0a, DSP-352）。
//
// **状態画面へ移ることは文書の切り替えである**（1.7, DSP-352）。本文を差し替える（または空にする）
// 前に、前の文書に結び付いた画面の状態を解く。**2 か所に書かず、この 1 つの関数にまとめる**——
// viewer.js（renderDocument の手順 0a で sameDocument が偽のとき）と navigate.js（状態画面を出す前。
// enterStateScreen）から呼ぶ。片方だけ直すと、状態画面へ移ったときだけ前の文書の状態が残る。
//
// **viewer.js にも overlay.js にも置かない。** viewer.js は overlay.js を import しており、overlay.js
// から viewer.js を呼ぶと循環する（IMP-201）。**このモジュールが import するモジュールは、viewer.js /
// overlay.js / main.js / navigate.js / docswitch.js を import しない。**

import { closeContextMenu } from "./contextmenu.js";
import { closeSearch } from "./search.js";
import { clearSort } from "./tablesort.js";
import { clearMedia } from "./media.js";
import { onDocumentSwitched } from "./expand.js";
import { leaveEditMode } from "./editmode.js";
import { clearFormatState } from "./format.js";

// leaveDocument は前の文書に結び付いた画面の状態を解く（IMP-250）。**足すときもこの関数の中に置き、
// 呼び出し元に書かない。**
export function leaveDocument() {
  // **編集欄（セル・コードブロック）は、ここでは開いていない**（4.75.0）。開いている間は文書を切り替える操作を拒み、
  // 切り替えの知らせは blockedit.js が控えて、閉じた時点で当てる（IMP-264, IMP-220 の手順 0a）。

  // 右クリックメニューを閉じる（FR-063, IMP-249）。メニューが指していたリンクや選択範囲が無くなる。
  closeContextMenu();

  // 検索を閉じる（FR-080, IMP-241）。包んだ <mark> を解いてから本文を差し替える。
  closeSearch();

  // 拡大画面を閉じる（FR-122, IMP-253）。移していた要素は捨て、フォーカスは戻さない（新しい文書の
  // renderDocument の手順 11 が移す）。
  onDocumentSwitched();

  // 並べ替えの状態を空にする（FR-130, IMP-229）。文書の切り替えでは解除する（DSP-352）。
  clearSort();

  // 原寸表示の状態を空にする（FR-121, IMP-228）。文書の切り替えでは縮小表示に戻す（DSP-352）。
  clearMedia();

  // 編集モードは文書の切り替えで終わる（FR-140, DSP-352）。Go 側は既に終えており（IMP-109 の
  // Loaded / Left）、ここは写しを揃えて画面へ写すだけである（editSeq は変えない。IMP-260）。
  leaveEditMode();

  // 修飾の控え（選択の Range と、再描画の後に当て直す位置）を捨てる（IMP-265。4.76.0）。
  clearFormatState();
}
