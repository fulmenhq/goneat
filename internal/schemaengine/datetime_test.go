package schemaengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// ownedFormatCorpusSHA256 pins the reviewed date/time/date-time corpus
// (owned-format-corpus/v1, contract structural-leap/v1). Changing the corpus
// needs a new version and a new review, not an updated digest.
const ownedFormatCorpusSHA256 = "0703c50dad40dc30cb0ccf8d597fc9aa95381911e8dd009dad912d033a8e5924"

type formatCase struct {
	ID       string `json:"id"`
	Format   string `json:"format"`
	Instance any    `json:"instance"`
	Valid    bool   `json:"valid"`
	Rule     string `json:"rule"`
}

func loadFormatCorpus(t *testing.T, path, wantSHA string) []formatCase {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != wantSHA {
		t.Fatalf("corpus %s sha256 = %s, want %s", path, got, wantSHA)
	}
	var doc struct {
		Cases []formatCase `json:"cases"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode corpus: %v", err)
	}
	if len(doc.Cases) == 0 {
		t.Fatal("corpus has no cases")
	}
	return doc.Cases
}

func TestOwnedDateTimeFormatsCorpus(t *testing.T) {
	cases := loadFormatCorpus(t, "testdata/owned-format-corpus-v1.json", ownedFormatCorpusSHA256)
	checkers := map[string]func(any) error{
		"date":      checkDate,
		"time":      checkTime,
		"date-time": checkDateTime,
	}
	counts := map[string]int{}
	for _, c := range cases {
		check, ok := checkers[c.Format]
		if !ok {
			t.Fatalf("%s: unexpected format %q", c.ID, c.Format)
		}
		counts[c.Format]++
		err := check(c.Instance)
		if (err == nil) != c.Valid {
			t.Errorf("%s %s %q (%s): got err=%v, want valid=%v", c.ID, c.Format, c.Instance, c.Rule, err, c.Valid)
		}
	}
	if len(cases) != 15498 {
		t.Errorf("corpus case count = %d, want 15498", len(cases))
	}
	t.Logf("cases per format: %v", counts)
}
