// Package flags parses the command line.
package flags

import "flag"

// Flags holds the parsed command line options.
//
// They are plain values rather than pointers so callers do not have to
// dereference them at every use, and so a Flags literal can be built in tests.
type Flags struct {
	// SetupDB runs the schema migration and exits without starting the bot.
	SetupDB bool
	// Verbose turns on GORM's statement logging.
	Verbose bool
}

// Parse reads the command line into a Flags value.
func Parse() Flags {
	var f Flags
	flag.BoolVar(&f.SetupDB, "s", false, "Set up the database, run the migration, and exit")
	flag.BoolVar(&f.Verbose, "v", false, "Log every SQL statement")
	flag.Parse()
	return f
}
