package trigger

import (
	"fmt"

	"basal.io/x/dex"
)

func Below(bg int) Trigger {
	return Predicate(func(e dex.Entry) string {
		if e.Value < bg {
			return fmt.Sprintf("%d < %d", e.Value, bg)
		} else {
			return ""
		}
	})
}

func Above(bg int) Trigger {
	return Predicate(func(e dex.Entry) string {
		if e.Value > bg {
			return fmt.Sprintf("%d > %d", e.Value, bg)
		} else {
			return ""
		}
	})
}

func Arrow(dir ...dex.Dir) Trigger {
	return Predicate(func(e dex.Entry) string {
		for _, d := range dir {
			if d == e.Dir {
				return d.Arrow()
			}
		}
		return ""
	})
}

// Delta in mg/dL/m
func Delta(d float64) Trigger {
	return Predicate2(func(e0, e1 dex.Entry) string {
		delta := float64(e1.Value-e0.Value) / e1.Time.Sub(e0.Time).Minutes()
		if delta < 0 && d < 0 && delta < d {
			return fmt.Sprintf("Delta(%.1f < %.1f", delta, d)
		} else if delta > 0 && d > 0 && delta > d {
			return fmt.Sprintf("Delta(%.1f > %.1f", delta, d)
		} else {
			return ""
		}
	})
}