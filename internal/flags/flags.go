package flags

import "flag"

type BotFlags struct {
	SetupDb *bool
	Verbose *bool
}

func GetFlags() *BotFlags {
	flags := &BotFlags{
		SetupDb: flag.Bool("s", false, "Only setup the database and run auto migration"),
		Verbose: flag.Bool("v", false, "Enable verbose mode for GORM"),
	}
	flag.Parse()
	return flags
}
