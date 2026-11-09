package main

// images.go — 画像ファイルの検証用データ（E2E-012。v1.2.0 の FR-027）。
//
//	generated/large-12mb.png   10 MiB 超 50 MiB 以下の、画像として読める PNG（E2E-395 の確認画面）
//	generated/large-60mb.png   50 MiB 超（E2E-395 の上限超過。中身は画像でなくてよい）
//	generated/huge-pixels.png  30000 x 30000 px の 1 色の PNG。ファイルは小さい（E2E-395）
//	generated/edit/swap.png      赤い 400 x 100 の PNG（E2E-236, E2E-394。BUG-026）
//	generated/edit/swap-blue.png 青い 500 x 100 の PNG（swap.png に上書きして使う）
//
// **画像は Go の標準ライブラリで作る**（葉の規約とは関係ないが、外部のツールに頼らない。BR-001）。

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"strings"
)

const (
	// large-12mb.png の画素数。**WebView が確実に解ける画素数にし、圧縮の効かない乱数の画素で大きくする**
	// （E2E-012。4.80.0。単色で画素数を増やして大きくすると、huge-pixels.png と同じく解けないことがある）。
	// 2048 x 1600 x 4 バイトで約 12.5 MiB になる。
	largePNGWidth  = 2048
	largePNGHeight = 1600

	// huge-pixels.png の一辺（E2E-395。v1.2.0 の実現性検討で、W1 / L1 とも固まらず壊れた画像の表示になった大きさ）。
	hugePixels = 30000
)

// writeLargePNG は large-12mb.png を書く。
//
// **α も乱数にする。** すべて不透明だと image/png は α の無い RGB（1 画素 3 バイト）で書き、
// 2048 x 1600 では 10 MiB を割る（実測 9,836,423 バイト）。**圧縮もしない**（png.NoCompression）——
// 大きさを画素数で決める（2048 x 1600 x 4 + 行ごとの 1 バイト）。乱数の種は固定し、作り直しても同じ画像にする。
// 画像ファイルとして開くと、半透明の部分に市松模様が透けて見える（DSP-129）。
func writeLargePNG(w *bufio.Writer) error {
	img := image.NewNRGBA(image.Rect(0, 0, largePNGWidth, largePNGHeight))
	random := rand.New(rand.NewPCG(2026, 1109))

	for i := 0; i < len(img.Pix); i += 8 {
		binary.LittleEndian.PutUint64(img.Pix[i:i+8], random.Uint64())
	}

	encoder := png.Encoder{CompressionLevel: png.NoCompression}

	return encoder.Encode(w, img)
}

// writeOverMaxPNG は large-60mb.png を書く。**画像ではない**（E2E-012 の「中身は画像でなくてよい」）。
// 上限超過の判定は os.Stat の大きさで行い、中身を読まない（FR-016, IMP-102）。
func writeOverMaxPNG(w *bufio.Writer) error {
	line := "This file is larger than 50 MiB and is not a real image (E2E-395, FR-016).\n"
	block := strings.Repeat(line, (1<<16)/len(line)+1)

	for written := int64(0); written < overMaxSize; {
		n, err := w.WriteString(block)
		if err != nil {
			return err
		}

		written += int64(n)
	}

	return nil
}

// writeHugePixelsPNG は huge-pixels.png を書く（30000 x 30000 px の 1 色）。
//
// **image/png では作らない。** 画素を持つ image.Image を作ると 900 MB を超える。パレット 1 色・1 ビットの
// PNG を行ごとに圧縮して書く（行は先頭のフィルタの 1 バイトと 3750 バイトの 0。圧縮すると 100 KB ほど）。
func writeHugePixelsPNG(w *bufio.Writer) error {
	var compressed bytes.Buffer

	z, err := zlib.NewWriterLevel(&compressed, zlib.BestCompression)
	if err != nil {
		return err
	}

	row := make([]byte, 1+(hugePixels+7)/8) // フィルタ 0（None）と、パレットの 0 番を指す 0 のビット
	for range hugePixels {
		if _, err := z.Write(row); err != nil {
			return err
		}
	}

	if err := z.Close(); err != nil {
		return err
	}

	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[0:4], hugePixels)
	binary.BigEndian.PutUint32(header[4:8], hugePixels)
	header[8] = 1  // ビット深度
	header[9] = 3  // パレット
	header[10] = 0 // 圧縮方式（deflate）
	header[11] = 0 // フィルタ方式
	header[12] = 0 // インターレースなし

	if _, err := w.WriteString("\x89PNG\r\n\x1a\n"); err != nil {
		return err
	}

	// パレットの 1 色は GitHub の強調色（#0969da）。解けたときに画像だと分かる色にする。
	chunks := []struct {
		kind string
		data []byte
	}{
		{"IHDR", header},
		{"PLTE", []byte{0x09, 0x69, 0xda}},
		{"IDAT", compressed.Bytes()},
		{"IEND", nil},
	}

	for _, chunk := range chunks {
		if err := writeChunk(w, chunk.kind, chunk.data); err != nil {
			return err
		}
	}

	return nil
}

// writeChunk は PNG のチャンク（長さ・種類・中身・CRC）を書く。CRC は種類と中身にかかる。
func writeChunk(w *bufio.Writer, kind string, data []byte) error {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(data)))

	crc := crc32.NewIEEE()
	crc.Write([]byte(kind)) //nolint:errcheck // hash.Hash の Write はエラーを返さない
	crc.Write(data)         //nolint:errcheck

	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], crc.Sum32())

	for _, part := range [][]byte{length[:], []byte(kind), data, sum[:]} {
		if _, err := w.Write(part); err != nil {
			return err
		}
	}

	return nil
}

// solidPNG は 1 色の PNG を返す（swap.png / swap-blue.png。E2E-236, E2E-394）。
// **大きさを変えておく**——画像ファイルとして表示しているとき、ステータス領域の寸法が変わることで
// 描き直しを見分ける（E2E-394 の確認内容 6）。
func solidPNG(width, height int, c color.NRGBA) (string, error) {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return "", err
	}

	return out.String(), nil
}
