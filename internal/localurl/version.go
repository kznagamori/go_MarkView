package localurl

import "io/fs"

// Version は画像の URL に付ける版を返す（IMP-118, IMP-128, IMP-160。4.75.0）。
//
// 更新時刻（UnixNano）と大きさを小文字の 16 進で "-" でつないだもの（"1000-a" など）。
// 中身は見ない（IMP-102）。配信の検査には使わない（IMP-161）。
//
// TODO(v1.2.0): 未実装（テストの先行実装で置いたスタブ）。
func Version(fi fs.FileInfo) string {
	return ""
}
