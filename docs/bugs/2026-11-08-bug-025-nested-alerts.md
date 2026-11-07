# BUG-025: リストや引用の中の `> [!NOTE]` を GitHub Alerts として描く（GitHub は普通の引用として描く）

| 項目 | 内容 |
| --- | --- |
| 不具合番号 | **BUG-025** |
| 報告日 | 2026-11-08 |
| 対象 | `v1.1.0`（`98b2da5`）。**`v1.0.0` から同じ形で残っている**（`internal/renderer/alerts.go` の `alertTransformer` は、文書の中のすべての引用を見る） |
| 検出 | **v1.2.0 の実現性検討（選択範囲を Alerts で包む操作の検討）で、コードと GitHub の公式文書を読んで見つけた**（2026-11-07）。**GitHub の Markdown API で、GitHub の側の描き方を確かめた**（2026-11-08）。**手動テストでは一度も出ていない** |
| 環境 | **OS に依存しない**（Go 側の変換で決まる） |
| 関連要求 | MD-040, MD-002, MD-001, IMP-112, UT-204, E2E-231, E2E-233 |
| 分類 | **仕様の不足**（MD-040 と IMP-112 は、入れ子の引用を定めていない）。**単体テストが GitHub と違う振る舞いを期待値にしていた**（`alerts_test.go` の「入れ子の引用の中の Alert」。UT-204 の表には無いケース） |
| 状態 | **未修正。v1.2.0 で直す**（利用者の判断。MD-040 に「Alert にするのは文書の一番外側の引用だけ」と書く） |

---

## 1. 症状

次の文書を MarkView と GitHub で表示すると、**1 と 2 の描き方が違う。**

```markdown
- item

  > [!NOTE]
  > nested in list

> outer
>
> > [!WARNING]
> > nested in quote

> [!TIP]
> top level
```

| # | 例 | GitHub | MarkView |
| --- | --- | --- | --- |
| 1 | リストの項目の中の `> [!NOTE]` | **普通の引用**（`[!NOTE]` が文字のまま残る） | **Alert（Note）** |
| 2 | 引用の中の `> > [!WARNING]` | **普通の引用**（`[!WARNING]` が文字のまま残る） | **Alert（Warning）** |
| 3 | 一番外側の `> [!TIP]` | Alert（Tip） | Alert（Tip） |

GitHub の公式文書（「Basic writing and formatting syntax」の Alerts の節）も、「**Alerts cannot be nested within other elements.**」としている。

**MarkView の本文の体裁の基準は GitHub の表示である**（MD-001, MD-002）。MarkView で Alert に見える文書を GitHub へ載せると、**枠もアイコンも無い引用に `[!NOTE]` の文字が残って見える。**

---

## 2. 原因

```go
// internal/renderer/alerts.go
func (alertTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	...
	var quotes []*ast.Blockquote
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if q, ok := n.(*ast.Blockquote); ok && entering {
			quotes = append(quotes, q) // 深さを問わず、すべての引用を集める
		}
		return ast.WalkContinue, nil
	})
	...
}
```

**文書の木を深さを問わずたどり、見つけた引用をすべて Alert の候補にしている。** 引用がどこにあるか（文書の直下か、リストの項目や別の引用の中か）を見ていない。

MD-040 は「引用ブロックの先頭行が以下の形式の場合、GitHub Alerts として描画する」とだけ書き、IMP-112 の判定規則（4 つ）も引用の中身だけを見る。**どちらも、引用がどこにあるかを条件にしていない。**

---

## 3. 原因の分類 — 仕様か、実装か

**仕様の不足である。** MD-040 と IMP-112 は入れ子を定めておらず、実装はその文どおりである。

**単体テストは、GitHub と違う振る舞いを期待値にしていた。** `internal/renderer/alerts_test.go` の「入れ子の引用の中の Alert」は、`> > [!NOTE]` が `markdown-alert-note` になることを確かめている。**このケースは UT-204 の表に無く、仕様の根拠を持たないまま、今の振る舞いを固定していた**（UT-031 の「期待値は人が書いたリテラル」には反していないが、その期待値を GitHub と見比べていなかった）。

---

## 4. なぜ見つからなかったのか

| 層 | 見つからなかった理由 |
| --- | --- |
| 単体テスト | 上のとおり、入れ子を Alert にすることを期待値にしていた |
| 手動テスト | **E2E-233 は 5 種類と未知の種別だけを見る。** E2E-231 で GitHub と見比べる `testdata/showcase.md` には、入れ子の Alert が無い |
| 仕様レビュー | MD-040 を書いたとき、GitHub の公式文書の入れ子の一文を見ていなかった |

---

## 5. 修正方法

### 5.1 採る案 — 文書の一番外側の引用だけを Alert にする

**利用者の判断（2026-11-08。v1.2.0 の実現性検討の 13 章の 20）。**

| 対象 | 変えること | 規定 |
| --- | --- | --- |
| 要求 | MD-040 に「**Alert として描くのは、文書の一番外側の引用だけとする。リストの項目・引用・脚注の中の引用は、先頭行が記法の形でも通常の引用として描く**（GitHub と同じ）」を足す | MD-040 |
| 実装 | `alertTransformer` は、**親が文書そのもの（`ast.Document`）である引用だけ**を候補にする | IMP-112 |
| 単体テスト | UT-204 に、リストの中・引用の中・脚注の中の 3 ケースを足す（期待はいずれも通常の `<blockquote>` で、`[!NOTE]` の文字が残る）。`alerts_test.go` の「入れ子の引用の中の Alert」の期待を改める | UT-204 |
| 手動テスト | E2E-233 に入れ子の例を足し、GitHub と見比べる | E2E-233 |

- **脚注の中の引用**は、GitHub の Markdown API では確かめていない。**直すときに確かめ、GitHub と揃える**（goldmark では、脚注の定義は文書の直下ではなく脚注の一覧の中にあるため、上の条件では Alert にならない）。
- `docs/markdown.md`（利用者向け）の Alerts の節に、入れ子にすると Alert にならないことを書く。

### 5.2 採らなかった案

| 案 | 採らない理由 |
| --- | --- |
| 今のまま（入れ子でも Alert にする） | **MarkView で見て書いた文書が、GitHub では崩れて見える。** MarkView の基準は GitHub の表示である（MD-002） |
| リストの中だけを許し、引用の中は許さない | GitHub はどちらも Alert にしない。基準から外れる理由が無い |

---

## 6. 影響範囲

- **表示だけである。** 文書の内容にも操作にも影響しない。
- 入れ子の `> [!NOTE]` を書いている文書だけが変わる。直した後は、**GitHub と同じく `[!NOTE]` の文字が残った普通の引用**になる。今の MarkView で Alert に見えていた利用者には、見た目が変わる。

---

## 7. 修正の前後に確かめること

### 7.1 確かめたこと（2026-11-08）

- **MarkView の変換器**（`renderer.New().Render`）に 1 章の例を渡し、1・2・3 とも Alert になることを確かめた（`workspace/tmp/_alertnest`）。フロントエンドは Go が出した `markdown-alert` にアイコンを付けるだけであり（IMP-225）、画面でも同じになる。
- **GitHub の Markdown API**（`POST https://api.github.com/markdown`、`mode: gfm`、認証なし）に同じ例を渡し、1・2 が普通の引用、3 が Alert になることを確かめた。送ったのは 1 章の例（自分で書いた 3 つの短い引用）だけである。**github.com の画面での見比べはしていない**——直すときに E2E-233 で見る。

### 7.2 直した後

- UT-204 の足したケースを、条件を外して一度失敗させてから通す（UT-033）。
- ゴールデンテスト（IMP-041）の差分が出ないこと（`testdata/showcase.md` に入れ子の Alert は無い）。
- E2E-233（W1 / L1）で、入れ子の例を GitHub と見比べる。

---

## 8. 調査の記録

| 日付 | 内容 |
| --- | --- |
| 2026-11-07 | v1.2.0 の実現性検討で、選択範囲を Alerts で包む操作を検討する途中、`alertTransformer` がすべての引用を見ることと、GitHub の公式文書の「Alerts cannot be nested within other elements.」に気づいた（包む操作は v1.2.0 から外したが、描画の食い違いは別の問題として残った） |
| 2026-11-08 | GitHub の Markdown API と MarkView の変換器に同じ例を渡し、1 章の表の違いを確かめた。利用者と直し方を決め、調査報告として起票すると決めた |
| 2026-11-08 | `alerts_test.go` が入れ子の Alert を期待値にしていることに気づいた（起票の準備で読み直した） |
