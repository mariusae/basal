package trigger

import "basal.io/x/dex"

type predicateTrigger struct {
	p   func(dex.Entry) string
	cur *dex.Entry
}

type predicate2Trigger struct {
	p         func(dex.Entry, dex.Entry) string
	last, cur *dex.Entry
}

func Predicate(p func(dex.Entry) string) Trigger {
	return &predicateTrigger{p: p}
}

func (p *predicateTrigger) Observe(e dex.Entry) error {
	p.cur = &e
	return nil
}

func (p *predicateTrigger) Active() bool {
	return p.p(*p.cur) != ""
}

func (p *predicateTrigger) String() string {
	return p.p(*p.cur)
}

func Predicate2(p func(dex.Entry, dex.Entry) string) Trigger {
	return &predicate2Trigger{p: p}
}

func (p *predicate2Trigger) Observe(e dex.Entry) error {
	p.last = p.cur
	p.cur = &e
	return nil
}

func (p *predicate2Trigger) Active() bool {
	if p.last == nil {
		return false
	}
	return p.p(*p.last, *p.cur) != ""
}

func (p *predicate2Trigger) String() string {
	if p.last == nil {
		return ""
	}
	return p.p(*p.last, *p.cur)
}
