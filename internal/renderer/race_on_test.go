//go:build race

package renderer

// raceEnabled は競合検出つきでビルドされたかを示す。
//
// 競合検出は実行時間を 10 倍以上に伸ばす。UT-213 の「時間内に完了する」を、
// 通常のビルドでは厳しく、`-race` では緩く判定するために分けている。
// 予算を race に合わせて一律に緩めると、通常ビルドでの退行を見逃す。
const raceEnabled = true
