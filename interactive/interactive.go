package interactive 

import (
  "github.com/bobappleyard/readline"
  "strings"
  "fmt"
  "io"
  "os"
  "text/tabwriter"
  "time"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ecs"
  "github.com/aws/aws-sdk-go/service/ec2"
  "github.com/aws/aws-sdk-go/service/s3"
  "github.com/mgutz/ansi"
  "github.com/op/go-logging"
  // "gopkg.in/alecthomas/kingpin.v2"
  "github.com/alecthomas/kingpin"

  // Careful now ...
  // "awslib"
  "github.com/jdrivas/awslib"

)

const(
  defaultCluster = "minecraft"
)

var (

  // State variables
  currentCluster = defaultCluster

  app *kingpin.Application

  exit *kingpin.CmdClause
  quit *kingpin.CmdClause
  debugCmd *kingpin.CmdClause
  verboseCmd *kingpin.CmdClause
  verbose bool
  debug bool
  testString []string

  clusterCmd *kingpin.CmdClause
  clusterListCmd *kingpin.CmdClause
  clusterStatusCmd *kingpin.CmdClause

  serverCmd *kingpin.CmdClause
  serverLaunchCmd *kingpin.CmdClause
  serverStartCmd *kingpin.CmdClause
  serverTerminateCmd *kingpin.CmdClause
  serverListCmd *kingpin.CmdClause
  serverDescribeAllCmd *kingpin.CmdClause
  serverDescribeCmd *kingpin.CmdClause

  // AWS paramaters
  clusterArg string
  serverTaskArg string

  // TODO: remove this. We don't use it anymore.
  serverContainerNameArg string
  serverTaskArnArg string
  bucketNameArg string

  // mclib Paramaters
  userNameArg string
  serverNameArg string
  snapshotNameArg string
  useFullURIFlag bool

  snapshotCmd *kingpin.CmdClause
  snapshotListCmd *kingpin.CmdClause

  log *logging.Logger

)

// Text Coloring
var (
  nullColor = fmt.Sprintf("%s", "\x00\x00\x00\x00\x00\x00\x00")
  defaultColor = fmt.Sprintf("%s%s", "\x00\x00", ansi.ColorCode("default"))
  defaultShortColor = fmt.Sprintf("%s", ansi.ColorCode("default"))

  emphBlueColor = fmt.Sprintf(ansi.ColorCode("blue+b"))
  emphRedColor = fmt.Sprintf(ansi.ColorCode("red+b"))
  emphColor = emphBlueColor

  titleColor = fmt.Sprintf(ansi.ColorCode("default+b"))
  titleEmph = emphBlueColor
  infoColor = emphBlueColor
  successColor = fmt.Sprintf(ansi.ColorCode("green+b"))
  warnColor = fmt.Sprintf(ansi.ColorCode("yellow+b"))
  failColor = emphRedColor
  resetColor = fmt.Sprintf(ansi.ColorCode("reset"))

)

func init() {
  log = logging.MustGetLogger("ecs-craft")

  // TODO: all of these don't have to be global. 
  // Better practice to move these into a build UI routine(s).
  app = kingpin.New("", "Interactive mode.").Terminate(doTerminate)

  // Basic housekeeping commands.
  debugCmd = app.Command("debug", "toggle debug.")
  verboseCmd = app.Command("verbose", "toggle verbose mode.")
  exit = app.Command("exit", "exit the program. <ctrl-D> works too.")
  quit = app.Command("quit", "exit the program.")

  // Cluster Commands
  clusterCmd = app.Command("cluster", "Context for cluster commands.")
  clusterListCmd = clusterCmd.Command("list", "List short status of all the clusters.")
  clusterStatusCmd  = clusterCmd.Command("status", "Detailed status on the cluster.")
  clusterStatusCmd.Arg("cluster", "The cluster you want to describe.").Action(setCurrent).StringVar(&clusterArg)

  // Server commands
  serverCmd = app.Command("server","Context for minecraft server commands.")

  serverLaunchCmd = serverCmd.Command("launch", "Launch a new minecraft server for a user in a cluster.")
  serverLaunchCmd.Arg("user", "User name of the server").Required().StringVar(&userNameArg)
  serverLaunchCmd.Arg("server-name","Name of the server. This is an identifier for the serve. (e.g. test-server, world-play).").Required().StringVar(&serverNameArg)
  serverLaunchCmd.Arg("cluster", "ECS cluster to launch the server in.").Action(setCurrent).StringVar(&clusterArg)
  serverLaunchCmd.Arg("ecs-task", "ECS Task that represents a running minecraft server.").Default("minecraft-ecs").StringVar(&serverTaskArg)
  serverLaunchCmd.Arg("ecs-conatiner-name", "Container name for the minecraft server (used for environment variables.").Default("minecraft").StringVar(&serverContainerNameArg)

  serverStartCmd = serverCmd.Command("start", "Start a server from a snapshot.")
  serverStartCmd.Flag("useFullURI", "Use a full URI for the snapshot as opposed to a named snapshot.").Default("false").BoolVar(&useFullURIFlag)
  serverStartCmd.Arg("user","User name for the server.").Required().StringVar(&userNameArg)
  serverStartCmd.Arg("server-name","Name of the server. This is an identifier for the serve. (e.g. test-server, world-play).").Required().StringVar(&serverNameArg)
  serverStartCmd.Arg("snapshot", "Name of snapshot for starting server.").Required().StringVar(&snapshotNameArg)
  serverStartCmd.Arg("cluster", "ECS Cluster for the server containers.").Action(setCurrent).StringVar(&clusterArg)
  serverStartCmd.Arg("ecs-task", "ECS Task that represents a running minecraft server.").Default("minecraft-ecs").StringVar(&serverTaskArg)
  serverStartCmd.Arg("ecs-conatiner-name", "Container name for the minecraft server (used for environment variables.").Default("minecraft").StringVar(&serverContainerNameArg)

  serverTerminateCmd = serverCmd.Command("terminate", "Stop this server")
  serverTerminateCmd.Arg("ecs-task-arn", "ECS Task ARN for this server.").Required().StringVar(&serverTaskArnArg)

  serverListCmd = serverCmd.Command("list", "List the servers for a cluster.")
  serverListCmd.Arg("cluster", "ECS cluster to look for servers.").Action(setCurrent).StringVar(&clusterArg)

  serverDescribeAllCmd = serverCmd.Command("describe-all", "Show details for all servers in cluster.")
  serverDescribeAllCmd.Arg("cluster", "The ECS cluster where the servers live.").Action(setCurrent).StringVar(&clusterArg)
  serverDescribeCmd = serverCmd.Command("describe", "Show some details for a users server.")
  serverDescribeCmd.Arg("user", "The user that owns the server.").Required().StringVar(&userNameArg)
  serverDescribeCmd.Arg("cluster", "The ECS cluster where the server lives.").Action(setCurrent).StringVar(&clusterArg)

  // Snapshot commands
  snapshotCmd = app.Command("snapshot", "Context for snapshot commands.")
  snapshotListCmd = snapshotCmd.Command("list", "List all snapshot for a user.")
  snapshotListCmd.Arg("user", "The snapshot's user.").Required().StringVar(&userNameArg)
  snapshotListCmd.Arg("bucket", "The name of the S3 bucket we're using to store snapshots in.").Default("craft-config-test").StringVar(&bucketNameArg)

}


func DoICommand(line string, sess *session.Session, ecsSvc *ecs.ECS, ec2Svc *ec2.EC2, s3Svc *s3.S3) (err error) {

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
      case debugCmd.FullCommand(): err = doDebug()
      case verboseCmd.FullCommand(): err = doVerbose()
      case exit.FullCommand(): err = doQuit(sess)
      case quit.FullCommand(): err = doQuit(sess)

      // Server Commands
      case serverLaunchCmd.FullCommand(): err = doLaunchServerCmd(sess)
      case serverStartCmd.FullCommand(): err = doStartServerCmd(sess)
      case serverTerminateCmd.FullCommand(): err = doTerminateServerCmd(sess)
      case serverListCmd.FullCommand(): err = doListServersCmd(sess)
      case serverDescribeAllCmd.FullCommand(): err = doDescribeAllServersCmd(sess)
      case serverDescribeCmd.FullCommand(): err = doDescribeServerCmd()

      // Snapshot commands
      case snapshotListCmd.FullCommand(): err = doSnapshotListCmd(sess)
    }
  }
  return err
}

func setCurrent(pc *kingpin.ParseContext) (error) {

  for _, pe := range pc.Elements {
    c := pe.Clause
    switch c.(type) {
    // case *kingpin.CmdClause : fmt.Printf("CmdClause: %s\n", (c.(*kingpin.CmdClause)).Model().Name)
    // case *kingpin.FlagClause : fmt.Printf("ArgClause: %s\n", c.(*kingpin.FlagClause).Model().Name)
    case *kingpin.ArgClause : 
      fc := c.(*kingpin.ArgClause)
      if fc.Model().Name == "cluster" {
        currentCluster = *pe.Value
      }
    }
  }

  return nil
}

func doVerbose() (error) {
  if toggleVerbose() {
    fmt.Println("Verbose is on.")
  } else {
    fmt.Println("Verbose is off.")
  }
  return nil
}

func toggleVerbose() bool {
  verbose = !verbose
  return verbose
}

func doDebug() (error) {
  if toggleDebug() {
    fmt.Println("Debug is on.")
  } else {
    fmt.Println("Debug is off.")
  }
  return nil
}

func toggleDebug() bool {
  debug = !debug
  return debug
}

func doQuit(sess *session.Session) (error) {

  clusters, err := awslib.GetAllClusterDescriptions(sess)
  clusters.Sort(awslib.ByReverseActivity)
  if err != nil {
    fmt.Printf("doQuit: Error getting cluster data: %s\n", err)
  } else {
    fmt.Println(time.Now().Local().Format(time.RFC1123))
    w := tabwriter.NewWriter(os.Stdout, 4, 10, 2, ' ', 0)
    fmt.Fprintf(w, "%sName\tStatus\tInstances\tPending\tRunning%s\n", emphColor, resetColor)
    for _, c := range clusters {
      instanceCount := *c.RegisteredContainerInstancesCount
      color := nullColor
      if instanceCount > 0 {color = infoColor}
      fmt.Fprintf(w, "%s%s\t%s\t%d\t%d\t%d%s\n", color, *c.ClusterName, *c.Status, 
        instanceCount, *c.PendingTasksCount, *c.RunningTasksCount, resetColor)
    }      
    w.Flush()
  }

  return io.EOF
}

func doTerminate(i int) {}

func promptLoop(process func(string) (error)) (err error) {
  errStr := "Error - %s.\n"
  prompt := fmt.Sprintf("%s[%s%s%s]:%s ", titleEmph, infoColor, currentCluster, titleEmph, resetColor)
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
func DoInteractive(config *aws.Config) {

  // Set up AWS
  session := session.New(config)

  // Print out some account specifics.
  // fmt.Printf("%s\n", awslib.AccountDetailsString(config))

  ecsSvc := ecs.New(session)
  ec2Svc := ec2.New(session)
  s3Svc := s3.New(session)
  xICommand := func(line string) (err error) {return DoICommand(line, session, ecsSvc, ec2Svc, s3Svc)}
  err := promptLoop(xICommand)
  if err != nil {fmt.Printf("Error - %s.\n", err)}
}




