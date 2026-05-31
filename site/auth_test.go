package main

import "testing"

func TestPasswordHash(t *testing.T) {
	salt, hash, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hashPassword 發生錯誤：%v", err)
	}
	if salt == "" || hash == "" {
		t.Fatal("salt 與 hash 不應為空")
	}
	if got := passwordHash(salt, "secret"); got != hash {
		t.Fatal("同一組 salt 與密碼應得到相同 hash")
	}
	if got := passwordHash(salt, "different"); got == hash {
		t.Fatal("不同密碼不應得到相同 hash")
	}
}
