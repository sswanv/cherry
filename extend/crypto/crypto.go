package cherryCrypto

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"hash/crc32"
)

func MD5(value string) string {
	data := []byte(value)
	return MD5WithBytes(data)
}

func MD5WithBytes(bytes []byte) string {
	has := md5.Sum(bytes)
	return fmt.Sprintf("%x", has)
}

func Base64Encode(value string) string {
	data := []byte(value)
	return base64.StdEncoding.EncodeToString(data)
}

func Base64Decode(value string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Base64DecodeBytes(value string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func CRC32(value string) uint32 {
	return crc32.ChecksumIEEE([]byte(value))
}
