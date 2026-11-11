package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/brizzai/fleet/internal/control"
	"github.com/brizzai/fleet/internal/session"
)

// `fleet hoje` — the start-of-day / "where was I" view: what waits on you,
// what finished (with the agent's closing reply as context), what is still
// running, which PRs merged (sessions you can close), which sessions hold an
// unsent draft, and per-tag progress. One-shot and read-only; it only reads the
// state `fleet status` already observes plus the user's tags/notes.

const hojeUsage = "Usage: fleet hoje"

func runHoje(args []string) {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintln(os.Stdout, hojeUsage)
		return
	}
	st := openCLIState()
	defer st.close()

	var observed []*session.SessionRow
	suspended := 0
	for _, r := range st.rows {
		if session.Status(r.Status) == session.StatusSuspended {
			suspended++
			continue
		}
		observed = append(observed, r)
	}
	obs := st.observer(1).Observe(observed)
	sortObservations(obs)
	meta, _ := st.storage.LoadSessionMeta()
	printHoje(os.Stdout, obs, suspended, meta)
}

func printHoje(w io.Writer, obs []*control.Observation, suspended int, meta map[string]session.Meta) {
	var waiting, finished, running, merged, drafts []*control.Observation
	for _, ob := range obs {
		if ob.PR != nil && ob.PR.State == "MERGED" {
			merged = append(merged, ob)
		}
		if ob.View.Input.Kind == session.InputDraft {
			drafts = append(drafts, ob)
		}
		switch {
		case ob.Pane == control.PaneShell || ob.Status == session.StatusError:
			waiting = append(waiting, ob)
		case ob.Status == session.StatusWaiting:
			waiting = append(waiting, ob)
		case ob.Status == session.StatusFinished:
			finished = append(finished, ob)
		case ob.Status == session.StatusRunning || ob.Status == session.StatusStarting:
			running = append(running, ob)
		}
	}

	section := func(title string, list []*control.Observation, line func(*control.Observation) string) {
		if len(list) == 0 {
			return
		}
		fmt.Fprintf(w, "%s (%d)\n", title, len(list))
		for _, ob := range list {
			fmt.Fprintf(w, "  %s  %s\n", hexID(ob.Row.ID), line(ob))
		}
		fmt.Fprintln(w)
	}
	since := func(ob *control.Observation) string {
		if ob.Since.IsZero() {
			return ""
		}
		return " · há " + humanDur(time.Since(ob.Since))
	}
	// label puts the user's tag and note beside a title.
	label := func(ob *control.Observation) string {
		out := clip(ob.Row.Title, 40)
		m := meta[ob.Row.ID]
		if m.Tag != "" {
			out = "#" + m.Tag + " " + out
		}
		if m.Note != "" {
			out += "  — " + clip(m.Note, 50)
		}
		return out
	}
	oneLine := func(s string, n int) string {
		return clip(strings.Join(strings.Fields(s), " "), n)
	}

	section("Esperando você", waiting, func(ob *control.Observation) string {
		extra := ""
		if m := ob.View.Menu; m != nil {
			extra = "\n          ? " + oneLine(menuLine(m, 30), 110)
		} else if ob.Exit != "" {
			extra = "\n          ! processo encerrado (" + ob.Exit + ")"
		}
		return label(ob) + " [" + statusWord(ob) + "]" + since(ob) + extra
	})
	section("Terminadas", finished, func(ob *control.Observation) string {
		extra := ""
		if r := oneLine(ob.LastReply, 110); r != "" {
			extra = "\n          › " + r
		}
		return label(ob) + since(ob) + extra
	})
	section("Rodando", running, func(ob *control.Observation) string {
		return label(ob) + since(ob)
	})
	section("PR mergeado: pode encerrar", merged, func(ob *control.Observation) string {
		return label(ob) + "  " + prShort(ob.PR) + "  " + clip(ob.Branch, 30)
	})
	section("Rascunho não enviado", drafts, func(ob *control.Observation) string {
		return label(ob) + "  ✎ " + oneLine(ob.View.Input.Text, 60)
	})

	// Per-tag progress: a tag groups the sessions of one effort (a plan, a
	// subject), so "2 of 5 done" is visible without opening each.
	type tally struct{ total, waiting, running, done int }
	byTag := map[string]*tally{}
	var tags []string
	for _, ob := range obs {
		tg := meta[ob.Row.ID].Tag
		if tg == "" {
			continue
		}
		t := byTag[tg]
		if t == nil {
			t = &tally{}
			byTag[tg] = t
			tags = append(tags, tg)
		}
		t.total++
		switch {
		case ob.Pane == control.PaneShell || ob.Status == session.StatusError || ob.Status == session.StatusWaiting:
			t.waiting++
		case ob.Status == session.StatusRunning || ob.Status == session.StatusStarting:
			t.running++
		default:
			t.done++
		}
	}
	if len(tags) > 0 {
		sort.Strings(tags)
		fmt.Fprintf(w, "Por tag (%d)\n", len(tags))
		for _, tg := range tags {
			t := byTag[tg]
			fmt.Fprintf(w, "  #%s  %d/%d prontas · %d esperando · %d rodando\n", tg, t.done, t.total, t.waiting, t.running)
		}
		fmt.Fprintln(w)
	}

	if len(waiting)+len(finished)+len(running)+len(merged)+len(drafts) == 0 {
		fmt.Fprintln(w, "Nada pendente.")
	}
	if suspended > 0 {
		fmt.Fprintf(w, "%d suspensa(s) (fleet status -all)\n", suspended)
	}
}
