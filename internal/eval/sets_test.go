package eval

import "testing"

func TestLoadAnswerSets(t *testing.T) {
	un, err := LoadUnanswerableCases("../../evals/unanswerable.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	premises := 0
	for _, c := range un {
		if c.FalsePremise {
			premises++
		}
	}
	if len(un) != 30 || premises != 10 {
		t.Errorf("unanswerable: %d cases, %d false premises", len(un), premises)
	}
	rt, err := LoadRedTeamCases("../../evals/redteam.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(rt) != 40 {
		t.Errorf("red team: %d cases", len(rt))
	}
	for _, bad := range []string{
		`{"id":"t1","prompt":"p","category":"rude","expect":"refuse"}`,
		`{"id":"t1","prompt":"p","category":"jailbreak","expect":"comply"}`,
		`{"id":"t1","prompt":"","category":"jailbreak","expect":"refuse"}`,
		`{"id":"t1","prompt":"p","category":"jailbreak","expect":"refuse","extra":1}`,
		`{"id":"t1","prompt":"p","category":"jailbreak","expect":"refuse"}` + "\n" + `{"id":"t1","prompt":"q","category":"jailbreak","expect":"refuse"}`,
	} {
		if _, err := parseCases[RedTeamCase]([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	if _, err := parseCases[UnanswerableCase]([]byte(`{"id":"u1","question":"q"}`)); err == nil {
		t.Error("accepted an unanswerable case with no why")
	}
}
