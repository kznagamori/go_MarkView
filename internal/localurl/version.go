package localurl

import (
	"io/fs"
	"strconv"
)

// Version は画像の URL に付ける版を返す（IMP-118, IMP-128, IMP-160。4.75.0）。
//
// 更新時刻（UnixNano）と大きさを小文字の 16 進で "-" でつないだもの（"1000-a" など）。
// **中身は見ない**（IMP-102。画像ファイルは中身を読まない）。名前も見ない。配信の検査には使わない
// （IMP-161。assetsrv はクエリを見ない）。
//
// **版を付けるのは、WebView がページの中で一度読んだ画像を、同じ URL なら no-store でも使い回す
// ため**である（BUG-026）。書き換えたファイルは URL が変わって取り直され、書き換えていないファイルは
// URL が変わらず取り直されない。片方だけを見ると、同じ時刻に大きさだけ変わった書き換え（あるいは
// その逆）を見落とす（UT-808 のケース 10）。
//
// **埋め込み画像（IMP-118）と画像ファイル（IMP-128）は、この 1 つの関数で版を作る。** 別々に作ると、
// 同じファイルを指す 2 つの URL が食い違う。fi が nil なら空文字を返す（呼び出し側は、読めない
// ファイルには版を付けない）。
func Version(fi fs.FileInfo) string {
	if fi == nil {
		return ""
	}

	return strconv.FormatInt(fi.ModTime().UnixNano(), 16) + "-" + strconv.FormatInt(fi.Size(), 16)
}
