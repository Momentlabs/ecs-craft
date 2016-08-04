package main

import (
  "fmt"
  "gopkg.in/alecthomas/kingpin.v2"
  "os"
  "app_template/interactive"
)

var (
  app                               *kingpin.Application
  verbose                           bool
  region                            string

  // Prompt for Commands
  interCommand *kingpin.CmdClause

  command1 *kingpin.CmdClause
  sub1_command1 *kingpin.CmdClause
)

func init() {
  app = kingpin.New("craft-config.go", "Command line to to manage minecraft configs.")
  app.Flag("verbose", "Describe what is happening, as it happens.").Short('v').BoolVar(&verbose)

  interCommand = app.Command("interactive", "Prompt for commands.")

  command1 = app.Command("command1", "Do stuff in a command-1 context.")
  sub1_command1= command1.Command("sub1", "Sub1 command for command-1")


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

  // List of commands as parsed matched against functions to execute the commands.
  commandMap := map[string]func(string) {
    sub1_command1.FullCommand(): doSub1_Command1,
  }

  // Execute the command.
  if interCommand.FullCommand() == command {
    interactive.DoInteractive()
  } else {
    commandMap[command](appContext)
  }
}

func doSub1_Command1(ctxt string) {
  fmt.Printf("Sub1 Command 1 with context %s\n", ctxt)
}

// func doInteractive(ctxt string) {
//   fmt.Println("Interactive not implemented yet.")
// }
