package fsys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateDirPath(t *testing.T) {
	cases := map[string]struct {
		path        string
		expectedErr bool
	}{
		"CurrentDir": {
			path:        ".",
			expectedErr: true,
		},
		"PreviousDir": {
			path:        "..",
			expectedErr: true,
		},
		"Relative": {
			path:        "./example",
			expectedErr: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateDirPath(tc.path)
			if err != nil {
				if tc.expectedErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				return
			}
			if err == nil && tc.expectedErr {
				t.Errorf("want error, got nil")
			}
		})
	}
}
