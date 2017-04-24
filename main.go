//
// main.go --- e3db command line tool.
//
// Copyright (C) 2017, Tozny, LLC.
// All Rights Reserved.
//

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jawher/mow.cli"
	"github.com/tozny/e3db-go"
)

type cliOptions struct {
	Logging *bool
	Profile *string
}

func (o *cliOptions) getClient() *e3db.Client {
	var client *e3db.Client
	var err error

	if *o.Profile == "" {
		client, err = e3db.GetDefaultClient()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		opts, err := e3db.GetConfig(*o.Profile)
		if err != nil {
			log.Fatal(err)
		}

		if *o.Logging {
			opts.Logging = true
		}

		client, err = e3db.GetClient(*opts)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(client)
	}

	return client
}

var options cliOptions

func cmdList(cmd *cli.Cmd) {
	data := cmd.BoolOpt("d data", false, "include data in JSON format")
	outputJSON := cmd.BoolOpt("j json", false, "output in JSON format")
	contentTypes := cmd.StringsOpt("t type", nil, "record content type")
	recordIDs := cmd.StringsOpt("r record", nil, "record ID")
	writerIDs := cmd.StringsOpt("w writer", nil, "record writer ID")
	userIDs := cmd.StringsOpt("u user", nil, "record user ID")

	cmd.Action = func() {
		client := options.getClient()

		cursor := client.Query(context.Background(), e3db.Q{
			ContentTypes: *contentTypes,
			RecordIDs:    *recordIDs,
			WriterIDs:    *writerIDs,
			UserIDs:      *userIDs,
			IncludeData:  *data,
		})

		first := true
		for cursor.Next() {
			record, err := cursor.Get()
			if err != nil {
				fmt.Fprintf(os.Stderr, "e3db-cli: ls: %s\n", err)
				os.Exit(1)
			}

			if *outputJSON {
				if first {
					first = false
					fmt.Println("[")
				} else {
					fmt.Printf(",\n")
				}

				bytes, _ := json.MarshalIndent(record, "  ", "  ")
				fmt.Printf("  %s", bytes)
			} else {
				fmt.Printf("%-40s %s\n", record.Meta.RecordID, record.Meta.Type)
			}
		}

		if *outputJSON {
			fmt.Println("\n]")
		}
	}
}

func cmdWrite(cmd *cli.Cmd) {
	recordType := cmd.String(cli.StringArg{
		Name:      "TYPE",
		Desc:      "type of record to write",
		Value:     "",
		HideValue: true,
	})

	data := cmd.String(cli.StringArg{
		Name:      "DATA",
		Desc:      "json formatted record data",
		Value:     "",
		HideValue: true,
	})

	cmd.Action = func() {
		client := options.getClient()
		record := client.NewRecord(*recordType)

		err := json.NewDecoder(strings.NewReader(*data)).Decode(&record.Data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "e3db-cli: write: %s\n", err)
		}

		id, err := client.Write(context.Background(), record)
		if err != nil {
			fmt.Fprintf(os.Stderr, "e3db-cli: write: %s\n", err)
		}

		fmt.Println(id)
	}
}

func cmdRead(cmd *cli.Cmd) {
	recordIDs := cmd.Strings(cli.StringsArg{
		Name:      "RECORD_ID",
		Desc:      "record ID to read",
		Value:     nil,
		HideValue: true,
	})

	cmd.Spec = "RECORD_ID..."
	cmd.Action = func() {
		client := options.getClient()

		for _, recordID := range *recordIDs {
			record, err := client.Read(context.Background(), recordID)
			if err != nil {
				log.Fatal(err)
			}

			bytes, err := json.MarshalIndent(record, "", "  ")
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(bytes))
		}
	}
}

func cmdRegister(cmd *cli.Cmd) {
	email := cmd.String(cli.StringArg{
		Name:      "EMAIL",
		Desc:      "client e-mail address",
		Value:     "",
		HideValue: true,
	})

	// TODO: minimally validate that email looks like an email address

	cmd.Action = func() {
		// Preflight check for existing configuration file to prevent a later
		// failure writing the file (since we use O_EXCL) after registration.
		if e3db.ProfileExists(*options.Profile) {
			var name string
			if *options.Profile != "" {
				name = *options.Profile
			} else {
				name = "(default)"
			}

			fmt.Fprintf(os.Stderr, "e3db-cli: register: profile %s already registered\n", name)
			os.Exit(1)
		}

		info, err := e3db.RegisterClient(*email, e3db.RegistrationOpts{
			Logging: *options.Logging,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "e3db-cli: register: %s\n", err)
			os.Exit(1)
		}

		err = e3db.SaveConfig(*options.Profile, info)
		if err != nil {
			fmt.Fprintf(os.Stderr, "e3db-cli: register: %s\n", err)
			os.Exit(1)
		}
	}
}

func main() {
	app := cli.App("e3db-cli", "E3DB Command Line Interface")

	app.Version("v version", "e3db-cli 0.0.1")

	options.Logging = app.BoolOpt("d debug", false, "enable debug logging")
	options.Profile = app.StringOpt("p profile", "", "e3db configuration profile")

	app.Command("register", "register a client", cmdRegister)
	app.Command("ls", "list records", cmdList)
	app.Command("read", "read records", cmdRead)
	app.Command("write", "write a record", cmdWrite)
	app.Run(os.Args)
}
