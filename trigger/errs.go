package trigger

import (
	"errors"
	"strings"
)

type errs struct {
	errs []error
}

func (e *errs) record(err error) {
	if err == nil {
		return
	}

	e.errs = append(e.errs, err)
}

func (e *errs) err() error {
	if len(e.errs) == 0 {
		return nil
	}

	strs := make([]string, len(e.errs))
	for i := range e.errs {
		strs[i] = e.errs[i].Error()
	}
	joined := strings.Join(strs, ",")

	return errors.New(joined)
}
