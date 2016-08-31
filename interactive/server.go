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

  //
  // Careful now ...
  //
  // "mclib"
  "github.com/jdrivas/mclib"
  // "awslib"
  "github.com/jdrivas/awslib"
)

const (
  MinecraftServerContainerName = "minecraft"
  MinecraftControllerContainerName = "minecraft-backup"
  MinecraftDefaultServerPort = 25565
)

//
// Server commands
//


const (
  // TODO: Probably want to set this up as a command line option at some point.
  // NOTE: We're explicitly NOT using the commaind line awsRegionArg here.
  // The archive and the rest of interacting with AWS should be separate. At 
  // least for now. Though I expect that as of this moment, this tool
  // doesn't work with using a different region for the archive as 
  // where you're running. But it fairly easily could.
  DefaultArchiveRegion = "us-east-1"
  )
func doLaunchServerCmd(sess *session.Session) (error) {
  env := getTaskEnvironment(userNameArg, serverNameArg, DefaultArchiveRegion, bucketNameArg)
  if verbose {
    fmt.Printf("Making container with environment: %#v\n", env)
  }
  err := launchServer(serverTaskArg, clusterNameArg, userNameArg, env, sess)
  return err
}

func doStartServerCmd(sess *session.Session) (err error) {

  env := getTaskEnvironment(userNameArg, serverNameArg, DefaultArchiveRegion, bucketNameArg)
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
          fmt.Printf("\n%sServer is now running for user %s on cluster %s (%s).%s\n",
           highlightColor, userName, clusterName, time.Since(startTime), resetColor)
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
      fmt.Printf("%sExpected 1 task in OnStop got (%d)%s\n", highlightColor, len(tasks), resetColor)
    }
    if len(failures) > 0 {
      fmt.Printf("Received (%d) failures in stopping task.", len(failures))
    }
    if len(tasks) == 1 {
      task := tasks[0]
      fmt.Printf("%sStopped task %s at %s\n%s", highlightColor, awslib.ShortArnString(task.TaskArn), task.StoppedAt.Local(), resetColor)
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



// Environment Variabls
const (
  // TODO: This needs to move somewhere (probaby mclib).
  // But until that get's done. these are copied over into
  // craft-config. Not very safe
  ServerUserKey = "SERVER_USER"
  ServerNameKey = "SERVER_NAME"
  BackupRegionKey = "CRAFT_BACKUP_REGION"
  ArchiveRegionKey = "CRAFT_ARCHIVE_REGION"
  ArchiveBucketKey = "ARCHIVE_BUCKET"
)


func getTaskEnvironment(userName, serverName, region, bucketName string) awslib.ContainerEnvironmentMap {
  cenv := make(awslib.ContainerEnvironmentMap)
  cenv[MinecraftServerContainerName] = map[string]string {
    ServerUserKey: userName,
    ServerNameKey: serverName,
    "OPS": userName,
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
    "MOTD": fmt.Sprintf("A neighborhood kept by %s.", userName),
    "PVP": "false",
    "LEVEL": "world", // World Save name
    "ONLINE_MODE": "true",
    "JVM_OPTS": "-Xmx1024M -Xms1024M",
  }

  // Set AWS_REGION to pass the region automatically
  // to the minecraft-controller. The AWS-SDK looks for this
  // env when setting up a session (this also plays well with
  // using IAM Roles for credentials).
  cenv[MinecraftControllerContainerName] = map[string]string{
    ServerUserKey: userName,
    ServerNameKey: serverName,
    ArchiveRegionKey: region,
    ArchiveBucketKey: bucketName,
    "AWS_REGION": region,
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
  fmt.Fprintf(w, "%sUser\tServer\tUptime\tAddress\tServer\tControl\tArn%s\n", emphColor, resetColor)
  for _, dt := range dtm {
    t := dt.Task
    inst := dt.EC2Instance
    if t != nil && inst != nil {
      cntrs := t.Containers
      // name := awslib.ShortArnString(t.TaskDefinitionArn)
      uptime := 0 * time.Millisecond
      address := fmt.Sprintf("%s:%s", *inst.PublicIpAddress, getMinecraftPort(cntrs))
      if t.StartedAt != nil {uptime = time.Since(*t.StartedAt)}
      sC := getContainer(cntrs, MinecraftServerContainerName)
      sCS := fmt.Sprintf("%s", *sC.LastStatus)
      bC := getContainer(t.Containers, MinecraftControllerContainerName)
      bCS := fmt.Sprintf("%s", *bC.LastStatus)
      tArn := awslib.ShortArnString(t.TaskArn)
      cOM := makeContainerOverrideMap(t.Overrides)
      userName, ok  := cOM.getEnv(MinecraftServerContainerName, ServerUserKey)
      if !ok {
        userName = "[NONAME]"
      }
      serverName, ok := cOM.getEnv(MinecraftServerContainerName, ServerNameKey)
      if !ok {
        serverName = "[NONAME]"
      }
      color := nullColor
      if !awslib.ContainerStatusOk(sC) || !awslib.ContainerStatusOk(bC) {
        color = highlightColor
      }
      fmt.Fprintf(w,"%s%s\t%s\t%s\t%s\t%s\t%s\t%s%s\n", color, userName, serverName, shortDurationString(uptime), address, sCS, bCS, tArn, resetColor)
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
      fmt.Printf("ContainerInstance failure - Reason: %s, Resource ARN: %s\n", *dtask.CIFailure.Reason, *dtask.CIFailure.Arn)
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


// Overrides by container.
type ContainerOverrideMap map[string]*ecs.ContainerOverride

func makeContainerOverrideMap(to *ecs.TaskOverride) (ContainerOverrideMap) { 
  coMap := make(ContainerOverrideMap)
  for _, co := range to.ContainerOverrides {
    coMap[*co.Name] = co
  }
  return coMap
}

func (c ContainerOverrideMap) getEnv(containerName, key string) (s string, ok bool) {
  env := c[containerName].Environment
  for _, kvp := range env {
    if *kvp.Name == key {
      s = *kvp.Value
      ok = true
      break
    }
  }
  return s, ok
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

