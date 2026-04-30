package flags

import "flag"

type BotFlags struct {
	SetupDb *bool
}

func GetFlags() *BotFlags {
	flags := &BotFlags{
		SetupDb: flag.Bool("s", false, "Only setup the database and run auto migration"),
	}
	flag.Parse()
	return flags
}
