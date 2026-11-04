package domain

import (
	"errors"
	"testing"
	"time"
)

func TestAnswerReceipt_FirstAnswerWins(t *testing.T) {
	at := time.Date(2026, 11, 4, 15, 0, 0, 0, time.UTC)
	o := Order{State: StatePending}
	if _, err := o.AnswerReceipt(true, at); !errors.Is(err, ErrReceiptNotReceived) {
		t.Fatalf("sin recibir: %v", err)
	}
	o.State = StateReceived
	if changed, err := o.AnswerReceipt(false, at); err != nil || !changed || o.ReceiptAnswer != ReceiptObserved || !o.ReceiptAnsweredAt.Equal(at) {
		t.Fatalf("observado: %v %v %+v", changed, err, o)
	}
	if changed, err := o.AnswerReceipt(true, at.Add(time.Hour)); err != nil || changed || o.ReceiptAnswer != ReceiptObserved || !o.ReceiptAnsweredAt.Equal(at) {
		t.Fatalf("la segunda respuesta no cambia nada: %v %v %+v", changed, err, o)
	}
	ok := Order{State: StateRepair}
	if changed, _ := ok.AnswerReceipt(true, at); !changed || ok.ReceiptAnswer != ReceiptOK {
		t.Fatalf("conforme: %+v", ok)
	}
}
