package sbom

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSourceJSONPreservesUnknownValues(t *testing.T) {
	input := []byte(`{"unknown":{"big":9007199254740993,"decimal":1.234567890123456789,"exponent":1e999,"negativeZero":-0,"empty":[],"nil":null},"name":"λ\ud83d\ude00","literal":"\\ud800"}`)
	document, err := decodeSourceJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"9007199254740993", "1.234567890123456789", "1e999", "-0"} {
		if !strings.Contains(string(encoded), token) {
			t.Fatalf("numeric token %s lost: %s", token, encoded)
		}
	}
	roundtrip, err := decodeSourceJSON(encoded)
	if err != nil || !reflect.DeepEqual(document, roundtrip) {
		t.Fatalf("unknown values changed: %v", err)
	}
}

func TestSourceJSONRejectsAmbiguousDocuments(t *testing.T) {
	for _, input := range []string{
		`{"name":"one","name":"two"}`,
		`{"nested":{"name":1,"\u006eame":2}}`,
		`{} {}`, `[]`, `{"a":`, `{"a":[1,]}`,
		`{"a":"\ud800"}`, `{"a":"\udc00"}`, `{"a":"\ud800\u0041"}`,
		"{\"a\":\"\xff\"}",
		`{"a":` + strings.Repeat("[", 258) + "0" + strings.Repeat("]", 258) + "}",
	} {
		if _, err := decodeSourceJSON([]byte(input)); err == nil {
			t.Fatalf("accepted invalid or ambiguous document %q", input)
		}
	}
}
