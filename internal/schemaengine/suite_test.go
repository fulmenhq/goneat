package schemaengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The vendored JSON-Schema-Test-Suite (testdata/json-schema-test-suite, pinned
// revision in PROVENANCE.md) runs through the adapter exactly as callers use
// it. Expected results are never edited. Every case where goneat's policy
// differs from the suite is listed with a reason in suite-deviations.json;
// an unlisted deviation or a listed case that no longer deviates fails.

const (
	suiteRoot      = "testdata/json-schema-test-suite"
	deviationsFile = "testdata/suite-deviations.json"
	remoteBase     = "http://localhost:1234/"
)

type suiteGroup struct {
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Tests       []struct {
		Description string          `json:"description"`
		Data        json.RawMessage `json:"data"`
		Valid       bool            `json:"valid"`
	} `json:"tests"`
}

type deviation struct {
	Case   string `json:"case"`
	Got    string `json:"got"`
	Reason string `json:"reason"`
}

func loadRemotes(t *testing.T) []Resource {
	t.Helper()
	var out []Resource
	dir := filepath.Join(suiteRoot, "remotes")
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		u := remoteBase + filepath.ToSlash(rel)
		// Resources are admitted under their own identity. A remote that
		// declares a different absolute $id is admitted under that $id.
		doc, err := DecodeJSON(raw, 0)
		if err != nil {
			return err
		}
		if id, ok := declaredID(doc); ok && strings.Contains(id, ":") {
			u = stripEmptyFragment(id)
		}
		out = append(out, Resource{URL: u, Bytes: raw})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestJSONSchemaTestSuite(t *testing.T) {
	remotes := loadRemotes(t)
	var files []string
	defaults := map[string]string{"draft4": "draft-04", "draft6": "draft-06", "draft7": "draft-07",
		"draft2019-09": "draft/2019-09", "draft2020-12": "draft/2020-12"}
	for _, draft := range []string{"draft4", "draft6", "draft7", "draft2019-09", "draft2020-12"} {
		root := filepath.Join(suiteRoot, "tests", draft)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".json") {
				files = append(files, p)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(files)

	got := map[string]string{} // case id -> observed deviation
	total, passed := 0, 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var groups []suiteGroup
		if err := json.Unmarshal(raw, &groups); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		rel := strings.TrimPrefix(filepath.ToSlash(f), suiteRoot+"/tests/")
		// The suite assumes the directory's draft for schemas that omit
		// $schema; pass it explicitly as the default.
		def := defaults[strings.SplitN(rel, "/", 2)[0]]
		for gi, g := range groups {
			compiled, cerr := Compile(g.Schema, Options{Resources: remotes, DefaultDraft: def})
			for ti, tc := range g.Tests {
				total++
				id := fmt.Sprintf("%s#%d.%d", rel, gi, ti)
				if cerr != nil {
					var ae *AdmissionError
					if !errors.As(cerr, &ae) {
						t.Fatalf("%s: compile error is not an AdmissionError: %v", id, cerr)
					}
					got[id] = "admission:" + ae.Reason
					continue
				}
				inst, err := DecodeJSON(tc.Data, 0)
				if err != nil {
					t.Fatalf("%s: decode instance: %v", id, err)
				}
				res := compiled.Validate(inst)
				if res.Valid != tc.Valid {
					got[id] = fmt.Sprintf("valid=%v", res.Valid)
					continue
				}
				passed++
			}
		}
	}

	raw, err := os.ReadFile(deviationsFile)
	if err != nil {
		t.Fatal(err)
	}
	var listed []deviation
	if err := json.Unmarshal(raw, &listed); err != nil {
		t.Fatal(err)
	}
	want := map[string]deviation{}
	for _, d := range listed {
		if d.Reason == "" {
			t.Errorf("%s: deviation has no reason", d.Case)
		}
		want[d.Case] = d
	}
	var unlisted []string
	for id, obs := range got {
		d, ok := want[id]
		switch {
		case !ok:
			unlisted = append(unlisted, id+" "+obs)
		case d.Got != obs:
			t.Errorf("%s: deviation is %q, listed as %q", id, obs, d.Got)
		}
	}
	sort.Strings(unlisted)
	for _, u := range unlisted {
		t.Errorf("unlisted deviation: %s", u)
	}
	for id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("listed deviation no longer occurs: %s", id)
		}
	}
	t.Logf("suite cases: %d, matched: %d, listed deviations: %d", total, passed, len(got))
	if os.Getenv("SCHEMAENGINE_DUMP_DEVIATIONS") != "" {
		ids := make([]string, 0, len(got))
		for id := range got {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var dump []deviation
		for _, id := range ids {
			dump = append(dump, deviation{Case: id, Got: got[id], Reason: want[id].Reason})
		}
		b, _ := json.MarshalIndent(dump, "", " ")
		_ = os.WriteFile(os.Getenv("SCHEMAENGINE_DUMP_DEVIATIONS"), append(b, '\n'), 0o600)
	}
}
