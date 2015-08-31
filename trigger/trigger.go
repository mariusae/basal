package trigger // import "basal.io/x/trigger"
import (
	"fmt"
	"strings"

	"basal.io/x/dex"
)

type Trigger interface {
	Observe(e dex.Entry) error
	Active() bool
	String() string
}

type anyTrigger []Trigger
type allTrigger []Trigger

func Any(trigger ...Trigger) Trigger {
	return anyTrigger(trigger)
}

func All(trigger ...Trigger) Trigger {
	return allTrigger(trigger)
}

func (a anyTrigger) Observe(e dex.Entry) error {
	var errs errs
	for _, t := range a {
		errs.record(t.Observe(e))
	}

	return errs.err()
}

func (a anyTrigger) Active() bool {
	for _, t := range a {
		if t.Active() {
			return true
		}
	}
	return false
}

func (a anyTrigger) String() string {
	strs := make([]string, 0, len(a))
	for i := range a {
		if a[i].Active() {
			strs = append(strs, a[i].String())
		}
	}
	list := strings.Join(strs, ",")
	return fmt.Sprintf("Any(%s)", list)
}

func (a allTrigger) Observe(e dex.Entry) error {
	var errs errs
	for _, t := range a {
		errs.record(t.Observe(e))
	}

	return errs.err()
}

func (a allTrigger) Active() bool {
	active := true
	for _, t := range a {
		active = active && t.Active()
	}
	return active
}

func (a allTrigger) String() string {
	strs := make([]string, 0, len(a))
	for i := range a {
		if a[i].Active() {
			strs = append(strs, a[i].String())
		}
	}
	list := strings.Join(strs, ",")
	return fmt.Sprintf("All(%s)", list)
}
