package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/orders/app"
	"sweveh/internal/orders/domain"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// ExpensesHandler expone lo gastado contra lo cotizado (V-08, HU-106). Interno: nunca va a la vista del cliente.
type ExpensesHandler struct {
	svc   *app.Expenses
	guard *authn.Guard
	log   *slog.Logger
}

func NewExpensesHandler(svc *app.Expenses, guard *authn.Guard, log *slog.Logger) *ExpensesHandler {
	return &ExpensesHandler{svc: svc, guard: guard, log: log}
}

const permExpense = "expense:manage"

func (h *ExpensesHandler) Routes(r chi.Router) {
	j := func(fn httpx.Func) http.HandlerFunc { return httpx.JSON(h.log, fn) }
	r.With(h.guard.Require(permExpense)).Get("/orders/{id}/expenses", j(h.list))
	r.With(h.guard.Require(permExpense)).Post("/orders/{id}/expenses", j(h.add))
	r.With(h.guard.Require(permExpense)).Post("/orders/{id}/expenses/{gid}/void", j(h.void))
}

type expenseDTO struct {
	ID          string    `json:"id"`
	Descripcion string    `json:"descripcion"`
	Valor       int64     `json:"valor"`
	Origen      string    `json:"origen"`
	FotoID      *string   `json:"fotoId"`
	CreadoEn    time.Time `json:"creadoEn"`
	Anulado     bool      `json:"anulado"`
}

type spendDTO struct {
	Cotizado int64 `json:"cotizado"`
	Gastado  int64 `json:"gastado"`
	Quedan   int64 `json:"quedan"`
}

func expenseOut(e domain.Expense) expenseDTO {
	d := expenseDTO{ID: e.ID.String(), Descripcion: e.Description, Valor: e.Value, Origen: e.Origin, CreadoEn: e.CreatedAt, Anulado: e.Voided}
	if e.PhotoID != nil {
		s := e.PhotoID.String()
		d.FotoID = &s
	}
	return d
}

func spendOut(s domain.Spend) spendDTO {
	return spendDTO{Cotizado: s.Quoted, Gastado: s.Spent, Quedan: s.Left}
}

func (h *ExpensesHandler) list(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	list, spend, err := h.svc.List(r.Context(), p.TallerID, id)
	out := make([]expenseDTO, 0, len(list))
	for _, e := range list {
		out = append(out, expenseOut(e))
	}
	return http.StatusOK, map[string]any{"gastos": out, "resumen": spendOut(spend)}, err
}

func (h *ExpensesHandler) add(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		ID          string  `json:"id"`
		Descripcion string  `json:"descripcion"`
		Valor       int64   `json:"valor"`
		Origen      string  `json:"origen"`
		FotoID      *string `json:"fotoId"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	gid, err := shared.ParseID(q.ID)
	if err != nil {
		return 0, nil, shared.Validation("gasto_id", "Falta el identificador del gasto (lo genera la pantalla para no anotarlo dos veces).").WithField("id")
	}
	var photo *shared.ID
	if q.FotoID != nil {
		f, err := shared.ParseID(*q.FotoID)
		if err != nil {
			return 0, nil, app.ErrExpensePhoto
		}
		photo = &f
	}
	e, spend, err := h.svc.Add(r.Context(), app.ExpenseInput{TallerID: p.TallerID, OrderID: id, ID: gid, Actor: p.Actor(),
		Description: q.Descripcion, Value: q.Valor, Origin: q.Origen, PhotoID: photo})
	if err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, map[string]any{"gasto": expenseOut(e), "resumen": spendOut(spend)}, nil
}

func (h *ExpensesHandler) void(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	gid, err := shared.ParseID(chi.URLParam(r, "gid"))
	if err != nil {
		return 0, nil, domain.ErrExpenseNF
	}
	spend, err := h.svc.Void(r.Context(), p.TallerID, id, gid, p.Actor())
	return http.StatusOK, map[string]any{"resumen": spendOut(spend)}, err
}
