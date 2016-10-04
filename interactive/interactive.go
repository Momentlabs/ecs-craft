package interactive 

import (
  "github.com/bobappleyard/readline"
  "strings"
  "fmt"
  "io"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ecs"
  "github.com/aws/aws-sdk-go/service/ec2"
  "github.com/aws/aws-sdk-go/service/s3"
  "github.com/jdrivas/sl"
  "github.com/mgutz/ansi"
  // "github.com/op/go-logging"
  // "gopkg.in/alecthomas/kingpin.v2"
  "github.com/alecthomas/kingpin"
  "github.com/Sirupsen/logrus"


  // Careful now ...
  // "mclib"
  "github.com/jdrivas/mclib"
  // "awslib"
  "github.com/jdrivas/awslib"

)

const(
  defaultCluster = "minecraft"
  // please see getProxyTaskDef to use these
  defaultServerTaskDef = mclib.DefaultServerTaskDefinition
  defaultProxyTaskDef = mclib.BungeeProxyDefaultPortTaskDef
)



var (

  // General State
  currentCluster = defaultCluster
  log = sl.New()

  // UI State
  app *kingpin.Application

  exit *kingpin.CmdClause
  quit *kingpin.CmdClause
  debugCmd *kingpin.CmdClause
  verboseCmd *kingpin.CmdClause
  verbose bool
  debug bool
  testString []string

  useClusterCmd *kingpin.CmdClause

  clusterCmd *kingpin.CmdClause
  clusterListCmd *kingpin.CmdClause
  clusterStatusCmd *kingpin.CmdClause
  clusterUseCmd *kingpin.CmdClause

  proxyCmd *kingpin.CmdClause
  proxyLaunchCmd *kingpin.CmdClause
  proxyListCmd *kingpin.CmdClause
  proxyAttachCmd *kingpin.CmdClause

  serverCmd *kingpin.CmdClause
  serverLaunchCmd *kingpin.CmdClause
  serverStartCmd *kingpin.CmdClause
  serverTerminateCmd *kingpin.CmdClause
  serverListCmd *kingpin.CmdClause
  serverDescribeAllCmd *kingpin.CmdClause
  serverDescribeCmd *kingpin.CmdClause
  serverAttachCmd *kingpin.CmdClause
  serverProxyCmd *kingpin.CmdClause
  serverUnProxyCmd *kingpin.CmdClause

  dnsCmd *kingpin.CmdClause

  envCmd *kingpin.CmdClause
  envListCmd *kingpin.CmdClause

  clusterArg string
  serverTaskArg string

  proxyNameArg string
  // Please See getProxyTaskDef() to use this.
  proxyTaskDefArg string

  serverTaskArnArg string
  bucketNameArg string

  userNameArg string
  serverNameArg string
  snapshotNameArg string
  useFullURIFlag bool

  archiveCmd *kingpin.CmdClause
  archiveListCmd *kingpin.CmdClause
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

  // TODO: all of these don't have to be global. 
  // Better practice to move these into a build UI routine(s).
  app = kingpin.New("", "Interactive mode.").Terminate(doTerminate)

  // Basic housekeeping commands.
  debugCmd = app.Command("debug", "toggle debug.")
  verboseCmd = app.Command("verbose", "toggle verbose mode.")
  exit = app.Command("exit", "exit the program. <ctrl-D> works too.")
  quit = app.Command("quit", "exit the program.")


  // This doesn't actually do anything but set a new default cluster.
  // It doesn't have an execution portion to it, this is all handled in the Action.
  useClusterCmd = app.Command("use", "Set the cluster to use as a default.")
  useClusterCmd.Arg("cluster", "New default cluster.").Action(setCurrent).StringVar(&clusterArg)

  // Cluster Commands
  clusterCmd = app.Command("cluster", "Context for cluster commands.")
  clusterListCmd = clusterCmd.Command("list", "List short status of all the clusters.")
  clusterStatusCmd  = clusterCmd.Command("status", "Detailed status on the cluster.")
  clusterStatusCmd.Arg("cluster", "The cluster you want to describe.").Action(setCurrent).StringVar(&clusterArg)
  clusterUseCmd = clusterCmd.Command("use", "Set the default cluster for the other commands.")
  clusterUseCmd.Arg("cluster", "Set the default cluster for the other commands.").Action(setCurrent).StringVar(&clusterArg)


  // Env Commands
  envCmd = app.Command("env", "Context for environment commands")
  envListCmd = envCmd.Command("list", "Print out an environment.")
  envListCmd.Arg("server-name", "List this proxy or server environment.").Required().StringVar(&serverNameArg)
  envListCmd.Arg("cluster", "The cluster where you'll find server.").Action(setCurrent).StringVar(&clusterArg)

  // Proxy commands
  proxyCmd = app.Command("proxy", "Context for the proxy commands.")

  proxyListCmd = proxyCmd.Command("list", "List all the proxies in a cluster.")
  proxyListCmd.Arg("cluster", "The cluster where you'll find proxy tasks.").Action(setCurrent).StringVar(&clusterArg)

  proxyLaunchCmd = proxyCmd.Command("launch", "Launch a proxy into the cluster")
  proxyLaunchCmd.Arg("proxy-name", "Name for the launched proxy.").Required().StringVar(&proxyNameArg)
  proxyLaunchCmd.Arg("cluster", "ECS Cluster for the lauched proxy.").Action(setCurrent).StringVar(&clusterArg)
  proxyLaunchCmd.Arg("ecs-task","ECS Task definig containers etc, to used in launching the proxy. You can choose \"defaultCraftPort\", \"defaultRandomPort\", or any valid task-definition").Default(defaultProxyTaskDef).StringVar(&proxyTaskDefArg)
  // proxyLaunchCmd.Flag("port", "Choose either the default craft port (25565) or a random port selected at container launch.").Default(proxyUnselectedPort).EnumVar(&proxyPortArg, proxyUnselectedPort, proxyDefaultPort, proxyRandomPort)

  proxyAttachCmd = proxyCmd.Command("attach", "Attach proxy to the network by hand.")
  proxyAttachCmd.Arg("proxy-name", "Name of the proxy you want to attach to the network.").Required().StringVar(&proxyNameArg)
  proxyAttachCmd.Arg("clsuter", "The cluster where you'll find the proxy.").Action(setCurrent).StringVar(&clusterArg)

  // Server commands
  serverCmd = app.Command("server","Context for minecraft server commands.")

  serverLaunchCmd = serverCmd.Command("launch", "Launch a new minecraft server for a user in a cluster.")
  serverLaunchCmd.Arg("user", "User name of the server").Required().StringVar(&userNameArg)
  serverLaunchCmd.Arg("server-name","Name of the server. This is an identifier for the serve. (e.g. test-server, world-play).").Required().StringVar(&serverNameArg)
  serverLaunchCmd.Arg("cluster", "ECS cluster to launch the server in.").Action(setCurrent).StringVar(&clusterArg)
  serverLaunchCmd.Arg("ecs-task", "ECS Task that represents a running minecraft server.").Default(defaultServerTaskDef).StringVar(&serverTaskArg)
  // serverLaunchCmd.Arg("ecs-conatiner-name", "Container name for the minecraft server (used for environment variables.").Default("minecraft").StringVar(&serverContainerNameArg)

  serverStartCmd = serverCmd.Command("start", "Start a server from a snapshot.")
  serverStartCmd.Flag("useFullURI", "Use a full URI for the snapshot as opposed to a named snapshot.").Default("false").BoolVar(&useFullURIFlag)
  serverStartCmd.Arg("user","User name for the server.").Required().StringVar(&userNameArg)
  serverStartCmd.Arg("server-name","Name of the server. This is an identifier for the serve. (e.g. test-server, world-play).").Required().StringVar(&serverNameArg)
  serverStartCmd.Arg("snapshot", "Name of snapshot for starting server.").Required().StringVar(&snapshotNameArg)
  serverStartCmd.Arg("cluster", "ECS Cluster for the server containers.").Action(setCurrent).StringVar(&clusterArg)
  serverStartCmd.Arg("ecs-task", "ECS Task that represents a running minecraft server.").Default(defaultServerTaskDef).StringVar(&serverTaskArg)
  // serverStartCmd.Arg("ecs-conatiner-name", "Container name for the minecraft server (used for environment variables.").Default("minecraft").StringVar(&serverContainerNameArg)

  serverTerminateCmd = serverCmd.Command("terminate", "Stop this server")
  serverTerminateCmd.Arg("ecs-task-arn", "ECS Task ARN for this server.").Required().StringVar(&serverTaskArnArg)

  serverListCmd = serverCmd.Command("list", "List the servers for a cluster.")
  serverListCmd.Arg("cluster", "ECS cluster to look for servers.").Action(setCurrent).StringVar(&clusterArg)

  serverDescribeAllCmd = serverCmd.Command("describe-all", "Show details for all servers in cluster.")
  serverDescribeAllCmd.Arg("cluster", "The ECS cluster where the servers live.").Action(setCurrent).StringVar(&clusterArg)
  serverDescribeCmd = serverCmd.Command("describe", "Show some details for a users server.")
  serverDescribeCmd.Arg("user", "The user that owns the server.").Required().StringVar(&userNameArg)
  serverDescribeCmd.Arg("cluster", "The ECS cluster where the server lives.").Action(setCurrent).StringVar(&clusterArg)

  // serverAttachCmd = serverCmd.Command("attach", "Attach a server to the network (creating DNS along the way.")
  // serverAttachCmd.Arg("server-name", "The name of the server to attach to the network").Required().StringVar(&serverNameArg)
  // serverAttachCmd.Arg("cluster", "The ECS cluster where the server lives.").Action(setCurrent).StringVar(&clusterArg)

  serverProxyCmd = serverCmd.Command("proxy", "This puts a server under a proxy. Making it avaible to proxy members, and using the proxy as a DNS proxy for the server.")
  serverProxyCmd.Arg("server", "Name of server to attach to proxy.").Required().StringVar(&serverNameArg)
  serverProxyCmd.Arg("proxy", "The name of the proxy.").Required().StringVar(&proxyNameArg)
  serverProxyCmd.Arg("cluster", "The ECS cluster where the server lives.").Action(setCurrent).StringVar(&clusterArg)

  serverUnProxyCmd = serverCmd.Command("unproxy", "Take the server out of proxies control and removes the host mapping from the proxy")
  serverUnProxyCmd.Arg("server", "Name of the server to detach").Required().StringVar(&serverNameArg)
  serverUnProxyCmd.Arg("proxy", "Name of the proxy with server to remove.").Required().StringVar(&proxyNameArg)
  serverUnProxyCmd.Arg("cluster", "The ECS cluster where the server lives.").Action(setCurrent).StringVar(&clusterArg)

  // DNS 
  dnsCmd = app.Command("dns", "List Craft DNS for the network.")  

  // Snapshot commands
  archiveCmd = app.Command("archive", "Context for snapshot commands.")
  archiveListCmd = archiveCmd.Command("list", "List all snapshot for a user.")
  archiveListCmd.Arg("user", "The snapshot's user.").Required().StringVar(&userNameArg)
  archiveListCmd.Arg("bucket", "The name of the S3 bucket we're using to store snapshots in.").Default("craft-config-test").StringVar(&bucketNameArg)

  setupLogs()
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

      case envListCmd.FullCommand(): err = doListEnv(sess)

      case proxyLaunchCmd.FullCommand(): err = doLaunchProxy(sess)
      case proxyListCmd.FullCommand(): err = doListProxies(sess)
      case proxyAttachCmd.FullCommand(): err = doAttachProxy(sess)

      // Cluster Commands
      case clusterListCmd.FullCommand(): err = doListClusters(sess)
      case clusterStatusCmd.FullCommand(): err = doClusterStatus(sess)
      case clusterUseCmd.FullCommand(): err = doUseCluster()

      // Server Commands
      case serverLaunchCmd.FullCommand(): err = doLaunchServerCmd(sess)
      case serverStartCmd.FullCommand(): err = doStartServerCmd(sess)
      case serverTerminateCmd.FullCommand(): err = doTerminateServerCmd(sess)
      case serverListCmd.FullCommand(): err = doListServersCmd(sess)
      case serverDescribeAllCmd.FullCommand(): err = doDescribeAllServersCmd(sess)
      case serverDescribeCmd.FullCommand(): err = doDescribeServerCmd()
      case serverProxyCmd.FullCommand(): err = doServerProxyCmd(sess)
      case serverUnProxyCmd.FullCommand(): err = doServerUnProxyCmd(sess)
      case dnsCmd.FullCommand(): err = doListDNS(sess)
      // case serverAttachCmd.FullCommand(): err = doServerAttachCmd(sess)

      // Snapshot commands
      // case archiveListCmd.FullCommand(): err = doArchiveListCmd(sess)
    }
  }
  return err
}

// setCurrent() is called via an Action command on a flag, arg or clause.
// It's intended to catch variable setting that persists after the variable
// has been set in the command. 
// Currently we only use this for cluster setting and the change is
// expressed in the prompt. see promptLoop below.
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
  configureLogs()
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
  configureLogs()
  return nil
}

func toggleDebug() bool {
  debug = !debug
  return debug
}

func configureLogs() {
  if debug || verbose {
    log.SetLevel(logrus.DebugLevel)
    mclib.SetLogLevel(logrus.DebugLevel)
    awslib.SetLogLevel(logrus.DebugLevel)
  } else {
    log.SetLevel(logrus.InfoLevel)
    mclib.SetLogLevel(logrus.InfoLevel)
    awslib.SetLogLevel(logrus.InfoLevel)
  }
}

func setupLogs() {
  formatter := new(sl.TextFormatter)
  formatter.FullTimestamp = true
  log.SetFormatter(formatter)
  log.SetLevel(logrus.InfoLevel)
}

func doQuit(sess *session.Session) (error) {
  err := doListClusters(sess)
  if err != nil {
    fmt.Printf("%sError: %s%s", failColor, err, resetColor)
  }
  return io.EOF
}

func doTerminate(i int) {}

func promptLoop(process func(string) (error)) (err error) {
  errStr := "Error: %s\n"
  for moreCommands := true; moreCommands; {
    prompt := fmt.Sprintf("%scraft [%s%s%s]:%s ", titleEmph, infoColor, currentCluster, titleEmph, resetColor)
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




