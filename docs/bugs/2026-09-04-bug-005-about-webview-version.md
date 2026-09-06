# BUG-005: アプリケーション情報に WebView のバージョンが表示されない

| 項目 | 内容 |
| --- | --- |
| 不具合番号 | **BUG-005** |
| 報告日 | 2026-09-04 |
| 対象 | `v1.0.0-rc.2`（`aeae862` + リリース CI の資産更新コミット） |
| 検出 | 手動テスト **E2E-301**（NG） |
| 環境 | Windows / WebView2（**Linux でも同じ**。空文字を渡しているのは OS 共通の経路） |
| 関連要求 | FR-100, UI-100, IMP-181, IMP-306, IMP-310, AR-001 |
| 分類 | **仕様と実装の不整合。実装が仕様（UI-100）を満たしていない。あわせて IMP-181 が「省略してよい」と書いたことで、不整合が矛盾として現れていない** |
| 混入時期 | `bind.go` の `GetAbout` を書いた時点（`GetAbout` は初版から `""` を渡している） |

---

## 1. 症状

ツールバー右端の `?` で情報ダイアログを開くと、`Environment` 行に **WebView のバージョンが出ない。**

実施者が記録した実測値:

```
Author
kznagamori
Repository
https://github.com/kznagamori/go_MarkView
License
MIT License
Environment
windows/amd64 Go 1.25.0        <- WebView2 の版が無い
Bundled
Mermaid 11.17.2
KaTeX 0.18.5
PlantUML 1.2026.7
```

E2E-301 の確認内容は次を求めている。

> 実行環境（OS / アーキテクチャ / Go / **WebView**）が表示される

**他の項目（アイコン・バージョン・作成者・リポジトリ・ライセンス・同梱資産・リンクの外部遷移・背後の操作抑止・`Esc` での閉じ）はすべて満たされている。**
NG になったのは `WebView` の 1 項目だけである。

---

## 2. 原因

### 2.1 直接の原因 — 呼び出し側が空文字を固定で渡している

```go
// bind.go 353
// GetAbout はアプリケーション情報を返す（IMP-310, FR-100, FR-101）。
func (a *App) GetAbout() AboutDTO {
	defer recoverQuiet()

	// WebView のバージョンは Wails v2 のランタイムが返さないため空文字を
	// 渡す。Environment は当該区画ごと省く（IMP-181）。
	return newAboutDTO(thirdPartyLicenses, "")
}
```

受け取る側は正しく実装されている。

```go
// internal/buildinfo/vendor.go 99
func Environment(webviewVersion string) string {
	parts := []string{
		runtime.GOOS + "/" + runtime.GOARCH,
		"Go " + strings.TrimPrefix(runtime.Version(), "go"),
	}

	if webviewVersion != "" {
		parts = append(parts, webviewName()+" "+webviewVersion)   // <- 通らない
	}

	return strings.Join(parts, "  ")
}
```

**値を組み立てる仕組みは全部そろっている。渡す値が無いだけである。**

### 2.2 コメントの主張は「半分だけ」正しい

> WebView のバージョンは Wails v2 のランタイムが返さないため

**Wails v2 の公開ランタイム API（`pkg/runtime`）が返さないのは事実である。**

```go
// wails/v2/pkg/runtime/ にあるのは EnvironmentInfo{BuildType, Platform, Arch} だけ
```

Wails 自身は取得している。しかし置き場所が `internal` である。

```go
// wails/v2/internal/system/system_windows.go 32
version, _ := webviewloader.GetAvailableCoreWebView2BrowserVersionString("")
```

`internal/` は外部からインポートできないため、**ここは確かに使えない。**

**ただし、その `webviewloader` 自体は別モジュールの公開パッケージである。**

```
github.com/wailsapp/go-webview2/webviewloader
  -> func GetAvailableCoreWebView2BrowserVersionString(browserExecutableFolder string) (string, error)
```

そして `go-webview2` は **すでに MarkView の依存関係に入っている**（`go.mod` の `// indirect` 群に `github.com/wailsapp/go-webview2 v1.0.22`）。

**つまり「取得できない」は誤りである。** Windows については、新しいモジュールを 1 つも足さずに取得できる。

### 2.3 IMP-181 が「省略してよい」と書いたため、不整合が表に出なかった

`internal/buildinfo/vendor.go` の doc コメント（IMP-181 の実装）:

> `webviewVersion` が空の場合、その区画ごと省略する。**Wails から取得できない
> ことがあり**、「WebView2 」とだけ書かれた行は情報として役に立たないため。

この規定自体は妥当である（取得に失敗したときに壊れた行を出さない）。

**問題は、「例外的な救済」として設けたものが、実際には「常に通る唯一の経路」になっていることである。**
`GetAbout` はリテラルの `""` を渡すため、**`if webviewVersion != ""` の真の側には一度も入らない。**

### 2.4 上位（UI-100）と下位（IMP-181）が食い違っている

| 層 | 定めていること |
| --- | --- |
| UI-100（要求 **MUST**） | 図に `Environment  windows/amd64  Go 1.24.0  **WebView2 120.x**` と明示 |
| E2E-301（テスト仕様） | 「実行環境（OS / アーキテクチャ / Go / **WebView**）が表示される」 |
| IMP-181（実装） | 「取得できないことがあり」**空なら省略してよい** |

**上位が下位を規定し、矛盾したら上位を正とする**（`docs/specs/README.md` の 5 層構成）。
したがって **UI-100 が正であり、実装が要求を満たしていない。**

IMP-181 の省略規定は「異常時の振る舞い」として残してよいが、**「常に空を渡す」ことを許す根拠にはならない。**

---

## 3. 原因の分類 — 仕様か、実装か

### 3.1 一次原因 — **実装の誤り**

UI-100 が MUST で表示を求めているのに、実装が値を渡していない。**取得手段が実際にはある**（2.2）以上、「できないから省いた」という説明は成り立たない。

### 3.2 二次原因 — **仕様の書き方（IMP-181）**

IMP-181 が「取得できないことがある」という**前提の誤り**にもとづいて省略規定を置いたため、
**実装が要求を満たしていない状態が、仕様上は正常に見えるようになっていた。**

- コメント（`vendor.go` 97〜98）も同じ誤りを繰り返している
- **さらに単体テストがその状態を「正しい」として固定してしまった**（4.1）

これは「仕様が間違っている」というより、**仕様が実装の妥協を追認してしまった**種類の不備である。

### 3.3 誤りではないもの

- `buildinfo.Environment` の実装は正しい。書式（`OS/ARCH  Go x.y.z  WebView2 a.b.c`）も UI-100 の図と一致する
- `webviewName()` の OS 分岐（Windows は `WebView2`、それ以外は `WebKitGTK`）も AR-001 と一致する
- `AboutDTO` の構造（IMP-306）と、フロントエンドが受け取った値を描くだけの作り（IMP-201）も正しい

**壊れているのは、`bind.go` の引数 1 つである。**

---

## 4. なぜ単体テストと E2E の自動テストで見つからなかったのか

### 4.1 単体テスト — **テストは通っている。しかも「空なら省く」ことを正しいと固定している**

`internal/buildinfo/vendor_test.go` の `TestEnvironment` は 3 つの部分テストを持つ。

```go
t.Run("WebView のバージョンがある", func(t *testing.T) {
    got := Environment("120.0.1")
    ...
    if !strings.Contains(got, "120.0.1") { ... }      // <- 通る
})

// バージョンが取れない場合、区画ごと省く（IMP-181）。
t.Run("WebView のバージョンがない", func(t *testing.T) {
    got := Environment("")
    if strings.Contains(got, "WebView") || strings.Contains(got, "WebKitGTK") {
        t.Errorf(...)                                  // <- ここも通る
    }
})
```

**両方の枝が緑である。関数の契約としては何も壊れていない。**

問題は次の点にある。

> [!IMPORTANT]
> **「関数が正しく動くこと」と「呼び出し側が正しい値を渡すこと」は別である。**
> `Environment("")` を検証するテストは、**呼び出し側が常に `""` を渡している状態を、そのまま『仕様どおり』として承認してしまう。**
> このテストがあることで、**不具合の存在がむしろ見えにくくなった。**

**呼び出し側は `bind.go`（package main）にある。** UT-002 が単体テストの対象を `internal/` に限っているため、**検証する場所が無い。**

31 章 31.10 の対応表も、この境界をそのまま記録している。

```
| FR-100 | UT-801 | 表示内容の取得元。表示はフロント側 |
| UI-100 | — (GUI) | |
```

**「取得元」は検証されている。「取得元に何を渡すか」は検証されていない。**

### 4.2 自動 E2E — 情報ダイアログを開かない

40 章の対応表も、FR-100 / UI-100 の検証は手動ケースだけだと記録している。

```
| FR-100 | E2E-301 |  |
| UI-100 | E2E-301 |  |
```

自動 E2E（E2E-101〜109）は配布アーカイブの検査と CLI の確認、および描画スモークテストであり、**ウィンドウを開いて `?` を押す経路が無い。**

`--version` の出力（E2E-102）はアプリケーションのバージョンだけで、`Environment` 行を含まない。

### 4.3 仕様レビューでも見つからなかった

`v4.7.0` / `v4.10.0` / `v4.22.0` で 20 文書の横断レビューを行っているが、**UI-100 の図と IMP-181 の省略規定の食い違いは検出されていない。**

図（`WebView2 120.x` と書いてある）と、実装仕様（「取得できないことがある」）は、**別々の章にあり、どちらも文としては自然に読める。**
「MUST で図に描かれた項目が、実装仕様では条件付き省略になっている」という形の矛盾は、目視のレビューでは見つけにくい。

### 4.4 まとめ

| 層 | 捕まえられたか | 理由 |
| --- | --- | --- |
| 単体テスト | **できない（かつ、見えにくくした）** | 呼び出し側が `bind.go`（対象外。UT-002）。`Environment("")` のテストが空文字を正当化していた |
| 自動 E2E | **できない** | ダイアログを開かない。`--version` には `Environment` が無い |
| 仕様レビュー | **できたはず** | UI-100 の図（MUST）と IMP-181 の省略規定を突き合わせれば矛盾している |
| 手動テスト | **できた** | E2E-301（今回が初回実施） |

---

## 5. 修正方法

### 5.1 Windows — `go-webview2/webviewloader` から取得する（**新しいモジュールは不要**）

`package main` に OS 別のファイルを 1 組足す。`console_windows.go` / `console_other.go` と同じ形である。

```go
// webview_windows.go
//go:build windows

package main

import "github.com/wailsapp/go-webview2/webviewloader"

// webviewVersion はインストールされている WebView2 ランタイムの版を返す（UI-100, IMP-181）。
//
// **Wails v2 の公開ランタイムはこれを返さない**（internal/system が持っている）。
// go-webview2 は Wails 自身が使っている同じパッケージであり、既に依存関係に
// 入っているため、新しいモジュールは増えない。
//
// 取得できない場合は空文字を返す。Environment が区画ごと省く（IMP-181）。
// **ここで起動を止めない**（FR-111）。
func webviewVersion() string {
	v, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString("")
	if err != nil {
		return ""
	}
	return v
}
```

```go
// webview_other.go
//go:build !windows

package main

// webviewVersion は Linux では WebKitGTK の版を返す（UI-100）。
func webviewVersion() string { ... }   // 5.2
```

```go
// bind.go
func (a *App) GetAbout() AboutDTO {
	defer recoverQuiet()

	return newAboutDTO(thirdPartyLicenses, webviewVersion())
}
```

- `go.mod` の `github.com/wailsapp/go-webview2` を `// indirect` から直接依存へ移す（`go mod tidy` が行う）
- **`internal/` には置かない。** IMP-012 が `internal/` を Wails 非依存に保っている。`go-webview2` は Wails 系の Windows 専用パッケージであり、`internal/` に入れると単体テストに OS 依存が持ち込まれる（UT-002）
- **`buildinfo.Environment` の引数の形は変えない。** 空文字のときに省く規定（IMP-181）は、取得に失敗したときの振る舞いとして残す

### 5.2 Linux — WebKitGTK の版

Linux には Windows のような公開ローダが無い。選択肢は 3 つある。

| 案 | 方法 | 評価 |
| --- | --- | --- |
| **A** | `package main` に cgo のファイルを足し、`webkit_get_major_version()` / `_minor_` / `_micro_` を呼ぶ | **正確。** Linux ビルドは既に cgo と WebKitGTK にリンクしている（AR-003, BR-010 の `-tags webkit2_41`）ため、ヘッダは揃っている。ビルドタグの扱いが増える |
| **B** | フロントエンドの `navigator.userAgent` から取り出して Go へ渡す | 両 OS を 1 つの経路で扱える。ただし**バインドメソッドが 1 つ増える**（IMP-300 が経路を絞っている）。UA 文字列は WebKitGTK の実版と一致しないことがある |
| **C** | Linux は空のままとし、UI-100 に「Linux では省略しうる」と明記する | **実装は最小。** ただし `docs/troubleshooting.md` が WebKitGTK 4.0 / 4.1 の取り違えを扱っており、**版が見えることに実用上の価値がある**（下記 NOTE） |

**案 A を推奨する。**

> [!NOTE]
> **Linux こそ表示する価値が大きい。** `docs/troubleshooting.md` の最初の項目は「WebKitGTK 4.1 が入っていません」であり、`README.md` も 4.0 系との取り違えに繰り返し触れている。
> **利用者に「あなたの環境の WebKitGTK は何版か」を見せられるのは、この情報ダイアログだけである。** Windows の WebView2 より、むしろ Linux で効く。

**案 C を採る場合でも、Windows は 5.1 で直す。** E2E-301 は W1 / L1 の両方が対象だが、**W1 の NG は案 C では解消しない。**

### 5.3 採らない案 — 仕様を実装に合わせる（UI-100 から WebView を削る）

**採らない。**

- 5.2 の NOTE のとおり、トラブルシューティングでの実用価値がある
- UI-100 は MUST であり、図にも明示されている。**要求を落とすには、落としてよい理由が要る。** 「取得できない」は誤りだった（2.2）ため、その理由が無い

### 5.4 あわせて直すべき仕様とコメント

**NFR-071 に従い、同じ変更で仕様書も直す。**

| 文書 | 直す内容 |
| --- | --- |
| `11-impl-backend.md` IMP-181 | **「Wails から取得できないことがある」という前提の誤りを削る。** Windows は `go-webview2/webviewloader`、Linux は WebKitGTK の API から取ることを書く。空文字のときに省く規定は**取得失敗時の振る舞い**として残し、そう明記する |
| `13-impl-interface.md` IMP-310 | `GetAbout` の説明に、**WebView の版を取得して渡す**ことを 1 行足す |
| `bind.go` / `internal/buildinfo/vendor.go` | **コメントの「取得できない」を訂正する。** 誤った説明が残っていると、次に読んだ人が同じ判断をする |
| `31-test-cases.md` | UT-801 の備考「表示内容の取得元。表示はフロント側」を、**取得元の検証であって呼び出し側は対象外である**と分かる形に改める（4.1 の IMPORTANT） |
| `docs/specs/README.md` | 改訂履歴に 1 行追加し、版を上げる（NFR-071） |

### 5.5 単体テストについて

`webviewVersion()` は `package main` にあり、**UT-002 により単体テストの対象外**である。これは変えない。

**代わりに、`TestEnvironment` の「WebView のバージョンがない」ケースに、それが異常系であることを明記する。**

```go
// バージョンを取得できなかった場合、区画ごと省く（IMP-181）。
// **これは異常系である。** 通常は webviewVersion()（package main）が
// 値を渡す。ここが常に通る状態になっていないかは E2E-301 が見る。
```

---

## 6. 影響範囲

| 範囲 | 影響 |
| --- | --- |
| 表示 | `Environment` 行が `windows/amd64  Go 1.25.0` の 2 項目で止まる。**行そのものは正しく、末尾に余分な空白も残らない**（`strings.Join` で連結しているため） |
| 機能 | **無い。** 表示だけの問題であり、動作には影響しない |
| ライセンス表示（FR-101, UI-101） | **影響しない。** `Third-party licenses` は `licenses/THIRD_PARTY.md` の全文であり、この値を使わない。E2E-302 も OK |
| Linux | **同じ状態。** ただし今回 L1 では実施していない |
| 深刻度 | **低。** ただし E2E-301 は優先度「高」であり、**UI-100 は MUST である** |
| 利用者への実害 | 不具合の問い合わせ時に WebView の版を確認できない（`docs/troubleshooting.md` の Linux の項目で効く） |

---

## 7. 修正後の検証方法

### 7.1 手動

1. **E2E-301**（W1）— `?` を開き、`Environment` 行が `windows/amd64  Go 1.25.0  WebView2 <版>` の 4 項目になっていること
2. **E2E-301**（L1）— 同じく `WebKitGTK <版>` が出ること（案 A / B を採った場合）
3. **E2E-302 の回帰** — `Third-party licenses` に Graphviz の EPL-2.0 全文が含まれること（BR-040, NFR-051）。**Viz.js / Graphviz / Expat が `Bundled` 行に出ないこと**（UI-100）
4. **異常系** — WebView2 ランタイムを取得できない環境で、`Environment` 行が `windows/amd64  Go 1.25.0` になり、**末尾に `WebView2 ` だけが残らないこと**（IMP-181）

### 7.2 機械的な確認

```bash
go build ./...
go test ./...          # TestEnvironment の 3 ケースが緑のままであること
go vet ./...
go mod tidy && git diff --exit-code go.mod go.sum   # 依存の移動が反映されていること
```

```bash
# 空文字を渡している箇所が残っていないこと
grep -rn "newAboutDTO" --include=*.go .
```

**Linux で案 A を採った場合は、両方のビルドが通ることを確かめる。**

```bash
wails build -platform windows/amd64 -ldflags "-s -w" -o MarkView.exe
wails build -platform linux/amd64 -tags webkit2_41 -ldflags "-s -w" -o MarkView
```

---

## 8. 調査の記録

| 確かめたこと | 結果 |
| --- | --- |
| `GetAbout` が何を渡すか | **リテラルの `""`**（`bind.go` 359） |
| `newAboutDTO` の呼び出し箇所 | **1 か所だけ**（`bind.go` 359） |
| `buildinfo.Environment` の実装 | 正しい。`webviewVersion != ""` のときだけ区画を足す |
| `webviewName()` の OS 分岐 | 正しい（Windows は `WebView2`、他は `WebKitGTK`。AR-001） |
| Wails v2 の公開ランタイムが版を返すか | **返さない**（`pkg/runtime` に該当 API が無い） |
| Wails 自身はどう取っているか | `internal/system/system_windows.go` 32 が `webviewloader.GetAvailableCoreWebView2BrowserVersionString("")` を呼ぶ。**`internal` なので外部から使えない** |
| `webviewloader` は使えるか | **使える。** `github.com/wailsapp/go-webview2/webviewloader` は公開パッケージ（`version.go` 37 / `native_module.go` 70） |
| そのモジュールは依存に入っているか | **入っている**（`go.mod` の `github.com/wailsapp/go-webview2 v1.0.22 // indirect`） |
| 単体テストの状態 | `TestEnvironment` の 3 ケースすべて緑。**空文字の枝も「正しい」として固定している** |
| UI-100 が表示を求めているか | **求めている**（図に `WebView2 120.x`。**MUST**） |
| Linux での取得手段 | 公開の Go API は無い。cgo で `webkit_get_major_version()` 等（案 A）か `navigator.userAgent`（案 B） |
| 利用者向け文書との関係 | `docs/troubleshooting.md` が WebKitGTK 4.0 / 4.1 の取り違えを扱っており、**版の表示に実用価値がある** |
