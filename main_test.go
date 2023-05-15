package main

import (
	"testing"
)

func TestGitClone(t *testing.T) {
	// 不正な owner、repo、token を渡したときにエラーが発生することを確認する
	err := GitClone("", "", "")
	if err == nil {
		t.Errorf("expected error, but got nil")
	}
}
