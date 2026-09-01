// theme.js — テーマの適用と切り替え（FR-070, FR-071, IMP-243, DSP-011）。
//
// 切り替えは #app の data-theme 属性のみで行う。要素の再生成や本文の
// 再変換を伴わない（UI-105）。これにより DSP-370 が求める維持（スクロール
// 位置・検索状態・ツリー・アウトライン・倍率）は、何もしなくても成り立つ。
// **明示的に引き直すのは Mermaid だけである**（IMP-231）。
//
// **prefers-color-scheme を見ない。** OS 設定の反映は起動時に Go 側が
// 解決した値で行う（FR-071, IMP-303, IMP-175）。両方に反応させると、
// 設定値と OS 設定が食い違ったときに表示が揺れる。

import { S } from "./strings.js";
import { state, saveConfig } from "./state.js";
import { setTip } from "./toolbar.js";
import { redrawMermaid } from "./lazy.js";
import { $ } from "./util.js";

// applyTheme はテーマを画面へ反映する。値は "light" | "dark"。
//
// **設定へ保存しない。** 起動時の適用と切り替え時の適用を同じ関数で行い、
// 保存するかどうかは呼び出し側が決める。起動時にここが保存すると、
// 利用者が選んでいないテーマが記録されてしまう（FR-071）。
export function applyTheme(theme) {
  const resolved = theme === "dark" ? "dark" : "light";
  state.theme = resolved;

  const app = $("app");

  // **切り替えの瞬間だけトランジションを止める**（DSP-011）。
  //
  // ツールバーのボタンはホバーの背景を 80ms でフェードさせる（DSP-050）。
  // その指定があると、テーマを切り替えたときに ON のトグルの背景だけが
  // 80ms かけて追いつき、他の要素から遅れる（実機で確認）。DSP-011 は
  // 「テーマ切り替えは即時に完了させる」と定めているため、属性を変える
  // 前後を挟んで無効にする。
  //
  // クラスを外す前に offsetWidth を読み、**その時点でスタイルを確定
  // させる。** これをしないと、追加と削除が同じフレームにまとまり、
  // ブラウザからは何も起きなかったことになる。
  app.classList.add("theme-switching");
  app.setAttribute("data-theme", resolved);
  void app.offsetWidth;
  app.classList.remove("theme-switching");

  // アイコンとツールチップは**切り替え先**を示す（IMP-203, UI-024）。
  const button = $("btn-theme");
  button.querySelector("use").setAttribute("href", resolved === "dark" ? "#icon-sun" : "#icon-moon");
  setTip(button, resolved === "dark" ? S.tipThemeLight : S.tipThemeDark, "theme");
}

// toggleTheme は Light と Dark を入れ替える（FR-070, UI-090）。
export function toggleTheme() {
  applyTheme(state.theme === "dark" ? "light" : "dark");

  // ここで初めて「利用者が選んだ」ことになる（FR-071）。この印が立つまで
  // 設定にはテーマを書かず、OS 設定への追従を残す（IMP-303）。
  state.themeExplicit = true;
  saveConfig();

  // Mermaid だけは配色を自前で持つため引き直す（IMP-231, DSP-370）。
  // **await しない。** 描画を待って画面全体の切り替えを遅らせない。
  redrawMermaid($("markdown"));
}
