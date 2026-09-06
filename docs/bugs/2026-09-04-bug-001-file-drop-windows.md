# BUG-001: Windows でドロップしたファイルが Go 側へ届かない

| 項目 | 内容 |
| --- | --- |
| 不具合番号 | **BUG-001** |
| 報告日 | 2026-09-04 |
| 対象 | `v1.0.0-rc.2`（`aeae862` + リリース CI の資産更新コミット） |
| 検出 | 手動テスト **E2E-223 / E2E-224 / E2E-225**（3 件とも NG。**同一原因**） |
| 環境 | Windows / WebView2（**Linux では発生しないと考えられる**。下記 6 章） |
| 関連要求 | FR-011, UI-070, IMP-245, IMP-313, AR-060 |
| 分類 | **仕様の誤り（IMP-245 の手順が不足）＋ 実装の欠落** |
| 混入時期 | `5c1f34c feat: P4 フロントエンド — 表示と操作の一式`（`dnd.js` の新規作成時点から） |

---

## 1. 症状

ウィンドウへファイル・ディレクトリをドラッグすると、**オーバーレイと `Drop a Markdown file to open` の案内は正しく出る。**
しかし**ドロップしても何も起こらない。**

| 確認内容 | 結果 |
| --- | --- |
| ドラッグ中にオーバーレイと案内が出る（UI-070, DSP-190） | **OK** |
| ドロップした文書が表示される（FR-011） | **NG** |
| ファイルツリーのルートがドロップ先の親に変わる | **NG** |
| ディレクトリをドロップするとルートになり README が開く（E2E-224） | **NG** |
| 対応しないファイル（`.png`）でステータス領域に英語のメッセージが出る（E2E-225） | **NG** |
| アプリが異常終了する | しない（**そこは正常**） |

「一切反応しない」のであって、誤ったファイルが開くわけではない。

---

## 2. 原因

### 2.1 直接の原因 — フロントエンドが `window.runtime.OnFileDrop()` を呼んでいない

`frontend/js/dnd.js` の `initDnd()` は、**オーバーレイの表示だけ**を配線している。

```js
export function initDnd() {
  buildOverlay();

  window.addEventListener("dragenter", onEnter);
  window.addEventListener("dragover", onOver);
  window.addEventListener("dragleave", onLeave);
  window.addEventListener("drop", onDrop);   // ← オーバーレイを隠すだけ
}
```

Go 側は `app.go` で受け口を用意している。

```go
// app.go 145
runtime.OnFileDrop(ctx, func(_, _ int, paths []string) { a.onFileDrop(paths) })
```

**この 2 つはつながっていない。**

### 2.2 なぜつながらないのか — Go の `runtime.OnFileDrop` は「購読」しかしない

Wails v2.15.0 の実体は次のとおりである。

```go
// wails/v2/pkg/runtime/draganddrop.go
func OnFileDrop(ctx context.Context, callback func(x, y int, paths []string)) {
    ...
    EventsOn(ctx, "wails:file-drop", func(optionalData ...interface{}) { ... })
}
```

**`wails:file-drop` を購読するだけである。** 発火させる側は別にいる。

Windows（WebView2）で発火させる経路は次の 5 段になっている。

```
1. JS : window.addEventListener("drop", onDrop)          <- wails の draganddrop.js
2. JS : window.runtime.ResolveFilePaths(e.x, e.y, files)
3. JS : chrome.webview.postMessageWithAdditionalObjects("file:drop:x:y", files)
4. Go : windows/frontend.go が ICoreWebView2File から絶対パスを取り出し
        dispatchMessage("DD:x:y:paths") へ渡す
5. Go : dispatcher が events.Emit("wails:file-drop", x, y, paths)
        -> ここでようやく app.go のコールバックが呼ばれる
```

**この 1 を取り付けるのは、JS 側の `OnFileDrop()` だけである。**

```js
// wails/v2/internal/frontend/runtime/desktop/draganddrop.js
export function OnFileDrop(callback, useDropTarget) {
    if (typeof callback !== "function") { ... return; }
    if (flags.registered) return;
    flags.registered = true;
    ...
    window.addEventListener("dragover",  onDragOver);
    window.addEventListener("dragleave", onDragLeave);
    window.addEventListener("drop",      onDrop);      // <- 1 はここ
    EventsOn("wails:file-drop", cb);
}
```

MarkView はこの関数を**一度も呼んでいない**（`grep -rn "OnFileDrop" frontend/js/` はコメント 1 行しか返さない）。
したがって `flags.registered` は false のまま、1 が付かず、2 以降が起きない。
**Go のコールバックは登録されているが、誰も呼ばない。**

### 2.3 前提を満たしている部分（切り分け済み）

**同じ機能の他の必要条件はすべて満たされている。** ここが原因ではない。

| 必要条件 | 状態 | 根拠 |
| --- | --- | --- |
| `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}` | **ある** | `main.go` 152 |
| `--wails-drop-target: drop` の宣言 | **ある** | `frontend/css/base.css` 78（`#app`。継承する） |
| `#dropzone { pointer-events: none }` | **ある** | `frontend/css/components.css` 1034 |
| `CSSDropProperty` / `CSSDropValue` の既定値 | **適用される** | `options.go` 165〜169 が空文字を既定へ埋める |
| Go 側の対象選択（Markdown / ディレクトリ / それ以外） | **ある** | `bind.go` の `dropTarget` / `dropRoot`、`app.go` の `onFileDrop` |
| 対応しないファイルのステータス通知 | **ある** | `app.go` 337 `a.emit(eventError, newErrorDTO(..., document.ErrNotMarkdown))` |

**つまり E2E-225 の「英語のメッセージが出ない」も、メッセージを出す実装が無いのではなく、そこへ到達していないだけである。**

---

## 3. 原因の分類 — 仕様か、実装か

### 3.1 仕様の誤り — IMP-245 が「JS 側の登録」を落としている

`docs/specs/12-impl-frontend.md` の IMP-245 は、ドロップに必要な条件を**4 つ挙げている**。

- `OnFileDrop` は既定では呼ばれない → Go の起動オプションが要る
- 受け口となる要素は CSS で宣言する（`--wails-drop-target: drop`）
- `#dropzone` は `pointer-events: none` とする
- 受け取ったパスの判定は Go 側で行う（IMP-313）

**5 つ目の「フロントエンドから `window.runtime.OnFileDrop()` を呼ぶ」が無い。**
それどころか `initDnd()` の説明は「**ドラッグ中の表示だけを配線する**」であり、**呼ばないことを積極的に指示している。**

IMP-313 も同じ理解の上に立っている。

```go
runtime.OnFileDrop(ctx, func(x, y int, paths []string) { ... })
```

とだけ書かれており、「これは購読であって、発火させる側が別に要る」ことに触れていない。

**実装は仕様どおりに書かれている。** `dnd.js` の冒頭コメント（「ここで行うのは**ドロップ可能であることの表示だけ**とする」）は IMP-245 の文をほぼそのまま写したものである。**仕様を直さずに実装だけ直すと、次に仕様から書き起こしたときに同じ状態へ戻る。**

### 3.2 仕様が誤った理由 — Windows と Linux で仕組みが違う

Linux（WebKitGTK）では JS を経由しない。

```c
// wails/v2/internal/frontend/desktop/linux/window.c 574
if (enableDragAndDrop) {
    g_signal_connect(G_OBJECT(webview), "drag-data-received", G_CALLBACK(onDragDataReceived), NULL);
    g_signal_connect(G_OBJECT(webview), "drag-drop",          G_CALLBACK(onDragDrop),         NULL);
}
```

GTK のシグナルが直接 `processMessage("DD:x:y:paths")` を呼ぶため、**JS 側の登録は要らない。**

「Go でコールバックを登録し、CSS で受け口を宣言すれば届く」という IMP-245 の理解は、**Linux では正しく、Windows では正しくない。** 仕様はプラットフォーム差に触れていない。

### 3.3 実装の欠落 — 呼び出し 1 行

仕様の不足がそのまま実装の欠落になっている。**実装側だけの誤りではない**が、**実装を直さなければ動かない**ことに変わりはない。

### 3.4 あわせて記録しておくこと — ドロップ地点の判定は Windows では効いていない

Windows の経路では、3 の `ResolveFilePaths` は**ドロップ地点を見ずに送る。**
`--wails-drop-target` の検査は、5 のあと **JS 側のコールバック**の中でしか行われない。

```js
cb = function (x, y, paths) {
    const element = document.elementFromPoint(x, y)
    if (!element || !checkStyleDropTarget(getComputedStyle(element))) return null;
    callback(x, y, paths);     // <- JS のコールバックだけが止まる
}
```

**Go 側のコールバック（`app.go`）はこの検査を通らずに必ず呼ばれる。**
したがって IMP-245 の「CSS で受け口を宣言する」は、Windows では**必須条件ではない**（Linux でも C 側が判定するため同様）。宣言自体は害が無く、UI-070 の意図を残す意味があるので消す必要はないが、**「これがあるからウィンドウ全体で受け付けられる」という説明は正確ではない。**

---

## 4. なぜ単体テストと E2E の自動テストで見つからなかったのか

### 4.1 単体テスト — 対象外（意図的な除外）

UT-002 が単体テストの対象を `internal/` に限っている。`main.go` / `app.go` / フロントエンドは対象外である。

- JS 側の登録漏れ → `frontend/` は対象外
- `runtime.OnFileDrop` の登録 → `app.go` は対象外
- 対象選択の `dropTarget` / `dropRoot` → **`bind.go`（package main）にある**ため、これも対象外

31 章 31.10 の対応表も、この状態をそのまま記録している。

```
| FR-011 | — (GUI) | ドロップは Wails のコールバック |
```

**FR-011 には単体テストが 1 件も無い。** これは漏れではなく、UT-002 に従った結果である。

> [!NOTE]
> `dropTarget` / `dropRoot` は「複数パスからどれを開くか」という**判断を伴うロジック**であり、IMP-012 の趣旨からは `internal/session` に置いて単体テストの対象にできた。ただしそれを移しても**今回の不具合は捕まらない**（壊れているのは呼び出しの手前である）。

### 4.2 E2E の自動テスト — 操作を行わない

40 章の自動テスト（E2E-101〜109）が行うのは次だけである。

- 配布アーカイブの中身・命名・サイズの検査（E2E-101, E2E-104, E2E-106, E2E-107）
- 実行ファイルの `--version` / `--help` / 未知のオプション（E2E-102, E2E-103, E2E-105）
- 描画スモークテスト（E2E-109。ヘッドレス Chromium で Markdown を描くだけ）

**ウィンドウを開いてマウス操作をする自動テストは存在しない。** 40 章の対応表も、FR-011 の検証は手動ケースだけだと明記している。

```
| FR-011 | E2E-223, E2E-224, E2E-225 |  |
```

### 4.3 描画スモークテスト（E2E-109）でも捕まらない

`scripts/smoke` は `frontend/vendor/` の資産（Mermaid / KaTeX / PlantUML）が描けるかを見るものであり、**Wails のランタイムを読み込まない。** `window.runtime` が存在しない環境なので、この経路を通ること自体がない。

### 4.4 手動テストで初めて出た

E2E-223 は優先度「高」であり、**手順にドロップが明記されている唯一のケース**である。
`v1.0.0-rc.1` を手動テストせずにリジェクトしたため（外部エディタと PlantUML を `v1.0.0` に含める判断）、**この機能は今回が初めての実行**だった。

### 4.5 まとめ

| 層 | 捕まえられたか | 理由 |
| --- | --- | --- |
| 単体テスト | **できない** | 対象が `internal/` のみ（UT-002）。壊れている場所が 3 か所とも対象外 |
| 自動 E2E | **できない** | 配布物の検査と CLI のみ。ウィンドウ操作を行わない |
| 描画スモーク | **できない** | Wails ランタイムを読み込まない |
| 手動テスト | **できた** | E2E-223（今回が初回実施） |

**この不具合は、現在のテスト体系では手動テストでしか見つからない。** それは設計上の割り切り（UT-002, AR-050）の帰結であり、体系の欠陥ではない。

---

## 5. 修正方法

### 5.1 修正 — `initDnd()` で Wails の drop リスナを取り付ける

```js
// frontend/js/dnd.js
import { OnFileDrop } from "../wailsjs/runtime/runtime.js";

export function initDnd() {
  buildOverlay();

  // **Wails のランタイム側の drop リスナをここで取り付ける。**
  //
  // Go の runtime.OnFileDrop（app.go）は "wails:file-drop" を購読するだけで
  // あり、発火させるのは JS 側である。Windows（WebView2）はドロップされた
  // File オブジェクトを postMessageWithAdditionalObjects で Go へ渡して
  // 初めて絶対パスが得られるため、**この呼び出しが無いとパスが届かない。**
  // Linux（WebKitGTK）は GTK のシグナルで直接届くため、そちらでは不要である。
  //
  // パスの処理は Go 側で行う（IMP-313）。ここで受け取るものは無い。
  OnFileDrop(() => {}, true);

  window.addEventListener("dragenter", onEnter);
  ...
}
```

- `frontend/wailsjs/runtime/runtime.js` は既にあり、`api.js` が同じ経路で `EventsOn` を取り込んでいる。**新しい依存は増えない。**
- 第 2 引数は `true`（既定）でよい。JS 側のコールバックが `--wails-drop-target` を検査するが、**Go 側は検査せずに呼ばれる**ため、`false` にしても動作は変わらない。`true` のままにして IMP-245 の CSS 宣言を意味のあるものに保つ。
- コールバックは空でよい。**フロントエンドは結果をイベント（`document:opened` / `tree:rootChanged` / `error`）で受け取る**（IMP-320, IMP-322）。ここで処理を書くと経路が 2 つになる。

### 5.2 あわせて直すべき仕様

**NFR-071 に従い、同じ変更で仕様書も直す。**

| 文書 | 直す内容 |
| --- | --- |
| `12-impl-frontend.md` IMP-245 | `initDnd()` の説明を「ドラッグ中の表示だけ」から改める。**`window.runtime.OnFileDrop()` を呼ぶこと**を条件として追加し、「Go の `runtime.OnFileDrop` は購読であって、JS 側の登録が別に要る」ことを理由とともに書く |
| 同上 | **Windows と Linux で仕組みが違う**ことを NOTE で明記する（WebView2 は JS 経由、WebKitGTK は GTK シグナル）。ここが今回の誤りの根である |
| 同上 | CSS の `--wails-drop-target` について、**Go 側のコールバックはこの検査を通らない**ことを注記する。「これがあるからウィンドウ全体が受け口になる」という説明を正す |
| `13-impl-interface.md` IMP-313 | 「バインドメソッドではなくコールバックで受け取る」の下に、**発火の経路（JS → Go）**を 1 行足す |
| `docs/specs/README.md` | 改訂履歴に 1 行追加し、版を上げる（NFR-071） |

### 5.3 あわせて検討すべきテスト仕様

E2E-223 の確認内容は「オーバーレイが出る」と「文書が表示される」を**同じ 1 ケースに混ぜている。**
今回、前者だけが OK で後者が NG になり、記録が「OK と NG が混在した NG」になった。

**オーバーレイの表示（UI-070）とパスの受け取り（FR-011）は、壊れる原因がまったく別である。** 分けたほうが切り分けが早い。ただし**これは 41 章の改訂であり、本報告では提案にとどめる**（本作業では調査報告のみを作成する）。

---

## 6. 影響範囲

| 範囲 | 影響 |
| --- | --- |
| Windows | **FR-011 が完全に機能しない。** ドロップによるファイル・ディレクトリの表示、対応しないファイルの通知のすべて |
| Linux | **発生しないと考えられる**（3.2 の GTK 経路）。ただし**未確認**である。E2E-223 は W1 / L1 の両方が対象だが、今回は W1 でしか実施していない |
| 他機能 | **無い。** ツールバーの `Open`（FR-010）・コマンドライン引数（FR-012）・ツリーからの選択（FR-033）・リンク遷移（FR-050）はいずれも別経路であり、すべて OK になっている |
| 異常終了 | **しない。** 「何も起きない」だけであり、FR-111 の観点では正常 |

> [!IMPORTANT]
> **Linux で動くという推定は、コードを読んだ結果であって実測ではない。** 修正後の検証では、**Windows と Linux の両方で E2E-223 を実施する**。Linux で元から動いていたのなら、JS 側の登録を足したあとも**二重に開かない**ことを確かめる意味がある（GTK 経路と JS 経路は別系統であり、`flags.registered` はプロセス内で 1 度だけだが、両方が `wails:file-drop` を発火しうる）。

---

## 7. 修正後の検証方法

### 7.1 手動

1. **E2E-223**（W1 / L1）— `showcase.md` をドロップ。文書が表示され、ツリールートが親へ変わること。ツールバー・サイドペインへのドロップでも同じ結果になること
2. **E2E-224**（W1）— `testdata/e2e` をドロップ。ルートになり、直下の `README.md` が開くこと
3. **E2E-225**（W1）— `.png` をドロップ。表示が変わらず、ステータス領域に英語のメッセージが数秒出ること
4. **Linux でも 1 を実施し、二重に開かないこと**を確かめる（6 章の IMPORTANT）

### 7.2 機械的な確認

```bash
# JS 側の登録があること
grep -rn "OnFileDrop" frontend/js/

# Go 側の受け口が 1 か所のままであること（経路を増やさない。IMP-300）
grep -rn "OnFileDrop" --include=*.go .
```

```bash
go test ./...          # 既存が緑のままであること（このパッケージ群に変更は無い）
go vet ./...
```

---

## 8. 調査の記録

| 確かめたこと | 結果 |
| --- | --- |
| `EnableFileDrop` が渡っているか | **渡っている**（`main.go` 152） |
| `CSSDropProperty` / `CSSDropValue` が空文字のまま JS へ渡っていないか | **既定値が埋まる**（`options.go` 165〜169）。空文字ではない |
| `--wails-drop-target` が継承で届くか | **届く**（`#app` に宣言。`#dropzone` は `pointer-events: none`） |
| `runtime.OnFileDrop`（Go）が何をするか | **`EventsOn("wails:file-drop", ...)` だけ。** リスナは付けない |
| `wails:file-drop` を誰が発火するか | Windows は `dispatcher/draganddrop.go` の `processDragAndDropMessage`。その入口は JS の `ResolveFilePaths` |
| JS の `drop` リスナを誰が付けるか | **`draganddrop.js` の `OnFileDrop()` だけ。** MarkView は呼んでいない |
| Linux は同じか | **違う。** `window.c` 574 が GTK の `drag-data-received` / `drag-drop` を直接つなぐ |
| Go 側の対象選択とエラー通知は実装済みか | **済み**（`bind.go` `dropTarget` / `dropRoot`、`app.go` `onFileDrop`） |

**参照した Wails のソース**（`v2.15.0`）:

- `pkg/runtime/draganddrop.go`
- `pkg/options/options.go` 162〜169, 201〜215
- `internal/frontend/runtime/desktop/draganddrop.js`
- `internal/frontend/desktop/windows/frontend.go` 730〜745, 783〜838, 961
- `internal/frontend/dispatcher/draganddrop.go`
- `internal/frontend/desktop/linux/frontend.go` 503〜525
- `internal/frontend/desktop/linux/window.c` 558〜580
