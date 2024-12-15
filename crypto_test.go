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
	nw, err := copyDecrypt(key, dst, out)
	if err != nil {
		print(err)
		t.Error(err)
	}

	if nw != 16+len(payload) {
		t.Fail()
	}
	if out.String() != payload {
		t.Errorf("decryption failed")
	}
	// fmt.Println("Decrypted data:", out.String())
}
