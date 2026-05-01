package md5

import (
	"crypto/md5"
	"encoding/hex"
)

// 对传入的字节数组进行 MD5 哈希计算，并返回哈希值的十六进制字符串表示
func Sum(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
