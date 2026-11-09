// treemenu.js — ファイルツリーの右クリックメニューの項目と実行（FR-036, UI-032, IMP-270）。
//
// **器・開閉・キー・フォーカス・閉じる契機は contextmenu.js（IMP-249 の場所の表の 5）が持つ。** ここは「どの項目を
// 出すか」と「押されたときに何をするか」だけを持つ。**contextmenu.js / filetree.js / navigate.js を import しない**
// （main.js が deps で配線する）。
//
// **パスの形や種類をフロントエンドで判断しない**（IMP-300 の 2）。項目の li の data-path（Go が一覧を返したときの
// 値）と state.treeRoot をそのまま渡し、検査は Go 側が行う（IMP-135, IMP-196）。

import { state } from "./state.js";
import { S } from "./strings.js";

const NO_DEPS = { api: null, notify() {}, openFromTree() {}, openEditorsForTree() {}, applyTreeAction() {} };
let deps = NO_DEPS;

// initTreeMenu は依存を受け取る（IMP-211）。
//
// given: { api, notify(error), openFromTree(path), openEditorsForTree(path, opener), applyTreeAction(dto) }。
//
// **右ボタンの pointerdown / mousedown の既定の動作を止める**（FR-036, DSP-330）。Chromium は右ボタンでもフォーカスを
// 移す既定の動作を持ち、ツリーの項目（tabindex）へ移ると、閉じたときにその項目へ戻り、roving tabindex も付け替わる。
// contextmenu イベントは止めても届く。mousedown も止める（pointerdown の既定の動作に委ねない。NFR-061。IMP-249 と同じ）
export function initTreeMenu(given) {
  deps = { ...NO_DEPS, ...given };

  for (const id of ["tree", "tree-root-name"]) {
    const element = document.getElementById(id);
    if (!element) continue;
    for (const type of ["pointerdown", "mousedown"]) {
      element.addEventListener(type, (event) => {
        if (event.button === 2) event.preventDefault();
      });
    }
  }
}

// treeMenuItems は右クリックした所（li.tree-item か #tree-root-name）の項目を返す（IMP-270 の表）。
// 要素は { action, label, enabled, ready } か、区切りの null。ready があれば、メニューを出した後に呼び、真が
// 返ったら使える状態にする（Set parent as tree root。**右クリックメニューの Paste の可否と同じ形**。IMP-249）。
export function treeMenuItems(place) {
  if (place.id === "tree-root-name") {
    return [
      item("treeFileManager", S.menuOpenInFileManager),
      // 根かどうかをフロントエンドでパスから判断しない。使えない状態で出し、Go に聞く（IMP-310）
      { ...item("treeParentRoot", S.menuSetParentRoot, false), ready: () => deps.api.canSetTreeRootUp() },
      null,
      item("treeCopyPath", S.menuCopyPath),
    ];
  }

  if (place.dataset.dir === "true") {
    return [item("treeFileManager", S.menuOpenInFileManager), item("treeSetRoot", S.menuSetTreeRoot), null, item("treeCopyPath", S.menuCopyPath)];
  }

  return [
    item("treeOpen", S.menuOpen),
    // 画像ファイルはエディタで開かない（FR-036。種類は TreeNodeDTO.kind を写した data-kind）
    item("treeEditor", S.menuOpenInEditor, place.dataset.kind !== "image"),
    item("treeContaining", S.menuOpenContaining),
    null,
    item("treeCopyPath", S.menuCopyPath),
  ];
}

// runTreeAction は項目を実行する（IMP-270）。contextmenu.js がメニューを閉じてフォーカスを開く前の位置へ戻して
// から呼ぶ。**Open は文書を開いて本文ペインへ、Open in editor... はエディタ選択ウィンドウへフォーカスが移る。**
export async function runTreeAction(action, place) {
  const path = place.id === "tree-root-name" ? state.treeRoot : place.dataset.path;

  switch (action) {
    case "treeOpen":
      await deps.openFromTree(path);
      return;
    case "treeEditor":
      // 閉じたらツリーの項目へ戻す（右クリックだけではフォーカスを動かしていない。IMP-270, IMP-252）
      await deps.openEditorsForTree(path, place);
      return;
    case "treeContaining":
      await apply(deps.api.openContainingFolder(path));
      return;
    case "treeFileManager":
      await apply(deps.api.openInFileManager(path));
      return;
    case "treeSetRoot":
      await apply(deps.api.setTreeRoot(path));
      return;
    case "treeParentRoot":
      // パスを渡さない。ツリールートの親は Go 側が求める（FR-036）
      await apply(deps.api.setTreeRootUp());
      return;
    case "treeCopyPath":
      await apply(deps.api.copyTreePath(path));
      return;
    default:
  }
}

// apply は TreeActionDTO（IMP-317）を反映する。失敗はステータス領域へ（Copy path の成功は知らせない。UI-060）。
// root が空でなければツリールートが変わった——読み直しとステータス領域のパスの差し替えは main.js が行う。
async function apply(pending) {
  const dto = await pending;
  if (!dto) return;

  if (dto.error) deps.notify(dto.error);
  if (dto.root) await deps.applyTreeAction(dto);
}

function item(action, label, enabled = true) {
  return { action, label, enabled };
}
