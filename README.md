# MarkView

Markdown を **GitHub と同じ見た目で読むためだけ**のデスクトップアプリケーションです。
実行ファイルは 1 つ。インストールも、ランタイムの導入も、ネットワーク接続も要りません。

> **MarkView** is a lightweight, single-executable Markdown viewer for Windows and Linux.
> No installation, no runtime, no network access. It renders Markdown the way GitHub does.

設計書や手順書を ZIP で渡したとき、受け取った側は「開けばそのまま読める」状態になります。
書く道具は既にあるので、MarkView は**書きません**。読むことだけをします。

## すぐ使う

展開したフォルダに `MarkView`（Windows は `MarkView.exe`）と `LICENSE`、そしてこの
`README.md` が入っています。階層はありません。

### Windows 11

実行ファイルをダブルクリックしてください。それだけです。
表示エンジンには OS 標準の WebView2 を使うため、追加のインストールはありません。

### Ubuntu 24.04 以降

WebKitGTK 4.1 が必要です。多くのデスクトップ環境では既に入っていますが、
起動しない場合は次を実行してください。

```sh
sudo apt install libwebkit2gtk-4.1-0
```

実行ビットが落ちている場合は付け直します。

```sh
chmod +x ./MarkView
./MarkView
```

### 起動したとき何が開くか

引数なしで起動すると、次の順に `README.md` を探して最初に見つかったものを開きます。
そのファイルのあるディレクトリが、ファイルツリーの起点になります。

1. コマンドを実行したディレクトリ
2. 実行ファイルが置かれているディレクトリ

つまり**実行ファイルと `README.md` を同じフォルダに入れて配れば、受け取った側は
ダブルクリックするだけ**で読み始められます。いま表示されているのがまさにその状態です。

引数を渡すこともできます。

```sh
MarkView docs/design.md    # そのファイルを開き、docs/ をツリーの起点にする
MarkView docs/             # docs/ を起点にし、直下の README.md を開く
MarkView --version         # バージョンを表示して終了する
MarkView --help            # 使い方を表示して終了する
```

## 使い方

### 文書を開く

開く経路は 6 つだけです。

| 経路 | 操作 |
| --- | --- |
| ダイアログ | ツールバーの Open、または `Ctrl+O` |
| ドラッグ＆ドロップ | ウィンドウへファイルかフォルダを落とす |
| コマンドライン引数 | 上記のとおり |
| ファイルツリー | 左ペインから選ぶ。**選んでもツリーの起点は動きません** |
| 本文中のリンク | `.md` へのリンクは同じウィンドウで開く |
| 表示履歴 | `Alt+←` / `Alt+→` |

扱う拡張子は `.md` `.markdown` `.mdown` `.mkd` です。
外部 URL は既定のブラウザへ、画像や PDF は OS の既定のアプリへ渡します。
**ウィンドウの中で別のサイトへ遷移することはありません。**

### キーボード

| キー | 動作 |
| --- | --- |
| `Ctrl+O` | ファイルを開く |
| `F5` / `Ctrl+R` | 再読み込み |
| `Ctrl+Shift+E` | ファイルツリーの表示 / 非表示 |
| `Ctrl+Shift+O` | アウトラインの表示 / 非表示 |
| `Ctrl+F` | 文書内検索（`Enter` / `Shift+Enter` で次 / 前、`Esc` で閉じる） |
| `Alt+←` / `Alt+→` | 表示履歴を戻る / 進む |
| `Ctrl` `+` / `Ctrl` `-` / `Ctrl+0` | 拡大 / 縮小 / 100 % に戻す |
| `Ctrl+Shift+T` | Light / Dark の切り替え |
| `Ctrl+C` | 選択範囲をコピー |
| `F1` | バージョンとライセンスの情報 |
| `Ctrl+Q` / `Alt+F4` | 終了 |

### 表示中のファイルが変わったら

エディタで保存すると、**スクロール位置を保ったまま自動で描き直します**。
別のアプリで書いている文書の仕上がりを、隣に置いて確かめられます。

## 読める Markdown

GitHub Flavored Markdown に沿っています。

- 見出し・リスト・表・引用・タスクリスト・脚注・取り消し線・絵文字
- コードブロックのシンタックスハイライト
- GitHub Alerts（`> [!NOTE]` `> [!TIP]` `> [!IMPORTANT]` `> [!WARNING]` `> [!CAUTION]`）
- Mermaid 図（`mermaid` のコードブロック）
- 数式（`$...$` と `$$...$$`、および `math` のコードブロック）

Mermaid と KaTeX は実行ファイルに埋め込んであり、**使う文書を開いたときだけ**
読み込みます。オフラインでも図と数式は欠けません。

10 MB を超えるファイルは、描画の前に確認画面を出します。50 MB を超えるものは開きません。

## 残らないもの

MarkView は**閲覧の痕跡をディスクに残しません**。

| | 内容 |
| --- | --- |
| **残さないもの** | 開いたファイルのパス、ツリーの起点、表示履歴、検索語、ウィンドウの位置 |
| **残すもの** | テーマ、表示倍率、ペインの表示状態と幅、ウィンドウの大きさ |

設定の保存先は**テンポラリディレクトリのみ**です（Windows は `%TEMP%\MarkView\`、
Linux は `$TMPDIR/MarkView-<uid>/`）。`%APPDATA%` にも `~/.config` にも
レジストリにも書きません。消えても既定値で起動するだけです。

ネットワークへは接続しません。更新確認もテレメトリもクラッシュレポートもありません。
唯一の例外は、文書が `https://` の画像を参照している場合に WebView がそれを取りに行くことです。

## 署名していません

配布物にコード署名を行っていません。個人開発の OSS として証明書の維持費が見合わないためです。

このため Windows では初回に SmartScreen の警告が出ることがあります。
実行する場合は「詳細情報」→「実行」を選んでください。ZIP から展開したファイルに
警告が残る場合は、ファイルのプロパティを開き「セキュリティ: このファイルは
他のコンピューターから取得したものです」の**「ブロックの解除」**にチェックを入れます。

ウイルス対策ソフトが誤検知することもあります。配布物は実行ファイル圧縮（UPX 等）を
使っていませんが、署名のない未知の実行ファイルは検知対象になりやすいためです。
気になる場合は `checksums.txt` の SHA-256 と照合してください。

```sh
sha256sum -c checksums.txt
```

```powershell
Get-FileHash MarkView-<version>-windows-amd64.zip -Algorithm SHA256
```

## `.md` の関連付けについて

MarkView は**関連付けを自分で登録しません**。レジストリや `~/.local/share/applications`
への書き込みを伴い、「システムに痕跡を残さない」方針と衝突するためです。

利用者が自分で設定するのは構いません。Windows なら `.md` ファイルを右クリックして
「プログラムから開く」→「別のプログラムを選択」から `MarkView.exe` を選びます。
その経路で起動しても、引数付き起動として正しく動作します。

## ライセンス

MIT License。同梱している OSS のライセンス全文は、アプリ内の `F1`
（アプリケーション情報）から読めます。

## 開発者向け

Go + [Wails v2](https://wails.io/) で作っています。フロントエンドに Node.js も
バンドラも UI フレームワークも使いません。

```sh
git clone https://github.com/kznagamori/go_MarkView.git
cd go_MarkView

go test ./...

wails build -platform windows/amd64 -ldflags "-s -w" -o MarkView.exe
wails build -platform linux/amd64 -tags webkit2_41 -ldflags "-s -w" -o MarkView
```

Linux のビルドには `-tags webkit2_41` が必須です。付け忘れると 4.0 系にリンクされ、
Ubuntu 24.04 で起動しない実行ファイルができます。ビルドには次のパッケージが要ります。

```sh
sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config
```

仕様書は
[`docs/specs/`](https://github.com/kznagamori/go_MarkView/tree/main/docs/specs)
にあります（要求・実装・表示・テストの 5 層、20 文書）。
実装の判断はすべてそちらに根拠があります。
