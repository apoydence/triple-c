package scheduler_test

import (
	"context"
	"testing"

	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
	"github.com/poy/triple-c/internal/scheduler"
)

type TBM struct {
	*testing.T
	m        *scheduler.BranchManager
	ctxs     []context.Context
	branches []string
}

func TestBranchManager(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) *TBM {
		tb := &TBM{
			T: t,
		}

		onStart := func(ctx context.Context, branch string) {
			tb.ctxs = append(tb.ctxs, ctx)
			tb.branches = append(tb.branches, branch)
		}

		tb.m = scheduler.NewBranchManager(onStart)

		return tb
	})

	o.Spec("it calls onStart for a new branch", func(t *TBM) {
		t.m.Add("a")
		t.m.Add("b")
		t.m.Add("a")
		Expect(t, t.branches).To(Equal([]string{"a", "b"}))
	})

	o.Spec("it cancels the context when a branch is removed", func(t *TBM) {
		t.m.Add("a")
		t.m.Remove("a")
		Expect(t, t.ctxs[0].Err()).To(Not(BeNil()))
	})

	o.Spec("it survives the race detector", func(t *TBM) {
		go func() {
			for i := 0; i < 100; i++ {
				t.m.Add("a")
			}
		}()
		for i := 0; i < 100; i++ {
			t.m.Remove("a")
		}
	})
}
