package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/opener"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// 本ファイルは本文中のリンクを踏んだときの判定を持つ（IMP-312）。
//
// **判定はすべて Go 側で行う**（IMP-300）。フロントエンドは href をそのまま
// 渡し、結果を描画するだけである。パスの解釈・種類の判定・存在確認を
// JavaScript でやり直さない。

// linkRef はリンクの生値を分解した結果。
type linkRef struct {
	scheme   string // 空ならローカルのパス
	path     string // scheme が空のときの参照（URL 復号済み）
	fragment string // # 以降。アンカー
}

// followLink はリンクの種類を判定し、必要な処理を行う（IMP-312, FR-050）。
//
// 判定の順序は IMP-312 が固定している。アンカー → スキーム → パス、の順で、
// 前段で決まったものは後段を見ない。
func (a *App) followLink(href string) LinkResultDTO {
	ref, err := parseLinkRef(href)
	if err != nil {
		return linkFailure(href, errLinkNotFound)
	}

	// 1. 同一文書内のアンカー（#section）。
	if ref.scheme == "" && ref.path == "" {
		if ref.fragment == "" {
			return linkFailure(href, errLinkNotFound)
		}
		return LinkResultDTO{Kind: linkAnchor, Anchor: ref.fragment}
	}

	// 2. スキームを持つものは OS へ委譲する（FR-050）。
	//
	// 受け付けるのは http / https / mailto だけである（IMP-170, NFR-030）。
	// それ以外は拒否され、ステータスに表示される。javascript: や file: を
	// OS へ渡さないことのほうが、対応スキームを増やすことより優先される。
	if ref.scheme != "" {
		if err := opener.OpenURL(href); err != nil {
			return linkFailure(href, err)
		}
		return LinkResultDTO{Kind: linkExternal}
	}

	// 3. ローカルのパス。基準は表示中の文書のディレクトリ（AR-042）。
	abs := renderer.ResolveRef(ref.path, a.currentDir())

	if _, err := os.Stat(abs); err != nil {
		return linkFailure(href, errLinkNotFound)
	}

	// 4. Markdown なら表示中のウィンドウで開く。それ以外（画像を含む）は
	//    OS の既定アプリケーションへ委譲する（FR-053）。
	if !mdfile.IsMarkdown(abs) {
		if err := opener.OpenFile(abs); err != nil {
			return linkFailure(href, err)
		}
		return LinkResultDTO{Kind: linkExternal}
	}

	dto, err := a.open(openRequest{path: abs, src: openFromLink, anchor: ref.fragment})
	if err != nil {
		return linkFailure(abs, err)
	}

	return LinkResultDTO{Kind: linkDocument, Document: dto}
}

// linkFailure は失敗を LinkResultDTO へ写す（IMP-305, IMP-315）。
//
// **ErrorDTO をそのまま載せる。** リンク先が大きな Markdown だった場合、
// Kind は needs-confirm になり、フロントエンドは他の経路と同じ確認画面を
// 出せる（FR-016）。文言だけを渡していたときは、サイズと上限が失われて
// 確認画面を組み立てられなかった。
func linkFailure(target string, err error) LinkResultDTO {
	return LinkResultDTO{Kind: linkError, Error: newErrorDTO(target, err)}
}

// parseLinkRef はリンクの生値を分解する（IMP-312）。
//
// **Windows のドライブレターをスキームと取り違えない。** url.Parse は
// "C:/docs/a.md" のスキームを "c" と解釈する。1 文字のスキームは存在しない
// ため、これをローカルパスとして扱う。
func parseLinkRef(href string) (linkRef, error) {
	href = strings.TrimSpace(href)
	if href == "" {
		return linkRef{}, fmt.Errorf("empty link")
	}

	u, err := url.Parse(href)
	if err != nil {
		// 解析できない href はパスとして扱う。Markdown の宛先は URL とは
		// 限らず、Windows のパスをそのまま書いた文書がありうる。
		path, fragment, _ := strings.Cut(href, "#")
		return linkRef{path: path, fragment: fragment}, nil
	}

	if len(u.Scheme) > 1 {
		return linkRef{scheme: strings.ToLower(u.Scheme), fragment: u.Fragment}, nil
	}

	// スキームなし、またはドライブレター。Opaque は "mailto:a@b" のように
	// // を持たない形で埋まる。ここへ来るのはドライブレターの場合だけである。
	path := u.Path
	if path == "" {
		path = u.Opaque
	}
	if len(u.Scheme) == 1 {
		path = u.Scheme + ":" + path
	}

	return linkRef{path: path, fragment: u.Fragment}, nil
}

// currentDir は相対パスの基準となるディレクトリを返す（AR-042）。
//
// 表示中の文書のディレクトリであり、ツリールートではない。文書がリンクを
// たどって移動した場合も、その時点で表示している文書の位置が基準となる。
func (a *App) currentDir() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.current == nil {
		return a.treeRoot
	}
	return filepath.Dir(a.current.Path)
}
