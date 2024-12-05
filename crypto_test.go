package main

import (
	"bytes"
	"testing"
)

// func TestNewEncryptionKey(t testing.T) {
// 	for i := 0; i < 1000; i++ {

// 	}
// }

func TestCopyEncryptDecrypt(t *testing.T) {
	payload := "Foo not Bar"
	src := bytes.NewReader([]byte(payload))
	dst := new(bytes.Buffer)
	key := newEncryptionKey()
	_, err := copyEncrypt(key, src, dst)
	if err != nil {
		t.Error(err)
	}

	// fmt.Println("\nEncrypted data:", dst.String())

	out := new(bytes.Buffer)
	if _, err := copyDecrypt(key, dst, out); err != nil {
		print(err)
		t.Error(err)
	}

	if out.String() != payload {
		t.Errorf("decryption failed")
	}
	// fmt.Println("Decrypted data:", out.String())
}
