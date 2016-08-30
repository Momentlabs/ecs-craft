package interactive 

import (
  // "gopkg.in/alecthomas/kingpin.v2"
  // "github.com/bobappleyard/readline"
  // "strings"
  "fmt"
  // "io"
  "os"
  "text/tabwriter"
  "time"
  // "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ecs"
  "github.com/aws/aws-sdk-go/service/ec2"
  // "github.com/mgutz/ansi"

  // Careful now ...
  "mclib"
  // "github.com/jdrivas/mclib"

  "awslib"
  // "github.com/jdrivas/awslib"
)

const (
  MinecraftServerContainerName = "minecraft"
  MinecraftControllerContainerName = "minecraft-backup"
  MinecraftDefaultServerPort = 25565
)

//
// Server commands
//


func doLaunchServerCmd(sess *session.Session) (error) {
  env := getServerEnvironment(serverContainerNameArg, userNameArg)
  if verbose {
    fmt.Printf("Making container with environment: %#v\n", env)
  }
  err := launchServer(serverTaskArg, clusterNameArg, userNameArg, env, sess)
  return err
}

func doStartServerCmd(sess *session.Session) (err error) {

  env := getServerEnvironment(serverContainerNameArg, userNameArg)
  serverEnv := env[serverContainerNameArg]
  if useFullURIFlag {
    serverEnv["WORLD"] = snapshotNameArg
  } else {
    serverEnv["WORLD"] = mclib.SnapshotURI(bucketNameArg, userNameArg, serverNameArg, snapshotNameArg)
  }
  if debug {
    fmt.Printf("Making the container with environment: %#v\n", env)
  }
  err = launchServer(serverTaskArg, clusterNameArg, userNameArg, env, sess)
  return err
}

// launches a server into the cluster for the user with the taskdefiniiton and the overide environment.
func launchServer(taskDefinition, clusterName, userName string, env awslib.ContainerEnvironmentMap, sess *session.Session) (err error) {
  ecsSvc := ecs.New(sess)
  resp, err := awslib.RunTaskWithEnv(clusterName, taskDefinition, env, ecsSvc)
  startTime := time.Now()
  tasks := resp.Tasks
  failures := resp.Failures
  if err == nil {
    fmt.Printf("Launched Server: %s\n", tasksDescriptionShortString(tasks, failures))
    if len(tasks) == 1 {
      waitForTaskArn := *tasks[0].TaskArn
      awslib.OnTaskRunning(clusterName, waitForTaskArn, ecsSvc, func(taskDescrip *ecs.DescribeTasksOutput, err error) {
        if err == nil {
          fmt.Printf("\nServer is now running for user %s on cluster %s (%s).\n", userName, clusterName, time.Since(startTime))
          fmt.Printf("%s\n", tasksDescriptionShortString(taskDescrip.Tasks, taskDescrip.Failures))
        } 
      })
    }
  } 
  return err
}

func doTerminateServerCmd(ecsSvc *ecs.ECS) (error) {
  _, err := awslib.StopTask(clusterNameArg, serverTaskArnArg, ecsSvc)
  if err != nil { return fmt.Errorf("terminate server failed: %s", err) }

  fmt.Printf("Server Task stopping: %s.\n", awslib.ShortArnString(&serverTaskArnArg))
  awslib.OnTaskStopped(clusterNameArg, serverTaskArnArg,  ecsSvc, func(stoppedTaskOutput *ecs.DescribeTasksOutput, err error){
    if stoppedTaskOutput == nil {
      fmt.Printf("Task %s stopped.\nMissing Task Object.\n", serverTaskArnArg)
      return
    }
    tasks := stoppedTaskOutput.Tasks
    failures := stoppedTaskOutput.Failures
    if len(tasks) > 1 {
      fmt.Printf("Expected 1 task in OnStop got (%d)\n", len(tasks))
    }
    if len(failures) > 0 {
      fmt.Printf("Received (%d) failures in stopping task.", len(failures))
    }
    if len(tasks) == 1 {
      task := tasks[0]
      fmt.Printf("Stopped task %s at %s\n", awslib.ShortArnString(task.TaskArn), task.StoppedAt.Local())
      if len(task.Containers) > 1 {
        fmt.Printf("There were (%d) conatiners associated with this task.\n", len(task.Containers))
      }
      for i, container := range task.Containers {
        fmt.Printf("%d. Stopped container %s, originally started: %s (%s)\n", i+1, *container.Name, task.StartedAt.Local(), time.Since(*task.StartedAt))
      }
    } else {
      for i, task := range tasks {
        fmt.Printf("%i. Stopped task %s at %s. Started at: %s (%s)\n", i+1, awslib.ShortArnString(task.TaskArn), task.StoppedAt.Local(), task.StartedAt.Local(), time.Since(*task.StartedAt))
      }
    }
    if len(failures) > 0 {
      for i, failure := range failures {
        fmt.Printf("%d. Failure on %s, Reason: %s\n", i+1, *failure.Arn, *failure.Reason)
      }
    }
  })

  return nil
}

func getServerEnvironment(containerName string, username string) awslib.ContainerEnvironmentMap {
  cenv := make(awslib.ContainerEnvironmentMap)
  cenv[containerName] = map[string]string {
    "OPS": username,
    // "WHITELIST": "",
    "MODE": "creative",
    "VIEW_DISTANCE": "10",
    "SPAWN_ANIMALS": "true",
    "SPAWN_MONSTERS": "false",
    "SPAWN_NPCS": "true",
    "FORCE_GAMEMODE": "true",
    "GENERATE_STRUCTURES": "true",
    "ALLOW_NETHER": "true",
    "MAX_PLAYERS": "20",
    "QUERY": "true",
    "QUERY_PORT": "25565",
    "ENABLE_RCON": "true",
    "RCON_PORT": "25575",
    "RCON_PASSWORD": "testing",
    "MOTD": fmt.Sprintf("A neighborhood kept by %s.", username),
    "PVP": "false",
    "LEVEL": "world", // World Save name
    "ONLINE_MODE": "true",
    "JVM_OPTS": "-Xmx1024M -Xms1024M",
  }
  return cenv
}

func tasksDescriptionShortString(tasks []*ecs.Task, failures []*ecs.Failure) (s string) {
  switch {
  case len(tasks) == 1:
    task := tasks[0]
    containers := tasks[0].Containers
    switch {
    case len(containers) == 1:
      s += containerShortString(containers[0])
    case len(containers) >= 0:
      s += fmt.Sprintf("%s\n", awslib.ShortArnString(task.TaskDefinitionArn))
      s += fmt.Sprintf("There are (%d) containers assocaited with this task.\n", len(containers))
      for i, c := range containers {
        s+= fmt.Sprintf("%d. %s\n", i+1, containerShortString(c))
      }
    }
  case len(tasks) > 0:
    s += fmt.Sprintf("There are (%d) tasks.\n", len(tasks))
    for i, task := range tasks {
      s += fmt.Sprintf("***** Task %d. %s", i+1, task)
    }
  case len(tasks) == 0:
    s += fmt.Sprintf("No tasks.")
  }
  if len(failures) > 0 {
    s += fmt.Sprintf("There are (%d) failures.\n", len(failures))
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


// ServerMap
// map[TaskID]{Task, []Container, ContainerInstance, EC2Instance}

func doListServersCmd(ecsSvc *ecs.ECS, ec2Svc *ec2.EC2) (err error) { 

  dtm, err := awslib.GetDeepTasks(clusterNameArg, ecsSvc, ec2Svc)
  if err != nil {return err}

  //name uptime ip:port arn server-name STATUS backup-name STATUS
  w := tabwriter.NewWriter(os.Stdout, 4, 8, 3, ' ', 0)
  fmt.Fprintf(w, "%sName\tUptime\tAddress\tServer\tControl\tArn%s\n", emphColor, resetColor)
  for _, dt := range dtm {
    t := dt.Task
    inst := dt.EC2Instance
    if t != nil && inst != nil {
      cntrs := t.Containers
      name := awslib.ShortArnString(t.TaskDefinitionArn)
      uptime := 0 * time.Millisecond
      address := fmt.Sprintf("%s:%s", *inst.PublicIpAddress, getMinecraftPort(cntrs))
      if t.StartedAt != nil {uptime = time.Since(*t.StartedAt)}
      sC := getContainer(cntrs, MinecraftServerContainerName)
      sCS := fmt.Sprintf("%s", *sC.LastStatus)
      bC := getContainer(t.Containers, MinecraftControllerContainerName)
      bCS := fmt.Sprintf("%s", *bC.LastStatus)
      tArn := awslib.ShortArnString(t.TaskArn)

      color := nullColor
      if !containerStatusOk(sC) || !containerStatusOk(bC) {
        color = highlightColor
      }
      fmt.Fprintf(w,"%s%s\t%s\t%s\t%s\t%s\t%s%s\n", color, name, shortDurationString(uptime), address, sCS, bCS, tArn, resetColor)
      } else {
        if t != nil {
          tArn := awslib.ShortArnString(t.TaskArn)
          fmt.Fprintf(w,"%s\tCan't find instance for task.\n", tArn)
        }
      }
  }
  w.Flush()

  return err
}

func getContainer(containers []*ecs.Container, name string) (c *ecs.Container) {

  // Find the first one matching the name
  for _, tC := range containers {
    if *tC.Name == name {
      c = tC
      break
    }
  }
  return c
}

const (
  ContainerStateRunning = "RUNNING"
  ContainerStatePending = "PENDING"
)

func containerStatusOk(c *ecs.Container) bool {
  return *c.LastStatus == "PENDING" || *c.LastStatus == "RUNNING"
}

func getMinecraftPort(containers []*ecs.Container) (s string) {
  var server *ecs.Container
  for _, container := range containers {
    if *container.Name == MinecraftServerContainerName { server = container}
  }
  
  var serverHostPort  *int64
  for _, binding := range server.NetworkBindings {
    if *binding.ContainerPort == MinecraftDefaultServerPort {
      serverHostPort = binding.HostPort
    }
  }
  if serverHostPort == nil {
    s = "PORT NOT ASSIGNED"
  } else {
    s = fmt.Sprintf("%d", *serverHostPort)
  }
  return s
}

// Name (ShortTaskDefArn), Update, State, Public IP, [PortMaps]
//      TaskArn
func shortServerString(task *ecs.Task, container *ecs.Container, instance *ec2.Instance) (s string) {
  uptime := time.Since(*task.StartedAt)
  s += fmt.Sprintf("%s (%s),", *container.Name, awslib.ShortArnString(task.TaskDefinitionArn))
  // s += fmt.Sprintf("%s,", *task.LastStatus)
  s += fmt.Sprintf(" %s, %s:%s",shortDurationString(uptime), *instance.PublicIpAddress, allBindingsString(container.NetworkBindings))
  // s += fmt.Sprintf("%s (%s), %s, started %s ago - %s %s", *container.Name, awslib.ShortArnString(task.TaskDefinitionArn), 
  //   *task.LastStatus, shortDurationString(uptime), *instance.PublicIpAddress, allBindingsString(container.NetworkBindings))
  s += fmt.Sprintf("\n\t%s", *task.TaskArn)    
  return s
}

func shortDurationString(d time.Duration) (s string) {
  days := int(d.Hours()) / 24
  hours := int(d.Hours()) % 24
  minutes := int(d.Minutes()) % 60
  if days == 0 {
    s = fmt.Sprintf("%dh %dm", hours, minutes)
  } else {
    s = fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
  }
  return s
}

func allBindingsString(bindings []*ecs.NetworkBinding) (s string) {
  s += "["
  for i, bind := range bindings {
    s += fmt.Sprintf("%d:%d", *bind.ContainerPort, *bind.HostPort)
    if i+1 < len(bindings) {s += ", "}
  }
  s += "]"
  return s
}

func doDescribeAllServersCmd(ecsSvc *ecs.ECS, ec2Svc *ec2.EC2) (error) {
  // TODO: This assumes that all tasks in a cluster a minecraft servers.
  dtm, err := awslib.GetDeepTasks(clusterNameArg, ecsSvc, ec2Svc)
  if err != nil {return err}

  taskCount := 0
  for _, dtask := range dtm {
    taskCount++
    task := dtask.Task
    ec2Inst := dtask.EC2Instance
    containers := task.Containers
    if task != nil && ec2Inst != nil {
      fmt.Printf("=========================\n")
      fmt.Printf("%s", longDeepTaskString(task, ec2Inst))
      if len(containers) > 1 {
        fmt.Printf("There were (%d) containers associated with this task.\n", len(containers))
      }
      coMap := makeContainerOverrideMap(task.Overrides)
      for i, container := range containers {
        fmt.Printf("* %d. Container Name: %s\n", i+1, *container.Name)
        fmt.Printf("Network Bindings:\n%s", networkBindingsString(container.NetworkBindings))
        fmt.Printf("%s\n", overrideString(coMap[*container.Name], 3))
      }

    }
    if dtask.Failure != nil {
      fmt.Printf("Task failure - Reason: %s, Resource ARN: %s\n", *dtask.Failure.Reason, *dtask.Failure.Arn)
    }
    if dtask.CIFailure != nil {
      fmt.Printf("ContainerInstane failure - Reason: %s, Resource ARN: %s\n", *dtask.CIFailure.Reason, *dtask.CIFailure.Arn)
    }
  }

  return nil
}

func longDeepTaskString(task *ecs.Task, ec2Inst *ec2.Instance) (s string) {
      fmt.Printf("Task Definition: %s\n", awslib.ShortArnString(task.TaskDefinitionArn))
      fmt.Printf("Instance IP: %s\n", *ec2Inst.PublicIpAddress)
      fmt.Printf("Instance ID: %s\n", *ec2Inst.InstanceId)
      fmt.Printf("Instance Type: %s\n", *ec2Inst.InstanceType)
      fmt.Printf("Location: %s\n", *ec2Inst.Placement.AvailabilityZone)
      fmt.Printf("Public DNS: %s\n", *ec2Inst.PublicDnsName)
      fmt.Printf("Started: %s (%s)\n", task.StartedAt.Local(), shortDurationString(time.Since(*task.StartedAt)))
      fmt.Printf("Status: %s\n", *task.LastStatus)
      fmt.Printf("Task: %s\n", *task.TaskArn)
      fmt.Printf("Task Definition: %s\n", *task.TaskDefinitionArn)
      return s
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
  fmt.Printf("Describe server for user \"%s\" in cluster \"%s\".\n", userNameArg, clusterNameArg)
  return nil
}

