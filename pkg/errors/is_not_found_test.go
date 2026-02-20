package errors_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestIsNotFound(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			"Generic NotFound",
			errors.NotFound("not found"),
			true,
		},
		{
			"Patent NotFound",
			errors.New(errors.CodePatentNotFound, "patent not found"),
			true,
		},
		{
			"Molecule NotFound",
			errors.New(errors.CodeMoleculeNotFound, "molecule not found"),
			true,
		},
		{
			"Portfolio NotFound",
			errors.New(errors.CodePortfolioNotFound, "portfolio not found"),
			true,
		},
		{
			"Internal Error",
			errors.Internal("internal error"),
			false,
		},
		{
			"Wrapped NotFound",
			errors.Wrap(errors.NotFound("not found"), errors.CodeInternal, "wrapped"),
			true,
		},
		{
			"Plain error",
			fmt.Errorf("plain error"),
			false,
		},
		{
			"Nil error",
			nil,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, errors.IsNotFound(tc.err))
		})
	}
}
