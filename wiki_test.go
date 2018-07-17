package main

import "testing"

func TestParseLogLine(t *testing.T) {
	input := `a926492 28 hours ago "asdfasdf asdf"asdfasdf asdf test test!!`
	logFile := parseLog([]byte(input))
	hash := "a926492"
	msg := `"asdfasdf asdf"asdfasdf asdf test test!!`
	time := "28 hours ago"

	if logFile.Hash != hash {
		t.Errorf("Hash mismatch. Expected: %s got %s", hash, logFile.Hash)
	}
	if logFile.Message != msg {
		t.Errorf("Message mismatch. Expected: %s got %s", msg, logFile.Message)
	}
	if logFile.Time != time {
		t.Errorf("Time mismatch. Expected: %s got %s", time, logFile.Time)
	}
}

func TestListDirectories_shouldReturnDirectories(t *testing.T) {
	path := "/test/test2"
	dirs := listDirectories(path)
	expectedLength := 3

	if len(dirs) != expectedLength {
		t.Errorf("Directories size should be %d, was %d", expectedLength, len(dirs))
	}
	if dirs[0].Path != "" {
		t.Errorf("Wrong root path, was %s", dirs[0].Path)
	}
}
