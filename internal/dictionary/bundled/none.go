//go:build !bundled_dictionary

// Package bundled carries a copy of dictionary.db inside the app, for release builds with no bundle folder to keep it
// in: Windows, Linux, Android, and the server image. Built with -tags bundled_dictionary, after copying dictionary.db
// into this folder, Data returns it, and main writes it into the data folder the first time the app starts. Other
// builds carry none.
package bundled

// Data returns the dictionary built into the app: none in this build.
func Data() []byte { return nil }
