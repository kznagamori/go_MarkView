// state.js — フロントエンド側の状態（IMP-210）。
//
// **状態の正は Go 側にある**（IMP-190）。ここに持つのは描画のための写しであり、
// 永続化に関わる値（テーマ・倍率・ペイン幅・表示状態）を変えたときは
// Go 側へ通知する（IMP-310 の UpdateConfig）。
//
// 表示中の文書パスを localStorage 等に保存しない（NFR-042）。

import * as api from "./api.js";

export const state = {
  doc: null, // DocumentDTO（13 章）。未表示なら null
  treeRoot: "", // 絶対パス
  theme: "light", // 実際に適用している値。Go 側が解決済みで渡す（IMP-303）
  themeExplicit: false, // 利用者が自分で切り替えたか（FR-071）
  zoom: 100,
  outlineVisible: true,
  fileTreeVisible: false,
  outlineWidth: 240,
  fileTreeWidth: 260,
  search: { open: false, query: "", hits: [], index: -1 },
  lazy: { mermaid: false, katex: false }, // 読み込み済みか
};

// configPatch は UpdateConfig へ渡す ConfigDTO を組み立てる（IMP-303）。
//
// **ウィンドウの大きさと最大化状態は含めない。** これらは保存の直前に
// Go 側が Wails のランタイムから読み出す（IMP-194）。
// **テーマは、利用者が自分で切り替えるまで空文字を送る**（FR-071, IMP-303）。
// state.theme は Go 側が OS 設定まで解決した値であり、それをそのまま返すと
// 「まだ選んでいない」状態が最初の保存で失われ、以後 OS 設定を変えても
// 追従しなくなる。ペイン幅の一時抑制と同じく、利用者の意思を別に持つ
// （IMP-246）。
// **state.zoom は含めない。** 倍率は保存しないため Go 側に対応するフィールドが
// なく、ConfigDTO にも無い（UI-111, UI-115, IMP-150, IMP-303）。
export function configPatch() {
  return {
    theme: state.themeExplicit ? state.theme : "",
    outlineVisible: state.outlineVisible,
    fileTreeVisible: state.fileTreeVisible,
    outlineWidth: state.outlineWidth,
    fileTreeWidth: state.fileTreeWidth,
  };
}

// 送信待ちの UpdateConfig。**同時に 1 つしか飛ばさない**（IMP-310）。
let sending = Promise.resolve();
let queued = false;

// saveConfig は現在の状態を Go 側へ通知する（UI-114, IMP-310）。
//
// **api.updateConfig を直接呼ばず、必ずここを通す。**
//
// バインドメソッドの呼び出しは Wails がメッセージごとに処理するため、
// 立て続けに 2 つ投げると **到着順が入れ替わりうる**。実際、ペインの開閉と
// 別の設定変更を続けて行うと、先に投げたほうが後に処理され、新しい値が
// 古い値で上書きされた。前の応答を待ってから次を送ることで順序を保つ。
//
// 送信待ちが既にあるときは新たに積まない。ConfigDTO は差分ではなく状態の
// 全体であり、待っている 1 つが送信時点の最新を読めば足りる。
export function saveConfig() {
  if (queued) return sending;

  queued = true;
  sending = sending.then(() => {
    queued = false;
    return api.updateConfig(configPatch());
  });

  return sending;
}
