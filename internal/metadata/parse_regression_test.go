package metadata

import "testing"

func TestParseHTML_InvalidUTF8BeforeTags(t *testing.T) {
	// Prepend invalid UTF-8 bytes before tags
	data := []byte{0xff, 0xfe, 0xfd}
	data = append(data, []byte(`<html><head><title>Hi</title></head><meta name="description" content="ok"></html>`)...)
	res := parseHTML(data)
	if res.Title != "Hi" {
		t.Errorf("title got %q want %q", res.Title, "Hi")
	}
	if res.Description != "ok" {
		t.Errorf("desc got %q want %q", res.Description, "ok")
	}
}

func TestParseHTML_UnicodeLengthChangingBeforeTags(t *testing.T) {
	// 'İ' (U+0130) lowercases in some cases to forms that can change byte length in UTF-8.
	// Include it before tags to ensure byte-offset based parsing remains stable.
	data := []byte("İİİ<html><head><title>Türkçe</title></head></html>")
	res := parseHTML([]byte(data))
	if res.Title != "Türkçe" {
		t.Errorf("title got %q want %q", res.Title, "Türkçe")
	}
}
