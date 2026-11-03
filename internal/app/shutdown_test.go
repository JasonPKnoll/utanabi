package app

import (
	"errors"
	"io"
	"slices"
	"testing"
)

type recordCloser struct {
	name  string
	order *[]string
	err   error
}

func (r recordCloser) Close() error {
	*r.order = append(*r.order, r.name)
	return r.err
}

func TestClosersCloseInReverseOrder(t *testing.T) {
	t.Parallel()

	var order []string
	c := &closers{list: []io.Closer{
		recordCloser{name: "first", order: &order},
		recordCloser{name: "second", order: &order},
	}}
	if err := c.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown: %v", err)
	}
	if want := []string{"second", "first"}; !slices.Equal(order, want) {
		t.Errorf("order = %q, want %q", order, want)
	}
}

func TestClosersReportEveryFailure(t *testing.T) {
	t.Parallel()

	var order []string
	errA, errB := errors.New("a failed"), errors.New("b failed")
	c := &closers{list: []io.Closer{
		recordCloser{name: "a", order: &order, err: errA},
		recordCloser{name: "b", order: &order, err: errB},
	}}
	err := c.ServiceShutdown()
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Errorf("ServiceShutdown() = %v, want both failures", err)
	}
}
