# BUG-022: GitHub が水平線を 1px に変え、MarkView の 4px の帯と食い違った

| 項目 | 内容 |
| --- | --- |
| 不具合番号 | **BUG-022** |
| 報告日 | 2026-10-04 |
| 対象 | **MarkView の側は v1.0.0 から変わっていない**（水平線は 0.25em の帯）。**GitHub の側が変えた**。いつ変えたかは分からない（`v1.1.0-rc.2` の E2E-231 は 2026-09-27 に OK だった） |
| 検出 | **`v1.1.0-rc.3` の手動テスト E2E-231**（利用者。2026-10-04。手順 2 の水平線） |
| 環境 | W1 / L1 |
| 関連要求 | FR-020, MD-001, MD-002, MD-025, DSP-121, DSP-010, E2E-231 |
| 分類 | **GitHub の側の変更への追随**（MarkView の退行ではない。判定基準が GitHub との見比べである以上、GitHub が変われば差が出る。MD-002） |
| 状態 | **修正済み（仕様 4.72.0）**。**修正後の実機での確認は rc.4 の手動テスト**（E2E-231 の手順 2） |

---

## 1. 症状

**GitHub 上の `showcase.md` の水平線が、MarkView の水平線より細い**（利用者の報告）。

> GitHubで2本目の線が細くなっている。

「2 本目の線」は、**上から数えて 2 本目**——1 本目が `# MarkView Showcase` の見出しの下の境界線、**2 本目が最初の水平線**（`---`）である（利用者に確かめた）。MarkView では太い帯、GitHub では細い線に見える。

## 2. 原因

**GitHub が、水平線の高さを 0.25em（4px）から 1px に変えていた。**

2026-10-04 に、GitHub 上の `testdata/showcase.md`（`v1.1.0-rc.3` のタグ）のページが読み込むスタイルシートを取得して確かめた。

```css
/* GitHub の現在の CSS（primer-4136ede8b2650a2d.css） */
.markdown-body hr{height:1px;margin:var(--base-size-24) 0;background-color:var(--borderColor-default,var(--color-border-default));border:0;padding:0}

/* MarkView が手本にした版（github-markdown-css 5.9.0） */
.markdown-body hr { height: .25em; padding: 0; margin: var(--base-size-24) 0; background-color: var(--borderColor-default); border: 0; }
```

MarkView の `markdown.css` は `github-markdown-css` 5.9.0 を手本にしており、DSP-121 も「GitHub と同じ 0.25em（倍率 100 % で 4px）」と書いていた。**仕様を書いた時点では GitHub と一致していた。**

### 2.1 ほかに変わったものが無いか

同じ E2E-231 で次の rc に別の差が出ないよう、**GitHub の現在の CSS の `.markdown-body` の規則すべて**を、手本にした版と突き合わせた（規則の数は 232 と 268）。

- **同じセレクタで値が違うものは 49 個あったが、見た目に効くのは `hr` の `height` だけだった。** 残りは書き方の違いである（手本の版は、GitHub が全体に掛けている基本の規則を `.markdown-body` の下へ写している。`word-wrap` と `overflow-wrap`、`rgba(0,0,0,0)` と `#0000` など）
- **色のトークン**は、GitHub の本文に対応する値のある 16 個を Light / Dark の両方で比べた。**ずれていたのは 2 つ**である（透明度を持つ値は背景に重ねた色で比べ、丸めの 1 段の差は数えない）。**アプリの画面だけで使うトークン**（`--fg-subtle` / `--canvas-overlay` / `--danger-subtle` / `--search-hit`）は MarkView が自分で決めた値であり、見比べの対象ではない

| トークン | テーマ | MarkView | GitHub の現在の値 | 使う場所 |
| --- | --- | --- | --- | --- |
| `--fg-default` | Dark | `#e6edf3` | `#f0f6fc` | 本文の文字。**アプリ全体の文字にも使う** |
| `--border-muted` | Light | `#d8dee4` | `#d1d9e0b3`（`#d1d9e0` の 70 %。白地に重ねると `#dfe4e9`） | `h1` / `h2` の下の境界線、`kbd` の枠、右クリックメニューの区切り、拡大画面の台紙の縁 |

どちらも並べても気づきにくい差であり、今回の NG ではない。**利用者の判断で、あわせて追随した。**

Dark の `--border-muted`（`#2f353d`）は、GitHub の `#3d444db3` を Dark の背景（`#0d1117`）に重ねた色と一致しており、変えていない。

## 3. なぜ今まで出ていなかったか

**GitHub が変える前は一致していた。** `v1.0.0` の rc.2〜rc.4 と `v1.1.0` の rc.1・rc.2 では、E2E-231 で水平線の差は報告されていない。

**GitHub は予告なく体裁を変える。** MarkView の判定基準は「GitHub 上の表示と見比べる」（MD-002）であり、**GitHub が変われば、MarkView を何も変えていなくても NG になる。** Mermaid の見た目（[BUG-021](2026-09-28-bug-021-mermaid-neo-look.md)）で決めた「GitHub に追従する」と同じ扱いにする。

## 4. 直し方

**GitHub に追随する**（利用者の判断。2026-10-04）。

```css
.markdown-body hr {
  height: 1px;          /* 4.71.0 までは var(--md-4)（0.25em） */
  padding: 0;
  margin: var(--md-24) 0;
  background-color: var(--color-border-default);
  border: 0;
}
```

- **倍率に追随しない**（GitHub も px で定めている）。上下の余白は変えない
- **脚注の区切りの上書き（`.footnotes > hr { height: 1px }`）を外した。** 本文の水平線が 4px だったときに、脚注の区切りだけを GitHub に合わせて細くするためのものだった。本文の側が 1px になり、要らなくなった
- `tokens.css` の Dark の `--fg-default` を `#f0f6fc` に、Light の `--border-muted` を `#dfe4e9` にした。**`--border-muted` は透明度を持たせず、白地に重ねた色を単色で持つ**（Dark の値と同じ持ち方）
- **MD-002 に「基準は GitHub の『いまの』表示であり、GitHub が変えたら追随する」を書いた**

**41 章は改訂していない。** E2E-231 の手順 2 が水平線を見ており、今回もそこで見つかった。

## 5. 影響

**表示だけである。** データにも操作にも影響しない。

- 水平線が細くなる（W1 / L1 とも）
- **Dark でアプリ全体の文字がわずかに明るくなる**（ツールバー・ペイン・ステータス領域を含む）
- Light で `h1` / `h2` の下の境界線、`kbd` の枠、右クリックメニューの区切り、拡大画面の台紙の縁がわずかに薄くなる

## 6. 直した後に確かめること

| 何を | どこで | 結果 |
| --- | --- | --- |
| 水平線の高さが 1px（倍率 50 % / 100 % / 200 %。脚注の区切りも）、余白 24px、色が `--border-default` | 本番の CSS を読む DOM 検査（`workspace/tmp/_hrcheck`。Chromium。31 項目） | **OK**（2026-10-04） |
| Dark の本文・見出し・コードの演算子の文字色が `#f0f6fc`、Light の `h1` / `h2` / `kbd` の線が `#dfe4e9` | 同上 | **OK**（2026-10-04） |
| 3 つの変更をそれぞれ元に戻すと検査が落ちる | CSS を 4 通りに戻して実行（UT-033 と同じ考え方） | **4 通りとも落ちた**（2026-10-04） |
| **水平線の太さが GitHub と同じ** | **rc.4 の E2E-231 の手順 2**（W1 / L1） | 未 |
| Dark の文字色と Light の境界線に違和感が無い | rc.4 の E2E-231 / E2E-281 ほか（アプリ全体に効く） | 未 |
