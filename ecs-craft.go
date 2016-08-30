package main

import (
  // "fmt"
  "gopkg.in/alecthomas/kingpin.v2"
  "os"
  "ecs-craft/interactive"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/op/go-logging"

  // Careful now ...
  // "ecs-pilot/awslib"
  "github.com/jdrivas/awslib"  
)

var (
  app                               *kingpin.Application
  verbose                           bool
  debug                             bool
  regionArg                         string
  profileArg                        string

  // Prompt for Commands
  interCommand *kingpin.CmdClause

  // serverCmd *kingpin.CmdClause
  // serverLaunchCmd *kingpin.CmdClause
  // serverListCmd *kingpin.CmdClause
  // serverDescribeCmd *kingpin.CmdClause

  log *logging.Logger
)

func init() {
  log = logging.MustGetLogger("ecs-craft")
  app = kingpin.New("craft-config.go", "Command line to to manage minecraft configs.")
  app.Flag("verbose", "Describe what is happening, as it happens.").Short('v').BoolVar(&verbose)
  app.Flag("debug", "Describe what is happening, as it happens.").Short('v').BoolVar(&debug)
  app.Flag("region", "Manage continers in this AWS region.").Default("us-east-1").StringVar(&regionArg)
  app.Flag("profile", "AWS profile to use for credentials.").Default("minecraft").StringVar(&profileArg)

  interCommand = app.Command("interactive", "Prompt for commands.")

  kingpin.CommandLine.Help = `A command-line minecraft config tool.`
}

func main() {

  // Parse the command line to fool with flags and get the command we'll execeute.
  command := kingpin.MustParse(app.Parse(os.Args[1:]))

  if verbose || debug {
    setLogLevel(logging.DEBUG)
  }

   // Set up AWS.
   config := awslib.GetConfig(profileArg, "")
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
    commandMap[command]("")
  }

}

func setLogLevel(l logging.Level) {
  logs := []string {"ecs-craft, ecs-craft/interactive, ecspilot/awslib"}
  for _, log := range logs {
    logging.SetLevel(l, log)
  }
}

