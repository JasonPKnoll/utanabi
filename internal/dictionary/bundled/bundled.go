//go:build bundled_dictionary

// Package bundled carries a copy of dictionary.db inside the app; see none.go.
package bundled

import _ "embed"

// A release build copies dictionary.db here before building with -tags bundled_dictionary.
//
//go:embed dictionary.db
var data []byte

// Data returns the dictionary built into the app.
func Data() []byte { return data }
