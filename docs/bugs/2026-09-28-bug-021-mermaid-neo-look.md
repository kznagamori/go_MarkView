# BUG-021: Mermaid の図の見た目（look）を、同梱資産の既定に委ねていた

| 項目 | 内容 |
| --- | --- |
| 不具合番号 | **BUG-021** |
| 報告日 | 2026-09-28 |
| 対象 | **`v1.1.0-rc.1` から**（Mermaid 12.0.0 を取り込んだ rc。v1.0.0 の 11.17.2 では出ない） |
| 検出 | **`v1.1.0-rc.2` の手動テスト E2E-238 と E2E-354**（利用者。2026-09-27〜28）。E2E-354 は確認内容 1〜5 がすべて OK で、NG の理由は図の見た目だけだった |
| 環境 | **W1 / L1 とも**（利用者の確認） |
| 関連要求 | FR-023, MD-002, MD-080, DSP-270, IMP-231, BR-043, BR-054, E2E-234, E2E-238, E2E-354 |
| 分類 | **仕様の不足**（IMP-231 と DSP-270 が Mermaid の `look` を定めておらず、同梱資産の既定に委ねていた） |
| 状態 | **修正済み（仕様 4.71.0）**。**修正後の実機での確認は rc.3 の手動テスト**（E2E-234 の確認内容 7。41 章を改訂したため全件） |

---

## 1. 症状

**Mermaid の図のノードに影と枠のグラデーションが付き、ぼけて見える**（利用者の報告）。

> Mermaid の図でテーマが Dark の場合に矩形の背景に影？が表示されボケた感じになる。丸は少ない。スタジアム型もボケが大きい。Light の時も表示されているかもしれないけどわかりづらい量。
>
> 影というか矩形のボーダに影やグラデーションがかかっている。（E2E-354 の備考）

**Dark で目立ち、Light では分かりにくい。** 影の色が**テーマに関係なく明るい灰色**（`rgba(185,185,185,1)`）であるため、暗い背景では光のにじみに見える。

## 2. 原因

**Mermaid 12.0.0 が、図種別ごとの設定の既定を `look: "neo"` に変えた。** 同梱の `mermaid.min.js` の既定値（抜粋）:

```text
flowchart: { useMaxWidth: true, theme: "redux-color", look: "neo", ... }
sequence:  { useMaxWidth: true, theme: "redux-color", look: "neo", ... }
swimlane / agentflow / class / state / er / requirement / usecase / venn も同じ（10 種）
全体:      { theme: "default", look: "classic", layout: "elk", ... }
```

v1.0.0 の 11.17.2 には、図種別ごとの `look` の既定が 1 つも無い（全体の `look: "classic"` だけ）。

**全体の既定は `classic` のままだが、図種別ごとの既定が優先される。** 全体の `look` を指定したときだけ、図種別ごとの既定より指定が勝つ（手元で確かめた。下の表）。

**IMP-231 の初期化は `look` を指定していなかった**（`startOnLoad` / `securityLevel` / `theme` / `dompurifyConfig` の 4 つだけ）。v1.0.0 の 11.17.2 は図種別ごとの既定を持たず、全体の既定の `classic` で描いていたため、**指定しなくても影は付かなかった。**

`neo` の図に Mermaid が当てる CSS（抜粋。図の id 付きのセレクタで、`data-look` を条件にする）:

```css
#<図の id> [data-look="neo"].node rect,
#<図の id> [data-look="neo"].node polygon { stroke: url(#<図の id>-gradient); filter: drop-shadow(1px 2px 2px rgba(185,185,185,1)); }
#<図の id> [data-look="neo"].node circle   { stroke: url(#<図の id>-gradient); filter: drop-shadow(1px 2px 2px rgba(185,185,185,1)); }
```

シーケンス図の参加者には `filter: url(#<図の id>-drop-shadow)` が付く。

### 2.1 手元で確かめたこと（2026-09-28）

`workspace/tmp/_mermaidlook`（使い捨て。コミットしない）で、**IMP-231 と同じ初期化**（`securityLevel: "strict"`、`theme` は `dark` / `default`、`SANITIZE_NAMED_PROPS`）で同じ図を描き、Chromium（Edge のヘッドレス）で計算値を数えた。

| Mermaid | `look` の指定 | flowchart | sequenceDiagram | stateDiagram-v2 |
| --- | --- | --- | --- | --- |
| 11.17.2（v1.0.0） | なし | `classic`、影 0 | 影 0 | `classic`、影 0 |
| **12.0.0（rc.1 / rc.2）** | **なし** | **`neo`、影 9** | **`neo`、影 4** | **`neo`、影 5** |
| 12.0.0 | `classic` | `classic`、影 0 | 影 0 | `classic`、影 0 |

**`look: "classic"` を渡せば、影もグラデーションも消える。** Dark と Light のどちらでも同じだった。描いた図を画像にして見比べ、**12.0.0 の既定の図にだけ、ノードのまわりの明るいにじみがある**ことも目で確かめた。

描画スモークの検証用文書（`testdata/smoke.md`。8 つの図）でも同じだった。`lazy.js` から `look` を外すと、`flowchart` は影の要素が 10 個、シーケンス図は 6 個、クラス図と ER 図は 2 個、状態遷移図は 10 個、ラベルに id を書いた図は 4 個になった。**ガントと円グラフは `data-look` を出さず、影も付かない。**

> [!NOTE]
> **ノードの大きさは `classic` を指定しても 12 系のまま**であり、v1.0.0 より少し大きい（円は特に大きい）。**12.0.0 は `layout` の既定も `dagre` から `elk` に変えている。** 単純な図では `dagre` を指定しても見た目がほぼ変わらず、報告も無いため、**どちらも固定しない**。**`look` を固定するのは、報告された症状（影とグラデーション）がそこから来ているためである。**

## 3. なぜ rc.1 で見つからなかったか

**rc.1 も同じ資産だった**（`git diff v1.1.0-rc.1 v1.1.0-rc.2 -- frontend/vendor` は空）。rc.1 では E2E-234 / E2E-238 / E2E-354 のいずれも OK だった。

**図の見た目を見る確認内容が、どのケースにも無かった。**

| ケース | 見ていたもの |
| --- | --- |
| E2E-231（GitHub と見比べる） | 見出し・表・タスクリストほか。**Mermaid は範囲外** |
| E2E-234（Mermaid 図） | 描画されるか・本文幅に収まるか・構文エラー・図の中のリンク。**見た目は見ない** |
| E2E-238（仕様書を読む） | 「図表が正しく描画される」。**何を正しいとするかが書かれていない** |
| 描画スモーク（BR-054） | SVG ができたか・寸法があるか。**見た目は見ない** |

CLAUDE.md の「リリース」には、rc.1 の前から「**図の見た目は手動テストで見る**」と書いてあった。**しかし手順も確認内容も無かった。** UI-031（実装が無いまま OK が付いた）と同じく、**確認内容に書かれていないものは見られない。**

## 4. 直し方

**`look` を明示し、その値を GitHub の描画に合わせる**（利用者の判断。2026-09-28）。

```js
window.mermaid.initialize({
  startOnLoad: false,
  securityLevel: "strict",
  theme: state.theme === "dark" ? "dark" : "default",
  look: "classic",
  dompurifyConfig: { SANITIZE_NAMED_PROPS: true },
});
```

- **値は GitHub の描画に合わせる**（MD-080, MD-002）。4.71.0 の時点は `classic` とする。**GitHub が `neo` に切り替えたら追従する**（利用者の判断）
- **同梱資産の既定に委ねない。** PlantUML の `maxSvgSize`（IMP-233。4.64.0）と同じく、**資産の更新（BR-043）だけで見た目が変わらないようにする**ための明示である
- **気づく場を 2 つ置く**:
  - **E2E-234 の手順 6 と確認内容 7**——GitHub 上の同じ `testdata/e2e/mermaid.md` と並べ、**影とグラデーションの有無**を見比べる。MarkView にだけ付けば退行、**GitHub にだけ付けば GitHub が切り替えた**ことになり、どちらも NG として報告する（利用者の判断）
  - **描画スモークの検査 11**（BR-054, E2E-109。判定は UT-816）——描けた図の `data-look` がすべて `classic` で、影（計算値の `filter`）の付いた要素が無いこと。**rc の CI で資産が更新されたとき、新しい版が `look` を無視した・名前を変えた、を捕まえる。** Mermaid の設定はエンジンに依らないため、Chromium だけで有効である

**`look` を変えるときは、`lazy.js` の値・`scripts/smoke` の `mermaidLook`・UT-816 の影の判定・IMP-231 / DSP-270 を一緒に変える。**

## 5. 影響

**表示だけである。** データにも操作にも影響しない。ただし MD-080 / DSP-270 の「GitHub と同じ体裁」を満たせておらず、**Dark では読みにくい。** Mermaid の図を含むすべての文書（仕様書・`showcase.md`・拡大画面）に出る。

## 6. 直した後に確かめること

| 何を | どこで | 結果 |
| --- | --- | --- |
| 描画スモークで 8 つの図がすべて `classic`、影 0 | `go run ./scripts/smoke`（assets 部） | **OK**（2026-09-28） |
| `look` の指定を外す・`neo` にする・`handDrawn` にすると描画スモークが落ちる | `lazy.js` を壊して実行（UT-033） | **3 通りとも落ちた**（2026-09-28） |
| 判定を壊すと UT-816 が落ちる | `checkMermaidLook` を 6 通りに壊して `go test` | **6 通りとも落ちた**（2026-09-28） |
| **影とグラデーションの有無が GitHub と同じ** | **rc.3 の E2E-234 の確認内容 7**（W1 / L1。Light と Dark） | 未（**GitHub の図に影が無いことも、ここで初めて確かめる**） |
| 仕様書の図・拡大画面の図に影が無い | rc.3 の E2E-238 / E2E-354 | 未 |
