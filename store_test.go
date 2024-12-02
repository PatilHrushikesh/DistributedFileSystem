package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestPathTransformFunc(t *testing.T) {
	key := "momsbestpicture"
	PathKey := CASPathTransformFunc(key)

	expectedOriginalKey := "6804429f74181a63c50c3d81d733a12f14a353ff"
	expectedPathName := "68044/29f74/181a6/3c50c/3d81d/733a1/2f14a/353ff"

	if PathKey.PathName != expectedPathName {
		t.Errorf("have %s want %s", PathKey.PathName, expectedPathName)
	}

	if PathKey.Filename != expectedOriginalKey {
		t.Errorf("have %s want %s", PathKey.Filename, expectedOriginalKey)
	}

}

func TestStoreDeleteKey(t *testing.T) {

	s := newStore()

	defer teardown(t, s)

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("foo_%d", i)

		formattedString := fmt.Sprintf("Some jpg byte %d", i)
		data := []byte(formattedString)

		if err := s.writeStream(key, bytes.NewReader(data)); err != nil {
			t.Error(err)
		}

		if !s.Has(key) {
			t.Errorf("file does not exist %s", key)
		}

		r, err := s.Read(key)
		if err != nil {
			t.Error(err)
		}

		b, _ := io.ReadAll(r)
		fmt.Printf("file content: %s\n", string(b))

		if string(b) != string(data) {
			t.Errorf("want %s have %s", data, b)
		}

		// time.Sleep(5 * time.Second)
		if err := s.Delete(key); err != nil {
			t.Error(err)
		}

		if s.Has(key) {
			t.Errorf("file exists even after delete %s", key)
		}
	}
}

func newStore() *Store {
	opts := StoreOpts{
		PathTransformFunc: CASPathTransformFunc,
	}
	return NewStore(opts)
}

func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Errorf("error in teardown %s", err)
	}
}

// func TestStore(t *testing.T) {
// 	opts := StoreOpts{
// 		PathTransformFunc: CASPathTransformFunc,
// 	}

// 	s := NewStore(opts)

// 	key := "myspecialpicture"
// 	data := []byte("Some jpg byte2")

// 	if err := s.writeStream(key, bytes.NewReader(data)); err != nil {
// 		t.Error(err)
// 	}

// 	r, err := s.Read(key)
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	b, _ := io.ReadAll(r)
// 	fmt.Println(string(b))

// 	if string(b) != string(data) {
// 		t.Errorf("want %s have %s", data, b)
// 	}

// }
