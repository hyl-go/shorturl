package base62

import (
	"math"
	"strings"
)

var base62 string

func MustInt(bs string) {
	if len(bs) == 0 {
		panic("需要传入base62字符串")
	}
	base62 = bs
}

// 十进制数转换为62进制数
func IntToString(seq uint64) string {
	if seq == 0 {
		return string(base62[0])
	}
	bl := []byte{}
	for seq > 0 {
		mod := seq % 62
		div := seq / 62
		bl = append(bl, base62[mod])
		seq = div
	}
	return string(reverse(bl))
}

func StringToInt(s string) (seq uint64) {
	bl := []byte(s)
	bl = reverse(bl)
	for idx, b := range bl {
		base := math.Pow(62, float64(idx))
		seq += uint64(strings.Index(base62, string(b))) * uint64(base)
	}
	return seq
}

func reverse(s []byte) []byte {
	for i, j := 0, len(s)-1; i < len(s)/2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
