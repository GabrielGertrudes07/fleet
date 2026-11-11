package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/brizzai/fleet/internal/control"
	"github.com/brizzai/fleet/internal/github"
	"github.com/brizzai/fleet/internal/session"
)

func hojeObs(id, title string, st session.Status) *control.Observation {
	return &control.Observation{Row: &session.SessionRow{ID: id + "-1", Title: title}, Status: st}
}

func TestPrintHojeSectionsAndOrder(t *testing.T) {
	waiting := hojeObs("aaaaaaaa", "espera", session.StatusWaiting)
	done := hojeObs("bbbbbbbb", "pronta", session.StatusFinished)
	done.LastReply = "Publicado\n em   produção"
	merged := hojeObs("cccccccc", "mergeada", session.StatusIdle)
	merged.PR = &github.PR{Number: 7, State: "MERGED"}

	var buf bytes.Buffer
	meta := map[string]session.Meta{
		waiting.Row.ID: {Tag: "folha", Note: "espero deploy"},
		done.Row.ID:    {Tag: "folha"},
	}
	printHoje(&buf, []*control.Observation{waiting, done, merged}, 2, meta)
	out := buf.String()

	for _, want := range []string{"Esperando você (1)", "Terminadas (1)", "› Publicado em produção", "PR mergeado: pode encerrar (1)", "#7(merged)", "2 suspensa(s)", "#folha  1/2 prontas · 1 esperando · 0 rodando", "— espero deploy"} {
		if !strings.Contains(out, want) {
			t.Errorf("saída sem %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "Esperando você") > strings.Index(out, "Terminadas") {
		t.Errorf("'Esperando você' deve vir antes de 'Terminadas':\n%s", out)
	}
	if strings.Contains(out, "Rodando") || strings.Contains(out, "Rascunho") {
		t.Errorf("seção vazia não deve aparecer:\n%s", out)
	}
}

func TestPrintHojeNothingPending(t *testing.T) {
	var buf bytes.Buffer
	printHoje(&buf, []*control.Observation{hojeObs("dddddddd", "ociosa", session.StatusIdle)}, 0, nil)
	if !strings.Contains(buf.String(), "Nada pendente.") {
		t.Errorf("esperava 'Nada pendente.', veio:\n%s", buf.String())
	}
}
