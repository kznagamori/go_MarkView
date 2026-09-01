# MarkView

[![Latest Release](https://img.shields.io/github/v/release/kznagamori/go_MarkView?label=release)](https://github.com/kznagamori/go_MarkView/releases) ![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux-0078D4)
![UI](https://img.shields.io/badge/UI-Wails%20v2-CF3A2B) ![Language](https://img.shields.io/badge/language-Go-00ADD8) ![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)

## 概要

MarkView は、**Markdown ドキュメントを受け取った側が、追加のセットアップなしに GitHub と
同じ見た目で読める**ようにするためのビューアです。

設計書・手順書・API 仕様といった一次資料は Markdown で書かれ、ZIP や共有フォルダでリポジトリの
外へ渡されます。しかし受け取った側の環境に、それを GitHub と同じ体裁で読む手段があるとは
限りません。テキストエディタでは記法が生のまま出て読みにくく、ブラウザは Markdown を
描画せず、VS Code の導入を相手に強いるのは配布物としては重すぎます。

MarkView はこの隙間を埋めます。**実行ファイル 1 つを Markdown と一緒に配れば、受け取った側は
ダブルクリックするだけで読める。** インストールも、ランタイムの導入も、ネットワーク接続も
要りません。書く道具は既にあるので、MarkView は書きません。読むことだけをします。

> **MarkView** is a lightweight, single-executable Markdown viewer for Windows and Linux.
> No installation, no runtime, no network access. It renders Markdown the way GitHub does.

## 主な機能

- GitHub Flavored Markdown を、GitHub のファイルビューと同じ見た目で描画
- コードブロックのシンタックスハイライト（Go・Python・TypeScript など 40 言語以上）
- GitHub Alerts 5 種（`> [!NOTE]` `> [!TIP]` `> [!IMPORTANT]` `> [!WARNING]` `> [!CAUTION]`）
- Mermaid 図の描画。ライブラリを同梱しているため**オフラインでも図が出る**
- 数式の描画（KaTeX 同梱、`$...$` / `$$...$$` / ` ```math ` の 3 記法）
- 見出しからアウトラインを自動生成し、本文のスクロールに連動して現在位置を強調
- Markdown ファイルとディレクトリだけを並べるファイルツリー（遅延展開、`.git` などは除外）
- **ツリーからファイルを選んでも、ツリーの起点は動かない**
- 相対リンクで文書間を移動し、`Alt+←` で**スクロール位置ごと**元の場所へ戻れる
- 文書内のインクリメンタル検索（全ヒットをハイライト、現在位置 / 総数を表示）
- 表示倍率 50 %〜300 %（`Ctrl` + ホイールにも対応）
- Light / Dark テーマ。**初回は OS の設定に追従**し、一度切り替えるとその選択を記憶
- コードブロックのコピーボタン
- 表示中のファイルが外部で保存されると、**スクロール位置を保ったまま自動で再描画**
- 外部 URL は既定のブラウザ、画像は OS の既定アプリへ委譲。ウィンドウ内で遷移しない
- 開いたファイルのパス・表示履歴・検索語を**ディスクのどこにも書かない**

## インストール

[GitHub Releases](https://github.com/kznagamori/go_MarkView/releases) から、環境に合った
アーカイブをダウンロードして展開します。中身は実行ファイルと `LICENSE`、`README.md` の
3 点だけで、ディレクトリ階層はありません。インストーラはありません。

| ファイル | 対象環境 |
| --- | --- |
| `MarkView-<version>-windows-amd64.zip` | Windows 11 (x64) |
| `MarkView-<version>-windows-arm64.zip` | Windows 11 (ARM64) |
| `MarkView-<version>-linux-amd64.tar.gz` | Ubuntu 24.04+ (x64) |
| `MarkView-<version>-linux-arm64.tar.gz` | Ubuntu 24.04+ (ARM64) |

展開したフォルダは USB メモリなどに入れて持ち運べます。

### Windows 11

`MarkView.exe` をダブルクリックします。それだけです。
表示エンジンには OS 標準の WebView2 を使うため、追加のインストールはありません。

### Ubuntu 24.04 以降

WebKitGTK 4.1 が必要です。多くのデスクトップ環境には既に入っていますが、
起動しない場合は次を実行してください。

```sh
sudo apt install libwebkit2gtk-4.1-0
```

```sh
chmod +x ./MarkView
./MarkView
```

### 起動したときに開くもの

引数なしで起動すると、**カレントディレクトリ → 実行ファイルのあるディレクトリ**の順に
`README.md` を探し、最初に見つかったものを開きます。そのファイルのあるディレクトリが
ファイルツリーの起点になります。

つまり**実行ファイルと `README.md` を同じフォルダに入れて配れば、受け取った側は
ダブルクリックするだけ**で読み始められます。

```sh
MarkView docs/design.md    # そのファイルを開き、docs/ をツリーの起点にする
MarkView docs/             # docs/ を起点にし、直下の README.md を開く
MarkView --version         # バージョンを表示して終了する
MarkView --help            # 使い方を表示して終了する
```

## ドキュメント

| ドキュメント | 内容 |
| --- | --- |
| [使い方](https://github.com/kznagamori/go_MarkView/blob/main/docs/usage.md) | 起動のしかた、文書の開き方、ペイン、検索、倍率、テーマ、ショートカット |
| [対応する Markdown 記法](https://github.com/kznagamori/go_MarkView/blob/main/docs/markdown.md) | 見出しから Mermaid・数式まで、何がどう表示されるか |
| [設定と保存されるもの](https://github.com/kznagamori/go_MarkView/blob/main/docs/settings.md) | 何が保存され、何が保存されないか。保存先と消し方 |
| [困ったときは](https://github.com/kznagamori/go_MarkView/blob/main/docs/troubleshooting.md) | 起動しない、図が出ない、誤検知される、といった場合の対処 |

開発者向けの [仕様書](https://github.com/kznagamori/go_MarkView/tree/main/docs/specs)
（要求・実装・表示・テストの 5 層、20 文書）もあります。実装の判断はすべてそちらに根拠があります。

## 署名について

配布物にコード署名を行っていません。個人開発の OSS として、証明書の取得・維持コストが
運用に見合わないためです。

このため Windows では初回に SmartScreen の警告が出ることがあります。実行する場合は
**「詳細情報」→「実行」** を選んでください。ウイルス対策ソフトが誤検知することもあります。
リリースに添付している `checksums.txt` で配布物が改変されていないことを確認できます。

```sh
sha256sum -c checksums.txt
```

```powershell
Get-FileHash MarkView-<version>-windows-amd64.zip -Algorithm SHA256
```

詳しい手順は
[困ったときは](https://github.com/kznagamori/go_MarkView/blob/main/docs/troubleshooting.md#windows-の警告が出る)
を参照してください。

## ビルド方法

### 必要条件

- [Go](https://go.dev/dl/) 1.25 以上
- [Wails CLI](https://wails.io/) v2 系
- Linux では次のパッケージ

  ```sh
  sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config
  ```

フロントエンドにビルド工程を持たないため、**Node.js は不要**です。

### ビルド手順

```bash
git clone https://github.com/kznagamori/go_MarkView.git
cd go_MarkView

go test ./...

wails build -platform windows/amd64 -ldflags "-s -w" -o MarkView.exe
wails build -platform linux/amd64 -tags webkit2_41 -ldflags "-s -w" -o MarkView
```

実行ファイルは `build/bin/` に生成されます。

> [!IMPORTANT]
> Linux のビルドでは `-tags webkit2_41` が必須です。付け忘れると WebKitGTK 4.0 系に
> リンクされ、Ubuntu 24.04 で起動しない実行ファイルができます。

Linux 版は cgo を必要とするため、Windows ホストからのクロスコンパイルはできません。

## ライセンス

MIT License - 詳細は [LICENSE](LICENSE) を参照してください。

同梱している OSS のライセンス全文は、アプリケーション内の `F1`（アプリケーション情報）から
読めます。

## 開発者

kznagamori

- GitHub: [https://github.com/kznagamori](https://github.com/kznagamori)
