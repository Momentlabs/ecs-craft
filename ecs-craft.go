package main

import (
  "fmt"
  "gopkg.in/alecthomas/kingpin.v2"
  "os"
  "ecs-craft/interactive"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/defaults"
)

var (
  app                               *kingpin.Application
  verbose                           bool
  regionArg                         string

  // Prompt for Commands
  interCommand *kingpin.CmdClause

  // serverCmd *kingpin.CmdClause
  // serverLaunchCmd *kingpin.CmdClause
  // serverListCmd *kingpin.CmdClause
  // serverDescribeCmd *kingpin.CmdClause
)

func init() {
  app = kingpin.New("craft-config.go", "Command line to to manage minecraft configs.")
  app.Flag("verbose", "Describe what is happening, as it happens.").Short('v').BoolVar(&verbose)
  app.Flag("region", "Manage continers in this AWS region.").Default("us-east-1").StringVar(&regionArg)

  interCommand = app.Command("interactive", "Prompt for commands.")

  kingpin.CommandLine.Help = `A command-line minecraft config tool.`
}

func main() {

  // Parse the command line to fool with flags and get the command we'll execeute.
  command := kingpin.MustParse(app.Parse(os.Args[1:]))

   if verbose {
    fmt.Printf("Starting up.")
   }

   // This some state passed to each command (eg. an AWS session or connection)
   // So not usually a string.
   appContext := "AppContext"

   // Set up AWS.
   config := defaults.Get().Config
   if *config.Region == "" {
    config.Region = aws.String(regionArg)
   }


  // List of commands as parsed matched against functions to execute the commands.
  commandMap := map[string]func(string) {
    // sub1_command1.FullCommand(): doSub1_Command1,
  }

  // Execute the command.
  if interCommand.FullCommand() == command {
    interactive.DoInteractive(config)
  } else {
    commandMap[command](appContext)
  }
}

func doSub1_Command1(ctxt string) {
  fmt.Printf("Sub1 Command 1 with context %s\n", ctxt)
}
