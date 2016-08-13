package interactive 

import (
  "gopkg.in/alecthomas/kingpin.v2"
  "github.com/bobappleyard/readline"
  "strings"
  "fmt"
  "io"
  "ecs-pilot/awslib"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ecs"
  "github.com/aws/aws-sdk-go/service/ec2"
)

var (

  app *kingpin.Application

  exit *kingpin.CmdClause
  quit *kingpin.CmdClause
  verboseCmd *kingpin.CmdClause
  verbose bool
  testString []string

  serverCmd *kingpin.CmdClause
  serverLaunchCmd *kingpin.CmdClause
  serverListCmd *kingpin.CmdClause
  serverDescribeCmd *kingpin.CmdClause
  userNameArg string
  clusterNameArg string
  serverTaskArg string
  serverContainerNameArg string

)

func init() {
  app = kingpin.New("", "Interactive mode.").Terminate(doTerminate)

  // state
  verboseCmd = app.Command("verbose", "toggle verbose mode.")
  exit = app.Command("exit", "exit the program. <ctrl-D> works too.")
  quit = app.Command("quit", "exit the program.")

  serverCmd = app.Command("server","Context for minecraft server commands.")
  serverLaunchCmd = serverCmd.Command("launch", "Launch a new minecraft server for a user in a cluster.")
  serverLaunchCmd.Arg("user", "User name of the server").Required().StringVar(&userNameArg)
  serverLaunchCmd.Arg("cluster", "ECS cluster to launch the server in.").Default("minecraft").StringVar(&clusterNameArg)
  serverLaunchCmd.Arg("ecs-task", "ECS Task that represents a running minecraft server.").Default("itz-minecraft-25565").StringVar(&serverTaskArg)
  serverLaunchCmd.Arg("ecs-conatiner-name", "Container name for the minecraft server (used for environment variables.").Default("itz-minecraft_25565").StringVar(&serverContainerNameArg)
  serverListCmd = serverCmd.Command("list", "List the servers for a cluster.")
  serverListCmd.Arg("cluster", "ECS cluster to look for servers.").Default("minecraft").StringVar(&clusterNameArg)
  serverDescribeCmd = serverCmd.Command("describe", "Show some details for a users server.")
  serverDescribeCmd.Arg("user", "The user that owns the server.").Required().StringVar(&userNameArg)
  serverDescribeCmd.Arg("cluster", "The ECS cluster where the server lives.").Default("minecraft").StringVar(&clusterNameArg)
}


func DoICommand(line string, ecsSvc *ecs.ECS, ec2Svc *ec2.EC2) (err error) {

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
      case serverLaunchCmd.FullCommand(): err = doLaunchServerCmd(ecsSvc)
      case serverListCmd.FullCommand(): err = doListServersCmd(ecsSvc, ec2Svc)
      case serverDescribeCmd.FullCommand(): err = doDescribeServerCmd()
    }
  }
  return err
}

func doLaunchServerCmd(ecsSvc *ecs.ECS) (error) {
  taskDefinition := serverTaskArg
  env := getServerEnvironment(serverContainerNameArg, userNameArg)
  resp, err := awslib.RunTaskWithEnv(clusterNameArg, taskDefinition, env, ecsSvc)
  tasks := resp.Tasks
  failures := resp.Failures
  if err == nil {
    fmt.Printf("Launched %s\n", tasksDescriptionShortString(tasks, failures))
    if len(tasks) == 1 {
      waitForTaskArn := *tasks[0].TaskArn
      awslib.OnTaskRunning(clusterNameArg, waitForTaskArn, ecsSvc, func(taskDescrip *ecs.DescribeTasksOutput, err error) {
        if err == nil {
          fmt.Printf("\nServer is now running for user %s on cluster %s.\n", userNameArg, clusterNameArg)
          fmt.Printf("%s", tasksDescriptionShortString(taskDescrip.Tasks, taskDescrip.Failures))
        } else {
          fmt.Printf("Couldn't run the task: %s.\n", err)
        }
      })
    }
  } 
  return err
}

func getServerEnvironment(containerName string, username string) awslib.ContainerEnvironmentMap {
  cenv := make(awslib.ContainerEnvironmentMap)
  cenv[containerName] = map[string]string {
    "OPS": username,
    "MODE": "creative",
    "VIEW_DISTANCE": "50",
    "SPAWN_ANIMALS": "true",
    "SPAWN_MONSTERS": "false",
    "SPAWN_NPCS": "true",
    "FORCE_GAMEMODE": "true",
    "GENERATE_STRUCTURES": "true",
    "ALLOW_NETHER": "true",
    "MAX_PLAYERS": "50",
    "QUERY": "true",
    // "QUERY_PORT": "25565",
    "RCON": "true",
    "RCON_PORT": "25575",
    "MOTD": fmt.Sprintf("A neightborhood kept by %s.", username),
    "PVP": "false",
    // "LEVEL": "world", // World Save name
    "ONLINE_MODE": "true",
    "JVM_OPTS": "-Xmx1024M -Xms1024M",
  }
  return cenv
}

func tasksDescriptionShortString(tasks []*ecs.Task, failures []*ecs.Failure) (s string) {
  switch {
  case len(tasks) == 1:
    containers := tasks[0].Containers
    switch {
    case len(containers) == 1:
      s += containerShortString(containers[0])
    case len(containers) >= 0:
      s += fmt.Sprintf("There were (%d) containers for this task.", len(containers))
      for i, c := range containers {
        s+= fmt.Sprintf("\t%d. %s\n", i+1, containerShortString(c))
      }
    }
  case len(tasks) > 0:
    s += fmt.Sprintf("There were (%d) tasks.", len(tasks))
    for i, task := range tasks {
      s += fmt.Sprintf("***** Task %d. %s", i+1, task)
    }
  case len(tasks) == 0:
    s += fmt.Sprintf("No tasks.")
  }
  if len(failures) > 0 {
    s += fmt.Sprintf("There were (%d) failures.", len(failures))
    for i, failure := range failures {
      s += fmt.Sprintf("\t%d. %s.\n", i+1, failureShortString(failure))
    }
  }

  return s
}

func containerShortString(container *ecs.Container) (descrip string) {
  descrip += fmt.Sprintf("%s", *container.Name)
  bindings := container.NetworkBindings
  switch {
  case len(bindings) == 1:
    descrip += fmt.Sprintf(" - %s", bindingShortString(bindings[0]))
  case len(bindings) > 1:
    descrip += fmt.Sprintf("\nPorts:\n.")
    for i, bind := range container.NetworkBindings {
      descrip += fmt.Sprintf("\t%d. %s\n", i+1, bindingShortString(bind))
    }
  case len(bindings) == 0:
    descrip += fmt.Sprintf(" - no port bindings.")
  }
  return descrip
}

func bindingShortString(bind *ecs.NetworkBinding) (s string) {
  s += fmt.Sprintf("%s container %d => host %d (%s)",*bind.BindIP, *bind.ContainerPort, *bind.HostPort, *bind.Protocol)
  return s
}

func failureShortString(failure *ecs.Failure) (s string){
  s += fmt.Sprintf("%s - %s", *failure.Arn, *failure.Reason)
  return s
}

func doListServersCmd(ecsSvc *ecs.ECS, ec2Svc *ec2.EC2) (error) {
  // TODO: This assumes that all tasks in a cluster a minecraft servers. 
  ctMap, err := awslib.GetAllTaskDescriptions(clusterNameArg, ecsSvc)
  if err != nil {
    return fmt.Errorf("Couldn't get tasks for cluster %s: %v\n.", clusterNameArg, err)
  }

  ciMap, err := awslib.GetAllContainerInstanceDescriptions(clusterNameArg, ecsSvc)
  if err != nil {
    return fmt.Errorf("Couldn't get container instances for cluster %s: %v\n", clusterNameArg, err)
  }

  ec2Map, err := awslib.DescribeEC2Instances(ciMap, ec2Svc)
  if err != nil {
    return fmt.Errorf("Couldn't get the EC2 intances for the cluster: %s: %v\n", clusterNameArg, err)
  }

  for taskArn, ct := range ctMap {
    task := ct.Task
    failure := ct.Failure
    fmt.Printf("=========================\n")
    if task != nil {
      ctrs := task.Containers
      if len(ctrs) > 1 {
        fmt.Printf("There were (%d) containers associated with this task.\n", len(ctrs))
      }

      ciMapEntry := ciMap[*task.ContainerInstanceArn]
      coMap := makeContainerOverrideMap(task.Overrides)
      cInst := ciMapEntry.Instance
      cFail := ciMapEntry.Failure
      if cFail != nil {
        fmt.Printf("Failure for container: %s\n", *cFail.Reason)
      }
      ec2Inst := ec2Map[*cInst.Ec2InstanceId]
      for _, container := range ctrs {
        fmt.Printf("Server: %s\n", *container.Name)
        fmt.Printf("Instance IP: %s\n", *ec2Inst.PublicIpAddress)
        fmt.Printf("Instance ID: %s\n", *ec2Inst.InstanceId)
        fmt.Printf("Instance Type: %s\n", *ec2Inst.InstanceType)
        fmt.Printf("Location: %s\n", *ec2Inst.Placement.AvailabilityZone)
        fmt.Printf("Public DNS: %s\n", *ec2Inst.PublicDnsName)
        fmt.Printf("Network Bindings:\n%s", networkBindingsString(container.NetworkBindings))
        fmt.Printf("Container Override:\n%s\n", overrideString(coMap[*container.Name], 3))

        fmt.Printf("Started: %s\n", task.StartedAt.Local())
        fmt.Printf("Status: %s\n", *task.LastStatus)

        fmt.Printf("Task: %s\n", taskArn)
        fmt.Printf("Task Definition: %s\n", *task.TaskDefinitionArn)
      }
    }
    if failure != nil {
      fmt.Printf("Failure for Task: %s\n")
      fmt.Printf("Reason: %s\n", *failure.Reason)
      fmt.Printf("On resource arn: %s\n", *failure.Arn)
    }
  }

  return nil
}

func networkBindingsString(bindings []*ecs.NetworkBinding) (s string) {
  for i, b := range bindings {
    s += fmt.Sprintf("\t%d  %s\n", i+1, bindingShortString(b))
  }
  return s
}

type ContainerOverrideMap map[string]*ecs.ContainerOverride

func makeContainerOverrideMap(to *ecs.TaskOverride) (ContainerOverrideMap) { 
  coMap := make(ContainerOverrideMap)
  for _, co := range to.ContainerOverrides {
    coMap[*co.Name] = co
  }
  return coMap
}

func overrideString(co *ecs.ContainerOverride, perLine int) (s string) {
  if perLine == 0 {perLine = 1}
  command := "<EMPTY>"
  if co.Command != nil {command = commandString(co.Command)}
  s += fmt.Sprintf("Command: %s\n", command)
  s += fmt.Sprintf("Environment: ")
  for i, kvp := range co.Environment {
    s += fmt.Sprintf("%s = %s", *kvp.Name, *kvp.Value)
    if len(co.Environment) != i+1 { s += ", " }
    if (i+1)%perLine == 0 {s += "\n"}
  }

  return s
}

func commandString(c []*string) (s string) {
  for _, com := range c {
    s+= *com + " "
  }
  return s
}


func doDescribeServerCmd() (error) {
  fmt.Printf("Launch server for user \"%s\" in cluster \"%s\".\n", userNameArg, clusterNameArg)
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
  verbose = verbose
  return verbose
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
func DoInteractive(config *aws.Config) {

  // Set up AWS
  session := session.New(config)

  // Print out some account specifics.
  fmt.Printf("%s", awslib.AccountDetailsString(config))

  ecsSvc := ecs.New(session)
  ec2Svc := ec2.New(session)
  xICommand := func(line string) (err error) {return DoICommand(line, ecsSvc, ec2Svc)}
  prompt := "> "
  err := promptLoop(prompt, xICommand)
  if err != nil {fmt.Printf("Error - %s.\n", err)}
}




