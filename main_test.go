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

func TestCdRepository(t *testing.T) {
	// 不正な repo を渡したときにエラーが発生することを確認する
	err := CdRepository("")
	if err == nil {
		t.Errorf("expected error, but got nil")
	}
}

func TestSplitRepositoryName(t *testing.T) {
	// 不正な repository を渡したときにエラーが発生することを確認する
	_, _, err := SplitRepositoryName("a/b/c")
	if err == nil {
		t.Errorf("expected error, but got nil")
	}

	// 正しい repository を渡したときにエラーが発生しないことを確認する
	_, _, err = SplitRepositoryName("a/b")
	if err != nil {
		t.Errorf("expected nil, but got %v", err)
	}
}
