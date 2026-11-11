package session

import "testing"

func TestSessionMetaRoundTrip(t *testing.T) {
	db, err := Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.SetSessionMeta("a", Meta{Tag: "folha", Note: "espero o deploy"}); err != nil {
		t.Fatal(err)
	}
	got, err := db.LoadSessionMeta()
	if err != nil || got["a"] != (Meta{Tag: "folha", Note: "espero o deploy"}) {
		t.Fatalf("got %+v err %v", got, err)
	}
	if err := db.SetSessionMeta("a", Meta{}); err != nil {
		t.Fatal(err)
	}
	got, _ = db.LoadSessionMeta()
	if _, ok := got["a"]; ok {
		t.Fatalf("empty meta must delete the row, got %+v", got)
	}
}
