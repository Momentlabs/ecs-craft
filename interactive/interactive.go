package interactive 

import (
  "gopkg.in/alecthomas/kingpin.v2"
  "github.com/bobappleyard/readline"
  "strings"
  "fmt"
  "io"
)

var (

  app *kingpin.Application

  exit *kingpin.CmdClause
  quit *kingpin.CmdClause
  verboseCmd *kingpin.CmdClause
  verbose bool
  testString []string

  test *kingpin.CmdClause

  command1 *kingpin.CmdClause
  subCommand1 *kingpin.CmdClause
  subCommand2 *kingpin.CmdClause
  sub2FirstArg string
)

func init() {
  app = kingpin.New("", "Interactive mode.").Terminate(doTerminate)

  // state
  verboseCmd = app.Command("verbose", "toggle verbose mode.")
  exit = app.Command("exit", "exit the program. <ctrl-D> works too.")
  quit = app.Command("quit", "exit the program.")

  test = app.Command("test", "Test command for demonstration")
  command1 = app.Command("command1","The first command.")
  subCommand1 = command1.Command("sub1", "The first sub command.")
  subCommand2 = command1.Command("sub2", "Second sub command.")
  subCommand2.Arg("command-arg", "The required argument for sub2").Required().StringVar(&sub2FirstArg)
}


func DoICommand(line string, ctxt string) (err error) {

  // This is due to a 'peculiarity' of kingpin: it collects strings as arguments across parses.
  testString = []string{}

  // Prepare a line for parsing
  line = strings.TrimRight(line, "\n")
  fields := []string{}
  fields = append(fields, strings.Fields(line)...)
  if len(fields) <= 0 {
    return nil
  }

  command, err := app.Parse(fields)
  if err != nil {
    fmt.Printf("Command error: %s.\nType help for a list of commands.\n", err)
    return nil
  } else {
    switch command {
      case verboseCmd.FullCommand(): err = doVerbose()
      case exit.FullCommand(): err = doQuit()
      case quit.FullCommand(): err = doQuit()
      case test.FullCommand(): err = doTest()
      case subCommand1.FullCommand(): err = doSubCommand1()
      case subCommand2.FullCommand(): err = doSubCommand2()
    }
  }
  return err
}

func doTest() (error) {
  fmt.Println("Test command executed.")
  return nil
}

func doSubCommand1() (error) {
  fmt.Println("Sub command 1\n")
  return nil
}

func doSubCommand2() (error) {
  fmt.Printf("Sub command 2 with argument \"%s\".\n", sub2FirstArg )
  return nil
}
func toggleVerbose() bool {
  verbose = verbose
  return verbose
}

func doVerbose() (error) {
  if toggleVerbose() {
    fmt.Println("Verbose is on.")
  } else {
    fmt.Println("Verbose is off.")
  }
  return nil
}

func doQuit() (error) {
  return io.EOF
}

func doTerminate(i int) {}

func promptLoop(prompt string, process func(string) (error)) (err error) {
  errStr := "Error - %s.\n"
  for moreCommands := true; moreCommands; {
    line, err := readline.String(prompt)
    if err == io.EOF {
      moreCommands = false
    } else if err != nil {
      fmt.Printf(errStr, err)
    } else {
      readline.AddHistory(line)
      err = process(line)
      if err == io.EOF {
        moreCommands = false
      } else if err != nil {
        fmt.Printf(errStr, err)
      }
    }
  }
  return nil
}

// This gets called from the main program, presumably from the 'interactive' command on main's command line.
func DoInteractive() {
  xICommand := func(line string) (err error) {return DoICommand(line, "craft-config")}
  prompt := "> "
  err := promptLoop(prompt, xICommand)
  if err != nil {fmt.Printf("Error - %s.\n", err)}
}




